param(
  [Parameter(Mandatory = $true)][string]$Story,
  [string]$StoriesRoot = "stories"
)

$ErrorActionPreference = "Stop"

$lq  = [char]0x201C   # " LEFT DOUBLE QUOTATION MARK
$rq  = [char]0x201D   # " RIGHT DOUBLE QUOTATION MARK
$lsq = [char]0x2018   # ' LEFT SINGLE QUOTATION MARK
$rsq = [char]0x2019   # ' RIGHT SINGLE QUOTATION MARK

function Remove-Utf8Bom {
  param([string]$Path)
  $bytes = [System.IO.File]::ReadAllBytes($Path)
  if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
    $withoutBom = New-Object byte[] ($bytes.Length - 3)
    [Array]::Copy($bytes, 3, $withoutBom, 0, $withoutBom.Length)
    [System.IO.File]::WriteAllBytes($Path, $withoutBom)
    Write-Host "removed UTF-8 BOM: $Path"
  }
}

$chunkDir = Join-Path (Join-Path $StoriesRoot $Story) "chunk"
$doneDir  = Join-Path (Join-Path $StoriesRoot $Story) "done"

if (-not (Test-Path -LiteralPath $chunkDir -PathType Container)) {
  throw "chunk directory not found: $chunkDir"
}

$sourceFiles = @(Get-ChildItem -LiteralPath $chunkDir -Filter "*.json" -File | Sort-Object Name)
$fixed = 0; $ok = 0; $skipped = 0

foreach ($srcFile in $sourceFiles) {
  $donePath = Join-Path $doneDir $srcFile.Name
  if (-not (Test-Path -LiteralPath $donePath -PathType Leaf)) {
    $skipped++
    continue
  }

  Remove-Utf8Bom -Path $donePath

  $src      = Get-Content -LiteralPath $srcFile.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
  $doneText = [System.IO.File]::ReadAllText($donePath, [System.Text.Encoding]::UTF8)

  $changed = $false

  foreach ($para in $src.paragraphs) {
    foreach ($sent in $para.sentences) {
      $eng = $sent.english

      # Build the ASCII-substituted (corrupted) version of the English text.
      $corrupted = $eng.Replace($lq, '"').Replace($rq, '"').Replace($lsq, "'").Replace($rsq, "'")
      if ($corrupted -eq $eng) { continue }  # No smart quotes; nothing to fix.

      # Include the surrounding JSON field delimiters and trailing comma in the
      # search and replacement strings. This prevents accidental matches against
      # JSON structural characters when the English text is very short (e.g. just
      # a single curly quote character).
      $search  = '"english": "' + $corrupted + '",'
      $replace = '"english": "' + $eng       + '",'

      if ($doneText.Contains($search)) {
        $doneText = $doneText.Replace($search, $replace)
        $changed = $true
      }
    }
  }

  if ($changed) {
    [System.IO.File]::WriteAllText($donePath, $doneText, [System.Text.Encoding]::UTF8)
    Write-Host "fixed: $($srcFile.Name)"
    $fixed++
  } else {
    Write-Host "ok: $($srcFile.Name)"
    $ok++
  }
}

Write-Host "Total: $fixed fixed, $ok ok, $skipped missing done file"
if ($fixed -gt 0) { exit 0 }
exit 0
