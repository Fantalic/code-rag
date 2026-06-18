# Current Context

## Status
- **Phase**: 1 — Making It Run
- **Steps 1.1–1.5**: ✅ Complete (1.3 deferred)
- **Current step**: 1.6 🔄 — Verifiable

---

## What's Been Done

### 1.1 — MCP name clash fix
Renamed `server` → `mcp_server` in `server.py`. Fixed `__init__.py` export. Patch target resolves correctly now.

### 1.2 — Single dev install
`scripts/setup.ps1` — one-command `pip install -e` for all three sub-projects. AGENTS.md updated.

### 1.3 — LightRAG server starts (deferred)
`config.parse_args()` runs at module import (config.py:417). Not blocking mock-based testing.

### 1.4 — MCP connects to LightRAG
`tests/test_integration_mcp.py` — 4 tests prove real `LightRAGClient` works against mock FastAPI server via ASGITransport + lifespan.

### 1.5 — Watchdog ingests via API
Replaced placeholder `ingest_file` with real `httpx.post` to LightRAG's `/documents/text`. Uses `LIGHTRAG_URL` and `LIGHTRAG_API_KEY` env vars.

### Docs
- `docs/` folder with PLAN.md, KEEPTRACK.md, CURRENT_CONTEXT.md, DOCUMENTATION.md
- All `.md` moved from root

---

## Test Suite (25 passing)
```
tests/test_smoke_lightrag.py       — 9 tests (core + API imports)
tests/test_smoke_mcp.py            — 9 tests (imports, models, enums)
tests/test_smoke_watchdog.py       — 0 selected (skipped if watchdog unavailable)
tests/test_integration_lightrag.py — 12 tests (API endpoints)
tests/test_integration_mcp.py      — 4 tests (MCP client through LightRAG)
```

## Documentation Candidates
- Architecture diagram
- Dev setup guide
- Mock-based testing approach
- Known quirks: `config.parse_args()`, Pydantic `file_path` validator bug

## Next Steps
- **1.6**: Start script that launches all 3 services
- **Phase 2**: Strip dead code, flatten configs, simplify

---

## E2E C# Test Results

**Pipeline proven end-to-end**: C# analyser → LightRAG API → query. See `tests/test_e2e_csharp.py`.

### What Works ✅
- C# analyser runs (needs .NET 9), outputs per-file `.txt`
- LightRAG server starts with runtime patches (emoji, frontend, sys.argv)
- `POST /documents/text` ingests files → 200 with track_id
- `GET /documents/status_counts` / `pipeline_status` return correct state
- `POST /query` responds (empty with mocked LLM, real with qwen2.5-coder)
- Real Ollama LLM qwen2.5-coder:1.5b + embedding bge-m3 work after `ollama` upgrade

### Issues Found 🐛
1. **`config.parse_args()` at module import** — `sys.argv` must be set before first config import. File: `lightrag/api/config.py:417`
2. **Emoji crash on Windows cp1252** — `ASCIIColors.green("...🚀...")` fails. Patch: `ASCIIColors.green = lambda t, **k: None`
3. **`check_frontend_build` exits on missing webui** — Patch to no-op for API-only
4. **`DocStatusResponse.file_path` Pydantic bug** — `GET /documents/track_status/{id}` 500 when `file_path` is null. File: `document_routes.py:2563`
5. **`ollama` needs ≥0.4.0 for `.embed()`** — v0.2.0 missing `AsyncClient.embed()`. Upgraded to 0.6.2
6. **Mock functions must be `async def`** — Sync lambdas in `ollama_embed`/`ollama_model_complete` crash with `await`
7. **Doc routes use `/documents/` prefix** — Not `/pipeline_status` but `/documents/pipeline_status`
8. **No analyser→LightRAG auto-pipeline** — Must manually/API ingest output
9. **Infinite retry loop** — Failed documents cycle FAILED→PENDING endlessly

## Fix Sprint — Making It Run Without Runtime Patches

**Goal**: Fix all known bugs so LightRAG server starts cleanly (no patches) and the pipeline is observable. Data extraction (P0) deferred.

### Planned Fixes (in order)

1. **`check_frontend_build` → warn, don't exit** (`lightrag_server.py:148`)
   - Change `sys.exit(1)` → print warning + continue
   - Why: Server should not crash when frontend isn't built. API-only mode is valid.

2. **Emoji crash on Windows cp1252** (`lightrag_server.py:336`)
   - Wrap `ASCIIColors.green("...🚀...")` in a try/except or encode safely
   - Why: Unicode emoji crashes on Windows terminals with cp1252 encoding.

3. **`DocStatusResponse.file_path` Pydantic bug** (`document_routes.py:382`)
   - Make `file_path` field truly optional / default to `""` not `None`
   - Why: `track_status/{id}` returns 500 when `file_path` is null — blocks pipeline monitoring.

4. **`config.parse_args()` module-level guard** (`config.py:417`)
   - Guard with `if not global_args` or make it lazy
   - Why: Must set `sys.argv` before any import — fragile and surprising.

5. **Infinite retry loop on failed docs** (document processing logic)
   - Add max-retry count or circuit breaker in doc status
   - Why: Failed documents spin FAILED→PENDING endlessly and waste LLM quota.

6. **Create `scripts/start_all.ps1`**
   - One-command launcher for LightRAG server with correct env vars
   - Why: Step 1.6 deliverable — verifiable single command.

### After Fixes
- Verify server starts with `python -m lightrag.api.lightrag_server` (no patches)
- Run `tests/test_integration_lightrag.py` to confirm no regressions
- Update PLAN.md step 1.6 to ✅

---

### Next Steps
- Move to Phase 2: strip dead code, flatten configs
