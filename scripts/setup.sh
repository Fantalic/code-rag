#!/usr/bin/env bash
# codeRAG -- interactive setup for Linux/macOS
# Usage: cd code-rag && bash scripts/setup.sh
#   CI=1 (or non-TTY) takes defaults for every question without prompting.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

INFO='\033[0;34m'; OK='\033[0;32m'; WARN='\033[0;33m'; ERR='\033[0;31m'; NC='\033[0m'
info() { printf "${INFO}[INFO]${NC} %s\n" "$*"; }
ok()   { printf "${OK}[OK]${NC}   %s\n" "$*"; }
warn() { printf "${WARN}[WARN]${NC} %s\n" "$*"; }
err()  { printf "${ERR}[ERR]${NC}  %s\n" "$*"; }

MIN_GO="1.25"

# Unattended mode: -y/--yes/-s/--silent/--ci flag, CI=1 env var,
# CODERAG_NONINTERACTIVE=1, or stdin not connected to a terminal.
# Lets package managers (choco, brew) and agents run this without input.
CI="${CI:-}"
for arg in "$@"; do
    case "$arg" in
        -y|--yes|-s|--silent|--ci) CI=1 ;;
    esac
done
if [ -z "$CI" ] && [ "${CODERAG_NONINTERACTIVE:-}" = "1" ]; then CI=1; fi
if [ -z "$CI" ] && ! [ -t 0 ]; then CI=1; fi

# ---------------------------------------------------------------------------
# Prompt helpers
# ---------------------------------------------------------------------------

# ask QUESTION DEFAULT [HINT] -> echoes answer (DEFAULT in CI mode / empty input)
ask() {
    local q="$1" def="${2:-}" hint="${3:-}"
    if [ -n "$CI" ]; then printf '%s' "$def"; return; fi
    local prompt="$q"
    [ -n "$def" ] && prompt="$prompt [$def]"
    [ -n "$hint" ] && prompt="$prompt ($hint)"
    read -r -p "$prompt: " ans
    printf '%s' "${ans:-$def}"
}

# ask_yesno QUESTION [default=y] -> returns 0 for yes, 1 for no
ask_yesno() {
    local q="$1" def="${2:-y}" ans
    if [ -n "$CI" ]; then [ "$def" = "y" ]; return; fi
    while true; do
        read -r -p "$q $([ "$def" = "y" ] && echo '[Y/n]' || echo '[y/N]'): " ans
        ans="${ans:-$def}"
        case "$(printf '%s' "$ans" | tr '[:upper:]' '[:lower:]')" in
            y|yes) return 0 ;;
            n|no)  return 1 ;;
            *) echo "  please answer y or n" ;;
        esac
    done
}

# Merge KEY=VALUE into the repo-root .env, preserving all other lines.
set_env_key() {
    local key="$1" val="$2" file="$ROOT/.env" tmp
    touch "$file"
    tmp="$(mktemp)"
    grep -v "^${key}=" "$file" >"$tmp" || true
    printf '%s=%s\n' "$key" "$val" >>"$tmp"
    mv "$tmp" "$file"
}

# ---------------------------------------------------------------------------
# Prerequisite checks / installers (each honours the interactive prompt unless CI)
# ---------------------------------------------------------------------------

go_ok() {
    command -v go >/dev/null 2>&1 || return 1
    local ver
    ver="$(go version | grep -oE 'go[0-9]+\.[0-9]+' | tr -d 'go')"
    awk "BEGIN{exit(!($ver >= $MIN_GO))}"
}

install_git() {
    if command -v git >/dev/null 2>&1; then
        ok "Git found: $(git --version)"
        return
    fi
    if ! ask_yesno "Git not found. Install it now? (the file watcher uses git diff)" y; then
        warn "Skipping Git -- change detection will be degraded."
        return
    fi
    info "Installing Git ..."
    if command -v apt-get >/dev/null 2>&1; then sudo apt-get update -qq && sudo apt-get install -y -qq git
    elif command -v dnf >/dev/null 2>&1; then sudo dnf install -y git
    elif command -v brew >/dev/null 2>&1; then brew install git
    else err "No supported package manager -- install git manually."; fi
    command -v git >/dev/null 2>&1 && ok "Git installed" || warn "Git install failed."
}

install_go() {
    if go_ok; then
        ok "Go $(go version | grep -oE 'go[0-9]+\.[0-9]+') found"
        return
    fi
    command -v go >/dev/null 2>&1 && warn "Go $(go version | grep -oE 'go[0-9]+\.[0-9]+') is too old (need $MIN_GO+)"
    if ! ask_yesno "Install Go $MIN_GO+ now?" y; then
        warn "Skipping Go install -- build steps will fail without it."
        return
    fi
    info "Installing Go $MIN_GO+ ..."
    if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update -qq && sudo apt-get install -y -qq golang-go 2>/dev/null ||
        sudo apt-get install -y -qq golang 2>/dev/null ||
        curl -fsSL "https://go.dev/dl/go1.25.5.linux-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | sudo tar -C /usr/local -xz
    elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y golang
    elif command -v brew >/dev/null 2>&1; then
        brew install go
    else
        ARCH="$(uname -m)"; case "$ARCH" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; esac
        curl -fsSL "https://go.dev/dl/go1.25.5.linux-${ARCH}.tar.gz" | sudo tar -C /usr/local -xz
        export PATH="/usr/local/go/bin:$PATH"
    fi
    if go_ok || command -v go >/dev/null 2>&1; then
        ok "Go installed: $(go version)"
    else
        err "Go installation failed -- install manually from https://go.dev/dl/"
        exit 1
    fi
}

install_engine() {
    local engine="$1"
    if ! command -v "$engine" >/dev/null 2>&1; then
        local label="Docker"
        [ "$engine" = "podman" ] && label="Podman"
        if ! ask_yesno "$engine not found. Install $label now?" y; then
            err "codeRAG needs $engine to run LightRAG + MCP containers."
            exit 1
        fi
        info "Installing $label ..."
        if [ "$engine" = "docker" ]; then
            if command -v apt-get >/dev/null 2>&1; then curl -fsSL https://get.docker.com | sudo sh
            elif command -v dnf >/dev/null 2>&1; then
                sudo dnf install -y docker-ce docker-ce-cli containerd.io 2>/dev/null ||
                curl -fsSL https://get.docker.com | sudo sh
            elif command -v brew >/dev/null 2>&1; then brew install --cask docker
            else err "Install Docker manually: https://docs.docker.com/engine/install/"; exit 1; fi
            sudo systemctl enable docker 2>/dev/null || true
            sudo systemctl start docker 2>/dev/null || true
            sudo usermod -aG docker "$USER" 2>/dev/null || true
        else
            if command -v apt-get >/dev/null 2>&1; then sudo apt-get update -qq && sudo apt-get install -y -qq podman
            elif command -v dnf >/dev/null 2>&1; then sudo dnf install -y podman
            elif command -v brew >/dev/null 2>&1; then brew install podman
            else err "Install Podman manually: https://podman.io/docs/installation"; exit 1; fi
        fi
        command -v "$engine" >/dev/null 2>&1 && ok "$label installed" ||
            { err "$engine installation failed."; exit 1; }
    else
        ok "$engine found: $("$engine" --version 2>/dev/null | head -1)"
    fi

    # Compose must be usable through the engine CLI (`docker compose` / `podman compose`)
    if ! "$engine" compose version >/dev/null 2>&1; then
        err "'$engine compose' is not available."
        if [ "$engine" = "podman" ]; then
            info "Podman needs a compose provider (podman-compose or docker-compose)."
            if command -v pipx >/dev/null 2>&1 || command -v pip3 >/dev/null 2>&1 \
               || command -v apt-get >/dev/null 2>&1 || command -v dnf >/dev/null 2>&1; then
                if ask_yesno "Install podman-compose now?" y; then
                    if command -v apt-get >/dev/null 2>&1; then sudo apt-get install -y -qq podman-compose
                    elif command -v dnf >/dev/null 2>&1; then sudo dnf install -y podman-compose
                    elif command -v pipx >/dev/null 2>&1; then pipx install podman-compose
                    else pip3 install --user podman-compose; fi
                fi
                "$engine" compose version >/dev/null 2>&1 && ok "compose provider installed" ||
                    { err "Still no compose provider -- install podman-compose, then re-run."; exit 1; }
            fi
        else
            info "Docker Engine installs usually ship compose; re-run after installing Docker properly."
            exit 1
        fi
    fi

    # Daemon reachability
    if ! "$engine" info >/dev/null 2>&1; then
        warn "$engine daemon not reachable."
        if [ "$engine" = "docker" ]; then
            info "Try 'sudo systemctl start docker', log out/in (docker group), and re-run."
        else
            info "Rootless podman needs no daemon service -- check 'podman info' output for errors."
        fi
        exit 1
    fi
    ok "$engine is running"
}

install_ollama() {
    if command -v ollama >/dev/null 2>&1; then
        ok "Ollama found: $(ollama --version 2>/dev/null || echo installed)"
        return
    fi
    if ! ask_yesno "Ollama not found. Install it now?" y; then
        err "codeRAG needs Ollama for embeddings. Install from https://ollama.com/ and re-run."
        exit 1
    fi
    info "Installing Ollama ..."
    if [ "$(uname)" = "Darwin" ] && command -v brew >/dev/null 2>&1; then
        brew install --cask ollama
    else
        curl -fsSL https://ollama.com/install.sh | sh
    fi
    command -v ollama >/dev/null 2>&1 && ok "Ollama installed" ||
        warn "Ollama may need a new shell to appear in PATH."
}

# ---------------------------------------------------------------------------
# Models
# ---------------------------------------------------------------------------

# Known embedding dimensions; unknown models must be entered by the user.
embedding_dim_for() {
    case "$1" in
        bge-m3*|mxbai-embed-large*|snowflake-arctic-embed*) echo "1024" ;;
        nomic-embed-text*)                                  echo "768" ;;
        all-minilm*)                                        echo "384" ;;
        *)                                                  echo "" ;;
    esac
}

pull_models() {
    local embed_model="$1" embed_dim="$2" llm_model="$3"

    if ! ollama list >/dev/null 2>&1; then
        warn "Ollama service not reachable -- start Ollama, then run:"
        echo "  ollama pull $embed_model"
        return
    fi

    local models=("$embed_model")
    [ -n "$llm_model" ] && [ "$llm_model" != "none" ] && models+=("$llm_model")

    local m
    for m in "${models[@]}"; do
        if ollama list 2>/dev/null | grep -qF "$m"; then
            ok "Model $m already pulled"
        else
            info "Pulling $m (this may take a while) ..."
            if ollama pull "$m"; then ok "Model $m pulled"
            else err "Pull of $m failed"; fi
        fi
    done

    # Pre-warm the embedding model: its first load can take ~35 s, which would
    # trip LightRAG's 60 s worker timeout on the first scan.
    local bare="${embed_model%%:*}"
    info "Pre-warming embedding model '$bare' ..."
    if curl -fsS -X POST http://localhost:11434/api/embeddings \
        -H 'Content-Type: application/json' \
        -d "{\"model\":\"$bare\",\"input\":\"warmup\"}" >/dev/null; then
        ok "Embedding model warmed up"
    else
        warn "Pre-warm failed -- first scan may be slower."
    fi
}

# ---------------------------------------------------------------------------
# Build steps
# ---------------------------------------------------------------------------

build_coderag() {
    if ! command -v go >/dev/null 2>&1; then
        err "Go not found -- cannot build coderag."
        exit 1
    fi
    info "Building coderag CLI ..."
    go build -C "$ROOT/coderag/cli" -o "$ROOT/coderag/cli/coderag" .
    ok "coderag built: $ROOT/coderag/cli/coderag"

    # The PATH binary resolves the analyser next to itself (pipeline.findCSharpAnalyser).
    info "Building C# analyser ..."
    if (cd "$ROOT/coderag/cli/analysers/csharp" &&
        go build -o "$ROOT/csharp_analyser" ./cmd/csharp_analyser 2>/dev/null); then
        ok "C# analyser built: $ROOT/csharp_analyser"
        cp -f "$ROOT/csharp_analyser" "$ROOT/coderag/cli/analysers/csharp/csharp_analyser"
    else
        warn "C# analyser build skipped (only needed for indexing C# solutions)"
    fi
}

build_images() {
    local engine="$1"
    info "Pre-building container images with $engine (first-time only)..."
    if "$engine" compose build 2>&1 | tail -5; then
        ok "Container images built"
    else
        warn "Image build failed -- the first 'coderag start' will retry."
    fi
}

add_to_path() {
    local rc="$HOME/.bashrc"
    case "${SHELL:-}" in *zsh*) rc="$HOME/.zshrc" ;; esac
    local cliDir="$ROOT/coderag/cli"
    if ! grep -qsF "$cliDir" "$rc"; then
        {
            echo ""
            echo "# codeRAG"
            echo "export PATH=\"$cliDir:$ROOT:\$PATH\""
        } >> "$rc"
        info "Added $cliDir and $ROOT to PATH in $rc (restart shell or 'source $rc')"
    else
        ok "$cliDir already on PATH"
    fi
}

# ---- main -------------------------------------------------------------------

echo ""
echo "======================================="
echo "  codeRAG Setup"
echo "======================================="
echo ""
echo " This wizard installs prerequisites, pulls Ollama models,"
echo " builds the coderag CLI and configures the repo .env."
if [ -z "$CI" ]; then echo " Press ENTER to accept the [default] answers."; fi
echo ""

# Q1: container engine (needed before we can check/install it)
ENGINE="$(ask "Which container engine do you want to use?" "docker" "docker|podman")"
while [ "$ENGINE" != "docker" ] && [ "$ENGINE" != "podman" ]; do
    ENGINE="$(ask "Please type 'docker' or 'podman'" "docker")"
done
ENGINE="$(printf '%s' "$ENGINE" | tr '[:upper:]' '[:lower:]')"
ok "Container engine: $ENGINE"

install_git
install_go
install_engine "$ENGINE"
install_ollama

# Q2/Q3: Ollama models
EMBED_MODEL="$(ask "Embedding model" "bge-m3:latest" "required by every graph operation")"
EMBED_DIM="$(embedding_dim_for "$EMBED_MODEL")"
if [ -z "$EMBED_DIM" ]; then
    EMBED_DIM="$(ask "Embedding dimension for '$EMBED_MODEL'" "1024" "must match the model!")"
fi
LLM_ANSWER="$(ask "Code LLM for 'coderag query'" "none" "phi3.5:latest | qwen3-coder:480b-cloud | none | <custom>")"
LLM_ANSWER="${LLM_ANSWER:-none}"

pull_models "$EMBED_MODEL" "$EMBED_DIM" "$LLM_ANSWER"

build_coderag
build_images "$ENGINE"
add_to_path

# Persist the wizard's choices in the repo-root .env. The Go CLI reads
# CODERAG_ENGINE from here; docker/podman compose picks up the model vars via .env.
set_env_key "CODERAG_ENGINE" "$ENGINE"
set_env_key "EMBEDDING_MODEL" "$EMBED_MODEL"
set_env_key "EMBEDDING_DIM" "$EMBED_DIM"
[ "$LLM_ANSWER" != "none" ] && set_env_key "LLM_MODEL" "$LLM_ANSWER"
ok "Settings written to $ROOT/.env"

echo ""
echo "======================================="
echo "  Setup Complete!"
echo "======================================="
echo ""
echo "  Engine:         $ENGINE"
echo "  Embedding:      $EMBED_MODEL (dim $EMBED_DIM)"
[ "$LLM_ANSWER" != "none" ] && echo "  Code LLM:       $LLM_ANSWER"
echo "  coderag CLI:    $ROOT/coderag/cli/coderag"
echo "  LightRAG API:   http://localhost:9621"
echo "  MCP SSE:        http://localhost:9622"
echo ""
echo "  Quick start (in a NEW shell so PATH applies):"
echo "    cd <any project folder>"
echo "    coderag start .      # services + live watcher"
echo "    coderag scan .       # index the .sln found here"
echo "    coderag status"
echo ""
