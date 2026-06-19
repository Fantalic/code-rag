# Current Context

## Status
- 🧪 **Experimental** — building the hard part
- **Phase 11**: Efficient incremental graph updates 🔄

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

## What's Being Built (Phase 11)

```
File change → file-type router → analyser (per ext) → diff engine → graph CRUD → security filter
```

### 11.1 ✅ Done
- File hash cache — skip unchanged files
- Versioned re-ingest — bypass LightRAG dedup
- Cache cleanup on delete

### 11.3 ✅ Done (all 7 steps)
- ✅ `watchdog/graph_crud.py` — HTTP client for entity/relation CRUD (create_entity, create_relation, update_entity, update_relation, delete_entity, delete_relation, get_all_entities)
- ✅ `watchdog/security_filter.py` — regex patterns for API keys, passwords, JWTs, private keys, emails, IPs, internal paths, env secrets. `[REDACTED]` replacement.
- ✅ `watchdog/file_router.py` — extension → analyser mapping (`.cs` → c_sharp; all others → unsupported for now)
- ✅ `watchdog/graph_diff.py` — diff engine: EntityAction and RelationAction with CREATE/UPDATE/DELETE. Compares old/new entity sets and relation sets, ignores server-managed fields.
- ✅ `watchdog/analyser_parsers.py` — parses C# analyser `.txt` output into structured entity/relation dicts. Format: `CLASS: <name>`, `METHOD: <name> -> RETURNS -> <type>`, `PROJECT REFERENCES:` → list of project deps. Entities: class with file_path, methods list. Relations: CLASS→method (CALLS), CLASS→project_ref (DEPENDS_ON).
- ✅ Pipeline wired into `watchdog/main.py` — `_run_pipeline()` → `_analyse_file()` → C# analyser → `parse_csharp_output()` → diff → security filter → graph CRUD. Unsupported files fall back to old `POST /documents/text` (LLM pipeline).

### Future
- Zig analysers for: Python, JS/TS, Go, Rust (performance-critical)
- Per-language entity extraction with consistent schema
- Watchdog → MCP server integration (instead of direct HTTP to LightRAG)

## Key Technical Decisions

1. **Entity-level CRUD** > document pipeline — no LLM calls for analyser output
2. **Analysers in Zig** for future languages — performance-critical path
3. **Security filter built into pipeline** — secret data never reaches graph
4. **File-type routing pattern** — each extension has its own analyser entry point
5. **No `tests/__init__.py`** — causes hang with session-scoped lifespan fixture

## Learnings

### Event loop trap
Session-scoped async fixtures + function-scoped test event loops = deadlock. Fix: `asyncio_default_test_loop_scope = session` in `pytest.ini`, no `event_loop` fixture.

### LightRAG dedup
`POST /documents/text` with existing `file_source` returns `status="duplicated"` and does nothing. No update-in-place. Must use versioned file_source or graph CRUD endpoints.

### Graph CRUD exists separately
LightRAG has `/graph/entity/create`, `/graph/relation/create`, `/graph/entity/edit`, etc. — these operate directly on the graph + vector DB, independent of the document pipeline.
