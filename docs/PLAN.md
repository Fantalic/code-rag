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

## Phase 5 — Own Tests
Targeted test suite for the unified codebase.

## Phase 6 — Polish
Performance profiling, Windows compat, CI/CD.
