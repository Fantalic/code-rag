# Current Context

## Status
- **Phase**: 10 — Publish-Ready 🔄 In Progress
- **Phases 1–7**: ✅ Complete (all committed)
- **Phase 8**: ✅ Committed at `e32a1acc` (test fixes + calling convention cleanup)
- **Phase 9**: ✅ Committed at `bc6e1463` (dotenv, E2E verification with Docker LightRAG)

---

## What's Been Done

### Phases 1–9 ✅
- Monorepo unified (LightRAG + MCP + watchdog), Pydantic V2, C# analyser pipeline, Docker deployment, MCP E2E verified with 27 nodes / 17 edges in KG
- `insert_text` tool now accepts `title` parameter for dedup-safe inserts
- `_create_success_response` returns raw dicts (MCP library handles wrapping)
- Error handling uses `raise` instead of `return _create_error_response()` (MCP library creates proper error responses)
- Dotenv support via `python-dotenv`

### Phase 10 — Publish-Ready (Starting)

**Phase A (Blockers):**
- [ ] Commit pending `server.py` changes (title parameter, response wrapping fix, error raising)
- [ ] Fix ~50 test failures (response format changes broke test assertions)
- [ ] Fix root test crash (`test_e2e_csharp.py` sys.exit at import time)
- [ ] Remove `test_demo.py` / `test_demo2.py`

**Phase B (Project Identity):**
- [ ] Rewrite root `README.md` as primary entry point
- [ ] Add `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`

**Phase C (Code Quality):**
- [ ] Run `ruff --fix` (auto-fixes 383/624 violations)
- [ ] Fix remaining violations (explicit imports, line length, undefined names)

**Phase D (GitHub CI):**
- [ ] Add `.github/workflows/ci.yml` (tests + lint on push/PR)
- [ ] Add issue/PR templates

**Phase E (Cleanup):**
- [ ] Consolidate pytest config
- [ ] Fix stale doc references (AGENTS.md Python version)
- [ ] Clean up placeholders

---

## Key Technical Decisions

1. **Python 3.10+** — all packages target >=3.10 consistently
2. **Clean GitHub repo only** — no PyPI publishing for now
3. **C# analyser binary kept** — will be replaced long-term (roadmap item)
4. **Dead code preserved** — `update_relation` commented block kept for next feature work
5. **Response format** — handlers return raw dicts, MCP library wraps as `CallToolResult`

---

## Relevant Files

- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/server.py` — pending changes (title param, response wrapping, error raising style)
- `daniel-lightrag-mcp/tests/test_server.py` — tests need updating for new response format
- `daniel-lightrag-mcp/tests/test_client.py` — mock data mismatches
- `daniel-lightrag-mcp/tests/test_models.py` — model field mismatches
- `daniel-lightrag-mcp/tests/test_integration.py` — response format mismatches
- `tests/test_e2e_csharp.py` — placeholder with sys.exit crash
