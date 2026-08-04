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
├── coderag/                      ← All owned code
│   ├── cli/                      ← Go CLI (orchestrator)
│   │   ├── main.go               ← Entry point → builds `coderag` binary
│   │   ├── watcher/              ← fsnotify + git observer
│   │   ├── diff/                 ← Entity/relation diff engine
│   │   ├── pipeline/             ← Pipeline orchestration
│   │   ├── parser/               ← Analyser output parsers (csharp.go)
│   │   ├── router/               ← File extension → analyser mapping
│   │   ├── filter/               ← Security filter (PII/keys/secrets)
│   │   ├── crud/                 ← LightRAG HTTP CRUD client
│   │   └── analysers/csharp/     ← Go native C# analyser
│   ├── lightrag/                 ← Python RAG engine (Docker)
│   │   ├── lightrag/             ← Core Python package
│   │   ├── lightrag-api/         ← FastAPI service
│   │   └── lightrag_webui/       ← React 19 + TypeScript UI
│   ├── mcp/                      ← Python MCP server (Docker)
│   │   └── src/coderag_mcp/      ← MCP tools, SSE transport
│   └── tests/                    ← Integration tests
├── scripts/                      ← Setup & build scripts
├── Dockerfile                    ← Multi-stage: LightRAG + WebUI + MCP
├── docker-compose.yml            ← lightrag (:9621) + mcp (:9622)
└── _dev_docs/                    ← Private working docs (gitignored)
```

### Package roles

| Package | Import path | Purpose |
|---------|-------------|---------|
| `coderag/cli` | `github.com/coderag` | CLI binary (`start`, `mcp`, `scan`, `watch`, `status`, `stop`, `query`, `webui`) + watcher, diff engine, pipeline, CRUD client |
| `coderag/cli/analysers/csharp` | `github.com/coderag/analysers/csharp` | Go-native C# analyser (standalone binary, subprocess) |
| `coderag/mcp` | `coderag_mcp` (installed as `coderag-mcp`) | Python MCP server running in Docker (stdio↔SSE bridge) |

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

**Detection:** `coderag/lightrag/lightrag/api/config.py:find_project_root()` searches from cwd upward for `.coderag` folder. Falls back to `./rag_storage` (legacy).

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

The MCP server runs in Docker (`coderag-mcp` on port 9622, SSE transport) and the Go `coderag` binary on the host acts as a stdio↔SSE bridge between opencode and the Python MCP server.

```
opencode (stdio JSON-RPC) → coderag mcp (Go bridge) → SSE → Python MCP (Docker :9622) → HTTP → LightRAG API (Docker :9621)
```

### 17 MCP Tools

**Query (2 tools):**
| Tool | Description | Status |
|------|-------------|--------|
| `query` | LLM-powered natural language query (hybrid mode) | ⚠️ POST `/query` times out — Ollama connectivity issue |
| `query_data` | Structured query without LLM (entities + relationships) | ⚠️ POST `/query/data` times out — Ollama connectivity issue |

**Knowledge Graph & Traversal (8 tools):**
| Tool | Description | Status |
|------|-------------|--------|
| `get_knowledge_graph` | Retrieve full KG (nodes + edges); `max_depth` (1=types, 2=+members, 3=all) | ✅ Working |
| ~~`get_graph_labels`~~ | ~~Get all entity/relation label types~~ | ❌ Dropped (use `get_nodes` / `node_details`) |
| `check_entity_exists` | Check if entity exists by name | ✅ Working |
| `add_note_to_entity` | Attach embedded note to entity | ⚠️ Creates note but reports failure |
| `get_related_files` | Find files related to a query (semantic search) | ⚠️ Returns file paths only — no snippets |
| `node_details` | Unified node lookup; `view`: "details" / "relations" / "neighbors" | ✅ Working (merged 3 tools) |
| ~~`get_entity_neighbors`~~ | ~~1-hop neighbors~~ | ❌ Merged into `node_details(view="neighbors")` |
| ~~`get_entity_details`~~ | ~~Full entity info with relations~~ | ❌ Merged into `node_details(view="details")` |
| ~~`get_relation_detail`~~ | ~~All relations of a specific type~~ | ❌ Merged into `node_details(view="relations", relation_type=...)` |
| ~~`get_method_signature`~~ | ~~Method signature~~ | ❌ Dropped; lives in description field |
| `find_related_files` | Find all files that reference/relate to a given entity | ✅ Added |
| ~~`get_interface_methods`~~ | ~~Interface methods + implementing classes~~ | ❌ Dropped (use `node_details`) |

**Exploration (7 tools):**
| Tool | Description | Status |
|------|-------------|--------|
| `get_codebase_stats` | Entity/relation counts by type, unique files, visibility counts | ❌ HTTP 500 — internal split error |
| `list_entities_by_type` | Paginated list of entities by type (CLASS, METHOD, etc.) | ❌ Server-side filter broken — always empty |
| ~~`get_entities_in_file`~~ | ~~Filter graph by source file path~~ | ❌ Dropped (use `get_nodes(file_path=...)` — substring match, no Windows path issue) |
| `search_entities` | Regex search on entity names and descriptions | ✅ Working |
| `get_incoming_relations` | Entities pointing TO this entity | ✅ (via get_entity_neighbors) |
| `get_outgoing_relations` | Entities this entity points TO | ✅ (via get_entity_neighbors) |

### MCP Tool Effectiveness (Tested on test-csharp-project, July 2026)

| Tool | Rating | Notes |
|------|--------|-------|
| `check_entity_exists` | Very Useful | Fast, reliable, boolean responses |
| `get_knowledge_graph` | Very Useful | Returns full graph structure with nodes+edges |
| ~~`get_graph_labels`~~ | ~~Useful~~ | ❌ Dropped (use `get_nodes` / `node_details`) |
| `node_details` | Very Useful | Unified view: details, relations, neighbors (via `view` param) |
| `search_entities` | Useful | Regex pattern matching on names/descriptions |
| `query` (GET) | Moderately Useful | Natural language queries work; POST times out |
| `query_data` | Moderately Useful | Structured query without LLM |
| `get_related_files` | Limited | Returns file paths but no code snippets |
| `get_codebase_stats` | Broken | HTTP 500 internal error |
| `list_entities_by_type` | Broken | Server-side entity_type filter returns empty |
| ~~`get_entities_in_file`~~ | ~~Limited~~ | ❌ Dropped (use `get_nodes(file_path=...)`) |
| `add_note_to_entity` | Limited | Creates notes but reports false error |

### External Agent Quick Reference — Calling MCP Tools Directly

**Do NOT connect via MCP SSE protocol (port 9622) from external agents.** Use the HTTP REST API directly at `http://localhost:9621`. The MCP tools are Python wrappers around these HTTP endpoints:

| MCP Tool | HTTP Endpoint | Method |
|---|---|---|
| `check_entity_exists` | `/graph/entity/exists?name=XYZ` | GET |
| `get_knowledge_graph` | `/graphs?label=*&max_depth=3&max_nodes=1000` | GET |
| ~~`get_graph_labels`~~ | ~~`/graph/label/list`~~ | ❌ Dropped |
| `node_details` | `/graphs?label=*&max_depth=3&max_nodes=1000` + client-side filter (`view` param) | GET |
| `query` | `/query` | POST |
| `query_data` | `/query/data` | POST |
| `get_codebase_stats` | `/graph/label/list` + `/graphs?label=*&max_depth=3` | GET + GET |
| `list_entities_by_type` | `/graphs?label=*&max_depth=3&max_nodes=1000` + filter | GET |
| `search_entities` | `/graphs?label=*&max_depth=3&max_nodes=1000` + regex filter | GET |
| ~~`get_entities_in_file`~~ | ~~`/graphs?label=*&max_depth=3&max_nodes=1000` + filter by `file_path`~~ | ❌ Dropped |
| `get_related_files` | `/query/data` | POST |

### MCP Server Architecture

The Go bridge (`coderag/cli/main.go`) handles:
1. Ensuring Docker services are up
2. Starting watchdog in background goroutine
3. Bridging stdio JSON-RPC ↔ SSE to Python MCP server

The Python MCP server (`coderag/mcp`) handles:
- Tool dispatch via LightRAG HTTP client
- 17 tool definitions and handlers (was 15, now 17 with the 4 Phase 32 additions)
- SSE transport on port 9622

**Note on tool count:** The README and CLI docs still reference "15 MCP tools" and "22 MCP tools" — the actual count is **17** (13 original + 4 newly implemented Phase 32 tools). The README should be updated accordingly.

---

## 6. Watchdog Pipeline (Go package)

The file watcher is a Go package at `coderag/cli/` (module `github.com/coderag`) imported by the `coderag` CLI.

```
coderag/cli/
├── watcher/observer.go          ← fsnotify + git watcher, worker pool, debounce
├── diff/engine.go               ← Entity/relation diff engine (create/update/delete)
├── filter/security.go           ← Security filter (PII/keys/secrets)
├── router/file.go               ← File extension → analyser mapping
├── parser/csharp.go             ← C# analyser output parser
├── pipeline/run.go              ← Pipeline orchestration + relation dedup
└── crud/client.go               ← Graph CRUD HTTP client
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
go build -C coderag/cli -o coderag .
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
