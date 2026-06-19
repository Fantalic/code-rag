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

| Step | Status |
|------|--------|
| 6.1 Run full test suite | ✅ Done — 39/39 pass (unit + smoke + MCP edge) |
| 6.2 Fix regressions | ✅ Done — no regressions, venv fixes starlette conflict |
| 6.3 Build and verify WebUI | ✅ Done — 113 assets, auto-build in setup.ps1 |
| 6.4 Clean up docs | ✅ Done — AGENTS.md venv workflow, current context updated |

## Phase 7 — Analyser Integration into MCP Server

| Step | Status |
|------|--------|
| 7.1-7.3 Inventory, test project, Docker setup | ✅ Done |
| 7.4 Create analyser_runner.py module | ✅ Done (committed Phase 7 at f51ffb7a) |
| 7.5 Wire update_relations to run analyser + ingest into LightRAG | ✅ Done |
| 7.6 Test end-to-end pipeline with real analyser output | ✅ Done (Phase 7 E2E verified) |
| 7.7 Document findings and commit | ✅ Done |

## Phase 8 — Test Fixes & Cleanup

| Step | Status |
|------|--------|
| 8.1 Fix test_analyser_runner.py import + path bugs | ✅ Done |
| 8.2 Fix test_client.py UpdateRelations mock data | ✅ Done |
| 8.3 Fix test_server.py calling convention + tool list | ✅ Done |
| 8.4 Clean up handle_call_tool(self, request) workaround | ✅ Done |
| 8.5 Fix _create_error_response event loop crash | ✅ Done |
| 8.6 Commit Phase 8 | ✅ Done (e32a1acc) |

## Phase 9 — MCP End-to-End Verification

| Step | Status |
|------|--------|
| 9.1 Verify LightRAG Docker running | ✅ Done |
| 9.2 Start MCP server and verify tool listing | ✅ Done |
| 9.3 Test key tools (health, insert, KG, update_relations, query) | ✅ Done |
| 9.4 Verify with AI agent | ✅ Done (via direct tool calls / MCP protocol) |

## Phase 10 — Publish-Ready

Make the monorepo presentable on GitHub: fix test suite, establish project identity, clean code quality, add CI.

| Step | Status |
|------|--------|
| **A — Fix Blockers** | |
| A1 Commit pending server.py changes (title param, response fix, error raising) | ⬜ |
| A2 Fix ~50 test failures (response format + error raising changes) | ⬜ |
| A3 Fix root test crash (test_e2e_csharp.py sys.exit) | ⬜ |
| A4 Remove dev artifacts (test_demo.py, test_demo2.py) | ⬜ |
| **B — Project Identity** | |
| B1 Rewrite root README.md as primary entry point | ⬜ |
| B2 Add CHANGELOG.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md | ⬜ |
| **C — Code Quality** | |
| C1 Run ruff --fix (auto-fixes 383 violations) | ⬜ |
| C2 Fix remaining violations (explicit imports, line length, undefined names) | ⬜ |
| **D — GitHub CI** | |
| D1 Add .github/workflows/ci.yml (tests + lint) | ⬜ |
| D2 Add issue/PR templates | ⬜ |
| **E — Cleanup** | |
| E1 Consolidate pytest config (pytest.ini vs pyproject.toml) | ⬜ |
| E2 Fix stale doc references (AGENTS.md "Python 3.8+") | ⬜ |
| E3 Clean up placeholder files (test_e2e_csharp.py) | ⬜ |
