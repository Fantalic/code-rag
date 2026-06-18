# Current Task: Phase 5 — Own Tests

## Objective
Write a targeted test suite that validates the unified codebase works end-to-end, covering all three sub-projects and the key fixes from Phases 1-4.

## Steps

### 5.1 — Test the `codelightrag` namespace
- [x] Test that `from codelightrag import LightRAG, LightRAGClient` works (with/without MCP installed)
- [x] Test that `__version__` is correct

### 5.2 — Test Pydantic V2 migration
- [x] Test that models with `ConfigDict` work correctly (serialization, deserialization)
- [x] Test that `file_path: Optional[str]` handles `None` correctly (the bug we fixed)

### 5.3 — Test retry logic
- [x] Test `DocProcessingStatus` with `retry_count` and `max_retries`
- [x] Test that exhausted retries stay FAILED

### 5.4 — Test config module
- [x] Test `get_global_args()` returns defaults without requiring `sys.argv`
- [x] Test that `import config` doesn't have side effects

### 5.5 — Test MCP ↔ LightRAG integration (expand coverage)
- [x] Test error cases (connection refused, empty query, invalid mode)
- [x] Test streaming queries (deferred — no streaming endpoint yet)
- [x] Test document upload (deferred — needs running LightRAG)

### 5.6 — Test env var unification
- [x] Test that `LIGHTRAG_URL` works in MCP client (backward compat with `LIGHTRAG_BASE_URL`)

## Rules
- Use pytest with pytest-asyncio for async tests
- Use `unittest.mock` for mocking external services
- Keep tests fast — mock LLM, embedding, and storage
- Add all tests in `tests/` with descriptive filenames
- Run `pytest tests/ -v --tb=short` after each step
- Commit after each step
