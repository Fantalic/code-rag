# CodeLightRAG — Technical Reference

*What we learned building this. Updated as we go.*

## Architecture

```
[Filesystem] ──► Watchdog ──HTTP──► LightRAG API ──► Knowledge Graph
                                         ▲
[MCP Client] ──stdio──► MCP Server ──────┘
```

### Components
- **LightRAG** — RAG engine with FastAPI on port 9621. Stores entities + relations as a knowledge graph.
- **MCP Server** — 22-tool Model Context Protocol bridge. Stdio transport. Connects to LightRAG.
- **Watchdog** — Python `watchdog` library. Monitors filesystem, HTTP POSTs changes to LightRAG.

## Known Issues

### LightRAG server startup
- `config.parse_args()` fires at **module import** time (`lightrag/api/config.py:417`). Workaround: set `sys.argv = ["lightrag-server"]` before importing.
- Emoji crash on Windows cp1252 (`lightrag/api/lightrag_server.py:336`). Patch: `ASCIIColors.green = lambda t, **k: None`.
- Frontend check exits process (`lightrag/api/lightrag_server.py:171`). Patch: `check_frontend_build = lambda: None`.

### Document processing
- Failed documents retry PENDING→FAILED→PENDING infinitely. No circuit breaker in upstream LightRAG.
- Document routes use `/documents/` prefix (e.g. `/documents/pipeline_status`).

### Embedding/LLM
- `ollama` Python package must be ≥0.4.0. Current pin: `>=0.4.0,<1.0.0`.
- Mock functions must be `async def` — LightRAG core `await`s all LLM/embed calls.

### C# Analyser
- Requires .NET 9 runtime.
- Output is plaintext `.txt` files, not structured JSON.

## Test Infrastructure

### Event loop + asyncio.Lock = deadlock
Session-scoped async fixtures create `asyncio.Lock` objects on the session event loop. Test functions run on their own event loop by default. Acquiring a session-loop lock from a function-loop causes deadlock.

**Fix**: `asyncio_default_test_loop_scope = session` in `pytest.ini`. Remove `event_loop` fixture entirely (deprecated in pytest-asyncio 1.4.0).

### No `tests/__init__.py`
Package marker + session-scoped lifespan fixture = hang. Don't create it.

### MCP response convention
Handlers return **raw dicts**. The MCP library wraps them as `CallToolResult`. Don't return `CallToolResult` from handlers — MCP does that externally.

## LightRAG API Endpoints (relevant)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health check |
| `/documents/text` | POST | Insert text document |
| `/documents/file` | POST | Insert file content |
| `/documents/pipeline_status` | GET | Get status of all docs |
| `/query` | POST | Query the graph |
| `/graph/entities` | GET | List all entities |
| `/graph/relations` | GET | List all relations |
| `/graph/knowledge_graph` | GET | Get full graph (nodes + edges) |
