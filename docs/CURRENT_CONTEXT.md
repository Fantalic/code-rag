# Current Context

## Status
- 🧪 **Experimental** — building the hard part
- **Phase 13**: Concurrent analyser race fix ✅

## What Exists

### Running system
```
Watchdog ──HTTP──► LightRAG API ──► Knowledge Graph
                        ▲
MCP Client ──stdio──► MCP Server ──┘
```

### Watchdog (`watchdog/main.py`)
- Watches `INPUT_DIR` recursively, debounces events (0.5s)
- **New in 11.1**: File hash tracking (MD5) — skips events where content hasn't changed
- **New in 11.1**: Versioned re-ingest — `file_path#v1`, `file_path#v2` bypasses LightRAG dedup
- Worker pool (4 threads), handles create/modify/delete/move
- POSTs file content to `/documents/text` → full LLM pipeline (to be replaced by graph CRUD)

### C# analyser (`daniel-lightrag-mcp/analysers/`)
- Roslyn-based, compiled binary (`CSharpAnalyzerTool.exe`)
- Extracts: classes, methods, inheritance, project references
- Output: `.txt` files per class (plaintext descriptions)
- Currently ingested via `insert_text` → LLM pipeline (wasteful — LLM re-parses structured text)

### Test suite
- `pytest tests\` — 63 integration tests ✅
- `pytest daniel-lightrag-mcp\tests\` — 163 MCP tests ✅

## What's Built (Phase 12 ✅)

Go watchdog (standalone CLI binary) replacing the Python prototype.

All modules live under `watchdog/internal/`:
- `crud/client.go` — graph CRUD HTTP client
- `diff/engine.go` — diff engine (EntityAction, RelationAction, CREATE/UPDATE/DELETE)
- `filter/security.go` — regex PII/keys/secrets filter (same patterns as Python)
- `router/file.go` — extension → analyser mapping
- `parser/csharp.go` — C# analyser output parser
- `pipeline/run.go` — pipeline orchestration with relation dedup
- `watcher/observer.go` — file watcher + worker pool + solution-level mutex
- `cmd/watchdog/main.go` — CLI with `watch`, `start`, `status`, `stop`, `query` commands

## Phase 13 Fixes (done)

**Problem 1 — Concurrent analyser race:**
- Multiple workers called the analyser on the same solution simultaneously
- Fix: solution-level mutex + analyser runs once per solution, results shared with all workers

**Problem 2 — LightRAG undirected graph alphabetical swap:**
- LightRAG uses `nx.Graph()` (undirected), serializes edges alphabetically in API response
- Relations created as `Calculator→Add` showed up as `source=Add, target=Calculator` in queries
- Fix: `loadCurrentRelations` reads `src_id`/`tgt_id` from edge **properties** (preserves real direction)

**Problem 3 — Duplicate relation creation:**
- Workers created same relations concurrently, LightRAG returned errors
- Fix: `sync.Map` relation dedup set in pipeline

**Files changed:**
- `watchdog/internal/pipeline/run.go` — `AnalyserResult` type, `RunEvent`, relation dedup
- `watchdog/internal/watcher/observer.go` — solution grouping, mutex, Event carries results
- `watchdog/internal/parser/csharp.go` — METHOD entities now created alongside CLASS

### Architecture Notes

- **Initial scan**: walks project, groups files by solution, runs analyser once per sln, enqueues all files with shared results
- **Git watcher**: polls `git diff --name-only HEAD` every 30s for changed files (USE_GIT_WATCHER env var, default true)
- **LightRAG graph storage**: uses undirected `nx.Graph()` — edge `source`/`target` in API responses are alphabetically sorted; always read `src_id`/`tgt_id` from edge properties for real direction

## Key Technical Decisions

1. **Entity-level CRUD** > document pipeline — no LLM calls for analyser output
2. **Analysers in Go** for now — CPU-bound parsing work, compiled, no runtime. Zig deferred.
3. **Security filter built into pipeline** — secret data never reaches graph
4. **File-type routing pattern** — each extension has its own analyser entry point
5. **No `tests/__init__.py`** — causes hang with session-scoped lifespan fixture
6. **Solution-level analyser mutex** — prevents concurrent analyse of same .sln
7. **LightRAG undirected graph** — edge `source`/`target` alphabetically sorted in API. Always use `src_id`/`tgt_id` from edge properties for real direction.

## Learnings

### Event loop trap
Session-scoped async fixtures + function-scoped test event loops = deadlock. Fix: `asyncio_default_test_loop_scope = session` in `pytest.ini`, no `event_loop` fixture.

### LightRAG dedup
`POST /documents/text` with existing `file_source` returns `status="duplicated"` and does nothing. No update-in-place. Must use versioned file_source or graph CRUD endpoints.

### Graph CRUD exists separately
LightRAG has `/graph/entity/create`, `/graph/relation/create`, `/graph/entity/edit`, etc. — these operate directly on the graph + vector DB, independent of the document pipeline.

### LightRAG undirected graph
LightRAG stores edges in `nx.Graph()` (undirected). When the `/graphs` API serializes edges, it alphabetically sorts source/target to create consistent edge IDs. The real direction is preserved in `src_id`/`tgt_id` inside the edge's `properties` dict. Always read from there.
