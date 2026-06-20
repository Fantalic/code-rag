# CodeLightRAG — Where We're Going

## The Big Idea

A **live, minimal-work knowledge graph** for codebases. When you save a file, only the affected entities and relations update — no full rebuilds, no wasted tokens, no secrets leaked.

`coderag` is the single CLI to rule them all. One native binary on the host, Docker/Podman for persistent services, opencode for AI agent access.

---

## What We Have ✅

- Monorepo: LightRAG + WebUI + watchdog + analysers working together
- 22 MCP tools defined (currently in Python `daniel-lightrag-mcp`)
- C# analyser (Roslyn-based, compiled binary) — extracts classes, methods, inheritance, deps
- Go file watcher with debouncing + hash tracking + git poll
- Graph CRUD client, diff engine, security filter, pipeline runner (all in Go)
- Project-based storage (`.coderag/rag_storage`)
- VDB `file_path` persistence fix — entity/relation updates preserve file_path in vector DB
- 74 passing tests

## Phase 11 — Efficient Incremental Updates ✅

All 7 steps done (Python prototype):
- Graph CRUD client (`watchdog/graph_crud.py`)
- Diff engine (`watchdog/graph_diff.py`)
- Security filter (`watchdog/security_filter.py`)
- File router (`watchdog/file_router.py`)
- C# analyser parser (`watchdog/analyser_parsers.py`)
- Pipeline wired in `watchdog/main.py`

## Phase 12 — Go Rewrite ✅

Go watchdog (standalone CLI binary) replacing the Python prototype.

## Phase 13 — Concurrent Analyser Race Fix ✅

Solution-level mutex, undirected graph direction fix, duplicate relation dedup.

## Phase 14 — Testing, MCP Tool Usage & Test Expansion ✅

Expanded test-csharp-project with inheritance, interfaces, properties. 168 MCP tests.

## Phase 15 — End-to-End System Testing ✅

Live system test: 50 nodes, 47 edges, queries work. Found entity deletion not working.

## Phase 16 — Fix Entity Deletion & File Change Handling ✅

- Entity deletion confirmed working with JSON body
- `DeleteEntitiesByFilePath()` added to Go watchdog
- Modified files refresh entities correctly

## Phase 17 — Relation Creation Fix ✅

- Router filters non-C# files
- Git watcher path cleaning
- `loadCurrentRelations()` fix for undirected graph edges
- CLASS entity update skip narrowed (only `description`/`methods`)

## Phase 18 — Project-Based Storage & GraphML Fix ✅

- `.coderag/rag_storage` per project, auto-detected
- GraphML NoneType fix
- No `.env` requirement

## Phase 19 — Incremental Update Pipeline Fixes ✅

- `RelationKey` sorts src/tgt
- ParseCSharpFile rewritten (per-class accumulator)
- Modified pipeline re-runs analyser when results empty

## Phase 20 — VDB file_path Persistence ✅

- `aedit_entity()` and `aedit_relation()` now include `file_path` in VDB upsert
- 11 new integration tests
- All 74 root tests pass

---

## Phase 21 — coderag Restructuring ✅ COMPLETE

✅ All 12 steps completed (see TASK.md for details)
✅ Docker multi-stage build (WebUI + LightRAG + MCP)
✅ Fixed Starlette 1.3.1 ASGI detection bug (`_ASGIApp` wrapper)
✅ Added `coderag mcp <dir>` stdio↔SSE bridge for opencode
✅ MCP SSE handshake verified
✅ opencode config updated to use `type: "local"`

## Phase 22 — MCP Tool Improvements for Large Codebases 🔧 IN PROGRESS

### Goal

Restructure the monorepo so `coderag` is the central CLI. Everything becomes modular Go packages. The Python MCP server is replaced by Go MCP built into the `coderag` binary.

### Architecture Decision

| Before | After |
|--------|-------|
| `watchdog/` = CLI + watcher | `cmd/coderag/` = CLI, `pkg/watchdog/` = importable watcher |
| `daniel-lightrag-mcp/` (Python MCP) | `pkg/mcp/` (Go MCP, mark3labs/mcp-go) |
| MCP + CRUD logic in Python | All Go, single binary on host |
| Docker hosts everything | Docker hosts LightRAG + WebUI only |
| Python MCP started from watchdog `start` | `coderag mcp` is the MCP server itself |

### Steps

| # | Step | What changes |
|---|------|-------------|
| 1 | Restructure `watchdog/internal/*` → `pkg/watchdog/` | Move watcher, diff, pipeline, parser, router, filter, crud to `pkg/watchdog/` with shared `go.mod` |
| 2 | Create `pkg/lightrag/` HTTP client | Extract from `pkg/watchdog/crud/` — shared between MCP and watchdog |
| 3 | Create `pkg/mcp/` with mark3labs/mcp-go | Go MCP server: 22 tools, stdio transport |
| 4 | Create `cmd/coderag/main.go` | CLI with `start/mcp/watch/status/stop/query/webui` subcommands |
| 5 | Wire `coderag mcp` → Docker + watcher + MCP | Auto-start Docker compose, wait for health, start watcher goroutine, enter stdio loop |
| 6 | Wire `coderag start` → Docker + watcher | Same Docker logic, foreground watcher, Ctrl+C shutdown |
| 7 | Wire `coderag webui` → open browser | Check Docker, print URL |
| 8 | Standalone `watchdog` entry point | Thin wrapper at `watchdog/cmd/watchdog/main.go` importing `pkg/watchdog` |
| 9 | Remove `daniel-lightrag-mcp/` | All functionality replaced by `pkg/mcp/` |
| 10 | Move `analysers/` to root | From `daniel-lightrag-mcp/analysers/` to `analysers/` |
| 11 | Update go.mod, Dockerfile, docker-compose.yml | Single Go module, Docker image has LightRAG + WebUI only |
| 12 | Update .opencode/opencode.json | MCP points to `["coderag", "mcp", "."]` |

### Package Dependency

```
cmd/coderag/
├── pkg/mcp/          (mark3labs/mcp-go)
│   └── pkg/lightrag/ (HTTP client)
│       └── (net/http)
├── pkg/watchdog/
│   ├── watcher/      (fsnotify)
│   ├── diff/
│   ├── pipeline/
│   ├── parser/
│   ├── router/
│   ├── filter/
│   └── crud/ → pkg/lightrag/ (or replaced by it)
│
watchdog/cmd/watchdog/
  └── pkg/watchdog/
```

### Key CLI behavior

```
coderag mcp .
  → docker compose up -d (if LightRAG not healthy)
  → wait for :9621
  → start file watcher on .
  → enter MCP stdio loop

coderag start .
  → same Docker check
  → start file watcher in foreground
  → block until Ctrl+C

coderag webui
  → open http://localhost:9621 in browser

coderag status
  → check LightRAG health + watcher status
```

### Docker + Podman

- `coderag mcp` and `coderag start` first try Docker, fall back to Podman
- Detection: `docker compose` → `podman-compose` → `podman`
- No direct subprocess fallback — containers are required for LightRAG + WebUI
- WebUI static files built into the Docker image, served by LightRAG API

---

## Future Phases

### Phase 22 — MCP Tool Improvements ✅ COMPLETE

✅ All 13 tools implemented and verified
✅ Fixed `get_related_files` to use `/query/data` endpoint
✅ Docker image rebuilt, container running with 13 tools

### Phase 23 — MCP Bug Fixes & Auto-Initialization 🔧 IN PROGRESS

**Goal:** Make the MCP tool set genuinely useful for large, unfamiliar codebases.

### Why

The current 6-tool set (query, get_related_files, check_entity_exists, get_knowledge_graph, get_graph_labels, add_note_to_entity) is theoretically correct but has practical gaps:
- `get_related_files` is **broken** — calls `POST /query` (LLM endpoint) but expects `results` field which is always `None`
- No way to **browse** entities (list by type, filter by file, search by pattern)
- No **graph traversal** (find neighbors, incoming/outgoing relations)
- `get_knowledge_graph` returns **everything** — too large for big codebases

### Problems to Solve

| Problem | Impact |
|---------|--------|
| `get_related_files` uses wrong endpoint | Always returns empty results |
| No entity browsing | Can't answer "What classes exist?" |
| No graph traversal | Can't answer "What calls this method?" |
| Full graph dump | Hangs on large codebases |
| No pattern search | Can't answer "Find classes with 'Service' in name" |

### Tools to Add/Fix

**Fix existing:**
- `get_related_files` → use `POST /query/data` (returns structured entities + references), not `/query`

**New exploration tools:**
- `query_data` — structured query using `/query/data` endpoint (entities, relationships, chunks, references)
- `list_entities_by_type` — paginated list of CLASS/METHOD/PROPERTY entities
- `get_entities_in_file` — all entities from a specific file path
- `search_entities` — regex search on entity names/descriptions

**New graph traversal:**
- `get_entity_neighbors` — directly connected entities (1-hop)
- `get_incoming_relations` — what points TO this entity
- `get_outgoing_relations` — what this entity points TO
- `get_entity_details` — full entity info with all relations

**New overview:**
- `get_codebase_stats` — entity counts by type, relation counts by type, file count

### Implementation Notes

- `/query/data` returns: `{status, data: {entities, relationships, chunks, references}}`
- `/graphs` accepts `label` param — can filter by entity_type
- LightRAG graph CRUD: `POST /graph/entity/create`, `POST /graph/relation/create`, `GET /graph/entity/exists`
- Entity browsing via `/graphs?label=CLASS` etc. (no dedicated list endpoint needed)
- For neighbor queries: fetch full graph, filter locally (no server-side traversal needed for small hops)

### Steps

| # | Step | What changes |
|---|------|-------------|
| 1 | Fix `get_related_files` client method | Use `/query/data` instead of `/query` |
| 2 | Add `query_data` MCP tool | Structured query (entities + references) |
| 3 | Add `list_entities_by_type` tool | Paginated entity browsing by type |
| 4 | Add `get_entities_in_file` tool | Filter graph by file_path |
| 5 | Add `search_entities` tool | Regex on entity names/descriptions |
| 6 | Add `get_entity_neighbors` tool | 1-hop traversal from an entity |
| 7 | Add `get_incoming_relations` / `get_outgoing_relations` | Directional relation browsing |
| 8 | Add `get_entity_details` tool | Full entity info + relations |
| 9 | Add `get_codebase_stats` tool | Summary counts by type |
| 10 | Update docs | DOCUMENTATION.md, CURRENT_CONTEXT.md |
| 11 | Verify all tools work | Test against real codebase |

### Phase 23 — MCP Bug Fixes & Auto-Initialization 🔧 IN PROGRESS

**Agent test revealed critical bugs:**

| Tool | Issue |
|------|-------|
| `get_entities_in_file` | **Bug:** `file_path` stored at node level in LightRAG, not inside `properties`. Client code looked in wrong place. |
| `search_entities` | **Bug:** Uses `label="*"` which may be invalid at `/graphs`. Returns empty or errors. |
| `query` | Falls back to LLM training data when graph is empty → unreliable results |
| All tools | **Gap:** Graph is always empty — nothing auto-indexes on startup |

**Root cause analysis:**
1. LightRAG's `acreate_entity` stores `file_path` at node level (`node["file_path"]`), not nested in `node["properties"]["file_path"]`
2. `/graphs?label=*` is invalid — `*` is not a real entity type label
3. MCP server starts without running the C# analyser on the project

**Fixes to implement:**

1. **`get_entities_in_file`:** Change `node.get("properties", {}).get("file_path")` → `node.get("file_path")` at node level. Add null-check for `properties=None`.

2. **`search_entities`:** Instead of `label=entity_type or "*"` (invalid), fetch ALL entities by calling `/graphs` with a valid label (e.g., first label from `get_graph_labels`), then filter locally.

3. **Auto-initialization on MCP startup:** When MCP server starts, detect the project directory (the directory opencode was started from — available via `$PWD` or passed as argument). If a `*.sln` or `*.csproj` exists there, run the C# analyser and ingest results. This populates the graph before the first query.

4. **Better empty-graph messages:** When `get_codebase_stats` returns 0 entities, include a message suggesting to configure the project and wait for indexing.

**Auto-initialization approach:**
- MCP server receives no project path on startup (stdio mode)
- opencode passes `cwd` when invoking `coderag mcp .` — but this goes to the Go bridge, not to MCP
- Better: MCP server reads `LIGHTRAG_PROJECT_PATH` env var at startup (set by Go bridge before spawning MCP)
- Or: MCP server checks current working directory for `.sln`/`.csproj` files on startup
- Run C# analyser via `update_relations` method, ingest output, then begin accepting queries

**Steps:**

| # | Step | What changes |
|---|------|-------------|
| 1 | Fix `get_entities_in_file` | `node.get("file_path")` not `node.get("properties",{}).get("file_path")` |
| 2 | Fix `search_entities` | Use valid label from `get_graph_labels` or local filtering without label |
| 3 | Add null-check guard | Handle `properties=None` on nodes |
| 4 | Auto-initialization | Check for `.sln`/`.csproj` in cwd on startup, run analyser |
| 5 | Better empty-graph messages | Add helpful text when stats show 0 entities |
| 6 | Rebuild Docker, verify | Test with real codebase |
| 7 | Update docs | DOCUMENTATION.md, CURRENT_CONTEXT.md |

**Key technical decision:** MCP auto-initialization uses cwd (where opencode was started) rather than env var. If `*.sln` exists in cwd, run the analyser automatically.

### Phase 24 — MCP Tool Return Empty Results Bug Fix ✅ COMPLETE
- Fixed `get_all_labels()` returning node IDs instead of entity types
- Added `get_all_entity_types()` and `get_all_relation_types()` to networkx_impl.py
- `/graph/label/list` now returns `{entity_labels: [...], relation_labels: [...]}` structure
- Fixed 4 MCP client methods to use `label="*"` and filter client-side by `properties.entity_type`

### Phase 25 — Architecture Cleanup & MCP Fixes ✅ COMPLETE

**Completed changes:**

| # | Change | Files |
|---|--------|-------|
| 1 | Move C# analyser from `daniel-lightrag-mcp/analysers/` to `analysers/c_sharp/` | Create `analysers/`, delete old location |
| 2 | Update Go pipeline path for analyser binary | `pkg/watchdog/pipeline/run.go:300` |
| 3 | Add `coderag scan` command | `cmd/coderag/main.go` |
| 4 | Start watchdog in background when `coderag mcp` runs | `cmd/coderag/main.go` |
| 5 | Deprecate `update_relations` in MCP client (keep, may be used in future) | `client.py:380-423` |
| 6 | Delete old Python watchdog code (`watchdog/` dir) | `watchdog/` |
| 7 | Update analyzer runner for new location | `analyser_runner.py` |

**New architecture:**
```
coderag mcp <project-dir>
  → ensureMCP()         (start Docker, wait for health)
  → startWatchdog()     (goroutine, background, starts file watcher)
  → runSSEBridge()      (blocks, bridging stdio↔SSE)
```

**New `coderag scan` command:**
```bash
coderag scan <project-dir>   → find .sln, run C# analyser, ingest results
```

### Phase 26 — Fix Broken MCP Tools ✅ COMPLETE

**Fixed all 6 broken/partial MCP tools:**

| # | Fix | Location |
|---|-----|----------|
| 1 | Add `_safe_entity_data()` helper with fallback chain | `client.py` |
| 2 | `get_entity_details` - Use safe entity data construction | `client.py:542-562` |
| 3 | `list_entities_by_type` - Case-insensitive comparison + safe name | `client.py:599-631` |
| 4 | `get_entities_in_file` - Safe name extraction with hash fallback | `client.py:633-668` |
| 5 | `search_entities` - Safe name extraction, case-insensitive filter | `client.py:670-718` |
| 6 | `get_entity_neighbors` - Safe node ID resolution + entity construction | `client.py:470-540` |
| 7 | `add_note_to_entity` - Check both 'created' flag AND message | `server.py:436-480` |

### Phase 27 — Verify All MCP Tools
- Rebuild Docker image with fixed Python MCP
- Run full tool test against test-csharp-project
- Verify all 13 tools work correctly

### Phase 28 — Python Analyser
- Add Python analyser (AST-based)
- Register in `pkg/watchdog/router/file.go`
- Entity types: CLASS, FUNCTION, IMPORT, DECORATOR

### Phase 29 — Git Commit Watcher
- Extend git watcher to detect new commits (not just uncommitted changes)
- Support `git pull` → re-scan changed files

### Phase 30 — Stale Entity Cleanup
- Periodic sweep: entities whose `file_path` no longer exists on disk
- Configurable interval, dry-run mode

### Phase 31 — WebUI Integration
- WebUI served from LightRAG API (static files)
- Browse entity graph, search, query
- Visualize file → entity → relation mapping

---

## Security

All analysers **must** filter before inserting into the graph:
- 🔴 API keys, tokens, passwords (`sk-...`, `ghp_...`, etc.)
- 🔴 Email addresses, usernames, personal names
- 🔴 IP addresses, internal URLs, server paths
- 🔴 Any `SECRETS`/`PASSWORDS`/`TOKENS` env var values

The security filter lives in the pipeline **after** extraction but **before** graph insertion. This prevents sensitive data from ever reaching the vector DB.

---

## What We're Not Doing (Yet)

- ❌ PyPI publishing
- ❌ Premature corporate docs
- ❌ Full-language support before the pipeline pattern is proven with C#
- ❌ Perfection — experimental project that ships
