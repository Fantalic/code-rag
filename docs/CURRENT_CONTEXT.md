# Current Context

## Status
- 🧪 **Experimental** — building the hard part right now
- **Foundation**: ✅ Complete (Phases 1-10: monorepo, tests, MCP tools, CI)
- **Next**: 🔥 **Phase 11** — efficient incremental graph updates via watchdog

## What Exists

### Running system
```
Watchdog ──HTTP──► LightRAG API ──► Knowledge Graph
                        ▲
MCP Client ──stdio──► MCP Server ──┘
```

- LightRAG runs on `:9621`, stores entities + relations as a knowledge graph
- MCP server exposes 22 tools (health, insert, query, KG, documents, system)
- Watchdog watches filesystem, sends file contents to LightRAG on change
- C# analyser extracts classes, methods, inheritance, dependencies

### Test suite
- `pytest tests\` — 63 integration tests ✅
- `pytest daniel-lightrag-mcp\tests\` — 163 MCP tests ✅

## What's Missing (The Hard Part)

Current watchdog flow:
```
File changes → re-read entire file → re-embed everything → replace in graph
```

What we need:
```
File changes → diff the AST → update only affected entities/relations → efficient
```

This is the core technical challenge. If one method signature changes in a 2000-line file, we should update exactly the relations that method participates in — not re-embed the whole file.

## Key Files

- `watchdog/main.py` — file watcher entry point
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/server.py` — MCP tools
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/models.py` — response models
- `daniel-lightrag-mcp/analysers/` — C# analyser pipeline
- `tests/` — 63 root tests
- `pytest.ini` — asyncio loop scope = session
- `AGENTS.md` — dev guidelines

## Technical Details (Learnings)

### Event loop trap
Session-scoped async fixtures + function-scoped test event loops = deadlock. Fix: `asyncio_default_test_loop_scope = session` in `pytest.ini`, no `event_loop` fixture.

### No `tests/__init__.py`
Package marker causes hang with session-scoped lifespan fixture. The file just shouldn't exist.

### MCP response format
Handlers return raw dicts, MCP library wraps as `CallToolResult`. Don't try to return `CallToolResult` from handlers.
