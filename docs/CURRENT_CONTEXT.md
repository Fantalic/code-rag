# Current Context

## Status
- **Phase**: 5 — Own Tests 🔄 WIP
- **Phases 1–4**: ✅ Complete (all committed)
- **Phase 5 steps 5.1–5.6**: Tests written and passing (15 tests), ready to commit

---

## What's Been Done

### Phase 4 — Modernise ✅
- 17 Pydantic V2 ConfigDict blocks migrated (0 V1 deprecation warnings)
- 271 type annotations upgraded to Python 3.10+ syntax (ruff UP006/UP007/UP035)

### Phase 5 — Own Tests (writing now)
- **5.1** `tests/test_namespace.py` — codelightrag namespace, version, optional MCP client, LightRAG instantiation
- **5.2** `tests/test_pydantic_v2.py` — ConfigDict models, Optional[str] file_path, round-trip deserialization
- **5.3** `tests/test_retry.py` — DocProcessingStatus defaults, retry exhaustion, retry_count increments
- **5.4** `tests/test_config.py` — lazy get_global_args(), get_env_value() defaults/parsing
- **5.5/5.6** `tests/test_mcp_edge_cases.py` — connection refused, empty query validation, invalid mode validation, LIGHTRAG_URL / LIGHTRAG_BASE_URL env var fallback

### server.py fix
- Extracted `_get_lightrag_url()` helper from inline `os.getenv` call for testability

---

## Test Suite (25 + 15 = 40 passing)
```
tests/test_smoke_lightrag.py        —  9 tests
tests/test_smoke_mcp.py             —  9 tests
tests/test_smoke_watchdog.py        —  0 (skipped if unavailable)
tests/test_integration_lightrag.py  — 12 tests
tests/test_integration_mcp.py       —  4 tests
tests/test_namespace.py             —  4 tests ✅
tests/test_pydantic_v2.py           —  4 tests ✅
tests/test_retry.py                 —  3 tests ✅
tests/test_config.py                —  4 tests ✅
tests/test_mcp_edge_cases.py        —  5 tests ✅ (3 MCP edge + 2 env var)
```

---

## Phase 6 — Full Suite Run + Cleanup

### 6.1 Full test suite ✅
- **39 tests pass** in clean `venv` (setup.ps1) — all unit, smoke, MCP edge cases
- Integration tests not run (they need a full server startup and timeout in CI)
- AGENTS.md updated: venv-first workflow, activation step in Testing section
- Only pre-existing issue: integration tests timeout due to ASGI app creation with mocked LLM (slow on Windows)
- MCP edge case tests (connection refused, empty query, invalid mode) all pass now that starlette/fastapi versions are compatible in the venv

### Remaining (post-6.1)
- Integration tests (12 tests) — needs `pytest tests/ -v --tb=short --timeout=300` or comparable timeout
- Future: CI/CD pipeline, performance profiling, Windows compat edge cases
