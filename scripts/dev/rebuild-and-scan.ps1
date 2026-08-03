# codeRAG — build the C# analyser + coderag CLI, then run a scan
# Usage:
#   .\scripts\dev\rebuild-and-scan.ps1                     # scan testdata sample
#   .\scripts\dev\rebuild-and-scan.ps1 -Clean              # wipe graph first
#   .\scripts\dev\rebuild-and-scan.ps1 -ProjectDir C:\src\MyApp
#   .\scripts\dev\rebuild-and-scan.ps1 -SkipBuild          # reuse existing binaries

param(
    [string]$ProjectDir = "coderag\cli\analysers\csharp\testdata",
    [switch]$Clean,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Continue"

$ROOT = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSCommandPath))

if (-not $SkipBuild) {
    Write-Host "[..] Building C# analyser ..." -ForegroundColor Cyan
    & go build -C "$ROOT\coderag\cli\analysers\csharp" -o "$ROOT\coderag\cli\analysers\csharp\csharp_analyser.exe" ./cmd/csharp_analyser
    if ($LASTEXITCODE -ne 0) { Write-Error "C# analyser build failed"; exit $LASTEXITCODE }
    Write-Host "[OK] C# analyser built" -ForegroundColor Green

    Write-Host "[..] Building coderag CLI ..." -ForegroundColor Cyan
    & go build -C "$ROOT\coderag\cli" -o "$ROOT\coderag.exe" .
    if ($LASTEXITCODE -ne 0) { Write-Error "coderag build failed"; exit $LASTEXITCODE }
    Write-Host "[OK] coderag built" -ForegroundColor Green
}

if (-not [System.IO.Path]::IsPathRooted($ProjectDir)) {
    $ProjectDir = Join-Path $ROOT $ProjectDir
}
if (-not (Test-Path -LiteralPath $ProjectDir)) {
    Write-Error "Project directory not found: $ProjectDir"
    exit 1
}

$scanArgs = @("scan")
if ($Clean) { $scanArgs += "--clean" }
$scanArgs += $ProjectDir

Write-Host "[..] Scanning $ProjectDir ..." -ForegroundColor Cyan
& "$ROOT\coderag.exe" @scanArgs 2>&1 |
    Select-String -Pattern "Extracted|Entities:|Relations:|Skipping|Scan complete|Analyser produced|ERROR"
exit $LASTEXITCODE
