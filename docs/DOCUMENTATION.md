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
| **Watchdog** | File watcher — detects changes, syncs to graph | Go (standalone binary) | — |

---

## 2. LightRAG

Forked from [HKUDS/LightRAG](https://github.com/HKUDS/LightRAG). Stores entities + relations in a graph DB and vector DB (embeddings).

**Data flow (document pipeline):**
```
POST /documents/text -> background enqueue -> text chunked -> LLM extracts entities+relations -> graph DB + vector DB
```

**Dedup:** LightRAG deduplicates by `file_source` in `doc_status`. If `file_source` already exists, returns `status="duplicated"`. No in-place update. See `document_routes.py:1824-1840`.

**Graph CRUD** bypasses the LLM pipeline — endpoints like `/graph/entity/create` and `/graph/relation/create` operate directly on the graph + vector DB. See [API_REFERENCE.md](API_REFERENCE.md#3-entity-crud-endpoints).

### Project-Based Storage

Each project folder contains a `.coderag/` directory:
```
project-root/
├── .coderag/
│   └── rag_storage/           # LightRAG data (graphml, vdb, kv stores)
├── .env                       # Optional project config (minimal)
└── src/                       # Your code
```

**Detection:** `LightRAG/lightrag/api/config.py:find_project_root()` searches from cwd upward for `.coderag` folder. Falls back to `./rag_storage` (legacy).

**Env var override:** `LIGHTRAG_WORKING_DIR` forces the working directory. Used by watchdog's `start` command to point LightRAG at the correct `.coderag/rag_storage`.

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

### Architecture (Go binary)

The watchdog is a standalone Go CLI binary at `watchdog/watchdog.exe`:

```
watchdog/
├── cmd/watchdog/main.go       # CLI entry (urfave/cli/v2)
├── internal/
│   ├── crud/client.go         # Graph CRUD HTTP client
│   ├── diff/engine.go         # Diff engine (EntityAction, RelationAction)
│   ├── filter/security.go     # Security filter (PII/keys/secrets)
│   ├── router/file.go         # Extension -> analyser mapping
│   ├── parser/csharp.go       # C# analyser output parser
│   ├── pipeline/run.go        # Pipeline orchestration
│   └── watcher/observer.go    # File watcher + worker pool + git watcher
```

### CLI Commands

```bash
watchdog watch <dir>            # Watch project folder (connects to existing LightRAG)
watchdog start <dir>            # Start LightRAG + watchdog together
watchdog status                 # Show running services
watchdog stop                   # Stop all services
watchdog query "<text>"         # Query the graph directly
```

### Dual-path Design

The watchdog supports **two ingestion paths**:

**Path A — Direct graph CRUD (no LLM):**
```
File event -> debounce -> hash check ->
  router -> analyser (structured output) ->
  diff against current graph -> 
  security filter ->
  create/update/delete entities+relations via API
```

**Path B — LLM pipeline via insert_text:**
```
File event -> debounce -> hash check ->
  POST /documents/text with file content ->
  LightRAG LLM pipeline (chunking, entity extraction, embedding)
```

**Routing:** Path A is attempted first. If the file has no registered analyser, falls back to Path B.

### Change Detection

- **Initial sync:** On startup, full scan of input directory, groups files by solution, runs analyser once per solution, queues all files
- **Git watcher:** Polls `git diff --name-only HEAD` every 30 seconds for uncommitted changes (debounce mechanism)
- **File hash tracking:** MD5 hash comparison skips unchanged content between poll cycles

### Analyser Scope

The C# analyser operates at **solution level** (requires `.sln` file). When any file in a solution changes, the entire solution is re-analysed. The diff engine then determines which entities/relations to create, update, or delete. This ensures cross-file relations (inheritance, interface implementations) are always consistent.

### C# Entity Types Extracted

| Type | Example | Relation |
|------|---------|----------|
| CLASS | `Calculator` | defines METHOD, has PROPERTY |
| METHOD | `Calculator.Add()` | belongs_to CLASS |
| PROPERTY | `User.Email` | has CLASS |
| INTERFACE | `IRepository<T>` | implemented_by CLASS |
| INHERITS_FROM | AdvancedCalculator → Calculator | extends CLASS |
| DEPENDS_ON | Calculator → IOperation | implements INTERFACE |
| CALLS | MethodA → MethodB | cross-file call reference |

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
| Test project | `../test-csharp-project/` (outside repo) | C# + git |

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
| `LIGHTRAG_PORT` | `9621` | Server port |
| `LIGHTRAG_WORKING_DIR` | auto-detect `.coderag/rag_storage` | Storage path |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama endpoint |
| `INPUT_DIR` | positional arg | Watchdog monitor path |
| `DEBOUNCE_SECONDS` | `0.5` | Per-file debounce |
| `WATCHER_WORKERS` | `4` | Thread pool size |

---

## 8. Filesystem Layout

```
code-lightrag/
├── LightRAG/lightrag/
│   ├── api/
│   │   ├── config.py                    <- find_project_root(), get_default_working_dir()
│   │   ├── utils_api.py                 <- check_env_file() (always returns True)
│   │   ├── lightrag_server.py           <- FastAPI server entry point
│   │   └── routers/
│   │       ├── document_routes.py       <- Doc CRUD + dedup
│   │       └── graph_routes.py          <- Entity/relation CRUD endpoints
│   ├── kg/
│   │   └── networkx_impl.py            <- GraphML write with None filtering
│   └── constants.py                    <- Code/software entity types
│
├── daniel-lightrag-mcp/src/daniel_lightrag_mcp/
│   ├── server.py                       <- 22 MCP tools
│   ├── client.py                       <- HTTP client + C# analyser runner
│   └── analyser_runner.py              <- Subprocess runner for analyser binary
│
├── watchdog/
│   ├── cmd/watchdog/main.go            <- CLI entry
│   ├── internal/
│   │   ├── crud/client.go              <- Graph CRUD HTTP client
│   │   ├── diff/engine.go              <- Diff engine
│   │   ├── filter/security.go          <- Security filter
│   │   ├── router/file.go              <- Extension -> analyser
│   │   ├── parser/csharp.go            <- C# analyser parser
│   │   ├── pipeline/run.go             <- Pipeline orchestration
│   │   └── watcher/observer.go         <- File watcher + git watcher
│   └── watchdog.exe                    <- Compiled binary
│
├── docs/                               <- Documentation
│
├── scripts/
│   └── init-project.ps1               <- Project initialization script
│
└── ../test-csharp-project/             <- Test project (separate git repo)
    ├── .coderag/rag_storage/           <- LightRAG storage
    ├── TestCodebase.sln
    ├── TestLib/
    │   ├── Calculator.cs
    │   ├── AdvancedCalculator.cs
    │   ├── DataStore.cs
    │   ├── ExtendedTypes.cs
    │   └── Class1.cs
    └── TestConsole/
        └── Program.cs
```

---

## 9. Known Issues

| Issue | Status | Notes |
|-------|--------|-------|
| GraphML NoneType error | ✅ FIXED | None values filtered in `write_nx_graph()` and `upsert_node/edge()` |
| .env requirement | ✅ FIXED | `check_env_file()` always returns True (single-instance) |
| LIGHTRAG_WORKING_DIR not set | ✅ FIXED | Watchdog passes it in `start` command |
| Git watcher detects uncommitted only | ⚠️ BY DESIGN | Polls `git diff HEAD` every 30s. New commits need restart. |
| Old entities not cleaned up on file change | ⚠️ PARTIAL | Modified files delete old entities first. No periodic cleanup. |
