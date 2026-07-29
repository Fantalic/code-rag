# Comprehensive Test Coverage Push

## Current Task

Achieve full test coverage across both the LightRAG Python test suite and the Go main project. All tests must run without Docker (mocked HTTP, pure unit tests).

## Coverage Before

| Area | Coverage | Lines Untested |
|------|----------|---------------|
| LightRAG Python (all modules) | ~4% | ~36,608 / ~38,200 |
| Go main project (all packages) | ~17% | ~2,900 / ~3,250 |
| Go analysers/csharp module | 0% | ~700+ |

## Target Coverage After

| Area | Target |
|------|--------|
| `pkg/watchdog/diff` | 100% (pure logic, no deps) |
| `pkg/watchdog/crud` | 90%+ (mock HTTP server) |
| `pkg/watchdog/filter` | 100% (pure regex) |
| `pkg/watchdog/router` | 100% (simple routing) |
| `pkg/lightrag` | 90%+ (mock HTTP server) |
| `pkg/watchdog/parser` | Already 90%+ (maintain) |
| `pkg/watchdog/pipeline` | Improve integration test fixture |
| `pkg/watchdog/watcher` | Already ~30% (maintain) |
| `analysers/csharp` | 50%+ (core walker/index/scope) |
| `LightRAG/tests/test_utils_graph.py` | New file: adelete_by_entity, merge, cascade |
| `LightRAG/tests/test_storage_backends.py` | New file: KV/vector/doc-status backends |
| `LightRAG/tests/test_api_routes.py` | New file: document/graph route tests |
| `LightRAG/tests/test_lightrag_core.py` | New file: LightRAG class with mocks |

## Todo

- [ ] Go: `pkg/watchdog/diff/engine_test.go` — full diff engine tests
- [ ] Go: `pkg/watchdog/crud/client_test.go` — mock HTTP server tests
- [ ] Go: `pkg/watchdog/filter/security_test.go` — regex filter tests
- [ ] Go: `pkg/watchdog/router/file_test.go` — routing tests
- [ ] Go: `pkg/lightrag/client_test.go` — mock HTTP for lightrag client
- [ ] Go: `analysers/csharp/*_test.go` — walker, scope, index, output
- [ ] Python: `tests/test_utils_graph.py` — adelete_by_entity, merge
- [ ] Python: `tests/test_storage_backends.py` — KV/vector/doc-status backends
- [ ] Python: `tests/test_api_routes.py` — document/graph routes
- [ ] Python: `tests/test_lightrag_core.py` — LightRAG class with mocks
- [ ] Run all tests and fix failures
- [ ] Commit
