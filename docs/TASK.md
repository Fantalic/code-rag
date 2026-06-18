# Current Task: Phase 6 — Full Suite Run + Cleanup

## Objective
Run the complete test suite, fix any failures or regressions, and ensure everything works together.

## Steps

### 6.1 — Run full test suite
- [x] `pytest tests/ -v --tb=short` — 39/39 unit + smoke + MCP edge cases pass

### 6.2 — Fix regressions
- [x] No regressions from Phase 5 changes (all 15 new tests pass, old unaffected)
- [x] Starlette/fastapi version conflict resolved by using clean `venv` from setup.ps1

### 6.3 — Build and verify WebUI
- [x] `bun run build` succeeds — 113 assets generated to `lightrag/api/webui/`
- [x] `check_frontend_build()` detects build and prints "Frontend build is up-to-date"
- [x] `setup.ps1` updated: auto-builds webui after Python install (if Bun available)
- [x] WebUI mount at `/webui` no longer needs `lambda: None` patch

### 6.4 — Clean up docs
- [x] AGENTS.md: venv-first workflow, activation in Testing section
- [x] `setup.ps1`: added webui build step
- [x] CURRENT_CONTEXT.md updated
- [ ] Commit remaining changes
