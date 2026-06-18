# Current Context

## Status
- **Phase**: 3 — Unify 🔄 WIP
- **Phase 1**: ✅ Complete — server starts without patches, 11/12 integration tests pass
- **Phase 2**: ✅ Complete — 6 removals/fixes committed (39a7bdeb)
- **Current step**: 3.1 — Single build system (root pyproject.toml)

---

## What's Been Done

### Phase 1 — Making It Run ✅
- MCP name-clash fix (server → mcp_server)
- Single dev install (scripts/setup.ps1)
- MCP connects to LightRAG (4 integration tests)
- Watchdog ingests via HTTP POST
- Fix sprint: 6 bugs fixed (emoji, frontend, file_path, config.parse_args, retry loop, start_all.ps1)
- Server starts without any runtime patches
- End-to-end C# pipeline verified (real Ollama qwen2.5-coder + bge-m3)

### Phase 2 — Strip & Simplify ✅
- Fix .cs/.rel extension bug (missing commas)
- Fix DEFAULT_WOKERS → DEFAULT_WORKERS typo
- Remove dead AGEStorage + ChromaVectorDBStorage entries from kg/__init__.py
- Remove 4 duplicate requirements-offline-*.txt files
- Remove legacy setup.py shim
- Consolidate 6 .gitignore files → single root

---

## Test Suite (25 passing)
```
tests/test_smoke_lightrag.py       — 9 tests (core + API imports)
tests/test_smoke_mcp.py            — 9 tests (imports, models, enums)
tests/test_smoke_watchdog.py       — 0 selected (skipped if watchdog unavailable)
tests/test_integration_lightrag.py — 12 tests (API endpoints)
tests/test_integration_mcp.py      — 4 tests (MCP client through LightRAG)
```

---

## Next Steps
- **Phase 3**: Unify build system, shared config, consistent imports
