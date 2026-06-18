# CodeLightRAG Documentation

*Built incrementally. Sections added when knowledge stabilises.*

## Overview

CodeLightRAG transforms codebases into knowledge graphs for AI agents. It combines:

- **LightRAG** — RAG engine with FastAPI server (port 9621)
- **MCP Server** — Model Context Protocol bridge (22 tools, stdio transport)
- **Watchdog** — File-system watcher that auto-ingests changes

## Architecture

```
[Filesystem] ──► Watchdog ──HTTP──► LightRAG API ──► Knowledge Graph
                                        ▲
[MCP Client] ──stdio──► MCP Server ────┘
```

## Known Issues & Workarounds

### LightRAG Server Startup
- **`config.parse_args()` at module import** (`lightrag/api/config.py:417`): Set `sys.argv = ["lightrag-server"]` before any import from `lightrag.api.config`.
- **Emoji crash on Windows cp1252** (`lightrag/api/lightrag_server.py:336`): Patch `ASCIIColors.green = lambda t, **k: None` before server creation.
- **Frontend check exits process** (`lightrag/api/lightrag_server.py:171`): Patch `check_frontend_build = lambda: None` for API-only mode.

### Document Processing
- **`file_path` Pydantic validation error** (`document_routes.py:2563`): `DocStatusResponse` field `file_path` is `Optional[str]` but Pydantic v2 rejects `None`. Temporary workaround: patch or wrap `get_track_status`.
- **Failed documents retry infinitely**: Processing errors cause FAILED→PENDING→FAILED cycle. No circuit breaker.
- **Document routes use `/documents/` prefix**: e.g. `/documents/pipeline_status`, not `/pipeline_status`.

### Embedding / LLM
- **`ollama` Python package**: Must be ≥0.4.0 (v0.2.0 lacks `AsyncClient.embed()`). Current: 0.6.2.
- **Mock functions must be `async def`**: LightRAG core `await`s all LLM/embed calls; sync lambdas crash.

### C# Analyser
- Requires .NET 9 runtime installed separately.
- Output is plaintext `.txt` files (not structured JSON).
- No automatic ingestion pipeline from analyser→LightRAG.
