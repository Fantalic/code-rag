# Architecture

> How the project is organized, why it is built that way, and where it is going.

---

## 1. System Overview

```
[Your Codebase] --> [Watchdog CLI] --> [LightRAG API] --> [Knowledge Graph]
                                                    ^
[MCP Client] -----> [MCP Server] -----------------|
```

The system transforms source code into a queryable knowledge graph. AI agents query it via MCP instead of reading files directly.

---

## 2. Component Ownership

| Component | Language | Why |
|-----------|----------|-----|
| **LightRAG API** | Python (FastAPI) | LLM/embedding integrations, FastAPI server, rich ecosystem for RAG |
| **MCP Server** | Python | MCP protocol, already built and working |
| **Watchdog CLI** | Go | I/O-bound router - file watching, HTTP calls, process spawning. Fast to build, single portable binary. |
| **Code Analysers** | Go (future: Rust/Zig) | CPU-bound parsing work. Compiled to native binary, no runtime needed. |

Python stays for the parts that are already working well (LightRAG, MCP) and where Python's ecosystem is a strength. Go for the parts that need to be fast, portable, and distribution-friendly.

---

## 3. Watchdog CLI (watchdog/)

A standalone Go binary that monitors a project folder and keeps the knowledge graph in sync.

### Usage Modes

**Mode 1 - Watch standalone (connects to existing LightRAG):**
```
watchdog watch ./myproject --url=http://localhost:9621
```
- Watches project folder for changes
- Processes supported files through the pipeline
- Posts entity/relation updates to existing LightRAG via HTTP
- Falls back to LLM pipeline for unsupported file types

**Mode 2 - Start everything (launches LightRAG + watchdog):**
```
watchdog start ./myproject --ollama=http://localhost:11434
```
- Starts LightRAG API as a subprocess
- Starts MCP server as a subprocess
- Starts file watcher
- All in one portable binary

**Mode 3 - No watchdog (original usage):**
```
docker-compose up lightrag
# Open http://localhost:9621 for WebUI
# Use MCP tools to query
```
- LightRAG + WebUI only, no file watching
- Manual file upload via WebUI
- Existing behavior preserved

### Commands

| Command | Description |
|---------|-------------|
| `watch <dir>` | Watch a project folder, connect to existing LightRAG |
| `start <dir>` | Start LightRAG + MCP + watchdog all together |
| `status` | Show running services |
| `stop` | Stop all services |
| `query "<text>"` | Query the graph directly |

### Configuration

All via environment variables and CLI flags:

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `LIGHTRAG_URL` | `--url` | `http://localhost:9621` | LightRAG API endpoint |
| `OLLAMA_HOST` | `--ollama` | `http://localhost:11434` | Ollama endpoint |
| `INPUT_DIR` | positional arg | - | Project folder to watch |
| `DEBOUNCE_SECONDS` | `--debounce` | `0.5` | Debounce delay per file path |
| `WATCHER_WORKERS` | `--workers` | `4` | Thread pool size |
| `LIGHTRAG_API_KEY` | `--api-key` | - | API authentication |

### File Routing

Supported file types (extension -> analyser):

| Extension | Analyser | Notes |
|-----------|----------|-------|
| `.cs` | C# (Roslyn-based) | Calls CSharpAnalyzerTool.exe, output parsed to entities |
| `.py` | - | Future: Go analyser |
| `.js` / `.ts` | - | Future: Go analyser |
| `.go` | - | Future: Go analyser |
| `.rs` | - | Future: Go analyser |
| `.md` / `.txt` / `.yaml` | - | Future: Go analyser |

Unsupported files fall back to POST /documents/text (LLM pipeline) - both paths are always live.

---

## 4. Watchdog Pipeline

```
File event
  -> debounce (per file path, 0.5s default)
  -> hash check (MD5, skip if unchanged)
  -> file_router (extension -> analyser?)
  -> analyser (extract entities + relations)
  -> security_filter (strip PII/keys/secrets)
  -> graph_diff (old vs new -> CREATE/UPDATE/DELETE actions)
  -> graph_crud (HTTP calls to LightRAG entity/relation endpoints)
```

**Path A - Direct graph CRUD** (supported files):
- Analyser produces structured output
- Diffed against current graph state
- Security-filtered
- Entity/relation CRUD via /graph/entity/create, /graph/relation/create, etc.
- No LLM involved

**Path B - LLM pipeline** (unsupported files or fallback):
- POST /documents/text with file content
- LightRAG LLM pipeline extracts entities/relations
- Full document re-insert

Both paths remain available permanently.

---

## 5. Analyser Design

Analysers are standalone executables invoked by the watchdog. They take a project path and produce structured entity/relation output.

### Interface

```
analyser <project-path> <output-dir>
  Writes: <output-dir>/<ClassName>.txt  (one file per class)
  Format: plain text, parser-dependent
  Exit code: 0 = success, non-zero = error
```

### C# Analyser (current)

- Binary: CSharpAnalyzerTool.exe (C# / .NET 9)
- Path: daniel-lightrag-mcp/analysers/c_sharp/
- Output format:
  ```
  FILE: <relative-path>

  CLASS: <ClassName>
    METHOD: <name> -> RETURNS -> <type>

  PROJECT REFERENCES:
    - <ProjectName>
  ```
- Parser: Go reimplementation of analyser_parsers.go logic

### Future Analysers (Go)

Analysers for Python, JS/TS, Go, Rust, and config formats (md, yaml, ini) will be written in Go. The interface stays the same - watchdog invokes the binary and parses output.

---

## 6. LightRAG API (LightRAG/)

Python FastAPI server on port 9621. Two endpoint groups:

**Document pipeline** (Path B):
- POST /documents/text - ingest file content, LLM extracts entities/relations
- POST /documents/texts - batch ingest
- DELETE /documents - clear all

**Graph CRUD** (Path A):
- POST /graph/entity/create - create entity directly
- POST /graph/relation/create - create relation directly
- POST /graph/entity/edit - update entity properties
- POST /graph/relation/edit - update relation properties
- DELETE /documents/delete_entity - delete entity + its relations
- DELETE /documents/delete_relation - delete single relation
- POST /graph/entities/merge - merge duplicate entities
- GET /graphs?label=* - read full graph
- GET /graph/entity/exists?name=... - check entity exists

See API_REFERENCE.md for detailed request/response schemas.

---

## 7. MCP Server (daniel-lightrag-mcp/)

Model Context Protocol server with 22 tools. Connects AI agents to LightRAG.

**Categories:**
- Document management (7 tools): insert, upload, scan, query, delete
- Query (2 tools): text query, streaming query
- Knowledge graph (8 tools): get graph, check entity, update entity/relation, delete, get related files
- System (4 tools): health, pipeline status, track status, status counts

Key tool: update_relations - runs the C# analyser on a .sln file and ingests output via insert_text.

---

## 8. Docker / Distribution

### Services

| Service | Dockerfile | Purpose |
|---------|-----------|---------|
| `lightrag` | LightRAG/Dockerfile | LightRAG API server + WebUI |
| `watchdog` | watchdog/Dockerfile | Standalone Go binary, watch or start mode |
| `mcp` | daniel-lightrag-mcp/Dockerfile | MCP server (optional) |

### Ollama

Ollama runs on the **host machine** (not in Docker). Services connect to it via:
- On Windows with Docker Desktop: host.docker.internal:11434 (if host network used) or Docker bridge IP
- Configure via OLLAMA_HOST environment variable

### Volume Mounts

```yaml
watchdog:
  volumes:
    - /path/to/your/project:/project:ro  # read-only project mount
  environment:
    - INPUT_DIR=/project
```

---

## 9. Directory Layout

```
code-lightrag/
+-- LightRAG/                   <- Python: FastAPI server, RAG engine
|   +-- lightrag/               <- Core RAG + graph CRUD implementation
|   +-- lightrag-api/           <- FastAPI app + routers
|   +-- lightrag_webui/         <- React 19 + TypeScript UI
+-- daniel-lightrag-mcp/        <- Python: MCP server
|   +-- src/daniel_lightrag_mcp/
|   |   +-- server.py           <- 22 MCP tools
|   |   +-- client.py           <- HTTP client for LightRAG
|   |   +-- analyser_runner.py  <- Subprocess runner for C# analyser
|   |   +-- analysers/c_sharp/  <- CSharpAnalyzerTool.exe binary
|   +-- Dockerfile
+-- watchdog/                   <- Go: Standalone CLI binary
|   +-- cmd/watchdog/           <- main.go, CLI entry point
|   +-- internal/
|   |   +-- pipeline/           <- Pipeline orchestration
|   |   +-- analyser/           <- Analyser invoker + parsers
|   |   +-- router/             <- Extension -> analyser mapping
|   |   +-- diff/               <- Entity/relation diff engine
|   |   +-- filter/             <- Security filter (PII/keys/secrets)
|   |   +-- crud/               <- Graph CRUD HTTP client
|   +-- go.mod
|   +-- Dockerfile
+-- tests/                      <- Python: 63 integration tests
+-- test-csharp-project/        <- C# test project for watchdog testing
+-- docs/
|   +-- ARCHITECTURE.md         <- This file
|   +-- DOCUMENTATION.md        <- High-level system reference
|   +-- API_REFERENCE.md        <- Detailed API schemas
|   +-- PLAN.md                 <- Roadmap
|   +-- CURRENT_CONTEXT.md      <- Session state
|   +-- TASK.md                 <- Active task breakdown
+-- docker-compose.yml          <- Multi-service orchestration
```

---

## 10. Decisions Made

| Decision | Rationale |
|----------|-----------|
| Watchdog in Go, not Python | Single portable binary, fast build, no Python dependency for users |
| Watchdog as CLI, not service | Can run standalone (watch mode) or as master (start mode) |
| Analysers in Go | CPU-bound parsing work. Compiled, no runtime. Keep it simple for now. |
| Keep LightRAG + MCP in Python | Already working, rich ecosystem for LLM/embedding |
| Both ingestion paths live permanently | Path A (direct CRUD) + Path B (LLM pipeline) - user chooses via file type |
| Ollama on host, not in Docker | Keeps Ollama GPU access simple, avoids Docker GPU passthrough complexity |
| WebUI as separate concern | LightRAG WebUI is independent, opens in browser at port 9621 |