# CodeLightRAG — Project Plan

## Big Picture
Transform a forked monorepo (LightRAG + MCP server + watchdog) into a **professional, unified, ownable product**. Clean code, good docs, solid tests, fast performance. No orphaned forks — one cohesive project.

---

## Phase 1 — Making It Run
Get all three components working together as a single system.

| Step | Status |
|------|--------|
| 1.1 Fix MCP name-clash bug (`server` → `mcp_server`) | ✅ Done |
| 1.2 Single dev installation recipe (`scripts/setup.ps1`) | ✅ Done |
| 1.3 LightRAG server starts (with mocked LLM fallback) | ⏸️ Deferred — blocked by `config.parse_args()` at module level |
| 1.4 MCP server connects to LightRAG | ✅ Done — 4 integration tests pass |
| 1.5 Watchdog ingests via API | ✅ Done — `ingest_file` now HTTP POSTs to LightRAG |
| 1.6 Verifiable — one command starts all, tests pass | ✅ Done — `scripts/start_all.ps1`, server starts without patches, 11/12 integration tests pass |

## Phase 2 — Strip & Simplify
Remove dead code, flatten duplicate configs, drop orphaned scripts.

| Step | Status |
|------|--------|
| Fix .cs/.rel broken extension bug | ✅ Done |
| Fix DEFAULT_WOKERS typo | ✅ Done |
| Remove dead AGEStorage + ChromaVectorDBStorage entries | ✅ Done |
| Remove duplicate requirements-offline-*.txt (4 files) | ✅ Done |
| Remove legacy setup.py shim | ✅ Done |
| Consolidate 6 .gitignore files → 1 root | ✅ Done |

## Phase 3 — Unify
Single build system, shared config, consistent imports.

| Step | Status |
|------|--------|
| 3.1 Single build system (root pyproject.toml) | ✅ Done |
| 3.2 Shared config (env vars + .env.example) | ✅ Done |
| 3.3 Consistent imports (codelightrag namespace) | ✅ Done |
| 3.4 Dependency unification (MCP/watchdog pyproject) | ✅ Done |

## Phase 4 — Modernise
Python 3.10+ features, Pydantic V2, clean async patterns.

| Step | Status |
|------|--------|
| 4.1 Pydantic V2 ConfigDict (17 blocks, 0 warnings) | ✅ Done |
| 4.2 Python 3.10+ type annotations (271 auto-fixes) | ✅ Done |
| 4.3 Async patterns (TaskGroup) | ⏸️ Deferred — low impact, risky |
| 4.4 Match/case | ⏸️ Deferred — cosmetic only |

## Phase 5 — Own Tests
Targeted test suite for the unified codebase.

| Step | Status |
|------|--------|
| 5.1 Test codelightrag namespace | ✅ Done (test_namespace.py — 4 tests) |
| 5.2 Test Pydantic V2 migration | ✅ Done (test_pydantic_v2.py — 4 tests) |
| 5.3 Test retry logic | ✅ Done (test_retry.py — 3 tests) |
| 5.4 Test config module | ✅ Done (test_config.py — 4 tests) |
| 5.5 Test MCP ↔ LightRAG integration | ✅ Done (test_mcp_edge_cases.py — 3 edge cases) |
| 5.6 Test env var unification | ✅ Done (test_mcp_edge_cases.py — 2 env var tests) |

## Phase 6 — Polish
Performance profiling, Windows compat, CI/CD.
