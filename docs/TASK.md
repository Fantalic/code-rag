# Current Task: Phase 10 — Publish-Ready

## Objective
Make the monorepo presentable on GitHub. Fix test suite, establish project identity, clean code quality, and add CI.

## Background
- Phases 1–9 complete — MCP server has 22 tools, C# analyser pipeline works, Docker deployment verified
- Recent changes (Phase 9 fixes) broke ~50 tests by changing response format and error handling style
- Root tests crash entirely due to `sys.exit(1)` in placeholder file
- 624 ruff lint violations exist
- No GitHub CI, no root-level CONTRIBUTING/CHANGELOG/CODE_OF_CONDUCT
- C# analyser binary kept for now (long-term replacement planned)

## Steps

### Phase A — Fix Blockers
- [ ] A1: Commit pending `server.py` changes
- [ ] A2: Fix ~50 test failures across `test_server.py`, `test_client.py`, `test_models.py`, `test_integration.py`
- [ ] A3: Fix `test_e2e_csharp.py` root test crash
- [ ] A4: Remove `test_demo.py` / `test_demo2.py`

### Phase B — Project Identity
- [ ] B1: Rewrite root `README.md` as primary entry point
- [ ] B2: Add `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`

### Phase C — Code Quality
- [ ] C1: Run `ruff --fix` (auto-fixes 383/624 violations)
- [ ] C2: Fix remaining violations (explicit imports, line length, undefined names)

### Phase D — GitHub CI
- [ ] D1: Add `.github/workflows/ci.yml`
- [ ] D2: Add issue/PR templates

### Phase E — Cleanup
- [ ] E1: Consolidate pytest config (pytest.ini vs pyproject.toml)
- [ ] E2: Fix stale doc references (AGENTS.md "Python 3.8+")
- [ ] E3: Clean up placeholder files (test_e2e_csharp.py)

---

## Notes
- Keep commented-out dead code (`update_relation` etc.) — needed for next features
- C# analyser binary stays in repo — long-term plan to replace documented in ROADMAP
- All packages target Python 3.10+
- No PyPI publishing — clean GitHub repo only
