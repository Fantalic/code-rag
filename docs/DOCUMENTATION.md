# codeRAG — System Documentation

> Everything about how this project works: components, data flow, pipeline, and security.
> For detailed API request/response schemas, see [API_REFERENCE.md](API_REFERENCE.md).

> **⚠️ Local-only Ollama:** This project supports **only local Ollama** for LLM inference and embeddings.
> No cloud API keys, no API key auth needed — `LIGHTRAG_API_KEY` is unused. See §7 for env vars.

---

## 1. Architecture Overview

### Host vs Docker split (design rationale)

```
┌─────────────────────────────────────────────────────────────────┐
│  HOST MACHINE (coderag CLI — native Go binary)                 │
│  Fast, direct filesystem access, no Docker overhead            │
│                                                                 │
│  ┌──────────────┐   stdio    ┌───────────────────────────┐     │
│  │   opencode    │◄─────────►│  coderag (Go binary)      │     │
│  │  (AI agent)   │   (MCP)   │  • File watcher (fsnotify) │     │
│  └──────────────┘            │  • C# analyser (subprocess)│     │
│                              │  • Docker orchestration    │     │
│                              │  • MCP stdio↔SSE bridge    │     │
│                              └─────────┬─────────────────┘     │
│                                        │                       │
│                               HTTP :9621│      SSE :9622       │
│                                        │                       │
└────────────────────────────────────────┼───────────────────────┘
                                         │
         ┌───────────────────────────────┼───────────────────────┐
         │  DOCKER / PODMAN             │                       │
         │  Long-running services       │                       │
         │                              │                       │
         │  ┌────────────────────┐      │  ┌────────────────┐   │
         │  │ LightRAG API       │◄─────┘  │ MCP Server     │   │
         │  │ (Python FastAPI)   │  HTTP   │ (Python)       │   │
         │  │ :9621              │         │ :9622           │   │
         │  │ • Graph DB         │         │ • 15 MCP tools  │   │
         │  │ • Vector DB        │         │ • SSE transport │   │
         │  │ • WebUI            │         └────────────────┘   │
         │  └────────────────────┘                              │
         │                                                      │
         │  ┌────────────────────┐                              │
         │  │ Ollama (optional)  │◄─── localhost:11434          │
         │  │ LLM + embeddings   │                              │
         │  └────────────────────┘                              │
         └──────────────────────────────────────────────────────┘
```

**Why this split?** The `coderag` CLI runs on the host because:
- File watching (fsnotify) needs low latency — Docker volume mounts add overhead
- The C# analyser spawns subprocesses on the host filesystem — Docker would require complex mounts
- The MCP bridge connects opencode (on host) to the MCP server (in Docker) via lightweight stdio↔SSE proxying

Docker hosts the long-running Python services (LightRAG, MCP) where containerization provides isolation, reproducibility, and resource management.

| Component | Role | Language | Runs on |
|-----------|------|----------|---------|
| **coderag CLI** | Central CLI — MCP bridge, file watcher, project orchestrator | Go | Host (native binary) |
| **LightRAG** | RAG engine — entities/relations as a knowledge graph, LLM-powered queries | Python (FastAPI) | Docker/Podman container |
| **MCP Server** | MCP tool definitions, SSE transport | Python | Docker/Podman container |
| **WebUI** | React 19 UI for browsing the knowledge graph | TypeScript/React | Docker container (served by LightRAG) |
| **Ollama** | Local LLM backend for inference + embeddings | Go | Docker container or host |

### How it works

1. User runs `coderag mcp .` in their project directory (or opencode launches it)
2. `coderag` checks if LightRAG is healthy at `http://localhost:9621`:
   - **No** → runs `docker compose up -d`, waits for health
   - **Yes** → skips startup
3. `coderag` starts the Go file watcher on the project directory (goroutine)
4. `coderag` enters the MCP stdio loop, exposing 15 tools to opencode
5. Tool calls are proxied via SSE to the Python MCP server in Docker, which talks to LightRAG's HTTP API

---

## 2. Project Structure

```
code-rag/
├── cmd/
│   └── coderag/main.go          ← CLI entry point → builds `coderag` binary
├── pkg/
│   ├── watchdog/                ← File watcher (importable Go package)
│   │   ├── watcher/             ← fsnotify + git observer
│   │   ├── diff/                ← Entity/relation diff engine
│   │   ├── pipeline/            ← Pipeline orchestration
│   │   ├── parser/              ← Analyser output parsers (csharp.go)
│   │   ├── router/              ← File extension → analyser mapping
│   │   ├── filter/              ← Security filter (PII/keys/secrets)
│   │   └── crud/                ← LightRAG HTTP CRUD client
│   ├── lightrag/                ← LightRAG HTTP API client
│   └── mcp/                     ← MCP server SDK (mark3labs/mcp-go)
├── analysers/                   ← Language analysers (host-side, native Go)
│   └── csharp/                  ← Go native C# analyser
├── LightRAG/                    ← Python RAG engine (Docker)
│   ├── lightrag/                ← Core Python package
│   ├── lightrag-api/            ← FastAPI service
│   └── lightrag_webui/          ← React 19 + TypeScript UI
├── daniel-lightrag-mcp/         ← Python MCP server (Docker)
│   └── src/daniel_lightrag_mcp/ ← 22 MCP tools, SSE transport
├── Dockerfile                   ← Multi-stage: LightRAG + WebUI + MCP
├── docker-compose.yml           ← lightrag (:9621) + mcp (:9622)
├── go.mod
└── .opencode/
    └── opencode.json            ← MCP config: ["coderag", "mcp", "."]
```

### Package roles

| Package | Import path | Purpose |
|---------|-------------|---------|
| `pkg/watchdog` | `github.com/coderag/pkg/watchdog` | File watching, diff engine, pipeline, CRUD client |
| `pkg/lightrag` | `github.com/coderag/pkg/lightrag` | LightRAG HTTP client |
| `cmd/coderag` | — | CLI binary entry point (`start`, `mcp`, `scan`, `watch`, `status`, `stop`, `query`, `webui`) |
| `daniel-lightrag-mcp` | — | Python MCP server running in Docker (stdio↔SSE bridge) |

---

## 3. LightRAG

Forked from [HKUDS/LightRAG](https://github.com/HKUDS/LightRAG). Stores entities + relations in a graph DB and vector DB (embeddings).

**Data flow (document pipeline):**
```
POST /documents/text -> background enqueue -> text chunked -> LLM extracts entities+relations -> graph DB + vector DB
```

**Dedup:** LightRAG deduplicates by `file_source` in `doc_status`. If `file_source` already exists, returns `status="duplicated"`. No in-place update. See `document_routes.py:1824-1840`.

**Graph CRUD** bypasses the LLM pipeline — endpoints like `/graph/entity/create` and `/graph/relation/create` operate directly on the graph + vector DB. See [API_REFERENCE.md](API_REFERENCE.md#3-entity-crud-endpoints).

### Project-Based Storage

Each project folder contains a `.coderag/` directory:
```
project-root/
├── .coderag/
│   └── rag_storage/           # LightRAG data (graphml, vdb, kv stores)
├── .env                       # Optional project config (minimal)
└── src/                       # Your code
```

**Detection:** `LightRAG/lightrag/api/config.py:find_project_root()` searches from cwd upward for `.coderag` folder. Falls back to `./rag_storage` (legacy).

**Env var override:** `LIGHTRAG_WORKING_DIR` forces the working directory. Used by `coderag start` to point LightRAG at the correct `.coderag/rag_storage`.

---

## 4. coderag CLI

The `coderag` binary is the central CLI tool for the entire codeRAG system. It is a single native binary built on the user's machine (Windows, Linux, macOS).

### CLI Commands

```bash
coderag start   <dir>    →  Start Docker services + file watcher in foreground
coderag mcp     <dir>    →  MCP bridge mode (stdio, for opencode/agent connections)
                             Auto-starts Docker + watchdog in background.
                             Bridges stdio↔SSE to Python MCP in Docker.
                              Exposes 15 tools over MCP stdio protocol.
coderag scan    <dir>    →  Run C# analyser and populate knowledge graph
                             Use `--clean`/`-c` to clear graph + VDB first
coderag webui   <dir>    →  Open WebUI in browser (ensure Docker is running)
coderag watch   <dir>    →  File watcher only (no Docker orchestration)
coderag status           →  Health check all services
coderag stop             →  Stop all services (docker compose down)
coderag query   "<text>" →  Quick query against the knowledge graph
```

### `coderag mcp <dir>` — detailed flow

```
Step 0:  Parse flags, validate <dir> exists
Step 1:  Check MCP SSE health at http://localhost:9622/health
         │
         ├─ Healthy (HTTP 200) → skip to Step 3
         │
         └─ Not healthy → ensure LightRAG is running
              │
              └─ Check LightRAG at :9621, start Docker if needed

Step 2:  Print "LightRAG ready at http://localhost:9621"
Step 3:  Start file watcher on <dir> (goroutine, fsnotify + git poll)
         - Watchdog runs C# analyser on solution files
         - Diffs entities/relations against current graph
         - Applies security filter, creates/updates/deletes via LightRAG API
Step 4:  Enter MCP stdio loop (via SSE bridge):
         ├─ tools/list    → return 22 tool definitions
         ├─ tools/call    → proxy to LightRAG HTTP API or handle internally
         └─ notifications → handle lifecycle
```

### `coderag start <dir>` — detailed flow

```
Step 1: Same Docker/Podman health check as above
Step 2: Start file watcher in foreground
Step 3: Print status + WebUI URL
Step 4: Block until Ctrl+C
Step 5: On shutdown → stop watcher (Docker containers stay running)
```

### Supported containers

The Docker/Podman stack includes:
- **LightRAG API** — Python FastAPI server on `:9621` (includes WebUI)
- **MCP Server** — Python MCP tool server on `:9622` (SSE transport)
- **Ollama** (optional, on host or in container) — LLM backend on `:11434`

Containers are configured in `docker-compose.yml` at the project root.

---

## 5. MCP Bridge & Server

The MCP server runs in Docker (`daniel-lightrag-mcp` on port 9622, SSE transport) and the Go `coderag` binary on the host acts as a stdio↔SSE bridge between opencode and the Python MCP server.

```
opencode (stdio JSON-RPC) → coderag mcp (Go bridge) → SSE → Python MCP (Docker :9622) → HTTP → LightRAG API (Docker :9621)
```

### 15 MCP Tools

**Query (2 tools):**
| Tool | Description |
|------|-------------|
| `query` | LLM-powered natural language query (hybrid mode) |
| `query_data` | Structured query without LLM (entities + relationships) |

**Knowledge Graph & Traversal (8 tools):**
| Tool | Description |
|------|-------------|
| `get_knowledge_graph` | Retrieve full KG (nodes + edges); `max_depth` (1=types, 2=+members, 3=all) |
| `get_graph_labels` | Get all entity/relation label types |
| `check_entity_exists` | Check if entity exists by name |
| `add_note_to_entity` | Attach embedded note to entity |
| `get_related_files` | Find files related to a query (semantic search) |
| `get_entity_neighbors` | 1-hop neighbors (incoming + outgoing relations) |
| `get_incoming_relations` | Entities pointing TO this entity |
| `get_outgoing_relations` | Entities this entity points TO |

**Exploration (5 tools):**
| Tool | Description |
|------|-------------|
| `get_codebase_stats` | Entity/relation counts by type, unique files, visibility counts |
| `list_entities_by_type` | Paginated list of entities by type (CLASS, METHOD, NAMESPACE, etc.) |
| `get_entities_in_file` | Filter graph by source file path |
| `search_entities` | Regex search on entity names and descriptions |
| `get_entity_details` | Full entity info with all relations; `depth` (1=entity, 2=structural, 3=full) |

The Go bridge (`cmd/coderag/main.go`) handles:
1. Ensuring Docker services are up
2. Starting watchdog in background goroutine
3. Bridging stdio JSON-RPC ↔ SSE to Python MCP server

The Python MCP server (`daniel-lightrag-mcp`) handles:
- Tool dispatch via LightRAG HTTP client
- 15 tool definitions and handlers
- SSE transport on port 9622

---

## 6. Watchdog Pipeline (Go package)

The file watcher is a Go package at `pkg/watchdog/` imported by the `coderag` CLI.

```
pkg/watchdog/
├── watcher/observer.go         ← fsnotify + git watcher, worker pool, debounce
├── diff/engine.go              ← Entity/relation diff engine (create/update/delete)
├── filter/security.go          ← Security filter (PII/keys/secrets)
├── router/file.go              ← File extension → analyser mapping
├── parser/csharp.go            ← C# analyser output parser
├── pipeline/run.go             ← Pipeline orchestration + relation dedup
└── crud/client.go              ← Graph CRUD HTTP client
```

### Dual-path Design

The watchdog supports **two ingestion paths**:

**Path A — Direct graph CRUD (no LLM):**
```
File event -> debounce -> hash check ->
  router -> analyser (structured output) ->
  diff against current graph ->
  security filter ->
  create/update/delete entities+relations via API
```

**Path B — LLM pipeline via insert_text:**
```
File event -> debounce -> hash check ->
  POST /documents/text with file content ->
  LightRAG LLM pipeline (chunking, entity extraction, embedding)
```

**Routing:** Path A is attempted first. If the file has no registered analyser, falls back to Path B.

### Change Detection

Three complementary mechanisms:

**1. Initial sync (startup)** — git-based incremental:
- Walk filesystem, build MD5 hash map for all supported files
- `getGitChangedFiles()` — files that exist on disk AND differ from git HEAD (modified + created)
- `getGitDeletedFiles()` — files in git HEAD but removed from disk (deleted)
- Only changed files grouped by solution → analyser runs ONCE per affected solution → enqueued as "modified"
- Deleted files → `queueDeletionWithCascade()` (see below)
- If git unavailable → falls back to full scan (process everything)

**2. Git watcher (30s poll):** `git diff --name-only HEAD` + `git diff --cached --name-only`. Handles modified, created, and deleted files. Deleted files trigger cascade cleanup.

**3. FSNotify watcher (real-time, non-git mode):** Watches filesystem events directly. Create/Write → debounce + hash check → process. Remove/Rename → cascade cleanup.

### Cascade-Aware Deletion

When a file is deleted, simply removing its entities is insufficient — other files in the project may reference those entities. The cascade system handles this:

```
queueDeletionWithCascade(delPath):
  1. Remove file from hash map (future detection)
  2. Query LightRAG: GetEntitiesByFilePath(delPath)
  3. For each entity in the deleted file:
     a. Query Graph: GetAllRelations() → find relations where source or target matches
     b. From matching relations, find OTHER files (source_id) that reference those entities
  4. Queue "modified" events for affected files → triggers analyser + diff engine
  5. Queue "deleted" event → removes entities + relations from graph + vector DB
```

**Fallback when `relation.source_id` is broken:** Currently `Relation` parser lacks `SourceID`/`FilePath` fields, so all relations in LightRAG get `source_id` set to whatever file triggered the last scan. The cascade must use entity-based lookup as fallback: query `GetAllEntities()` for the related entity's `file_path` instead of reading it from the relation's properties.

### Analyser Scope

The C# analyser operates at **solution level** (requires `.sln` file). When any file in a solution changes, the entire solution is re-analysed. The diff engine then determines which entities/relations to create, update, or delete. This ensures cross-file relations (inheritance, interface implementations) are always consistent.

### Known Bugs

| Bug | Impact | Fix |
|-----|--------|-----|
| `parser.Relation` missing `SourceID`/`FilePath` | All relations stored with `source_id` = trigger file, not the actual source file | Add fields to struct + populate in `parserRelationsToMap` |
| `parserRelationsToMap` omits `source_id`/`file_path` from data map | `applyCSharpResults` falls back to caller's `filePath` for every relation | Include parsed fields in the data map |
| Cascade `findAffectedFiles` reads `rel.SourceID` | Gets wrong file paths since `source_id` is broken | Use entity-based lookup instead (query `GetAllEntities()` for related entity's `file_path`) |

### C# Entity Types Extracted

| Type | Example | Relation |
|------|---------|----------|
| NAMESPACE | `SnakeGame` | contains CLASS, INTERFACE, STRUCT, ENUM |
| CLASS | `Snake.Calculator` | defines METHOD, has PROPERTY |
| METHOD | `Snake.Calculator.Add()` | belongs_to CLASS |
| PROPERTY | `Snake.User.Email` | has CLASS |
| INTERFACE | `Snake.IRepository<T>` | implemented_by CLASS |
| STRUCT | `Snake.Point` | value type |
| ENUM | `Snake.Direction` | enumeration |
| INHERITS_FROM | AdvancedCalculator → Calculator | extends CLASS |
| DEPENDS_ON | Calculator → IOperation | implements INTERFACE |
| CALLS | MethodA → MethodB | cross-file call reference |

---

## 7. Security

| Category | Matches |
|----------|---------|
| API keys | `sk-...`, `pk-...`, `ghp_...`, `AKIA...` |
| Auth tokens | `Bearer ...`, JWT (`eyJ...`), `token=` |
| Passwords | `password=`, `passwd=`, `PASS=`, `secret=` |
| Private keys | `-----BEGIN ... PRIVATE KEY-----` |
| PII | Emails, IPv4/IPv6 addresses |
| Internal paths | `\\server\share`, `//internal/` |
| Env secrets | Values of vars named `*SECRET*`, `*TOKEN*` |

Runs **after** extraction, **before** graph insertion. Matched values -> `[REDACTED]`.

---

## 8. Test Infrastructure

| Suite | Location | Count |
|-------|----------|-------|
| Root | `tests/` | 74 |
| Test project | `../test-csharp-project/` (outside repo) | C# + git |

### Key test files

| File | Tests | What it covers |
|------|-------|----------------|
| `test_integration_lightrag.py` | 11 | Health, document, query endpoints |
| `test_integration_graph_vdb.py` | 11 | Entity/relation VDB embedding + `file_path` persistence |
| `test_smoke_lightrag.py` | 9 | Import/instantiation smoke tests |
| `test_pydantic_v2.py` | 4 | Pydantic v2 migration checks |
| `test_retry.py` | 3 | Document retry logic |
| `test_namespace.py` | 4 | Package imports and version |
| `test_config.py` | 4 | Configuration helpers |

### Vector DB test model
- Session-scoped mocked FastAPI server (`conftest.py`) replaces `ollama_model_complete` + `ollama_embed` with dummy functions
- Embedding mock returns `np.array([[0.1] * 1024 for _ in texts])` — constant 1024-dim vectors
- All API endpoints tested through `httpx.AsyncClient` via `ASGITransport`
- Entity/relation CRUD responses include `vector_data` (from `get_entity_info` / `get_relation_info` with `include_vector_data=True`)
- VDB state assertions: `file_path`, `entity_name`, `src_id`, `tgt_id` fields directly on response

### Gotchas
- No `tests/__init__.py` — session-scoped async fixture + package marker = deadlock
- `asyncio_default_test_loop_scope = session` in `pytest.ini` — prevents `asyncio.Lock` deadlock

---

## 9. Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `LIGHTRAG_URL` | `http://localhost:9621` | API address |
| `LIGHTRAG_PORT` | `9621` | Server port |
| `LIGHTRAG_WORKING_DIR` | auto-detect `.coderag/rag_storage` | Storage path |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama endpoint (the only supported LLM backend) |
| `INPUT_DIR` | positional arg | coderag monitor path |
| `DEBOUNCE_SECONDS` | `0.5` | Per-file debounce |
| `WATCHER_WORKERS` | `4` | Thread pool size |
| `DOCKER_HOST` | auto-detect | Docker or Podman socket |

---

## 10. Local-Only Design

This project is intentionally scoped to **local-only Ollama**:

- **LLM backend:** Only `OLLAMA_HOST` (default `http://localhost:11434`). No OpenAI, Azure, AWS Bedrock, etc.
- **Embedding backend:** Only Ollama (`bge-m3:latest` or similar). No cloud embedding APIs.
- **Auth:** `LIGHTRAG_API_KEY` is optional and unused in local mode. When unset, all API routes accept requests without headers (`utils_api.py:116`).
- **Why:** This is an experimental project for local codebase analysis. Cloud dependencies would add complexity, cost, and security surface for no benefit in this use case.

---

## 11. Deployment

### Docker Compose

```yaml
services:
  lightrag:
    build: .
    ports:
      - "9621:9621"
    volumes:
      - ./rag_storage:/app/rag_storage
    environment:
      - OLLAMA_HOST=http://ollama:11434

  ollama:  # optional, can use host Ollama instead
    image: ollama/ollama
    ports:
      - "11434:11434"
    volumes:
      - ollama:/root/.ollama

volumes:
  ollama:
```

### Podman Compatibility

The Docker Compose file is compatible with Podman. `coderag` auto-detects Podman when:
- `DOCKER_HOST=podman://...` is set, or
- Docker is not found but Podman is in `PATH`

`coderag mcp` and `coderag start` use:
```bash
docker compose -f <compose-file> up -d  # or podman-compose
```

### Building the binary

```bash
go build -o coderag ./cmd/coderag
```

The binary is self-contained (~20MB, no runtime dependencies) and works on Windows, Linux, and macOS.

---

## 12. Known Issues

| Issue | Status | Notes |
|-------|--------|-------|
| GraphML NoneType error | ✅ FIXED | None values filtered in `write_nx_graph()` and `upsert_node/edge()` |
| .env requirement | ✅ FIXED | `check_env_file()` always returns True (single-instance) |
| LIGHTRAG_WORKING_DIR not set | ✅ FIXED | coderag passes it in `start`/`mcp` commands |
| Entity `file_path` lost in VDB on update | ✅ FIXED | `aedit_entity()` / `aedit_relation()` now include `file_path` in VDB upsert |
| Git watcher detects uncommitted only | ⚠️ BY DESIGN | Polls `git diff HEAD` every 30s. New commits need restart. |
| Old entities not cleaned up on file change | ⚠️ PARTIAL | Modified files delete old entities first. No periodic cleanup. |
