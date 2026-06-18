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

# Build webui frontend (if Bun is available)
if (Get-Command bun -ErrorAction SilentlyContinue) {
    Write-Host "Building webui frontend..."
    Push-Location "$ROOT\LightRAG\lightrag_webui"
    & bun install --frozen-lockfile 2>&1 | Out-Null
    & bun run build 2>&1
    Pop-Location
    Write-Host "WebUI built successfully."
} else {
    Write-Host "WARNING: Bun not found. Skipping webui build."
    Write-Host "  Install Bun from https://bun.sh or build manually:"
    Write-Host "    cd LightRAG\lightrag_webui"
    Write-Host "    bun install && bun run build"
}

Write-Host "Done. Use .\$VENV\Scripts\activate to activate."
