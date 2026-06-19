# 🔥 Phase 11 — Efficient Incremental Graph Updates

## The Vision

A watchdog that watches your codebase and updates the knowledge graph through **two paths**:

```
Path A (direct graph CRUD, no LLM):
  File change → file_router → analyser → diff → security_filter → graph CRUD

Path B (LLM pipeline via insert_text):
  File change → POST /documents/text → LLM extraction → graph
```

- **Path A** — only changed entities/relations get updated. No LLM calls for structured analyser output.
- **Path B** — kept intentionally. LLM re-parsing adds value for complex/unstructured files.
- **Security filter** strips PII/keys before anything reaches the graph.

## 11.1 ✅ Done — File hash tracking + dedup fix

- `watchdog/main.py`: MD5 hash cache skips unchanged files
- Versioned file_source (`file#v1`, `file#v2`) bypasses LightRAG dedup on re-ingest
- Cache cleanup on file deletion

## 11.3 ✅ Done — Generic graph diff pipeline

### Architecture

```
watchdog/main.py
  │ file change event (path, change_type)
  ▼
watchdog/file_router.py       ← routes by extension
  │ .cs → C# analyser
  │ .py/.js/etc → ignored (future: Zig)
  ▼
watchdog/graph_diff.py         ← old vs new entities → CRUD actions
watchdog/security_filter.py    ← strips PII/keys/secrets
watchdog/graph_crud.py         ← calls LightRAG entity/relation endpoints
  ▼
LightRAG Graph (no LLM pipeline)
```

### Steps

| # | Step | Status |
|---|------|--------|
| 1 | Read LightRAG graph CRUD API request/response formats | ✅ |
| 2 | Create `watchdog/graph_crud.py` — HTTP client for entity/relation CRUD | ✅ |
| 3 | Create `watchdog/graph_diff.py` — diff engine (old vs new → CRUD actions) | ✅ |
| 4 | Create `watchdog/security_filter.py` — regex patterns for PII/keys/secrets | ✅ |
| 5 | Create `watchdog/file_router.py` — extension → analyser mapping | ✅ |
| 6 | Wire pipeline into `watchdog/main.py` (fallback to old POST for unsupported) | ✅ |
| 7 | Connect C# analyser output to graph CRUD (bypass `insert_text`) | ✅ |

### Pipeline behavior

- **Supported files** (e.g. `.cs`): routed to analyser → entities+relations extracted → diffed against graph → security filtered → graph CRUD applied
- **Unsupported files**: falls back to old-style `POST /documents/text` (LLM pipeline)
- The pipeline skeleton is wired; step 7 will add structured entity extraction from the C# analyser output

### Security filter patterns (must catch)

| Pattern | Example | Risk |
|---------|---------|------|
| API keys | `sk-...`, `ghp_...`, `AKIA...` | High |
| Passwords | `password=...`, `PASS=...` | High |
| Tokens | `token=...`, `eyJ...` (JWT) | High |
| Emails | `user@domain.com` | Medium |
| IPs | `192.168.x.x`, `10.x.x.x` | Medium |
| Private keys | `-----BEGIN RSA PRIVATE KEY-----` | Critical |

## Key Files

- `watchdog/main.py` — entry point, already has hash tracking
- `watchdog/` — new modules go here
- `LightRAG/lightrag/api/routers/graph_routes.py` — graph CRUD endpoints
- `LightRAG/lightrag/api/routers/document_routes.py` — document dedup logic
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/client.py` — reference for HTTP calls

## Verify

- `pytest tests\` → 63 passed
- `pytest daniel-lightrag-mcp\tests\` → 163 passed
- Manual: edit a supported file → watchdog detects change → routes through pipeline (Path A or fallback to Path B)
