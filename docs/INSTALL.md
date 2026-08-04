# Install & Troubleshooting

> Everything you need to get codeRAG running — prerequisites, automated and manual
> install, offline mode, and a troubleshooting guide. Quick start lives in the
> [README](../README.md).

---

## 1. Prerequisites

| Tool | Min version | Purpose |
|------|-------------|---------|
| **Go** | 1.25+ | Builds the `coderag` CLI + C# analyser (only needed to build) |
| **Docker** | 24+ | Runs the LightRAG API (`:9621`) and MCP server (`:9622`) |
| **Ollama** | 0.3+ | Local LLM + embeddings (runs on the host) |
| **Git** | any | The watcher uses `git diff HEAD` for change detection |

Check what you already have:

```bash
go version        # go1.25.x or newer
docker --version
ollama --version
```

> **Windows:** Docker Desktop must be running (WSL2 backend). PowerShell 5.1+ or
> Git Bash both work. The C# analyser is a native `.exe` — no .NET runtime needed.

## 2. Ollama models

There are two kinds of models:

- **Local models** — downloaded by `ollama pull`, run on your machine, work offline.
- **Cloud models** — served by Ollama from the cloud; **nothing is downloaded** and
  they work immediately through your local Ollama (small free-token allowance, needs
  internet + an Ollama account).

### Embeddings (required)

```bash
ollama pull bge-m3:latest   # local, ~1.2 GB
```

`bge-m3` backs every graph read/write — the graph tools don't work without it.

### Code LLM (optional, for `coderag query` + prose ingestion)

| Model | Kind | Notes |
|-------|------|-------|
| `phi3.5:latest` (default) | local | `ollama pull phi3.5:latest`; offline-friendly |
| `qwen3-coder:480b-cloud` | **cloud** | No pull needed — register it in your Ollama account and it serves from the cloud with free tokens |

The default `LLM_MODEL` in `docker-compose.yml` is `phi3.5:latest` (a real local
model); `qwen3-coder:480b-cloud` is a cloud model, which is why it isn't the default.
Switch with the `LLM_MODEL` env var (see §6).

- If you only index C# and use the deterministic tools, you don't need a code LLM at
  all — the C# analyser never calls one.
- `ollama serve` must be running and reachable at `http://localhost:11434`.

## 3. Automated setup

```bash
git clone https://github.com/Fantalic/code-rag.git
cd code-rag

# Linux / macOS
bash scripts/setup.sh

# Windows (PowerShell)
.\scripts\setup.ps1
```

Both scripts do the same things:

1. Install Go, Docker, and Ollama if missing.
2. Pull the embeddings model — from `EMBEDDING_MODEL`, default `bge-m3:latest`.
3. Build the `coderag` CLI and the C# analyser.
4. Pre-build the Docker image (`docker compose build`).
5. Add the repo folder to your `PATH`.

Model pulls respect your env vars:

```bash
# default: embeddings only — MCP usage needs no code LLM
bash scripts/setup.sh

# explicit embedding model + a code LLM for 'coderag query'
EMBEDDING_MODEL=bge-m3:latest LLM_MODEL=qwen3-coder:480b-cloud bash scripts/setup.sh
```

If `LLM_MODEL` is set, the setup script pulls that model too; otherwise it pulls
only the embedding model (the default is "no query model" — the MCP tools don't
need an LLM).

When it finishes:

```bash
coderag status
```

If the services show `not running`, run `coderag start ./myproject` once to start them.

## 4. Manual install

### 4.1 Build the CLI

```bash
cd code-rag

# Linux / macOS
go build -C coderag/cli -o coderag .

# Windows
go build -C coderag/cli -o coderag.exe .
```

The binary is self-contained (~20 MB, no runtime dependencies). Put it on your `PATH`
or use it as `./coderag`.

### 4.2 Build the C# analyser (only if you index C#)

```bash
cd coderag/cli/analysers/csharp
go build -o csharp_analyser ./cmd/csharp_analyser/    # add .exe on Windows
```

The analyser is a separate binary. `coderag` finds it by name in the same directory
as itself, so keep `csharp_analyser` next to `coderag` (the setup scripts build both
into the repo root).

### 4.3 Build the Docker image

```bash
docker compose build
```

The image contains LightRAG API + WebUI + the MCP server. The first `coderag start`
also builds it automatically.

### 4.4 Verify

```bash
coderag status
#   LightRAG API:   running    http://localhost:9621
#   MCP SSE:        running    http://localhost:9622
#   Workspace:      -
```

Then index something and check the graph:

```bash
coderag scan ./myproject
coderag graph files
```

## 5. Offline / no-internet mode

Everything runs locally, but the first install needs internet for: Go/Docker/Ollama
downloads, model pulls, and the Docker image build. To run on an air-gapped machine,
prepare once on a connected machine:

```bash
# 1. Pre-build the image and export it
docker compose build
docker save -o coderag-image.tar lightrag-server:latest coderag-mcp:latest

# 2. Pre-pull the LOCAL models (they are cached in ~/.ollama)
ollama pull bge-m3:latest
ollama pull phi3.5:latest       # or any other local LLM you want

# 3. Copy the repo + tar + model cache to the offline machine, then:
docker load -i coderag-image.tar
cp -r ~/.ollama <offline-home>/.ollama    # or use `ollama`'s OLLAMA_MODELS env var
```

> Offline means **local models only** — cloud models (e.g. `qwen3-coder:480b-cloud`)
> can't be used without internet. Set `LLM_MODEL` to the local model you pre-pulled
> (default `phi3.5:latest` already is).

After that, `coderag start` / `coderag scan` never need the network. The setup scripts
re-download nothing once binaries and models are present.

> Note: the compose file uses `restart: unless-stopped` and volume `rag_data` — your
> graph survives container restarts and image rebuilds.

## 6. Configuration

| Variable | Default | When to change |
|----------|---------|----------------|
| `OLLAMA_HOST` | `http://host.docker.internal:11434` | Linux with host networking → `http://localhost:11434` |
| `LLM_MODEL` | `phi3.5:latest` | Code LLM for `coderag query` / prose ingestion — any local model you pulled, or an Ollama cloud model (e.g. `qwen3-coder:480b-cloud`, free tokens) |
| `EMBEDDING_MODEL` | `bge-m3:latest` | Keep unless you swap embedding models |
| `EMBEDDING_DIM` | `1024` | Must match the embedding model |
| `LIGHTRAG_URL` | `http://localhost:9621` | Custom LightRAG deployment |
| `DEBOUNCE_SECONDS` | `0.5` | Higher = fewer re-analyses, slower feedback |
| `WATCHER_WORKERS` | `4` | Higher = faster scanning on big machines |

Set them in your shell before running `coderag`, or in a `.env` file at the repo root
(compose reads `OLLAMA_HOST`/`WORKSPACE` from the environment or `.env`).

## 7. Troubleshooting

### Docker services won't start

- **Docker Desktop not running** → start it, wait for the whale icon, retry.
- **Ports 9621/9622 in use** → `netstat -ano | findstr 9621`; stop whatever owns them,
  or remap ports in `docker-compose.yml`.
- **"Workspace mismatch"** → codeRAG restarts the stack with the right `WORKSPACE`
  automatically. If it keeps failing, `coderag stop` then `coderag start <dir>`.

### Ollama unreachable from the container

`coderag query` or prose ingestion times out with an LLM error. Check connectivity:

```bash
sh scripts/dev/diag_ollama.sh
```

- On **Windows/macOS** the container reaches the host via `host.docker.internal`
  (the default). If Docker Desktop's "Host networking" is off, this should still work.
- On **Linux**, containers reach the host via `localhost` only with `network_mode: host`
  or by setting `OLLAMA_HOST=http://localhost:11434` and `extra_hosts`. The simplest fix:

  ```bash
  export OLLAMA_HOST=http://host.docker.internal:11434
  docker run --add-host=host.docker.internal:host-gateway --rm -it curlimages/curl http://host.docker.internal:11434
  ```

  See `scripts/dev/diag_ollama.sh` for the full checks.

### Slow first scan / "worker timeout"

The first embedding of a freshly loaded model can take ~35 s (cold load). LightRAG's
worker timeout is 60 s. Either wait for the scan to warm up, or pre-warm the model:

```bash
curl -X POST http://localhost:11434/api/embeddings \
  -H "Content-Type: application/json" \
  -d '{"model":"bge-m3","input":"warmup"}'
```

### `coderag scan` says "no .sln file found"

codeRAG looks for a `.sln` in the given directory and up to 10 levels above it.
Point it at a folder containing (or inside) your solution:

```bash
coderag scan ./MySolution
```

### Entities look wrong / stale

- Re-index from scratch: `coderag scan --clean` (wipes graph + analyser output).
- Multiple projects: each project is a separate workspace. `coderag status` shows the
  active one. Use `coderag graph files --workspace <name>` to target another workspace.

### "MCP not reachable at :9622"

The MCP container takes a moment to become healthy. `coderag mcp` waits up to 60 s.
If it never comes up:

```bash
docker compose logs mcp      # look for a traceback
docker compose ps
```

### Reset everything

```bash
coderag stop                    # docker compose down
docker volume rm coderag_rag_data   # wipe graph data entirely (name may vary)
rm -rf <project>/.coderag       # wipe per-project state
```

---

Next: **[USAGE.md](USAGE.md)** — the full CLI reference · **[MCP_CLIENTS.md](MCP_CLIENTS.md)** — wiring AI agents.
