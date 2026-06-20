# Current Context

## Status
- 🔥 **Phase 22** — MCP Tool Improvements 🔧 COMPLETE
- **Phase 21** — coderag Restructuring ✅ COMPLETE
- **Phase 20** — VDB Embedding & file_path Persistence ✅ COMPLETE
- **Phase 19** — Incremental Update Pipeline Fixes ✅ COMPLETE

## Phase 21 — coderag Restructuring 🔧 IN PROGRESS

### Architecture Decision (June 2026)

**Goal:** `coderag` is the central CLI tool for the CodeLightRAG system.

**Key architecture decisions made so far:**
1. **`coderag` is the CLI binary** — replaces `watchdog` as main entry point
2. **Python MCP kept, runs in Docker** — `daniel-lightrag-mcp` stays Python, runs as separate Docker service with SSE transport on port 9622
3. **Everything modular in Go on host** — `pkg/watchdog/`, `pkg/lightrag/` are importable packages
4. **Docker-first** — LightRAG + WebUI + MCP run in containers, never as host subprocess
5. **WebUI served from LightRAG API** — static files built into Docker image
6. **Analysers at root** — `analysers/csharp/` standalone directory
7. **MCP and watchdog are independent** — both talk only to LightRAG API, never to each other

### Architecture
```
Host side:                        Docker:
  coderag (Go CLI)                  ├── lightrag (API + WebUI :9621)
    ├── pkg/watchdog/                └── mcp (Python MCP SSE :9622)
    ├── pkg/lightrag/ (HTTP client)
    └── cmd/coderag/main.go
```

### Implementation status

| Step | Status |
|------|--------|
| SSE transport added to Python MCP server | ✅ Done |
| MCP runs in Docker (port 9622) | ✅ Done |
| WebUI built into Docker image | ✅ Done |
| `cmd/coderag/main.go` CLI (start, watch, status, stop, query, webui) | ✅ Done |
| `.opencode/opencode.json` remote MCP config | ✅ Done |
| `pkg/mcp/` removed (stale Go dir) | ✅ Done |
| Move `watchdog/internal/*` → `pkg/watchdog/` | ⏳ Pending |
| Create `pkg/lightrag/` HTTP client | ⏳ Pending |
| Thin standalone `watchdog` entry point | ⏳ Pending |
| Move `analysers/` to root | ⏳ Pending |

### Key CLI behavior

```
coderag start <dir>   → docker compose up -d + file watcher
coderag watch <dir>   → file watcher only (Docker must be running)
coderag status        → check LightRAG health + watcher status
coderag stop          → docker compose down
coderag query "<txt>" → POST /query to LightRAG
coderag webui         → open http://localhost:9621
```

---

## Previous Phase Summary

### Phase 22 — MCP Tool Improvements ✅ COMPLETE

- **Reduced MCP tools from 22 to 6** (Phase 21 tool reduction)
- **Expanded from 6 to 13 tools** for large codebase exploration
- **Bug fix:** `get_related_files` was calling wrong endpoint (`/query`) — fixed to use `/query/data` which returns structured entities and references
- **8 new tools added:** `query_data`, `get_codebase_stats`, `get_entity_neighbors`, `get_entity_details`, `list_entities_by_type`, `get_entities_in_file`, `search_entities`, and the fixed `get_related_files`
- **New exploration capabilities:** graph traversal (neighbors), entity browsing (by type/file), pattern search, codebase statistics

### Phase 21 — coderag Restructuring ✅ COMPLETE

- Docker multi-stage build (WebUI + LightRAG + MCP)
- Fixed Starlette 1.3.1 ASGI detection bug (`_ASGIApp` wrapper in `sse_server.py`)
- Added `coderag mcp <dir>` stdio↔SSE bridge for opencode integration
- MCP SSE handshake verified
- opencode config updated to use `type: "local"`

### Phase 20 — VDB Embedding & file_path Persistence ✅ COMPLETE

- `aedit_entity()` and `aedit_relation()` now include `file_path` in VDB upsert
- Previously, ActionUpdate path overwrote VDB entries without `file_path`
- 11 new integration tests added
- All 74 root tests pass

### Phase 19 — Incremental Update Pipeline Fixes ✅ COMPLETE

- `RelationKey` sorts src/tgt to match LightRAG's undirected edge storage
- CLASS entity update filter narrowed (only `description`/`methods`)
- ParseCSharpFile rewritten with per-class accumulator
- 42 entities, 45 relations, zero errors

### Phase 18 — Project-Based Storage ✅ COMPLETE

- `.coderag/rag_storage` per project, auto-detected via `find_project_root()`
- GraphML NoneType filter in `networkx_impl.py`
- No `.env` file required

---

## What Exists

### Running system
```
opencode ──MCP SSE──► mcp (Docker :9622)     watchdog (host Go binary)
                          │                           │
                          └───── HTTP ──────► lightrag (Docker :9621)
                                                    ▲
                                               WebUI served internally
```

### Services (all running)
| Service | Type | Port | Status |
|---------|------|------|--------|
| `lightrag` (API + WebUI) | Docker | 9621 | ✅ Healthy, serves WebUI |
| `mcp` (MCP SSE) | Docker | 9622 | ✅ 13 tools, SSE handshake |

### coderag CLI
- `cmd/coderag/main.go` — CLI entry with `start`, `watch`, `status`, `stop`, `query`, `webui`
- No `mcp` subcommand (MCP is always in Docker now)
- `go build ./cmd/coderag` succeeds, `go vet ./...` passes
- Root `go.mod` module: `github.com/code-lightrag/coderag`, Go 1.25.5

### Go watchdog
- `watchdog/cmd/watchdog/main.go` — standalone entry (thin wrapper around `pkg/watchdog/`)
- `watchdog/internal/` — watcher, diff, pipeline, parser, router, filter, crud
- Initial sync on startup — full scan, solution-level analysis, diff-based CRUD
- Git-based change detection (30s poll) — `git diff --name-only HEAD`
- File hash tracking (MD5) — skips unchanged content
- C# analyser: extracts CLASS, METHOD, PROPERTY, INHERITS_FROM, DEPENDS_ON, CALLS

### Python MCP server (in Docker)
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/` — 13 MCP tools in Python
- **Core tools (6):** `query`, `get_related_files`, `check_entity_exists`, `get_knowledge_graph`, `get_graph_labels`, `add_note_to_entity`
- **New exploration tools (7):** `query_data`, `get_codebase_stats`, `get_entity_neighbors`, `get_entity_details`, `list_entities_by_type`, `get_entities_in_file`, `search_entities`
- SSE transport on port 9622 using Starlette + uvicorn
- `sse_server.py` — SSE entry point with `_ASGIApp` wrapper (fixes Starlette 1.3.1 ASGI detection)
- `server.py` — main MCP server with 13 tool handlers
- `client.py` — LightRAG HTTP client with all new methods
- **Bug fix:** `get_related_files` now correctly uses `/query/data` endpoint (was using `/query` which returned LLM text instead of structured results)

### Docker setup
- `Dockerfile` — multi-stage: WebUI (bun build) → LightRAG + MCP (pip install)
- `docker-compose.yml` — two services (`lightrag` :9621, `mcp` :9622), no Ollama
- Uses `CMD` (not `ENTRYPOINT`) so docker-compose `command` overrides correctly
- MCP service connects to `lightrag:9621` internally via Docker DNS

---

## Key Technical Decisions

1. **Python MCP kept, Docker-only** — No Go rewrite. `daniel-lightrag-mcp` runs as separate Docker service. SSE transport on port 9622. 13 tools for AI agent codebase exploration (query, query_data, get_related_files, check_entity_exists, get_knowledge_graph, get_graph_labels, add_note_to_entity, get_codebase_stats, get_entity_neighbors, get_entity_details, list_entities_by_type, get_entities_in_file, search_entities).
2. **Docker multi-service** — `lightrag` and `mcp` share same image but different `command`
3. **`CMD` not `ENTRYPOINT`** — Allows command override in docker-compose
4. **MCP and watchdog independent** — No communication between them. Both talk to LightRAG API only.
5. **Docker-first, no fallback** — LightRAG + WebUI + MCP always in containers
6. **No Ollama in Docker** — assumed on host at `http://localhost:11434`
7. **`--workers 1` required** — Default 4 causes `dial tcp` errors
8. **30s git watcher poll** — For debouncing
9. **Project isolation** — `.coderag/rag_storage` per project
10. **Entity-level CRUD > document pipeline** — No LLM calls for analyser output
11. **Security filter built into pipeline** — Secret data never reaches graph

## Learnings

### Starlette Route + ASGI detection
Starlette 1.3.1's `Route.__init__` treats any function/staticmethod as a request handler (wrapped with `request_response()`). Raw ASGI functions must be wrapped in a class to be dispatched correctly. Fix: `_ASGIApp` wrapper class in `sse_server.py`.

### Event loop trap
Session-scoped async fixtures + function-scoped test event loops = deadlock. Fix: `asyncio_default_test_loop_scope = session`.

### LightRAG dedup
`POST /documents/text` with existing `file_source` returns `status="duplicated"` and does nothing.

### GraphML NoneType
NetworkX's GraphML writer rejects `None` attribute values. Must filter before every write.

### Undirected graph relation key sorting
LightRAG stores edges with sorted `source`/`target`. Diff engine `RelationKey()` must sort both sides.

### CLASS entity update skip is too broad
Skip only `description`/`methods`, not the entire entity. `file_path` needs to update.

### VDB upsert replaces entirely
NanoVectorDB `upsert()` replaces the full entry. Every field must be included in every upsert, not just changed fields.

### LightRAG /query vs /query/data endpoints
LightRAG has two query endpoints: `POST /query` returns LLM-generated text + references (file paths). `POST /query/data` returns structured data: entities, relationships, chunks, references. Code expecting `results` field (from naive/local/global query modes) will get None when calling `/query` directly. Use `/query/data` for structured programmatic access.
