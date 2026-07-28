# Architecture

> How the project is organized, why it is built that way, and where it is going.

---

## 1. System Overview

```
[Your Codebase] --> [coderag CLI] --> [LightRAG API] --> [Knowledge Graph]
                                                    ^
[MCP Client] -----> [MCP Server] -----------------|
```

The system transforms source code into a queryable knowledge graph. AI agents query it via MCP instead of reading files directly.

The `coderag` binary runs on the **host machine** (native Go, fast filesystem access). LightRAG and the MCP server run in **Docker containers** (long-running Python services).

---

## 2. Component Ownership

| Component | Language | Where | Why |
|-----------|----------|-------|-----|
| **coderag CLI** | Go | Host | Orchestrator — file watching, subprocess spawning, MCP bridge, Docker management. Needs low latency and direct filesystem access. |
| **LightRAG API** | Python (FastAPI) | Docker | LLM/embedding integrations, graph storage, vector DB. Long-running service benefits from containerization. |
| **MCP Server** | Python | Docker | MCP tool definitions, SSE transport. Runs alongside LightRAG in Docker. |
| **C# Analyser** | Go | Host | Subprocess launched by coderag. CPU-bound parsing, needs host filesystem access. |
| **WebUI** | TypeScript/React | Docker | Served as static files by LightRAG API. |

Go for the performance-critical, I/O-bound orchestration layer. Python stays for the parts that are already working well (LightRAG LLM pipeline, MCP tools).

---

## 3. coderag CLI (cmd/coderag/)

The `coderag` binary is the single entry point. It runs on the host machine and orchestrates everything.

### Architecture

```
coderag start <dir>:
  → docker compose up -d (if LightRAG not healthy at :9621)
  → start file watcher on <dir>
  → block until Ctrl+C
  → stop watcher (Docker keeps running)

coderag mcp <dir>:
  → docker compose up -d (if LightRAG not healthy at :9621)
  → start file watcher on <dir> (background goroutine)
  → stdio↔SSE bridge to Python MCP server at :9622

coderag scan <dir>:
  → (optional --clean) DELETE /documents + remove analyser_output
  → find .sln
  → run C# analyser (csharp_analyser.exe)
  → parse output into entities + relations
  → diff against current graph → create/update/delete via LightRAG CRUD

coderag watch <dir>:
  → file watcher only (assumes LightRAG already running)
```

### Commands

| Command | Description |
|---------|-------------|
| `start <dir>` | Start Docker services + file watcher in foreground |
| `mcp <dir>` | MCP bridge mode (stdio↔SSE) for opencode/agent connections |
| `scan <dir>` | Run C# analyser and populate knowledge graph |
| `watch <dir>` | File watcher only (no Docker orchestration) |
| `status` | Show running services status |
| `stop` | Stop all services (`docker compose down`) |
| `query "<text>"` | Quick query against the knowledge graph |
| `webui` | Open WebUI in browser |

### File Routing

Supported file types (extension → analyser):

| Extension | Analyser | Notes |
|-----------|----------|-------|
| `.cs` | C# Go native analyser | `analysers/csharp/csharp_analyser.exe scan --solution --output` |
| `.py`, `.js`, `.ts`, `.go`, `.rs` | Future analysers | Planned |
| `.md`, `.yaml`, `.json`, etc. | LLM fallback | POST /documents/text |

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

Analysers are standalone executables invoked by `coderag`. They take a solution file and produce structured entity/relation output.

### Interface

```
csharp_analyser scan --solution <sln> --output <output-dir>
  Writes: <output-dir>/<ClassName>.txt  (one file per class, pipe-delimited)
  Also:   <output-dir>/_index.txt       (global symbol index)
  Exit code: 0 = success, non-zero = error
```

### C# Analyser (current)

- Binary: `csharp_analyser.exe` (Go native, fully static, no dependencies)
- Path: `analysers/csharp/csharp_analyser.exe`
- Parser: Go-native state machine (not tree-sitter, not CGo)
- Output format (pipe-delimited):
  ```
  # EXPORTS: ClassA, ClassB
  # IMPORTS: System, System.Collections.Generic
  CLASS|ClassName|path/to/file.cs|42|description
  METHOD|MethodName|path/to/file.cs|50|description
  RELATION|ClassA|ClassB|description|keywords|file.cs|1.0
  ```
- Entity types: NAMESPACE, CLASS, METHOD, PROPERTY, INTERFACE, STRUCT, ENUM
- Relation types: method_defines, property_has, inherits_from, implements, calls, contains

### C# Analyser Design Decisions

- **Go regex-based state machine** (not tree-sitter) → zero CGo, fully static binary, cross-compilation
- **Two-phase analysis:** Phase 1 builds global symbol index from all `.cs` files, Phase 2 resolves cross-file calls
- **Expression-bodied methods** (`=>`) handled inline (no `{` scope)
- **Class scope** tracked via `classDepth` (brace depth at declaration), not `braceDepth == 0`

### Future Analysers

Will follow the same binary interface pattern. Candidates: Go (goroutines/channels), Python (AST), Rust/Zig (zero-cost parsing).

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

Model Context Protocol server with 15 tools. Runs in Docker on port 9622 (SSE transport). The Go `coderag` binary bridges stdio↔SSE.

```
opencode (stdio JSON-RPC) → coderag mcp (Go bridge) → SSE → MCP Server (Docker :9622) → HTTP → LightRAG API (Docker :9621)
```

**Categories:**
- Query (2 tools): `query`, `query_data`
- Knowledge graph & traversal (8 tools): `get_knowledge_graph` (`max_depth`), `get_graph_labels`, `check_entity_exists`, `add_note_to_entity`, `get_related_files`, `get_entity_neighbors`, `get_incoming_relations`, `get_outgoing_relations`
- Exploration (5 tools): `get_codebase_stats` (`visibility_counts`), `list_entities_by_type`, `get_entities_in_file`, `search_entities`, `get_entity_details` (`depth`)

**Depth params on tools:**
- `get_entity_details(depth)` — 1=entity only, 2=structural (includes members), 3=full (all relations)
- `get_knowledge_graph(max_depth)` — 1=types only, 2=types+members, 3=all nodes

---

## 8. Docker / Distribution

### Services

| Service | Dockerfile | Port | Purpose |
|---------|-----------|------|---------|
| `lightrag` | root `Dockerfile` | `:9621` | LightRAG API server + WebUI |
| `mcp` | `daniel-lightrag-mcp/Dockerfile` | `:9622` | MCP server (SSE transport) |

The `coderag` CLI runs on the host, NOT in Docker. This is by design — the Go binary needs fast filesystem access for file watching and analyser subprocess calls.

### Docker Compose

Services are configured in `docker-compose.yml` at the project root. `coderag start` and `coderag mcp` call `docker compose up -d` automatically when LightRAG is not reachable.

### Ollama

Ollama runs on the **host machine** (not in Docker by default). Docker services connect via `host.docker.internal:11434` (Windows/macOS Docker Desktop) or `localhost` (Linux with host networking). Configure via `OLLAMA_HOST` environment variable. An optional Ollama sidecar container can be added to `docker-compose.yml` if desired.

---

## 9. Directory Layout

```
code-rag/
+-- cmd/
|   +-- coderag/main.go         <- CLI entry point -> builds `coderag` binary
+-- pkg/
|   +-- mcp/                    <- MCP server SDK (mark3labs/mcp-go)
|   +-- watchdog/               <- File watcher (importable Go package)
|   |   +-- watcher/            <- fsnotify + git observer
|   |   +-- diff/               <- Entity/relation diff engine
|   |   +-- pipeline/           <- Pipeline orchestration
|   |   +-- parser/             <- Analyser output parsers
|   |   +-- router/             <- Extension -> analyser mapping
|   |   +-- filter/             <- Security filter (PII/keys/secrets)
|   |   +-- crud/               <- LightRAG HTTP CRUD client
|   +-- lightrag/               <- LightRAG HTTP API client
+-- analysers/
|   +-- csharp/                 <- Go native C# analyser
+-- LightRAG/                   <- Python: FastAPI server, RAG engine
|   +-- lightrag/               <- Core RAG + graph CRUD implementation
|   +-- lightrag-api/           <- FastAPI app + routers
|   +-- lightrag_webui/         <- React 19 + TypeScript UI
+-- daniel-lightrag-mcp/        <- Python: MCP server
|   +-- src/daniel_lightrag_mcp/
|   |   +-- server.py           <- 22 MCP tools
|   |   +-- client.py           <- HTTP client for LightRAG
|   +-- Dockerfile
+-- docs/
|   +-- ARCHITECTURE.md         <- This file
|   +-- DOCUMENTATION.md        <- High-level system reference
|   +-- API_REFERENCE.md        <- Detailed API schemas
|   +-- PLAN.md                 <- Roadmap
|   +-- CURRENT_CONTEXT.md      <- Session state
|   +-- TASK.md                 <- Active task breakdown
+-- docker-compose.yml          <- Multi-service orchestration
+-- Dockerfile                  <- LightRAG + WebUI image
+-- go.mod
```

---

## 10. Decisions Made

| Decision | Rationale |
|----------|-----------|
| **coderag CLI on host, services in Docker** | File watching (fsnotify) and subprocess spawning need low latency + direct filesystem access. Docker adds overhead for I/O-bound orchestration. Python services (LightRAG, MCP) benefit from container isolation. |
| **Go for coderag, not Python** | Single portable binary, fast startup, no runtime dependency. Goroutines for concurrent watchers. |
| **C# analyser in Go, not C#/Roslyn** | Self-contained static binary (~5MB), no .NET runtime needed, cross-compilation, fast startup. |
| **Pipe-delimited analyser output** | Simple, parseable, language-agnostic interface between analysers and pipeline. |
| **Keep LightRAG + MCP in Python** | Already working, rich ecosystem for LLM/embedding. The MCP SSE bridge in Go is lightweight proxying. |
| **Both ingestion paths live permanently** | Path A (direct CRUD without LLM) for structured types. Path B (LLM pipeline) for everything else. Users don't choose — file type determines the path. |
| **Ollama on host, not in Docker by default** | Keeps Ollama GPU access simple, avoids Docker GPU passthrough complexity. Optional Docker sidecar available. |
| **Project-based storage (`.coderag/`)** | Isolates each project's RAG data. Auto-detected by LightRAG via parent directory search. |