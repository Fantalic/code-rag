# 🗺️ CodeLightRAG — Where We're Going

## The Big Idea

A **live, minimal-work knowledge graph** for codebases. When you save a file, only the affected entities and relations update — no full rebuilds, no wasted tokens, no secrets leaked.

## What We Have ✅

- Monorepo: LightRAG + MCP server + watchdog working together
- 22 MCP tools for AI agents to query the knowledge graph
- C# analyser (Roslyn-based, compiled binary) — extracts classes, methods, inheritance, deps
- File-watch with debouncing + hash tracking (skips no-op events)
- 226 passing tests (63 root + 168 MCP with Phase 14 additions)
- **Go watchdog** (Phase 12-13) — standalone CLI binary replaces Python watchdog

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

### Go package structure

```
watchdog/
├── cmd/watchdog/main.go       # CLI entry (urfave/cli/v2)
├── go.mod
├── internal/
│   ├── crud/client.go         # Graph CRUD HTTP client
│   ├── diff/engine.go         # Diff engine (EntityAction, RelationAction)
│   ├── filter/security.go     # Security filter (PII/keys/secrets)
│   ├── router/file.go         # Extension -> analyser mapping
│   ├── parser/csharp.go       # C# analyser output parser
│   ├── pipeline/run.go        # Pipeline orchestration
│   └── watcher/observer.go    # File watcher + worker pool
└── Dockerfile
```

### CLI commands

```bash
watchdog watch <dir>            # Watch project folder (connects to existing LightRAG)
watchdog start <dir>            # Start LightRAG + watchdog together
watchdog status                 # Show running services
watchdog stop                   # Stop all services
watchdog query "<text>"         # Query the graph directly
```

### Configuration (env vars + CLI flags)

| Variable | Flag | Default |
|----------|------|---------|
| LIGHTRAG_URL | --url | http://localhost:9621 |
| OLLAMA_HOST | --ollama | http://localhost:11434 |
| INPUT_DIR | positional | — |
| DEBOUNCE_SECONDS | --debounce | 0.5 |
| WATCHER_WORKERS | --workers | 4 |
| LIGHTRAG_API_KEY | --api-key | — |

## Phase 13 — Concurrent Analyser Race Fix ✅

Fixed three issues in the Go watchdog pipeline:

### Issue 1 — Concurrent analyser invocations
- Multiple workers (4 by default) each ran the C# analyser on the same solution independently
- Result: duplicate analyser runs, race conditions, duplicate relation creation errors
- **Fix**: Solution-level mutex (`sync.Mutex` + `chan struct{}` per sln path). Initial sync groups files by solution, runs analyser once, shares results with all workers.

### Issue 2 — LightRAG undirected graph direction swap
- LightRAG uses `nx.Graph()` (undirected). API serializes edges with `source`/`target` alphabetically sorted for consistency
- Relations like `Calculator→Add` appeared as `source=Add, target=Calculator` in graph queries
- **Fix**: `loadCurrentRelations` now reads `src_id`/`tgt_id` from edge `properties` (preserves real creation order)

### Issue 3 — Duplicate relation creation
- Concurrent workers tried to create same relations, LightRAG returned "already exists" errors
- **Fix**: `sync.Map` dedup set in pipeline prevents same relation from being created twice

### Files modified (Phase 13)
- `watchdog/internal/pipeline/run.go` — `AnalyserResult` type, `RunEvent`, relation dedup, fix `loadCurrentRelations`
- `watchdog/internal/watcher/observer.go` — solution grouping, mutex, Event carries pre-computed results
- `watchdog/internal/parser/csharp.go` — METHOD entities now created alongside CLASS entities

### Test result
- C# project: 7 entities (3 CLASS + 4 METHOD), 4 relations (Calculator→Add, Calculator→Subtract, DataStore→AddItem, DataStore→GetAll)
- Directions correct via `src_id`/`tgt_id` from edge properties

## Phase 14 — Testing, MCP Tool Usage & Test Expansion 🔥

### Goals
1. **Test system with code changes** — verify relations and updates work correctly
2. **Use MCP tools extensively** — gather insights on what's working and what needs improvement
3. **Expand test project** — add new tests, features, coverage
4. **Verify update_relations** — confirm the C# analyser + LightRAG integration pipeline works end-to-end

### Test-csharp-project expanded (Phase 14)
New files added:
- `TestLib/AdvancedCalculator.cs` — inheritance (AdvancedCalculator extends Calculator), IOperation interface
- `TestLib/ExtendedTypes.cs` — IRepository<T>, EntityBase → User inheritance, properties (Count, Email, etc.)

Analyser now extracts: INHERITS_FROM, DEPENDS_ON, PROPERTY, CALLS relations (not just CLASS/METHOD)

### Go parser fix
Updated `watchdog/internal/parser/csharp.go` to handle new relationship types:
- `INHERITS_FROM` → creates inheritance relation
- `DEPENDS_ON` → creates interface implementation relation
- `PROPERTY:` → creates PROPERTY entities alongside CLASS and METHOD
- `CALLS` → creates method call relations (cross-file references)

### New tests added
5 new tests in `daniel-lightrag-mcp/tests/test_analyser_runner.py`:
- `TestRealProjectAnalysis` class with tests for inheritance, interface, property relations
- Total MCP tests: **168 passing** (163 original + 5 new)

## Phase 15 — End-to-End System Testing ✅ COMPLETE

### Live System Test Results

**Verified Working:**
- LightRAG server healthy (qwen3-coder:480b-cloud + bge-m3:latest)
- Graph has 50 nodes, 47 edges from previous `update_relations`
- Versioned file_source (`#v2`) successfully adds new entities (Increment)
- Queries return relevant context about Calculator methods

**Verified NOT Working:**
- Entity deletion endpoint `DELETE /documents/delete_entity` returns **405 Method Not Allowed**
- No automatic entity cleanup when files are modified
- No relation cleanup when methods are removed

### Key Finding: Versioned file_source Required
LightRAG returns "duplicated" for same `file_source`. Must use incrementing versions:
- `TestLib_Calculator.txt` → first ingestion
- `TestLib_Calculator.txt#v2` → re-ingest after change
- `TestLib_Calculator.txt#v3` → next change

This means both old AND new entities exist after file changes — no replacement.

## Phase 16 — Fix Entity Deletion & File Change Handling ✅ COMPLETE

### What Was Fixed

1. **Entity deletion works** — confirmed `DELETE /documents/delete_entity` with JSON body works
2. **Entity deletion on file delete** — added `DeleteEntitiesByFilePath(filePath)` to Go watchdog
3. **Modified files refresh entities** — `runCSharpPipelineModified()` deletes old entities before re-processing
4. **Relation cleanup automatic** — LightRAG deletes relations when entity is deleted

### Files Modified
- `watchdog/internal/crud/client.go` — `GetEntitiesByFilePath()`, `DeleteEntitiesByFilePath()`
- `watchdog/internal/pipeline/run.go` — `RunEventWithChangeType()`, `runCSharpPipelineModified()`
- `watchdog/internal/watcher/observer.go` — handles "modified" and "deleted" with entity cleanup

### Test Results
- Go build: **SUCCESS**
- MCP tests: **168 passed**

## Phase 17 — Relation Creation Fix ✅ COMPLETE

### Issues Fixed

1. **Router processed non-C# files** — `.go`, `.py`, `.md` files were being routed to C# analyser. Fix: `router/file.go` now checks `.cs` extension AND not obj/bin directory.

2. **Git watcher reported files as deleted repeatedly** — git returning relative paths, security check failed. Fix: `observer.go` uses `filepath.Clean()` and verifies paths are under project root.

3. **Entity count shrank after initial sync** — wrong files creating invalid entities. Fix: Router filters to only C# source files.

4. **Relations never created (0 edges)** — `loadCurrentRelations()` looked for `src_id`/`tgt_id` in edge properties but LightRAG returns `source`/`target` at top level. Fix: `pipeline/run.go` reads edge source/target directly.

### Files Modified
- `watchdog/internal/router/file.go` — C# file filtering
- `watchdog/internal/watcher/observer.go` — path cleaning + git path validation
- `watchdog/internal/pipeline/run.go` — fix `loadCurrentRelations()`, skip CLASS entity updates

## Phase 18 — Project-Based Storage & GraphML Fix ✅ COMPLETE

### Changes Made

**Project-Based Storage:**
- `.coderag/rag_storage` per project folder, auto-detected on startup
- `LightRAG/lightrag/api/config.py`: `find_project_root()`, `get_default_working_dir()`
- Watchdog `start` command passes `LIGHTRAG_WORKING_DIR` and sets `Dir = inputDir`
- `scripts/init-project.ps1` for project initialization

**GraphML NoneType Fix:**
- `networkx_impl.py`: filter None values in `upsert_node()`, `upsert_edge()`, `write_nx_graph()`
- Prevents "GraphML does not support type <class 'NoneType'> as data values" error

**No `.env` Requirement:**
- `utils_api.py`: `check_env_file()` always returns `True`
- Single-instance design doesn't need multi-instance safety checks

### Test Results
- Fresh start: 29 nodes, 25 edges (all C# entities parsed)
- After adding IRepository + methods: 31 nodes, 28 edges
- After adding ClearAll: 32 nodes, 29 edges
- GraphML properly persisted (14,768 bytes, valid XML)
- Vector DB embeddings generated for new entities
- No GraphML errors

### Files Modified
- `LightRAG/lightrag/api/config.py` — `find_project_root()`, `get_default_working_dir()`
- `LightRAG/lightrag/api/utils_api.py` — simplified `check_env_file()`
- `LightRAG/lightrag/kg/networkx_impl.py` — None filtering in write/upsert
- `watchdog/cmd/watchdog/main.go` — LIGHTRAG_WORKING_DIR + Dir settings

## Phase 19 — Next Tasks

1. **Add stale entity cleanup** — periodic sweep of entities whose `file_path` no longer exists on disk
2. **Add git commit watcher** — extend git watcher to detect new commits (not just uncommitted changes)
3. **Add unit tests** for CRUD client methods (`GetEntitiesByFilePath`, `DeleteEntitiesByFilePath`)
4. **Add more language analysers** — Python, Go, TypeScript
5. **Handle `.sln` creation** — auto-create solution file if missing (currently required for C# analysis)

### Security

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
