# code-lightrag dev setup
# Run from repo root: .\scripts\setup.ps1

$VENV = "venv"
$ROOT = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

if (-not (Test-Path "$ROOT\$VENV\Scripts\python.exe")) {
    python -m venv "$ROOT\$VENV"
}

& "$ROOT\$VENV\Scripts\pip.exe" install -e "$ROOT\LightRAG\[api]"
& "$ROOT\$VENV\Scripts\pip.exe" install -e "$ROOT\daniel-lightrag-mcp\[dev]"
& "$ROOT\$VENV\Scripts\pip.exe" install -e "$ROOT\watchdog"

Write-Host "Done. Use .\$VENV\Scripts\activate to activate."
