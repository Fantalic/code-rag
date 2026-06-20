# Current Context

## Status
- 🔥 **Phase 20** — VDB Embedding & file_path Persistence ✅ COMPLETE
- **Phase 19** — Incremental Update Pipeline Fixes ✅ COMPLETE
- **Phase 18** — Project-Based Storage & GraphML Fix ✅ COMPLETE
- **Phase 17** — Relation Creation Fix ✅ COMPLETE
- **Phase 15-16**: End-to-End System Testing ✅ COMPLETE
- **Phase 14**: Testing, MCP tool usage, test expansion ✅

## Phase 20 — VDB Embedding & file_path Persistence ✅ COMPLETE

### Issues Found & Fixed

**1. Relation "already exists" errors on CREATE**
- **Root Cause**: `RelationKey` does not sort `src`/`tgt` — LightRAG stores edges in sorted order (undirected graph). Keys don't match → CREATE on existing → error.
- **Fix**: `watchdog/internal/diff/engine.go:RelationKey()` sorts `src` > `tgt` alphabetically.

**2. CLASS entity `file_path` not updated**
- **Root Cause**: `applyCSharpResults` skipped ALL CLASS entity updates. `file_path`, `source_id` never persisted.
- **Fix**: `watchdog/internal/pipeline/run.go` — delete only `description`/`methods` from CLASS update data instead of skipping entirely.

**3. Modified file pipeline fails — no analyser results**
- **Root Cause**: Git watcher sends events without pre-computed `Results`.
- **Fix**: `runCSharpPipelineModified` re-runs C# analyser when `results` is empty.

**4. ParseCSharpFile dropped all but the last class per output file**
- **Root Cause**: Old parser only emitted the last `CLASS:` match per output file. EntityBase, IRepository, IOperation, Id, CreatedAt, Name, Get, GetAll, Remove, FindById, Add, Contains were all lost.
- **Fix**: Rewrote `watchdog/internal/parser/csharp.go` — per-class accumulator with `flushClass()` at each `CLASS:` boundary; `cleanFilePath()` strips `[LINK: ...]` suffix.

### Issues Found & Fixed

**1. Entity `file_path` lost in VDB after update (`aedit_entity`)**
- **Root Cause**: `aedit_entity()` at `utils_graph.py:337` re-built the VDB entry without `file_path`. The diff engine emits `ActionUpdate` for every entity after the first `ActionCreate` (multiple .cs files share the same classes), so the edit path **overwrote** the VDB entry created by `acreate_entity()` (which correctly included `file_path`).
- **Fix**: Added `"file_path": new_node_data.get("file_path", "")` to the VDB upsert dict in `aedit_entity()`.
- **Verified**: Create → VDB has `file_path` ✅ ; Update → VDB preserves `file_path` ✅

**2. Relation `file_path` lost in VDB after update (`aedit_relation`)**
- **Root Cause**: Same pattern — `aedit_relation()` at `utils_graph.py:453` omitted `file_path` from VDB upsert.
- **Fix**: Added `"file_path": new_edge_data.get("file_path", "")` to the VDB upsert dict in `aedit_relation()`.
- **Verified**: Create → VDB has `file_path` ✅ ; Update → VDB preserves `file_path` ✅ ; Update without `file_path` falls back to graph edge data ✅

### Test Coverage Added
11 new integration tests in `tests/test_integration_graph_vdb.py`:
- **TestEntityVDB** (4 tests): create embeds file_path, update preserves file_path, create without file_path defaults, rename preserves vector
- **TestRelationVDB** (3 tests): create embeds file_path, update preserves file_path, create without file_path defaults
- **TestGraphEmbeddingIntegrity** (4 tests): full entity/relation create+update workflow, update without file_path keeps original for both entities and relations
- All tests run against mocked server (session-scoped) via httpx ASGITransport

### Verified: Initial Sync Results
- **42 entities** (up from 33) — all previously missing entities now present
- **45 relations**
- **Zero errors** — no "already exists", no NoneType crashes
- GraphML persists correctly, vector DB populated



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
8. **LightRAG undirected graph** — edge `source`/`target` not directional. `RelationKey` MUST sort src/tgt for correct diff comparison.

## Learnings

### Event loop trap
Session-scoped async fixtures + function-scoped test event loops = deadlock. Fix: `asyncio_default_test_loop_scope = session`.

### LightRAG dedup
`POST /documents/text` with existing `file_source` returns `status="duplicated"` and does nothing.

### GraphML NoneType
NetworkX's GraphML writer rejects `None` attribute values. Must filter before every write. Fix applied in both `upsert_node`/`upsert_edge` and `write_nx_graph`.

### Project detection
`find_project_root()` searches from cwd upward for `.coderag`. Watchdog's `start` command now sets `lightragProcess.Dir = inputDir` so cwd matches the project root.

### Undirected graph relation key sorting
LightRAG stores edges with sorted `source`/`target` (alphabetic). The watchdog diff engine compares relation keys (`src|tgt`). If the diff uses unsorted keys while the graph uses sorted ones, every existing relation appears "new" → duplicate CREATE → "already exists" error. Fix: `RelationKey()` must always sort both sides.

### CLASS entity update skip is too broad
The pipeline skips all CLASS entity updates because descriptions are "auto-generated". But `file_path` is also a CLASS property and needs updating. Either skip only the `description` field or update it selectively.
