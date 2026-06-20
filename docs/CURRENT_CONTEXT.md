# Current Context

## Status
- 🔥 **Phase 18** — Project-Based Storage & GraphML Fix ✅ COMPLETE
- **Phase 17** — Relation Creation Fix ✅ COMPLETE
- **Phase 15-16**: End-to-End System Testing ✅ COMPLETE
- **Phase 14**: Testing, MCP tool usage, test expansion ✅

## Phase 18 — Project-Based Storage & GraphML Fix ✅

### Changes Made

**LightRAG API (`config.py`):**
- Added `find_project_root()` — searches from cwd upward for `.coderag` folder
- Added `get_default_working_dir()` — returns `.coderag/rag_storage` if found, else `./rag_storage`
- Both functions use `LIGHTRAG_WORKING_DIR` / `WORKING_DIR` env vars as override

**LightRAG API (`utils_api.py`):**
- Removed `.env` requirement — `check_env_file()` now always returns `True`
- Rationale: single-instance deployment, no multi-instance safety needed

**LightRAG KG (`networkx_impl.py`):**
- Fixed GraphML save error: `NoneType as data values`
- `upsert_node()` and `upsert_edge()` filter None values before adding to graph
- `write_nx_graph()` pre-processes all nodes and edges to strip None values before serialization

**Watchdog (`cmd/watchdog/main.go`):**
- Added `LIGHTRAG_WORKING_DIR` env var in `startCmd` — points to `inputDir/.coderag/rag_storage`
- Added `lightragProcess.Dir = inputDir` — LightRAG runs from project folder so it finds `.env`

### End-to-End Test Results

| Test | Result |
|------|--------|
| Fresh startup with `.coderag/rag_storage` | ✅ 29 nodes, 25 edges |
| Add new interface + methods (IRepository, Add, FindById, Remove) | ✅ 31 nodes, 28 edges (2 new nodes, 3 new edges) |
| Add more methods (ClearAll) | ✅ 32 nodes, 29 edges |
| GraphML persistence (no NoneType errors) | ✅ 14,768 bytes, valid XML |
| Vector embeddings for new entities | ✅ vdb_entities.json updated (240KB) |
| Relation directions correct | ✅ DataStore → Remove, Calculator → Add, etc. |
| Cross-file relations | ✅ Calculator → AdvancedCalculator (inheritance) |
| `.env` not required | ✅ Server starts without `.env` file |
| Project isolation | ✅ Data in `test-project/.coderag/rag_storage`, not repo root |

### Known Limitations
- **Git watcher polls every 30s** — intentional debounce. Detects uncommitted changes only. New commits require restart.
- **No stale entity cleanup** — old entities from removed files persist unless watchdog runs with "modified" type on that file.
- **IRepository<T> not extracted** — C# analyser doesn't extract generic interfaces as standalone entities; only their concrete implementations.

### How Project Storage Works

```
User runs: .\watchdog\watchdog.exe start <project-path>

Watchdog:
  -> Sets LIGHTRAG_WORKING_DIR=<project-path>/.coderag/rag_storage
  -> Sets lightragProcess.Dir = <project-path>
  -> Starts LightRAG

LightRAG:
  -> Runs from <project-path> (finds .env if exists)
  -> get_default_working_dir() returns <project-path>/.coderag/rag_storage
  -> All graph + vector data stored in .coderag/rag_storage/
```

## Phase 15-16 Fixes Applied ✅

### Fixed Issues

1. **Entity deletion endpoint works** — `DELETE /documents/delete_entity` with JSON body works correctly.
2. **Go watchdog now deletes entities on file delete** — `DeleteEntitiesByFilePath(filePath)` queries all entities with matching `file_path` property and deletes them.
3. **Modified files trigger entity refresh** — `runCSharpPipelineModified()` deletes existing entities before re-processing.
4. **Entity lookup by file_path** — Added `GetEntitiesByFilePath(filePath)` to CRUD client.

### How It Works Now

**File Deleted:**
- `DeleteDocument(path)` — removes from document store
- `DeleteEntitiesByFilePath(path)` — removes graph entities matching that file_path

**File Modified:**
- `DeleteEntitiesByFilePath(path)` — removes old entities for this file
- `runCSharpPipelineWithResults()` — re-runs analyser, creates fresh entities/relations

**File Created:**
- `runCSharpPipelineWithResults()` — creates new entities/relations

## What Exists

### Running system
```
Watchdog ──HTTP──► LightRAG API ──► Knowledge Graph
                        ▲
MCP Client ──stdio──► MCP Server ──┘
```

### Watchdog (Go binary at `watchdog/watchdog.exe`)
- CLI commands: `watch`, `start`, `status`, `stop`, `query`
- Initial sync on startup — full scan, solution-level analysis, diff-based CRUD
- Git-based change detection (30s poll) — `git diff --name-only HEAD`
- File hash tracking (MD5) — skips unchanged content
- C# analyser: extracts CLASS, METHOD, PROPERTY, INHERITS_FROM, DEPENDS_ON, CALLS
- Solution-level analyser mutex — prevents concurrent runs
- `start` command launches LightRAG + watchdog together with correct working dir

### Go watchdog modules
All modules under `watchdog/internal/`:
- `crud/client.go` — graph CRUD HTTP client
- `diff/engine.go` — diff engine (EntityAction, RelationAction, CREATE/UPDATE/DELETE)
- `filter/security.go` — regex PII/keys/secrets filter
- `router/file.go` — extension → analyser mapping
- `parser/csharp.go` — C# analyser output parser
- `pipeline/run.go` — pipeline orchestration with relation dedup
- `watcher/observer.go` — file watcher + worker pool + git watcher
- `cmd/watchdog/main.go` — CLI with `start` command that sets LIGHTRAG_WORKING_DIR

### Test project (`../test-csharp-project/`)
- Separate git repo outside code-lightrag
- Contains `TestConsole` + `TestLib` projects
- `.coderag/rag_storage/` with LiveRAG data (graphml + vector DB)
- Source files: Calculator, AdvancedCalculator, DataStore, ExtendedTypes, Class1

## Key Technical Decisions

1. **Project-based storage** — `.coderag/rag_storage` per project, auto-detected
2. **No `.env` requirement** — single-instance, no multi-instance safety needed
3. **GraphML None filtering** — pre-process nodes/edges before serialization
4. **Git watcher polls 30s** — intentional debounce, not real-time
5. **Solution-level analysis** — entire solution re-analyzed on any file change (ensures cross-file consistency)
6. **Entity-level CRUD** > document pipeline — no LLM calls for analyser output
7. **Security filter built into pipeline** — secret data never reaches graph
8. **LightRAG undirected graph** — edge `source`/`target` not directional. Use edge type/properties for direction.

## Learnings

### Event loop trap
Session-scoped async fixtures + function-scoped test event loops = deadlock. Fix: `asyncio_default_test_loop_scope = session`.

### LightRAG dedup
`POST /documents/text` with existing `file_source` returns `status="duplicated"` and does nothing.

### GraphML NoneType
NetworkX's GraphML writer rejects `None` attribute values. Must filter before every write. Fix applied in both `upsert_node`/`upsert_edge` and `write_nx_graph`.

### Project detection
`find_project_root()` searches from cwd upward for `.coderag`. Watchdog's `start` command now sets `lightragProcess.Dir = inputDir` so cwd matches the project root.
