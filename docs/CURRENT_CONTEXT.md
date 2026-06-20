# Current Context

## Status
- 🔥 **Phase 28** — Python Analyser 🔧 IN PROGRESS
- **Phase 27** — Verify All MCP Tools ✅ COMPLETE
- **Phase 26** — Fix Broken MCP Tools ✅ COMPLETE
- **Phase 25** — Architecture Cleanup & MCP Fixes ✅ COMPLETE
- **Phase 24** — MCP Tool Return Empty Results Bug Fix ✅ COMPLETE
- **Phase 23** — MCP Bug Fixes & Auto-Initialization ✅ COMPLETE
- **Phase 22** — MCP Tool Improvements ✅ COMPLETE
- **Phase 21** — coderag Restructuring ✅ COMPLETE
- **Phase 20** — VDB Embedding & file_path Persistence ✅ COMPLETE
- **Phase 19** — Incremental Update Pipeline Fixes ✅ COMPLETE

## Phase 21 — coderag Restructuring ✅ COMPLETE

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

### Phase 26 — Fix Broken MCP Tools ✅ COMPLETE

**Fixed:**
- Added `_safe_entity_data()` and `_extract_entity_name()` helper functions
- `get_entity_details`, `search_entities`, `get_entities_in_file`, `list_entities_by_type` - all use safe name extraction with fallback
- `list_entities_by_type` - fixed case-insensitive entity_type comparison
- `get_entity_neighbors` - fixed edge source/target comparison, safe node lookup
- `add_note_to_entity` - fixed success check to check both `created` flag AND message

### Phase 27 — Verify All MCP Tools ✅ COMPLETE

**Verified all 13 MCP tools work:**
- Docker rebuilt with fixed Python MCP
- Graph populated via `coderag scan`: 43 entities, 46 relations
- All 6 previously broken tools now working
- `list_entities_by_type("CLASS")` → 8 entities
- `get_entity_details("Calculator")` → entity + 16 relations
- `search_entities("Calc")` → 10 matches
- `get_entities_in_file` → works with stored paths
- `get_entity_neighbors` → 16 neighbors for Calculator
- `add_note_to_entity` → success status

### Phase 25 — Architecture Cleanup & MCP Fixes ✅ COMPLETE

**Completed:**
- C# analyser moved from `daniel-lightrag-mcp/analysers/` to `analysers/c_sharp/`
- `coderag scan` command added to `cmd/coderag/main.go`
- Watchdog starts in background on `coderag mcp` (goroutine)
- `update_relations` deprecated in MCP client
- Old Python watchdog code deleted (`watchdog/` dir)
- Analyzer runner updated for new path

### Phase 24 — MCP Tool Return Empty Results Bug Fix ✅ COMPLETE

**Bug report:** `get_codebase_stats`, `get_graph_labels`, `list_entities_by_type` returned empty/zero results despite knowledge graph being fully populated (42 nodes, 33 edges in test-csharp-project).

**Root cause:** Two issues:
1. **LightRAG `get_all_labels()` returned node IDs (entity names like "Calculator") instead of entity_type values ("CLASS", "METHOD").** The MCP client used these labels incorrectly with `get_knowledge_graph(label="CLASS")` which does BFS from a node NAMED "CLASS" — not filtering for nodes with `entity_type="CLASS"`.

2. **`/graph/label/list` returned flat list** of node IDs instead of structured `{entity_labels: [...], relation_labels: [...]}`.

**Fixes implemented:**
- `networkx_impl.py`: Added `get_all_entity_types()` and `get_all_relation_types()` methods that extract distinct type values from node/edge properties
- `lightrag.py`: `get_graph_labels()` now returns `{entity_labels, relation_labels}` using the new type extraction methods, preserving backward compatibility of `get_all_labels()`
- `client.py`: Fixed `get_codebase_stats`, `list_entities_by_type`, `search_entities`, `get_entities_in_file` to use `label="*"` and filter client-side by `properties.entity_type`

**API change:**
- `/graph/label/list` now returns `{"entity_labels": ["CLASS", "METHOD", ...], "relation_labels": ["method", "defines", ...]}` instead of flat list of node IDs

### Phase 23 — MCP Bug Fixes & Auto-Initialization ✅ COMPLETE

**Agent test (June 2026) revealed critical bugs:**
- **`get_entities_in_file` broken:** `file_path` stored at node level in LightRAG, not inside `properties`. Client looked in wrong place.
- **`search_entities` broken:** Uses `label="*"` which is invalid at `/graphs`. Returns empty or validation error.
- **Empty graph on startup:** MCP starts without running analyser. `get_codebase_stats` shows 0 entities.
- **`query` unreliable:** Falls back to LLM training data when graph empty → generic/hallucinated answers.

**Fixes implemented:**
- `get_entities_in_file`: Fixed to iterate all labels from `get_graph_labels()`, lookup `node.get("file_path")` at node level, handle `properties=None`
- `search_entities`: Fixed to use real labels from `get_graph_labels()`, skip invalid labels, respect limit
- Auto-initialization: `server.py` now runs `_auto_initialize()` before starting MCP — detects `.sln`/`.csproj` in cwd, runs C# analyser via `update_relations()` to populate graph

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
coderag mcp <project-dir>
  → ensureLightRAG()  (start Docker, wait for :9621 health)
  → start watchdog    (goroutine, background, file watcher)
  → runSSEBridge()    (blocks, bridging stdio↔SSE)

opencode ──stdio──► coderag mcp (Go bridge)
                          │
                    SSE bridge ──HTTP──► mcp (Docker :9622)
                                           │
                                      talks to lightrag (Docker :9621)
                                                    ↑
                                               WebUI served internally
```

### Services (all running)
| Service | Type | Port | Status |
|---------|------|------|--------|
| `lightrag` (API + WebUI) | Docker | 9621 | ✅ Healthy, serves WebUI |
| `mcp` (MCP SSE) | Docker | 9622 | ✅ 13 tools, SSE handshake |

### coderag CLI
- `cmd/coderag/main.go` — CLI entry with `start`, `mcp`, `scan`, `watch`, `status`, `stop`, `query`, `webui`
- `coderag mcp <dir>` — stdio↔SSE bridge + starts watchdog in background
- `coderag scan <dir>` — run C# analyser and populate knowledge graph
- `go build ./cmd/coderag` succeeds, `go vet ./...` passes
- Root `go.mod` module: `github.com/code-lightrag/coderag`, Go 1.25.5

### Go watchdog (pkg/watchdog/)
- `pkg/watchdog/watcher/` — fsnotify + git observer, worker pool, debounce
- `pkg/watchdog/pipeline/` — orchestrator: analyser → diff → security filter → CRUD
- `pkg/watchdog/crud/` — LightRAG HTTP client for graph operations
- `pkg/watchdog/diff/` — entity/relation diff engine (create/update/delete)
- `pkg/watchdog/parser/` — C# analyser output parser
- `pkg/watchdog/router/` — file extension → analyser mapping
- `pkg/watchdog/filter/` — security filter (PII/keys/secrets)
- Initial sync on startup — full scan, solution-level analysis, diff-based CRUD
- Git-based change detection (30s poll) — `git diff --name-only HEAD`
- File hash tracking (MD5) — skips unchanged content
- C# analyser: extracts CLASS, METHOD, PROPERTY, INHERITS_FROM, DEPENDS_ON, CALLS

### C# Analyser
- `analysers/c_sharp/CSharpAnalyzerTool.exe` — Roslyn-based analyser binary
- Runs on solution files (`.sln`), outputs entity/relation data to `relations_output/`
- Entity types: CLASS, METHOD, PROPERTY, INTERFACE
- Relation types: INHERITS_FROM, DEPENDS_ON, CALLS, IMPLEMENTS

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
4. **`coderag mcp` starts watchdog in background** — watchdog runs as goroutine in the Go bridge process, watching the project directory for changes
5. **Docker-first, no fallback** — LightRAG + WebUI + MCP always in containers
6. **No Ollama in Docker** — assumed on host at `http://localhost:11434`
7. **`--workers 1` required** — Default 4 causes `dial tcp` errors
8. **30s git watcher poll** — For debouncing
9. **Project isolation** — `.coderag/rag_storage` per project
10. **Entity-level CRUD > document pipeline** — No LLM calls for analyser output
11. **Security filter built into pipeline** — Secret data never reaches graph
12. **C# analyser at project root** — `analysers/c_sharp/CSharpAnalyzerTool.exe` (moved from `daniel-lightrag-mcp/analysers/`)
13. **`update_relations` deprecated** — MCP client's `update_relations()` is deprecated (kept for future use). Analysis is now handled by Go watchdog via `coderag scan` or automatic on file change.
14. **`_auto_initialize()` removed** — MCP server no longer auto-runs analyser on startup. Watchdog handles initialization via background goroutine.

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

### LightRAG file_path storage location
LightRAG stores `file_path` at the **node level** (`node["file_path"]`), NOT inside `node["properties"]["file_path"]`. When filtering entities by file path, access `node.get("file_path")` directly. Also guard against `properties=None` — use `props = node.get("properties") or {}`.

### LightRAG /graphs requires valid label
The `/graphs` endpoint requires a valid entity type label (e.g., "CLASS", "METHOD"). Using `label="*"` is invalid and returns an error. When fetching all entities, use the first label from `get_graph_labels()` or iterate through available labels.

### LightRAG entity type vs node label distinction
**Critical bug:** `get_knowledge_graph(label="CLASS")` does BFS starting from a node NAMED "CLASS" — it does NOT filter for nodes with `properties.entity_type="CLASS"`. These are different things:
- **Node ID/name:** The identifier for a node (e.g., "Calculator", "Add", "Factorial")
- **entity_type:** A property stored in `node["properties"]["entity_type"]` (e.g., "CLASS", "METHOD", "PROPERTY")

When counting/querying entities by type, fetch all nodes with `label="*"` then filter client-side by `node["properties"]["entity_type"]`.

### LightRAG get_all_labels() returns node IDs, not entity types
The original `get_all_labels()` method returned all node IDs (entity names), not distinct `entity_type` values. New methods `get_all_entity_types()` and `get_all_relation_types()` were added to extract the actual type labels from graph properties.

### LightRAG edge field name inconsistency
**Critical schema mismatch:** LightRAG's `networkx_impl.py` returns edges with `source`/`target` field names, but MCP client code expected `src_id`/`tgt_id`. When implementing `get_entity_neighbors` and `get_entity_details`, must handle both naming conventions:
- LightRAG Python API: `source`, `target`
- Go CRUD client: `src_id`, `tgt_id` (internally converted to `source`/`target` when calling LightRAG)

Always use `.get("source") or .get("src_id")` and `.get("target") or .get("tgt_id")` for compatibility.
