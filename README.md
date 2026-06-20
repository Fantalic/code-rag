# CodeLightRAG — Live Codebase Knowledge Graphs for AI Agents

> **⚠️ Local-only Ollama:** This fork supports **only local Ollama** for LLM inference and embeddings. No cloud API keys, no OpenAI/Azure/Bedrock backends. See [docs/DOCUMENTATION.md](docs/DOCUMENTATION.md).

> Transform your codebase into a queryable knowledge graph. AI agents query via MCP instead of reading files — no more "read these 400 files" prompts.

```
[Your Codebase] ──► [Watchdog CLI] ──► [LightRAG API] ──► [Knowledge Graph]
                                                      ▲
[MCP Client] ──► [MCP Server] ────────────────────────│
```

## What It Does

CodeLightRAG watches your project folder and builds a live semantic knowledge graph — entities (classes, methods), their relationships (calls, dependencies), and descriptions — all indexed and queryable via [Model Context Protocol (MCP)](https://modelcontextprotocol.io).

When you change a file, only the affected entities and relations update. No full rebuilds. No wasted embeddings.

AI agents connect via MCP tools to ask questions like:
- *"What classes are in this project?"*
- *"What does the Calculator class depend on?"*
- *"Show all methods that call AddItem"*

## Key Features

- **22 MCP tools** — query, insert, knowledge graph, document management, system tools
- **Live file watching** — git-based change detection with initial full codebase scan
- **Dual ingestion paths** — direct graph CRUD (no LLM) for structured files, LLM pipeline for complex/unstructured content
- **Multi-language analysers** — C# (.NET 9 compiled binary), future: Go, Rust, Zig
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
| **Watchdog CLI** | Go (standalone binary) | File watcher with initial scan + git-based change detection |
| **C# Analyser** | C# (.NET 9 binary) | Extracts classes, methods, project references from .NET projects |

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

### Watchdog Pipeline

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
- Python 3.10+ with venv
- [.NET 9 runtime](https://dotnet.microsoft.com/download/dotnet/9.0) (for C# analyser)
- [Ollama](https://ollama.com/) running locally — **the only supported LLM/embedding backend**
  - Required models: `mistral-nemo:latest` (or any code-capable model), `bge-m3:latest`

### Quick Setup

```powershell
# Clone and setup
git clone https://github.com/your-repo/code-lightrag.git
cd code-lightrag

# One command — creates venv, installs deps, sets up .env
.\scripts\setup.ps1

# Three terminals:
# Terminal 1: LightRAG API + WebUI
lightrag-server

# Terminal 2: MCP server
python -m daniel_lightrag_mcp

# Terminal 3: Go watchdog — watches project, syncs to LightRAG
watchdog\watchdog.exe watch .\myproject --url http://localhost:9621
```

### Or use Docker Compose

```powershell
docker-compose up lightrag mcp
```

### Or start everything at once

```powershell
watchdog\watchdog.exe start .\myproject --ollama=http://localhost:11434
```
This starts LightRAG + MCP server + watchdog in one command.

## Usage

### Watchdog CLI Commands

```bash
# Watch a project folder (connects to existing LightRAG)
watchdog.exe watch ./myproject --url http://localhost:9621

# Start LightRAG + MCP + watchdog together
watchdog.exe start ./myproject --ollama=http://localhost:11434

# Check status
watchdog.exe status

# Query the graph
watchdog.exe query "What classes are in this project?"

# Stop all services
watchdog.exe stop
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
| `.cs` | C# Roslyn (compiled binary) | `daniel-lightrag-mcp/analysers/c_sharp/CSharpAnalyzerTool.exe` |
| `.py`, `.js`, `.ts`, `.go`, `.rs` | Future: Go analyser | Planned |
| `.md`, `.yaml`, `.json`, etc. | Future: Go analyser | Planned |

Unsupported files fall back to the LLM pipeline (Path B).

## Project Structure

```
code-lightrag/
├── LightRAG/                     ← Python: FastAPI server, RAG engine
│   ├── lightrag/                 ← Core RAG + graph CRUD implementation
│   ├── lightrag-api/             ← FastAPI app + routers
│   └── lightrag_webui/           ← React 19 + TypeScript UI
├── daniel-lightrag-mcp/          ← Python: MCP server (22 tools)
│   ├── src/daniel_lightrag_mcp/
│   │   ├── server.py             ← MCP tool implementations
│   │   ├── client.py             ← HTTP client for LightRAG
│   │   └── analysers/c_sharp/    ← CSharpAnalyzerTool.exe binary
│   └── Dockerfile
├── watchdog/                     ← Go: Standalone CLI binary
│   ├── cmd/watchdog/main.go      ← CLI entry point
│   ├── internal/
│   │   ├── crud/client.go        ← Graph CRUD HTTP client
│   │   ├── diff/engine.go        ← Entity/relation diff engine
│   │   ├── filter/security.go    ← PII/keys/secrets filter
│   │   ├── parser/csharp.go      ← C# analyser output parser
│   │   ├── pipeline/run.go       ← Pipeline orchestration
│   │   ├── router/file.go        ← Extension → analyser mapping
│   │   └── watcher/observer.go   ← File watcher + worker pool
│   ├── go.mod
│   └── watchdog.exe              ← Compiled binary
├── tests/                        ← 63 integration tests
├── daniel-lightrag-mcp/tests/    ← 163 MCP tests
├── docs/                         ← ARCHITECTURE, PLAN, CURRENT_CONTEXT, TASK, API_REFERENCE
├── docker-compose.yml
└── Dockerfile
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

**Status:** `experimental` 🧪 · **Build:** `226 tests passing` ✅ · **Go watchdog:** ✅

## Acknowledgements

Built on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and [daniel-lightrag-mcp](https://github.com/desimpkins/daniel-lightrag-mcp) (MIT).

## License

MIT