# CodeLightRAG — System Documentation

> Everything about how this project works: components, data flow, pipeline, and security.
> For detailed API request/response schemas, see [API_REFERENCE.md](API_REFERENCE.md).

---

## 1. Overview

```
[Your Code] --> [Watchdog] --> [LightRAG Graph] <--> [MCP Server] <--> [AI Agent]
```

| Component | Role | Language | Port |
|-----------|------|----------|------|
| **LightRAG** | RAG engine — entities/relations as a knowledge graph, LLM-powered queries | Python (FastAPI) | `:9621` |
| **MCP Server** | 22-tool bridge — AI agents call via Model Context Protocol | Python | stdio |
| **Watchdog** | File watcher — detects changes, syncs to graph | Python | — |

---

## 2. LightRAG

Forked from [HKUDS/LightRAG](https://github.com/HKUDS/LightRAG). Stores entities + relations in a graph DB and vector DB (embeddings).

**Data flow (document pipeline):**
```
POST /documents/text -> background enqueue -> text chunked -> LLM extracts entities+relations -> graph DB + vector DB
```

**Dedup:** LightRAG deduplicates by `file_source` in `doc_status`. If `file_source` already exists, returns `status="duplicated"`. No in-place update. See `document_routes.py:1824-1840`.

**Graph CRUD** bypasses the LLM pipeline — endpoints like `/graph/entity/create` and `/graph/relation/create` operate directly on the graph + vector DB. See [API_REFERENCE.md](API_REFERENCE.md#3-entity-crud-endpoints).

---

## 3. MCP Server

Wraps LightRAG's HTTP API into 22 MCP tools. The key tool is `update_relations`:

```
Agent calls update_relations(path)
  -> locates CSharpAnalyzerTool.exe (needs .NET 9)
  -> finds .sln file (scans directory if needed)
  -> runs analyser subprocess -> .txt output per class
  -> ingests each .txt via insert_text() -> LLM re-parses structured output
```

The `insert_text` path (LLM parsing) is **kept intentionally** for cases where LLM re-analysis adds value. Both paths are available — the watchdog can use graph CRUD directly or fall back to insert_text.

---

## 4. Watchdog Pipeline

### Dual-path design

The watchdog supports **two ingestion paths**:

**Path A — Direct graph CRUD (no LLM):**
```
File event -> debounce -> hash check ->
  file_router.py (ext -> analyser) ->
  analyser (structured output) ->
  graph_diff.py (diff against current graph) ->
  security_filter.py (strip PII/keys/secrets) ->
  graph_crud.py (create/update/delete entities+relations)
```

**Path B — LLM pipeline via insert_text:**
```
File event -> debounce -> hash check ->
  POST /documents/text with file content ->
  LightRAG LLM pipeline (chunking, entity extraction, embedding)
```

**Routing:** Path A is attempted first. If the file has no registered analyser or the analyser returns unstructured output, the watchdog falls back to Path B. This keeps both approaches available — LLM parsing stays useful for cases that benefit from it.

**File hash tracking:** `FILE_HASHES` (MD5) skips unchanged content. Versioned `file_source#v{N}` bypasses LightRAG dedup on re-ingest.

**Pipeline modules:**
- `file_router.py` — extension → analyser mapping (`.cs` → c_sharp; rest → unsupported)
- `graph_diff.py` — diffs new entities/relations against current graph, produces CREATE/UPDATE/DELETE actions
- `security_filter.py` — strips PII, keys, secrets before graph insertion
- `graph_crud.py` — HTTP client for LightRAG entity/relation CRUD endpoints

---

## 5. Security

| Category | Matches |
|----------|---------|
| API keys | `sk-...`, `pk-...`, `ghp_...`, `AKIA...` |
| Auth tokens | `Bearer ...`, JWT (`eyJ...`), `token=` |
| Passwords | `password=`, `passwd=`, `PASS=`, `secret=` |
| Private keys | `-----BEGIN ... PRIVATE KEY-----` |
| PII | Emails, IPv4/IPv6 addresses |
| Internal paths | `\\server\share`, `//internal/` |
| Env secrets | Values of vars named `*SECRET*`, `*TOKEN*` |

Runs **after** extraction, **before** graph insertion. Matched values -> `[REDACTED]`.

---

## 6. Test Infrastructure

| Suite | Location | Count |
|-------|----------|-------|
| Root | `tests/` | 63 |
| MCP | `daniel-lightrag-mcp/tests/` | 163 |

**Gotchas:**
- No `tests/__init__.py` — session-scoped async fixture + package marker = deadlock
- `asyncio_default_test_loop_scope = session` in `pytest.ini` — prevents `asyncio.Lock` deadlock
- MCP handlers return raw `dict`, not `CallToolResult`

---

## 7. Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `LIGHTRAG_URL` | `http://localhost:9621` | API address |
| `LIGHTRAG_API_KEY` | `None` | API auth (`X-API-Key` header) |
| `INPUT_DIR` | `./rag_input` | Watchdog monitor path |
| `DEBOUNCE_SECONDS` | `0.5` | Per-file debounce |
| `WATCHER_WORKERS` | `4` | Thread pool size |

---

## 8. Filesystem Layout

```
LightRAG/lightrag/
  api/routers/document_routes.py  <- Doc CRUD + dedup
  api/routers/graph_routes.py     <- Entity/relation CRUD endpoints
  utils_graph.py                  <- acreate_entity, acreate_relation,
                                     aedit_entity, aedit_relation,
                                     adelete_by_entity, adelete_by_relation,
                                     amerge_entities (with line numbers)
  kg/shared_storage.py            <- graph_db_lock

daniel-lightrag-mcp/src/daniel_lightrag_mcp/
  server.py                       <- 22 MCP tools
  client.py                       <- HTTP client + C# analyser runner
  analyser_runner.py              <- Subprocess runner for analyser binary
  analysers/c_sharp/              <- CSharpAnalyzerTool.exe

watchdog/
  main.py                  <- File watcher + hash tracking + pipeline
  graph_crud.py            <- Entity/relation CRUD HTTP client
  security_filter.py       <- PII/keys/secrets redaction
  file_router.py           <- Extension -> analyser mapping
  graph_diff.py            <- Old vs new diff -> CRUD actions
  analyser_parsers.py      <- C# analyser .txt -> entity/relation dicts
```
