# codeRAG -- interactive setup for Windows
# Usage:  cd code-rag; .\scripts\setup.ps1          (interactive)
#         .\scripts\setup.ps1 -CI                   (unattended: all defaults)
# Headless aliases: -CI | -Silent | -Yes, or env CI=1 / CODERAG_NONINTERACTIVE=1.
# Designed so package managers (choco/winget) and agents can run it without input.

param(
    [switch]$CI,
    [switch]$Silent,
    [switch]$Yes
)

$ROOT = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
Set-Location $ROOT

# Unattended mode when any alias flag is passed, CI=1 is set,
# or CODERAG_NONINTERACTIVE=1 (lets agents force defaults via env alone)
if ($env:CI -eq "1" -or $env:CODERAG_NONINTERACTIVE -eq "1") { $CI = $true }
if ($Silent -or $Yes) { $CI = $true }

$OK   = "[OK]"
$INFO = "[..]"
$WARN = "[!!]"
$ERR  = "[XX]"

$MIN_GO = "1.25"

function Write-OK   { Write-Host "$OK $($args -join ' ')" -ForegroundColor Green }
function Write-Info { Write-Host "$INFO $($args -join ' ')" -ForegroundColor Cyan }
function Write-Warn { Write-Host "$WARN $($args -join ' ')" -ForegroundColor Yellow }
function Write-Err  { Write-Host "$ERR $($args -join ' ')" -ForegroundColor Red }

# Ask a question with a default answer. Returns the chosen string.
# In CI mode the default is returned without prompting.
function Ask {
    param([string]$Question, [string]$Default, [string]$Hint = "")
    if ($CI) { return $Default }
    $prompt = "$Question"
    if ($Default -ne "") { $prompt += " [$Default]" }
    if ($Hint -ne "")    { $prompt += " ($Hint)" }
    $answer = Read-Host "$prompt"
    if ($answer -eq "") { return $Default }
    return $answer
}

# Ask yes/no. Returns $true on y/yes (also in CI when DefaultYes).
function Ask-YesNo {
    param([string]$Question, [bool]$DefaultYes = $true)
    $defaultText = if ($DefaultYes) { "Y/n" } else { "y/N" }
    if ($CI) { return $DefaultYes }
    while ($true) {
        $a = (Read-Host "$Question [$defaultText]").Trim().ToLower()
        if ($a -eq "") { return $DefaultYes }
        if ($a -eq "y" -or $a -eq "yes") { return $true }
        if ($a -eq "n" -or $a -eq "no")  { return $false }
        Write-Host "  please answer y or n"
    }
}

# Merge KEY=VALUE into the repo-root .env, preserving all other lines.
function Set-EnvKey {
    param([string]$Key, [string]$Value)
    $envFile = Join-Path $ROOT ".env"
    $lines = @()
    if (Test-Path $envFile) { $lines = @(Get-Content $envFile) }
    $found = $false
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match "^\s*$([regex]::Escape($Key))\s*=") {
            $lines[$i] = "$Key=$Value"
            $found = $true
            break
        }
    }
    if (-not $found) { $lines += "$Key=$Value" }
    Set-Content -Path $envFile -Value $lines -Encoding ASCII
}

function Test-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $p = New-Object Security.Principal.WindowsPrincipal($id)
    return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Refresh-Path {
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                [Environment]::GetEnvironmentVariable("Path", "User")
}

# Installs a package via Chocolatey (best for unattended runs) or winget.
# Returns $false when neither manager exists so callers can fall back to
# direct downloads. Both paths run without user interaction:
#   choco  : -y confirms all prompts (requires an elevated shell)
#   winget : agreement flags everywhere, plus --disable-interactivity headless
function Install-Pkg {
    param([string]$Name, [string]$WingetId, [string]$ChocoId)
    Write-Info "Installing $Name ..."
    if (Get-Command choco -ErrorAction SilentlyContinue) {
        if (-not (Test-Admin)) {
            Write-Warn "choco found but shell is not elevated -- trying anyway."
        }
        & choco install $ChocoId -y --no-progress 2>&1 | Out-Null
    } elseif (Get-Command winget -ErrorAction SilentlyContinue) {
        $wgArgs = @("install", $WingetId,
                    "--accept-package-agreements", "--accept-source-agreements")
        if ($CI) { $wgArgs += "--disable-interactivity" }
        & winget @wgArgs 2>&1 | Out-Null
    } else {
        return $false
    }
    Start-Sleep -Seconds 3
    Refresh-Path
    return $true
}

# --- prerequisite installers -------------------------------------------------

# Parses the version out of 'go version' output (0.0 when unavailable).
function Get-GoVersion {
    param([string]$GoExe = "go")
    $out = & $GoExe version 2>$null
    if ($out -match 'go(\d+\.\d+(\.\d+)?)') { return [version]$Matches[1] }
    return [version]"0.0"
}

function Install-Go {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        $verStr = & go version
        if ($verStr -match 'go(\d+\.\d+)') {
            if ([version]$Matches.1 -ge [version]$MIN_GO) {
                Write-OK "Go $($Matches.1) found at $($go.Source)"
                return
            }
            Write-Warn "Go $($Matches.1) is too old (need $MIN_GO+)"
        }
    }

    if (-not (Ask-YesNo "Install Go $MIN_GO+ now?" $true)) {
        Write-Warn "Skipping Go install -- build steps will fail without it."
        return
    }

    $null = Install-Pkg -Name "Go $MIN_GO+" -WingetId "GoLang.Go" -ChocoId "golang"
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        # no package manager or it failed -> direct zip download
        $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
        $url = "https://go.dev/dl/go1.25.5.windows-$arch.zip"
        $zip = "$env:TEMP\coderag-go.zip"
        Write-Info "Downloading Go from $url ..."
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $url -OutFile $zip
        Expand-Archive -Path $zip -DestinationPath "$env:ProgramFiles\Go" -Force
        [Environment]::SetEnvironmentVariable("Path", "$env:ProgramFiles\Go\bin;" +
            [Environment]::GetEnvironmentVariable("Path", "Machine"), "Machine")
        Remove-Item $zip -Force
    }
    Refresh-Path

    # An older Go earlier on PATH can shadow the fresh install -- if the copy
    # in Program Files is newer, prefer it for this session.
    $pfGoBin = Join-Path $env:ProgramFiles "Go\bin"
    if ((Test-Path (Join-Path $pfGoBin "go.exe")) -and
        ((Get-GoVersion (Join-Path $pfGoBin "go.exe")) -gt (Get-GoVersion))) {
        Write-Info "Preferring newly installed Go at $pfGoBin for this session"
        $env:Path = "$pfGoBin;$env:Path"
    }

    # Verify honestly: presence alone is not enough, the version must be new enough
    if ((Get-GoVersion) -ge [version]$MIN_GO) {
        Write-OK "Go ready: $(& go version)"
    } elseif (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Warn "Go $MIN_GO+ recommended, but 'go' still resolves to $(Get-GoVersion)."
        Write-Info "An older Go is shadowing the new one -- fix PATH order or uninstall the old Go."
    } else {
        Write-Err "Go installation failed. Install manually from https://go.dev/dl/"
        exit 1
    }
}

function Install-Git {
    if (Get-Command git -ErrorAction SilentlyContinue) {
        Write-OK "Git found: $(& git --version)"
        return
    }
    if (-not (Ask-YesNo "Git not found. Install it now? (the file watcher uses git diff)" $true)) {
        Write-Warn "Skipping Git -- change detection will be degraded."
        return
    }
    $null = Install-Pkg -Name "Git" -WingetId "Git.Git" -ChocoId "git"
    if (Get-Command git -ErrorAction SilentlyContinue) {
        Write-OK "Git installed"
    } else {
        Write-Warn "Git install failed -- get it from https://git-scm.com/"
    }
}

# Installs the user-chosen container engine and verifies the daemon talks to us.
function Install-Engine {
    param([string]$Engine)

    if (-not (Get-Command $Engine -ErrorAction SilentlyContinue)) {
        $label = if ($Engine -eq "docker") { "Docker Desktop" } else { "Podman Desktop" }
        if (-not (Ask-YesNo "$Engine not found. Install $label now?" $true)) {
            Write-Err "codeRAG needs $Engine to run LightRAG + MCP containers."
            exit 1
        }
        # choco has no Docker Desktop package; podman-cli covers the podman CLI
        $wingetId = if ($Engine -eq "docker") { "Docker.DockerDesktop" } else { "RedHat.Podman-Desktop" }
        $chocoId  = if ($Engine -eq "docker") { "docker-desktop" }         else { "podman-cli" }
        $ranMgr = Install-Pkg -Name $label -WingetId $wingetId -ChocoId $chocoId
        if (-not $ranMgr -and -not (Get-Command $Engine -ErrorAction SilentlyContinue)) {
            # neither choco nor winget available -> direct installer download
            $url = if ($Engine -eq "docker") {
                "https://desktop.docker.com/win/stable/Docker%20Desktop%20Installer.exe"
            } else {
                "https://github.com/containers/podman-desktop/releases/latest/download/podman-desktop-installer-setup.exe"
            }
            $exe = "$env:TEMP\$(Split-Path $url -Leaf)"
            Write-Info "Downloading from $url ..."
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri $url -OutFile $exe
            Start-Process -Wait -FilePath $exe -ArgumentList "/S"
            Remove-Item $exe -Force -ErrorAction SilentlyContinue
        }
        Refresh-Path
        if (-not (Get-Command $Engine -ErrorAction SilentlyContinue)) {
            Write-Err "$Engine installation failed or needs a new shell/reboot."
            exit 1
        }
        Write-OK "$Engine installed"
    } else {
        Write-OK "$Engine found: $(& $Engine --version 2>`$null)"
    }

    # Podman on Windows runs inside a WSL2/Hyper-V "podman machine" that must exist+run.
    if ($Engine -eq "podman") {
        $null = & podman info 2>&1
        if ($LASTEXITCODE -ne 0) {
            if (Ask-YesNo "Podman machine is not running. Initialize/start it now?" $true) {
                & podman machine init 2>$null
                & podman machine start
                Start-Sleep -Seconds 3
            }
        }
    }

    # Compose must be usable through the engine CLI (`docker compose` / `podman compose`)
    $null = & $Engine compose version 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Err "'$Engine compose' is not available."
        if ($Engine -eq "podman") {
            Write-Info "Podman needs a compose provider: install Podman Desktop, or 'pip install podman-compose'."
        } else {
            Write-Info "Docker Desktop provides compose -- make sure it is installed and running."
        }
        exit 1
    }

    # Daemon reachability check
    $null = & $Engine info 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Warn "$Engine daemon not reachable."
        if ($Engine -eq "docker") {
            Write-Info "Start Docker Desktop (whale icon) and re-run this script."
        } else {
            Write-Info "Try 'podman machine start', then re-run this script."
        }
        if (-not $CI) { pause }
        exit 1
    }
    Write-OK "$Engine daemon is running"
}

function Install-Ollama {
    if (Get-Command ollama -ErrorAction SilentlyContinue) {
        Write-OK "Ollama found"
        return
    }
    if (-not (Ask-YesNo "Ollama not found. Install it now?" $true)) {
        Write-Err "codeRAG needs Ollama for embeddings. Install from https://ollama.com/ and re-run."
        exit 1
    }
    $ranMgr = Install-Pkg -Name "Ollama" -WingetId "Ollama.Ollama" -ChocoId "ollama"
    if (-not (Get-Command ollama -ErrorAction SilentlyContinue)) {
        # no package manager or it failed -> direct installer download
        $exe = "$env:TEMP\OllamaSetup.exe"
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri "https://ollama.com/download/OllamaSetup.exe" -OutFile $exe
        Start-Process -Wait -FilePath $exe -ArgumentList "/S"
        Remove-Item $exe -Force -ErrorAction SilentlyContinue
    }
    Refresh-Path
    if (Get-Command ollama -ErrorAction SilentlyContinue) {
        Write-OK "Ollama installed"
    } else {
        Write-Err "Ollama installation failed -- install manually from https://ollama.com/"
        exit 1
    }
}

# --- models ------------------------------------------------------------------

# Known embedding dimensions; anything unknown must be entered by the user.
function Get-EmbeddingDim {
    param([string]$Model)
    switch -Wildcard ($Model.ToLower()) {
        "bge-m3*"             { return "1024" }
        "mxbai-embed-large*"  { return "1024" }
        "snowflake-arctic-embed*" { return "1024" }
        "nomic-embed-text*"   { return "768" }
        "all-minilm*"         { return "384" }
        default               { return "" }
    }
}

function Pull-Models {
    param([string]$EmbedModel, [string]$EmbedDim, [string]$LlmModel)

    $list = & ollama list 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Warn "Ollama service not reachable -- start Ollama, then run:"
        Write-Host "  ollama pull $EmbedModel"
        return
    }

    foreach ($m in @($EmbedModel) + @($LlmModel | Where-Object { $_ -and $_ -ne "none" })) {
        if ($list | Select-String -SimpleMatch $m) {
            Write-OK "Model $m already pulled"
        } else {
            Write-Info "Pulling $m (this may take a while) ..."
            & ollama pull $m
            if ($LASTEXITCODE -eq 0) { Write-OK "Model $m pulled" }
            else { Write-Err "Pull of $m failed" }
        }
    }

    # Pre-warm the embedding model: its first load can take ~35 s, which would
    # trip LightRAG's 60 s worker timeout on the first scan.
    $bareName = ($EmbedModel -replace ":.*$", "")
    try {
        Write-Info "Pre-warming embedding model '$bareName' ..."
        $body = '{"model":"' + $bareName + '","input":"warmup"}'
        $null = Invoke-RestMethod -Method Post -Uri "http://localhost:11434/api/embeddings" `
            -ContentType "application/json" -Body $body -TimeoutSec 300
        Write-OK "Embedding model warmed up"
    } catch {
        Write-Warn "Pre-warm failed ($($_.Exception.Message)) -- first scan may be slower."
    }
}

# --- build steps -------------------------------------------------------------

function Build-CodeRAG {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Err "Go not found. Cannot build coderag."
        exit 1
    }
    Write-Info "Building coderag CLI ..."
    & go build -C "$ROOT\coderag\cli" -o "$ROOT\coderag\cli\coderag.exe" .
    if ($LASTEXITCODE -ne 0) { Write-Err "coderag build failed"; exit 1 }
    Write-OK "coderag built: $ROOT\coderag\cli\coderag.exe"

    # The PATH binary resolves the analyser next to itself (pipeline.findCSharpAnalyser).
    Write-Info "Building C# analyser ..."
    Push-Location "$ROOT\coderag\cli\analysers\csharp"
    & go build -o "$ROOT\csharp_analyser.exe" ./cmd/csharp_analyser 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-OK "C# analyser built: $ROOT\csharp_analyser.exe"
        Copy-Item -Force "$ROOT\csharp_analyser.exe" "$ROOT\coderag\cli\analysers\csharp\csharp_analyser.exe"
    }
    else { Write-Warn "C# analyser build skipped (only needed for indexing C# solutions)" }
    Pop-Location
}

function Build-Docker {
    param([string]$Engine)
    Write-Info "Pre-building container images with $Engine (first-time only)..."
    & $Engine compose build 2>&1 | Select-Object -Last 5
    if ($LASTEXITCODE -eq 0) { Write-OK "Container images built" }
    else { Write-Warn "Image build failed -- the first 'coderag start' will retry." }
}

function Add-ToPath {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $cliDir = "$ROOT\coderag\cli"
    $newEntry = "$cliDir;$ROOT"
    if (($userPath -split ";") -notcontains $cliDir) {
        [Environment]::SetEnvironmentVariable("Path", "$newEntry;$userPath", "User")
        $env:Path = "$newEntry;$env:Path"
        Write-Info "Added $cliDir and $ROOT to your PATH (effective in new shells)"
    } else {
        Write-OK "$cliDir already on PATH"
    }
}

# ---- main -------------------------------------------------------------------

Write-Host ""
Write-Host "=======================================" -ForegroundColor Cyan
Write-Host "  codeRAG Setup"                        -ForegroundColor Cyan
Write-Host "=======================================" -ForegroundColor Cyan
Write-Host ""
Write-Host " This wizard installs prerequisites, pulls Ollama models,"
Write-Host " builds the coderag CLI and configures the repo .env."
if (-not $CI) { Write-Host " Press ENTER to accept the [default] answers."; }
Write-Host ""

# Q1: container engine (needed before we can check/install it)
$engineInput = Ask "Which container engine do you want to use?" "docker" "docker|podman"
while ($engineInput -notin @("docker", "podman")) {
    $engineInput = Ask "Please type 'docker' or 'podman'" "docker"
}
$ENGINE = $engineInput.ToLower()
Write-OK "Container engine: $ENGINE"

Install-Git
Write-Host ""
Install-Go
Write-Host ""
Install-Engine -Engine $ENGINE
Write-Host ""
Install-Ollama
Write-Host ""

# Q2/Q3: Ollama models
$embedModel = Ask "Embedding model" "bge-m3:latest" "required by every graph operation"
$embedDim = Get-EmbeddingDim -Model $embedModel
if ($embedDim -eq "") {
    $embedDim = Ask "Embedding dimension for '$embedModel'" "1024" "must match the model!"
}
$llmAnswer = Ask "Code LLM for 'coderag query'" "none" "phi3.5:latest | qwen3-coder:480b-cloud | none | <custom>"
if (-not $llmAnswer) { $llmAnswer = "none" }
Write-Host ""

Pull-Models -EmbedModel $embedModel -EmbedDim $embedDim -LlmModel $llmAnswer
Write-Host ""

Build-CodeRAG
Write-Host ""
Build-Docker -Engine $ENGINE
Write-Host ""
Add-ToPath

# Persist the wizard's choices in the repo-root .env. The Go CLI reads
# CODERAG_ENGINE from here; docker compose picks up the model vars via .env.
Set-EnvKey "CODERAG_ENGINE" $ENGINE
Set-EnvKey "EMBEDDING_MODEL" $embedModel
Set-EnvKey "EMBEDDING_DIM" $embedDim
if ($llmAnswer -and $llmAnswer -ne "none") {
    Set-EnvKey "LLM_MODEL" $llmAnswer
}
Write-OK "Settings written to $ROOT\.env"

Write-Host ""
Write-Host "=======================================" -ForegroundColor Cyan
Write-Host "  Setup Complete!"                      -ForegroundColor Cyan
Write-Host "=======================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Engine:         $ENGINE"
Write-Host "  Embedding:      $embedModel (dim $embedDim)"
if ($llmAnswer -and $llmAnswer -ne "none") { Write-Host "  Code LLM:       $llmAnswer" }
Write-Host "  coderag CLI:    $ROOT\coderag\cli\coderag.exe"
Write-Host "  LightRAG API:   http://localhost:9621"
Write-Host "  MCP SSE:        http://localhost:9622"
Write-Host ""
Write-Host "  Quick start (in a NEW shell so PATH applies):"
Write-Host "    cd <any project folder>"
Write-Host "    coderag start .      # services + live watcher"
Write-Host "    coderag scan .       # index the .sln found here"
Write-Host "    coderag status"
Write-Host ""
