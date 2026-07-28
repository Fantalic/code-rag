# codeRAG — Live Codebase Knowledge Graphs for AI Agents

> **⚠️ Local-only Ollama:** This fork supports **only local Ollama** for LLM inference and embeddings. No cloud API keys, no OpenAI/Azure/Bedrock backends. See [docs/DOCUMENTATION.md](docs/DOCUMENTATION.md).

> Transform your codebase into a queryable knowledge graph. AI agents query via MCP instead of reading files — no more "read these 400 files" prompts.

```
[Your Codebase] ──► [coderag CLI] ──► [LightRAG API] ──► [Knowledge Graph]
                                                      ▲
[MCP Client] ──► [MCP Server] ────────────────────────│
```

## What It Does

codeRAG watches your project folder and builds a live semantic knowledge graph — entities (classes, methods), their relationships (calls, dependencies), and descriptions — all indexed and queryable via [Model Context Protocol (MCP)](https://modelcontextprotocol.io).

When you change a file, only the affected entities and relations update. No full rebuilds. No wasted embeddings.

AI agents connect via MCP tools to ask questions like:
- *"What classes are in this project?"*
- *"What does the Calculator class depend on?"*
- *"Show all methods that call AddItem"*

## Key Features

- **22 MCP tools** — query, insert, knowledge graph, document management, system tools
- **Live file watching** — git-based change detection with initial full codebase scan
- **Dual ingestion paths** — direct graph CRUD (no LLM) for structured files, LLM pipeline for complex/unstructured content
- **Multi-language analysers** — C# (Go native binary), future: Go, Rust, Zig
- **Security filter** — strips API keys, passwords, PII, secrets before any data reaches the graph
- **Docker-ready** — LightRAG API + MCP server deploy in containers
- **Web UI** — React 19 interface for visual graph exploration at `http://localhost:9621`
- **226 passing tests** — 63 root + 163 MCP tests

## Architecture

### Components

| Component | Language | Purpose |
|-----------|----------|---------|
| **LightRAG API** | Python (FastAPI) | RAG engine, graph storage, query endpoint on port 9621 |
| **MCP Server** | Python | 22 MCP tools, stdio transport for AI agents |
| **coderag CLI** | Go (static binary) | Orchestrator — file watcher, scan, MCP bridge, Docker management |
| **C# Analyser** | Go (native) | Extracts classes, methods, project references from .NET projects |

### Two Ingestion Paths

**Path A — Direct Graph CRUD** (no LLM, for supported file types):
```
File change → Analyser → Diff engine → Security filter → Entity/Relation CRUD
```
Fast, deterministic, no LLM cost.

**Path B — LLM Pipeline** (for unsupported or complex files):
```
File change → POST /documents/text → LightRAG LLM extraction → Graph
```
LLM re-parses content for richer extraction. Both paths are always available.

### Pipeline

```
Initial scan (per solution)
  └─► Run C# analyser once → extract entities + relations
  └─► Enqueue all files with shared analyser results

Incremental changes (git-based or filesystem)
  └─► Debounce (0.5s per path)
  └─► Hash check (skip unchanged)
  └─► Route to analyser or LLM fallback
  └─► Diff against current graph
  └─► Apply entity/relation CREATE/UPDATE/DELETE actions
```

## Installation

### Prerequisites
- [Go](https://go.dev/dl/) 1.25+ (for building the `coderag` CLI)
- [Docker](https://docs.docker.com/engine/install/) (for LightRAG API + MCP server containers)
- [Ollama](https://ollama.com/) running locally — **the only supported LLM/embedding backend**
  - Required models: `qwen3-coder:480b-cloud` (or any code-capable model), `bge-m3:latest`

### Automatic Setup

```bash
# Clone and setup
git clone https://github.com/coderag/code-rag.git
cd code-rag

# Linux / macOS
bash scripts/setup.sh

# Windows (PowerShell)
.\scripts\setup.ps1
```

The setup script installs any missing prerequisites (Go, Docker, Ollama), pulls Ollama models, builds the `coderag` CLI and C# analyser, and pre-builds Docker images.

### Manual Setup

```bash
# Build coderag CLI
go build -o coderag ./cmd/coderag

# Build C# analyser
cd analysers/csharp && go build -o csharp_analyser . && cd ../..

# Start everything
./coderag start ./myproject
```

## Usage

### coderag CLI Commands

```bash
# Start Docker services + file watcher
coderag start ./myproject

# Watch a project folder (connects to existing LightRAG)
coderag watch ./myproject

# Run C# analyser and populate knowledge graph
coderag scan ./myproject

# MCP bridge mode (for opencode)
coderag mcp .

# Check status
coderag status

# Query the graph
coderag query "What classes are in this project?"

# Stop all services
coderag stop
```

### Configuration

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `LIGHTRAG_URL` | `--url` | `http://localhost:9621` | LightRAG API endpoint |
| `OLLAMA_HOST` | `--ollama` | `http://localhost:11434` | **Only supported LLM backend** — Ollama local |
| `INPUT_DIR` | positional | — | Project folder to watch |
| `USE_GIT_WATCHER` | — | `true` | Use git diff for changes (default), `false` for filesystem watching |
| `DEBOUNCE_SECONDS` | `--debounce` | `0.5` | Debounce delay per file path |
| `WATCHER_WORKERS` | `--workers` | `4` | Thread pool size |

### MCP Tools

**Document management:** `insert_text`, `insert_texts`, `upload_document`, `scan_documents`, `get_documents`, `get_documents_paginated`, `delete_document`

**Query:** `query_text`, `query_text_stream`

**Knowledge graph:** `get_knowledge_graph`, `get_graph_labels`, `check_entity_exists`, `update_entity`, `update_relation`, `delete_entity`, `delete_relation`, `update_relations`, `get_related_files`

**System:** `get_pipeline_status`, `get_track_status`, `get_document_status_counts`, `get_health`

## Supported File Types

| Extension | Analyser | Path |
|-----------|----------|------|
| `.cs` | C# Go native analyser | `analysers/csharp/csharp_analyser.exe` |
| `.py`, `.js`, `.ts`, `.go`, `.rs` | Future analysers | Planned |
| `.md`, `.yaml`, `.json`, etc. | LLM fallback | — |

Unsupported files fall back to the LLM pipeline (Path B).

## Project Structure

```
code-rag/
├── cmd/
│   └── coderag/main.go           ← CLI entry point → builds `coderag` binary
├── pkg/
│   ├── mcp/                      ← MCP server SDK (mark3labs/mcp-go)
│   ├── watchdog/                 ← File watcher (importable Go package)
│   │   ├── watcher/              ← fsnotify + git observer
│   │   ├── diff/                 ← Entity/relation diff engine
│   │   ├── pipeline/             ← Pipeline orchestration
│   │   ├── parser/               ← Analyser output parsers
│   │   ├── router/               ← File extension → analyser mapping
│   │   ├── filter/               ← Security filter (PII/keys/secrets)
│   │   └── crud/                 ← LightRAG HTTP CRUD client
│   └── lightrag/                 ← LightRAG HTTP API client
├── analysers/
│   └── csharp/                   ← Go native C# analyser
├── LightRAG/                     ← Python: FastAPI server, RAG engine
│   ├── lightrag/                 ← Core RAG + graph CRUD implementation
│   ├── lightrag-api/             ← FastAPI app + routers
│   └── lightrag_webui/           ← React 19 + TypeScript UI
├── daniel-lightrag-mcp/          ← Python: MCP server (22 tools)
│   ├── src/daniel_lightrag_mcp/
│   │   ├── server.py             ← MCP tool implementations
│   │   └── client.py             ← HTTP client for LightRAG
│   └── Dockerfile
├── docs/                         ← ARCHITECTURE, PLAN, CURRENT_CONTEXT, API_REFERENCE
├── tests/                        ← Integration tests
├── daniel-lightrag-mcp/tests/    ← MCP tests
├── docker-compose.yml
├── Dockerfile
└── go.mod
```

## Testing

```bash
# Root tests (63)
pytest tests/ -v

# MCP tests (163)
pytest daniel-lightrag-mcp/tests/ -v

# All Python tests
pytest tests/ daniel-lightrag-mcp/tests/ -v  # 226 total

# Go build
go build ./...
```

## Security

All analyser output is filtered before reaching the graph:

- 🔴 API keys (`sk-...`, `ghp_...`, `AKIA...`, JWTs, Stripe keys)
- 🔴 Passwords and secrets (`password=...`, `SECRET=...`)
- 🔴 Private keys (PEM formats: RSA, EC, OPENSSH)
- 🔴 PII (emails, IP addresses)
- 🔴 Internal paths (`\\server\share`, `//internal/`)
- 🔴 Environment secret patterns (`API_KEY=...`)

## Status

**Status:** `experimental` 🧪 · **Build:** `226 tests passing` ✅

## Acknowledgements

Built on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and [daniel-lightrag-mcp](https://github.com/desimpkins/daniel-lightrag-mcp) (MIT). CodeRAG Go packages are MIT — use freely.

## License

MIT