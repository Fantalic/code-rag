#!/usr/bin/env bash
set -euo pipefail

# codeRAG — one-shot setup for Linux/macOS
# Usage: cd code-rag && bash scripts/setup.sh

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

INFO='\033[0;34m'
OK='\033[0;32m'
WARN='\033[0;33m'
ERR='\033[0;31m'
NC='\033[0m'

info()  { printf "${INFO}[INFO]${NC} %s\n" "$*"; }
ok()    { printf "${OK}[OK]${NC}   %s\n" "$*"; }
warn()  { printf "${WARN}[WARN]${NC} %s\n" "$*"; }
err()   { printf "${ERR}[ERR]${NC}  %s\n" "$*"; }

MIN_GO="1.25"
DOCKER=${DOCKER:-docker}
OLLAMA=${OLLAMA:-ollama}

# ---------------------------------------------------------------------------
# Prerequisite checks / installers
# ---------------------------------------------------------------------------

check_go() {
    if command -v go &>/dev/null; then
        ver="$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')"
        if awk "BEGIN{exit(!($ver >= $MIN_GO))}"; then
            ok "Go $ver found"
            return 0
        fi
        warn "Go $ver is too old (need $MIN_GO+) — will upgrade"
    fi

    info "Installing Go $MIN_GO+ ..."
    if command -v apt-get &>/dev/null; then
        sudo apt-get update -qq && sudo apt-get install -y -qq golang-go 2>/dev/null ||
        sudo apt-get install -y -qq golang 2>/dev/null
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y golang
    elif command -v brew &>/dev/null; then
        brew install go
    else
        warn "Package manager not auto-detectable — installing Go manually"
        ARCH="$(uname -m)"; case "$ARCH" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; esac
        curl -fsSL "https://go.dev/dl/go1.25.5.linux-${ARCH}.tar.gz" | sudo tar -C /usr/local -xz
        export PATH="/usr/local/go/bin:$PATH"
    fi

    if command -v go &>/dev/null; then
        ok "Go installed: $(go version)"
    else
        err "Go installation failed — install manually from https://go.dev/dl/"
        exit 1
    fi
}

check_docker() {
    if command -v "$DOCKER" &>/dev/null; then
        ok "Docker found: $($DOCKER --version 2>/dev/null || $DOCKER version 2>/dev/null | head -1)"
        # Test we can talk to the daemon
        if $DOCKER info &>/dev/null; then
            return 0
        fi
        warn "Docker daemon not reachable — trying to start it"
        sudo systemctl start docker 2>/dev/null || true
        sleep 2
        if $DOCKER info &>/dev/null; then
            ok "Docker daemon is running"
            return 0
        fi
        err "Docker daemon not reachable — start Docker and re-run"
        exit 1
    fi

    info "Installing Docker ..."
    if command -v apt-get &>/dev/null; then
        curl -fsSL https://get.docker.com | sudo sh
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y docker-ce docker-ce-cli containerd.io 2>/dev/null ||
        curl -fsSL https://get.docker.com | sudo sh
    elif command -v brew &>/dev/null; then
        brew install --cask docker
    else
        err "No supported package manager — install Docker manually: https://docs.docker.com/engine/install/"
        exit 1
    fi

    sudo systemctl enable docker 2>/dev/null || true
    sudo systemctl start docker 2>/dev/null || true
    # Add user to docker group so sudo isn't needed
    sudo usermod -aG docker "$USER" 2>/dev/null || true

    if $DOCKER info &>/dev/null; then
        ok "Docker installed and running"
    else
        warn "Docker installed but may need a reboot or relogin to use without sudo"
        warn "For now, trying with sudo ..."
        DOCKER="sudo docker"
    fi
}

check_ollama() {
    if command -v "$OLLAMA" &>/dev/null; then
        ok "Ollama found: $($OLLAMA --version 2>/dev/null || echo 'installed')"
        return 0
    fi

    info "Installing Ollama ..."
    if [[ "$(uname)" == "Darwin" ]]; then
        if command -v brew &>/dev/null; then
            brew install --cask ollama
        else
            curl -fsSL https://ollama.com/install.sh | sh
        fi
    else
        curl -fsSL https://ollama.com/install.sh | sh
    fi

    if command -v "$OLLAMA" &>/dev/null; then
        ok "Ollama installed"
    else
        warn "Ollama may need a new shell to appear in PATH"
    fi
}

pull_models() {
    info "Pulling Ollama models (this may take a while)..."
    CMD="${OLLAMA:-ollama}"

    if ! $CMD --version &>/dev/null; then
        warn "Ollama not available yet — skip model pull; run manually:"
        echo "  ollama pull qwen3-coder:480b-cloud"
        echo "  ollama pull bge-m3:latest"
        return
    fi

    # Check if Ollama service is running
    if ! $CMD list &>/dev/null; then
        warn "Ollama service not running — skip model pull; start Ollama then run:"
        echo "  ollama pull qwen3-coder:480b-cloud"
        echo "  ollama pull bge-m3:latest"
        return
    fi

    for model in "qwen3-coder:480b-cloud" "bge-m3:latest"; do
        if $CMD list 2>/dev/null | grep -q "$model"; then
            ok "Model $model already pulled"
        else
            info "Pulling $model ..."
            $CMD pull "$model"
            ok "Model $model pulled"
        fi
    done
}

build_coderag() {
    info "Building coderag CLI ..."
    if ! command -v go &>/dev/null; then
        err "Go not found — cannot build coderag"
        exit 1
    fi
    go build -o "$ROOT/coderag" ./cmd/coderag/
    ok "coderag built: $ROOT/coderag"

    # Also build C# analyser
    info "Building C# analyser ..."
    cd "$ROOT/analysers/csharp"
    if [ -f go.mod ]; then
        go build -o "$ROOT/analysers/csharp/csharp_analyser" ./cmd/csharp_analyser/ 2>/dev/null ||
        go build -o "$ROOT/analysers/csharp/csharp_analyser" . 2>/dev/null ||
        warn "C# analyser build skipped (no main package found)"
    fi
    cd "$ROOT"
    ok "C# analyser built"
}

build_docker() {
    info "Pre-building Docker images (first-time setup)..."
    $DOCKER compose build 2>&1 | tail -5
    ok "Docker images built"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

echo ""
echo "==============================="
echo "  codeRAG — Setup Script"
echo "==============================="
echo ""

check_go
echo ""
check_docker
echo ""
check_ollama
echo ""
pull_models
echo ""
build_coderag
echo ""
build_docker
echo ""

# Add to PATH suggestion
BASHRC="$HOME/.bashrc"
if [[ "$SHELL" == *zsh* ]]; then BASHRC="$HOME/.zshrc"; fi
if ! grep -q "$ROOT" "$BASHRC" 2>/dev/null; then
    echo >> "$BASHRC"
    echo "# codeRAG" >> "$BASHRC"
    echo "export PATH=\"\$PATH:$ROOT\"" >> "$BASHRC"
    info "Added $ROOT to PATH in $BASHRC (restart shell or 'source $BASHRC' to use)"
fi

echo ""
echo "==============================="
echo "  Setup Complete!"
echo "==============================="
echo ""
echo "  coderag CLI:    $ROOT/coderag"
echo "  C# analyser:    $ROOT/analysers/csharp/csharp_analyser"
echo "  LightRAG API:   http://localhost:9621"
echo "  MCP SSE:        http://localhost:9622"
echo ""
echo "  Quick start:"
echo "    cd $ROOT"
echo '    ./coderag start ./myproject    # Docker + file watcher'
echo '    ./coderag scan ./myproject     # Index a .NET project'
echo '    ./coderag mcp .                # MCP bridge for opencode'
echo ""
echo "  Start using it in any other shell:"
echo "    export PATH=\"\$PATH:$ROOT\""
echo ""
