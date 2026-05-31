param(
  [Parameter(Mandatory = $false)][string]$Story,
  [string]$StoriesRoot = "stories",
  [string[]]$File = @(),
  [string]$SourceSheet = "",
  [string]$DoneSheet = "",
  [switch]$Check,
  [switch]$RewriteFromSource,
  [switch]$QuarantineInvalid,
  [string]$QuarantineDir = "",
  [string]$RepairLog = ""
)

# Repair and check completed jpstories translation sheets before import.
# Delegates to agent_sheet_tools.py so PowerShell and sh use the same logic.

$ErrorActionPreference = "Stop"

function Find-Python {
  foreach ($candidate in @("python", "python3", "py")) {
    $cmd = Get-Command $candidate -ErrorAction SilentlyContinue
    if ($cmd) { return $candidate }
  }
  throw "python, python3, or py is required"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$toolPath = Join-Path $scriptDir "agent_sheet_tools.py"
if (-not (Test-Path -LiteralPath $toolPath)) {
  throw "agent_sheet_tools.py not found next to repair_agent_sheets.ps1"
}

$argsList = @()
if ($Story) {
  $argsList += @("--story", $Story, "--stories-root", $StoriesRoot)
}
foreach ($name in $File) {
  if ($name) { $argsList += @("--file", $name) }
}
if ($SourceSheet -or $DoneSheet) {
  $argsList += @("--source-sheet", $SourceSheet, "--done-sheet", $DoneSheet)
}
if ($Check) {
  $argsList += "--check"
}
if ($RewriteFromSource) {
  $argsList += "--rewrite-from-source"
}
if ($QuarantineInvalid) {
  $argsList += "--quarantine-invalid"
}
if ($QuarantineDir) {
  $argsList += @("--quarantine-dir", $QuarantineDir)
}
if ($RepairLog) {
  $argsList += @("--repair-log", $RepairLog)
}

$python = Find-Python
& $python $toolPath @argsList
$exitCode = $LASTEXITCODE

if ($QuarantineInvalid -and -not $Check -and $Story -and $File.Count -gt 0 -and $exitCode -ne 0) {
  $storyDir = Join-Path $StoriesRoot $Story
  $doneDir = Join-Path $storyDir "agent-done"
  $quarantineDir = if ($QuarantineDir) { $QuarantineDir } else { Join-Path $storyDir "agent-done-quarantine" }
  $logPath = if ($RepairLog) { $RepairLog } else { Join-Path $storyDir "agent-repair-log.jsonl" }
  $recentLog = @()
  if (Test-Path -LiteralPath $logPath) {
    $recentLog = Get-Content -LiteralPath $logPath -Tail 100
  }
  New-Item -ItemType Directory -Path $quarantineDir -Force | Out-Null
  $timestamp = [DateTime]::UtcNow.ToString("yyyyMMddTHHmmssZ")
  foreach ($name in $File) {
    if (-not $name) { continue }
    $leaf = Split-Path -Leaf $name
    $invalidInLog = $false
    foreach ($line in $recentLog) {
      if ($line -like "*`"file`": `"$leaf`"*" -and $line -like "*`"status`": `"invalid`"*") {
        $invalidInLog = $true
        break
      }
    }
    if (-not $invalidInLog) { continue }
    $source = Join-Path $doneDir $leaf
    if (-not (Test-Path -LiteralPath $source)) { continue }
    $target = Join-Path $quarantineDir "$timestamp`_$leaf"
    $counter = 2
    while (Test-Path -LiteralPath $target) {
      $target = Join-Path $quarantineDir "$timestamp`_$counter`_$leaf"
      $counter++
    }
    Move-Item -LiteralPath $source -Destination $target
    Write-Host "quarantined: ${leaf}: $target"
  }
}

exit $exitCode
