param(
  [string]$Story,

  [string]$StoriesRoot = "stories",

  [string]$InputPath,

  [string]$OutputPath,

  [switch]$FixBom
)

$ErrorActionPreference = "Stop"

$supportedLevels = @("native", "n3", "n3_abridged")

function Remove-Utf8Bom {
  param([Parameter(Mandatory = $true)][string]$Path)

  $bytes = [System.IO.File]::ReadAllBytes($Path)
  if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
    $withoutBom = New-Object byte[] ($bytes.Length - 3)
    [Array]::Copy($bytes, 3, $withoutBom, 0, $withoutBom.Length)
    [System.IO.File]::WriteAllBytes($Path, $withoutBom)
    Write-Host "removed UTF-8 BOM: $Path"
  }
}

function Read-JsonFile {
  param([Parameter(Mandatory = $true)][string]$Path)

  try {
    return Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json
  } catch {
    throw "decode JSON ${Path}: $($_.Exception.Message)"
  }
}

function Json-Compact {
  param($Value)
  return ($Value | ConvertTo-Json -Depth 100 -Compress)
}

function Property-Names {
  param($Value)
  return @($Value.PSObject.Properties | ForEach-Object { $_.Name })
}

function Assert-Equal {
  param(
    [string]$Name,
    $Got,
    $Want
  )
  if ($Got -ne $Want) {
    throw "${Name} mismatch"
  }
}

function Translation-Keys {
  param($Sentence)
  return @(Property-Names $Sentence | Where-Object { $supportedLevels -contains $_ } | Sort-Object)
}

function Assert-Sentence-Shape {
  param(
    [string]$Path,
    $InputSentence,
    $OutputSentence,
    [hashtable]$LevelSet
  )

  $inputTop = @(Property-Names $InputSentence | Sort-Object)
  $outputTop = @(Property-Names $OutputSentence | Sort-Object)
  if ((Json-Compact $outputTop) -ne (Json-Compact $inputTop)) {
    throw "${Path} fields differ"
  }

  Assert-Equal -Name "${Path}.id" -Got $OutputSentence.id -Want $InputSentence.id
  Assert-Equal -Name "${Path}.english" -Got $OutputSentence.english -Want $InputSentence.english

  $inputKeys = @(Translation-Keys $InputSentence)
  $outputKeys = @(Translation-Keys $OutputSentence)
  if ((Json-Compact $outputKeys) -ne (Json-Compact $inputKeys)) {
    throw "${Path} translation level fields differ"
  }

  foreach ($key in $outputKeys) {
    if (-not $LevelSet.ContainsKey($key)) {
      throw "${Path} includes level not listed in levels: $key"
    }
    $value = $OutputSentence.$key
    if ($null -eq $value -or "$value".Trim() -eq "") {
      throw "${Path}.${key} is empty"
    }
  }
}

function Test-WorkItem {
  param(
    [Parameter(Mandatory = $true)][string]$SourcePath,
    [Parameter(Mandatory = $true)][string]$DonePath
  )

  if ($FixBom) {
    Remove-Utf8Bom -Path $SourcePath
    Remove-Utf8Bom -Path $DonePath
  }

  $inputJson = Read-JsonFile -Path $SourcePath
  $outputJson = Read-JsonFile -Path $DonePath

  $inputTop = @(Property-Names $inputJson | Sort-Object)
  $outputTop = @(Property-Names $outputJson | Sort-Object)
  if ((Json-Compact $outputTop) -ne (Json-Compact $inputTop)) {
    throw "top-level fields differ"
  }

  foreach ($field in @("story_id", "story_title", "chunk_id")) {
    Assert-Equal -Name $field -Got $outputJson.$field -Want $inputJson.$field
  }

  if ((Json-Compact $outputJson.levels) -ne (Json-Compact $inputJson.levels)) {
    throw "levels changed"
  }

  $levelSet = @{}
  foreach ($level in @($inputJson.levels)) {
    if ($supportedLevels -notcontains $level) {
      throw "unsupported level: $level"
    }
    if ($levelSet.ContainsKey($level)) {
      throw "duplicate level: $level"
    }
    $levelSet[$level] = $true
  }

  if (@($inputJson.paragraphs).Count -ne @($outputJson.paragraphs).Count) {
    throw "paragraph count changed"
  }

  for ($i = 0; $i -lt @($inputJson.paragraphs).Count; $i++) {
    $inputParagraph = @($inputJson.paragraphs)[$i]
    $outputParagraph = @($outputJson.paragraphs)[$i]
    Assert-Equal -Name "paragraphs[$i].id" -Got $outputParagraph.id -Want $inputParagraph.id

    if (@($inputParagraph.sentences).Count -ne @($outputParagraph.sentences).Count) {
      throw "paragraphs[$i].sentence count changed"
    }

    for ($j = 0; $j -lt @($inputParagraph.sentences).Count; $j++) {
      Assert-Sentence-Shape `
        -Path "paragraphs[$i].sentences[$j]" `
        -InputSentence @($inputParagraph.sentences)[$j] `
        -OutputSentence @($outputParagraph.sentences)[$j] `
        -LevelSet $levelSet
    }
  }
}

$singleMode = [string]::IsNullOrWhiteSpace($Story)
if ($singleMode) {
  if ([string]::IsNullOrWhiteSpace($InputPath) -or [string]::IsNullOrWhiteSpace($OutputPath)) {
    throw "provide either -Story <story> or both -InputPath <source-file> and -OutputPath <done-file>"
  }
  Test-WorkItem -SourcePath $InputPath -DonePath $OutputPath
  Write-Host "valid: $OutputPath"
  exit 0
}

if (-not [string]::IsNullOrWhiteSpace($InputPath) -or -not [string]::IsNullOrWhiteSpace($OutputPath)) {
  throw "use -Story for batch validation or -InputPath/-OutputPath for one file pair, not both"
}

$storyDir = Join-Path $StoriesRoot $Story
$chunkDir = Join-Path $storyDir "chunk"
$doneDir = Join-Path $storyDir "done"

if (-not (Test-Path -LiteralPath $chunkDir -PathType Container)) {
  throw "chunk directory not found: $chunkDir"
}

$sourceFiles = @(Get-ChildItem -LiteralPath $chunkDir -Filter "*.json" -File | Sort-Object Name)
$doneFiles = @()
if (Test-Path -LiteralPath $doneDir -PathType Container) {
  $doneFiles = @(Get-ChildItem -LiteralPath $doneDir -Filter "*.json" -File | Sort-Object Name)
}

$sourceByName = @{}
foreach ($file in $sourceFiles) {
  $sourceByName[$file.Name] = $file.FullName
}

$doneByName = @{}
foreach ($file in $doneFiles) {
  $doneByName[$file.Name] = $file.FullName
}

$valid = New-Object System.Collections.Generic.List[string]
$missing = New-Object System.Collections.Generic.List[string]
$invalid = New-Object System.Collections.Generic.List[string]
$extra = New-Object System.Collections.Generic.List[string]

foreach ($source in $sourceFiles) {
  if (-not $doneByName.ContainsKey($source.Name)) {
    $missing.Add($source.Name)
    continue
  }

  try {
    Test-WorkItem -SourcePath $source.FullName -DonePath $doneByName[$source.Name]
    $valid.Add($source.Name)
  } catch {
    $invalid.Add("$($source.Name): $($_.Exception.Message)")
  }
}

foreach ($done in $doneFiles) {
  if (-not $sourceByName.ContainsKey($done.Name)) {
    $extra.Add($done.Name)
  }
}

Write-Host "Story: $Story"
Write-Host "Source work items: $($sourceFiles.Count)"
Write-Host "Completed work items: $($doneFiles.Count)"
Write-Host "Valid: $($valid.Count)"
foreach ($name in $valid) {
  Write-Host "  valid: $name"
}
Write-Host "Missing: $($missing.Count)"
foreach ($name in $missing) {
  Write-Host "  missing: $name"
}
Write-Host "Invalid: $($invalid.Count)"
foreach ($item in $invalid) {
  Write-Host "  invalid: $item"
}
Write-Host "Extra: $($extra.Count)"
foreach ($name in $extra) {
  Write-Host "  extra: $name"
}

if ($missing.Count -gt 0 -or $invalid.Count -gt 0 -or $extra.Count -gt 0) {
  exit 1
}

exit 0
