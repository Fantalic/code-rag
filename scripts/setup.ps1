# codeRAG — one-shot setup for Windows
# Usage: cd code-rag; .\scripts\setup.ps1

$ROOT = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
Set-Location $ROOT

$OK  = "[OK]"
$INFO = "[..]"
$WARN = "[!!]"
$ERR = "[XX]"

$MIN_GO = "1.25"

function Write-OK   { Write-Host "$OK $($args -join ' ')" -ForegroundColor Green }
function Write-Info { Write-Host "$INFO $($args -join ' ')" -ForegroundColor Cyan }
function Write-Warn { Write-Host "$WARN $($args -join ' ')" -ForegroundColor Yellow }
function Write-Err  { Write-Host "$ERR $($args -join ' ')" -ForegroundColor Red }

function Test-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $p = New-Object Security.Principal.WindowsPrincipal($id)
    return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Install-Go {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        $verStr = & go version
        if ($verStr -match 'go(\d+\.\d+)') {
            $ver = $Matches.1
            if ([version]$ver -ge [version]$MIN_GO) {
                Write-OK "Go $ver found at $($go.Source)"
                return
            }
            Write-Warn "Go $ver is too old (need $MIN_GO+)"
        }
    }

    Write-Info "Installing Go $MIN_GO+ ..."
    $admin = Test-Admin

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        if ($admin) {
            & winget install "GoLang.Go" --silent --accept-package-agreements 2>&1 | Out-Null
        } else {
            & winget install "GoLang.Go" 2>&1 | Out-Null
        }
        Start-Sleep -Seconds 3
        $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                    [Environment]::GetEnvironmentVariable("Path", "User")
    }
    elseif (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
        $url = "https://go.dev/dl/go1.25.5.windows-$arch.zip"
        $zip = "$env:TEMP\go.zip"
        Write-Info "Downloading Go from $url ..."
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $url -OutFile $zip
        Expand-Archive -Path $zip -DestinationPath "$env:ProgramFiles\Go" -Force
        $newPath = "$env:ProgramFiles\Go\bin;$env:Path"
        $env:Path = $newPath
        [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
        Remove-Item $zip -Force
    }

    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-OK "Go installed: $(& go version)"
    } else {
        Write-Err "Go installation failed. Install manually from https://go.dev/dl/"
        exit 1
    }
}

function Install-Docker {
    if (Get-Command docker -ErrorAction SilentlyContinue) {
        Write-OK "Docker found: $(& docker --version 2>`$null)"
        $null = & docker info 2>&1
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Write-Warn "Docker daemon not reachable. Is Docker Desktop running?"
        Write-Info "Start Docker Desktop manually, then re-run."
        return
    }

    Write-Info "Installing Docker Desktop ..."

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        $admin = Test-Admin
        if ($admin) {
            & winget install "Docker.DockerDesktop" --silent --accept-package-agreements 2>&1 | Out-Null
        } else {
            & winget install "Docker.DockerDesktop" 2>&1 | Out-Null
        }
        Start-Sleep -Seconds 5
    }
    elseif (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        $url = "https://desktop.docker.com/win/stable/Docker%20Desktop%20Installer.exe"
        $exe = "$env:TEMP\DockerDesktopInstaller.exe"
        Write-Info "Downloading Docker Desktop from $url ..."
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $url -OutFile $exe
        Write-Info "Running Docker Desktop installer ..."
        Start-Process -Wait -FilePath $exe -ArgumentList "install", "--quiet", "--accept-license"
        Remove-Item $exe -Force
    }

    if (Get-Command docker -ErrorAction SilentlyContinue) {
        Write-OK "Docker Desktop installed"
        Write-Info "Start Docker Desktop manually, then re-run (or continue)."
    } else {
        Write-Warn "Docker Desktop may need a reboot to complete installation."
    }
}

function Install-Ollama {
    if (Get-Command ollama -ErrorAction SilentlyContinue) {
        Write-OK "Ollama found"
        return
    }

    Write-Info "Installing Ollama ..."

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        & winget install "Ollama.Ollama" --silent --accept-package-agreements 2>&1 | Out-Null
        Start-Sleep -Seconds 3
    } else {
        $url = "https://ollama.com/download/OllamaSetup.exe"
        $exe = "$env:TEMP\OllamaSetup.exe"
        Write-Info "Downloading Ollama from $url ..."
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $url -OutFile $exe
        Write-Info "Running Ollama installer ..."
        Start-Process -Wait -FilePath $exe -ArgumentList "/S"
        Remove-Item $exe -Force
    }

    $m = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $u = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($m -or $u) {
        $env:Path = "$m;$u"
    }

    if (Get-Command ollama -ErrorAction SilentlyContinue) {
        Write-OK "Ollama installed"
    } else {
        Write-Warn "Ollama may need a new shell. Run 'ollama pull' later."
    }
}

function Pull-Models {
    Write-Info "Checking / pulling Ollama models (this may take a while)..."
    if (-not (Get-Command ollama -ErrorAction SilentlyContinue)) {
        Write-Warn "Ollama not in PATH. Run later:"
        Write-Host "  ollama pull qwen3-coder:480b-cloud"
        Write-Host "  ollama pull bge-m3:latest"
        return
    }

    $null = & ollama list 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Warn "Ollama service not running. Start Ollama then run:"
        Write-Host "  ollama pull qwen3-coder:480b-cloud"
        Write-Host "  ollama pull bge-m3:latest"
        return
    }

    foreach ($model in @("qwen3-coder:480b-cloud", "bge-m3:latest")) {
        $list = & ollama list 2>$null
        $found = $list | Select-String -Pattern $model
        if ($found) {
            Write-OK "Model $model already pulled"
        } else {
            Write-Info "Pulling $model ..."
            & ollama pull $model 2>&1 | Out-Null
            Write-OK "Model $model pulled"
        }
    }
}

function Build-CodeRAG {
    Write-Info "Building coderag CLI ..."
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Err "Go not found. Cannot build coderag."
        exit 1
    }
    & go build -o "$ROOT\coderag.exe" .\cmd\coderag\
    if ($LASTEXITCODE -eq 0) {
        Write-OK "coderag built: $ROOT\coderag.exe"
    } else {
        Write-Err "coderag build failed"
        exit 1
    }

    Write-Info "Building C# analyser ..."
    Push-Location "$ROOT\analysers\csharp"
    if (Test-Path go.mod) {
        & go build -o "$ROOT\analysers\csharp\csharp_analyser.exe" . 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-OK "C# analyser built"
        } else {
            Write-Warn "C# analyser build skipped (no main package found)"
        }
    }
    Pop-Location
}

function Build-Docker {
    Write-Info "Pre-building Docker images (first-time setup)..."
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        Write-Warn "Docker not available. Skip Docker build."
        Write-Warn "First 'coderag start' will build the images."
        return
    }
    $null = & docker info 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Warn "Docker daemon not reachable. Skip Docker build."
        return
    }
    & docker compose build 2>&1 | Select-Object -Last 5
    Write-OK "Docker images built"
}

# ---- main ----

Write-Host @"
===============================
  codeRAG - Setup Script
===============================
"@ -ForegroundColor Cyan
Write-Host ""

Install-Go
Write-Host ""
Install-Docker
Write-Host ""
Install-Ollama
Write-Host ""
Pull-Models
Write-Host ""
Build-CodeRAG
Write-Host ""
Build-Docker
Write-Host ""

$cp = [Environment]::GetEnvironmentVariable("Path", "User")
if ($cp -notlike "*$ROOT*") {
    $np = "$ROOT;$cp"
    [Environment]::SetEnvironmentVariable("Path", $np, "User")
    Write-Info "Added $ROOT to user PATH (effective in new shells)"
}

Write-Host @"
===============================
  Setup Complete!
===============================

  coderag CLI:    $ROOT\coderag.exe
  C# analyser:    $ROOT\analysers\csharp\csharp_analyser.exe
  LightRAG API:   http://localhost:9621
  MCP SSE:        http://localhost:9622

  Quick start (in a new PowerShell):
    cd $ROOT
    .\coderag start .\myproject
    .\coderag scan .\myproject
    .\coderag mcp .

  Use in this shell:
    `$env:Path = "$ROOT;`$env:Path"
"@ -ForegroundColor Cyan
