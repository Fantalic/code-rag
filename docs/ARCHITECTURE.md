# Architecture

> A user-oriented explanation of how codeRAG is put together and why. For API request/
> response details see the repo's private `_dev_docs/API_REFERENCE.md`.

---

## 1. The big picture

```
[Your Code] → [C# analyser (Go)] → [coderag CLI (Go)] → [LightRAG API] → [Knowledge Graph]
                                                              ▲
[Your AI agent] → [MCP client] → [coderag mcp (stdio↔SSE)] → [MCP server (Python)]
```

Two kinds of processes:

**On the host (native Go binary `coderag`):**

- File watching (fsnotify + a 30 s git poll) — needs low latency + direct disk access.
- Spawning the C# analyser subprocess.
- Docker orchestration (`docker compose up/down`, health checks).
- The MCP stdio↔SSE bridge for desktop agents.

**In Docker (long-running Python services):**

- `lightrag-server` — the graph + vector store, the REST API, and the WebUI.
- `coderag-mcp` — the 10 MCP tools over SSE.

**Ollama** runs on the host and is reached by LightRAG over HTTP.

## 2. Data flow: how a `.cs` file becomes graph nodes

```
Save OrderService.cs
  → fsnotify event → debounce 0.5s → MD5 hash check (skip if unchanged)
  → router: .cs → C# analyser
  → csharp_analyser scan --solution X.sln   (whole solution, deterministic)
  → .coderag/analyser_output/*.txt          (pipe-delimited, human-readable)
  → parser → entities + relations
  → security filter (PII/keys → [REDACTED])
  → diff vs current graph (content-derived IDs → idempotent)
  → batched HTTP import into LightRAG       (one save, ~30x faster than per-item)
```

Non-code files (markdown, yaml, txt, …) take a different path: their content is sent
to LightRAG's LLM pipeline (`/documents/text`), which chunks and extracts entities
with Ollama. This is opt-in and only for prose.

## 3. Why the host/Docker split

| Component | Runs on | Because |
|-----------|---------|---------|
| `coderag` CLI | host | fsnotify needs low latency; the analyser spawns subprocesses; the MCP bridge connects host agents |
| LightRAG API | Docker | long-running service; containerization = isolation + reproducible env |
| MCP server | Docker | sits next to LightRAG; talks to it over an internal network |
| WebUI | Docker | served by LightRAG's static files |
| Ollama | host | GPU access is simplest without GPU passthrough |

## 4. Change detection

Three signals keep the graph accurate:

1. **Initial sync (startup)** — git-based: files that differ from `HEAD`, plus deleted
   files.
2. **fsnotify** — real-time create/write/remove/rename events.
3. **Git poll (30 s)** — catches branch switches and anything fsnotify missed.

Deletions **cascade**: removing a file queries the graph for entities in that file,
finds every file whose entities referenced them, re-analyses those files, and then
removes the deleted entities + relations from both the graph and the vector store.

## 5. The C# analyser

A standalone Go binary (`csharp_analyser`) driven by `coderag scan`:

- Runs at **solution level** — re-parses the whole `.sln`, and the diff engine decides
  what actually changed (cross-file relations stay consistent).
- Emits pipe-delimited text per source file:
  - `# EXPORTS:` — all public symbols in the file (used for incremental re-analysis).
  - `# MTIME:` — unix file mtime (feeds `dead_code`'s age filter).
  - `CLASS|`, `METHOD|`, `PROPERTY|`, `INTERFACE|`, `STRUCT|`, `ENUM|` lines, each with
    a `[public]`/`[private]`/`[internal]`/`[protected]` visibility suffix.
  - One `EXPORTS` entity per file is also written to the graph, carrying the export
    list + mtime, so the MCP container (which has no access to analyser files) can
    evaluate `dead_code`.

## 6. The knowledge graph

LightRAG stores two views of the same data:

- **Graph store** — nodes (entities) + edges (relations) with properties.
- **Vector store** — embeddings keyed by content hash, enabling semantic retrieval.

Each entity records `entity_id`, `entity_type`, `description`, `source_id`, and
`file_path`. Relation keywords carry the semantic type (`inheritance, extends`,
`implements, interface`, `calls`, `property, has`, `contains, nested`, …).

Per-project storage: `<project>/.coderag/rag_storage` (auto-detected by walking up
the directory tree; overridable via `WORKSPACE`/`LIGHTRAG_WORKING_DIR`).

## 7. The MCP server

FastMCP over SSE. Each tool:

- is deterministic,
- wraps one HTTP call or a client-side derivation over `GET /graphs`,
- returns `{"tool": ..., "ok": true, "result": ...}` or `{"ok": false, "error": ...}`.

The `query` tool (LLM answer) exists but is deliberately **not registered** — it's
CLI-only (`coderag query`).

## 8. Repository layout

```
code-lightrag/
├── coderag/
│   ├── cli/                      Go CLI + analyser
│   │   ├── main.go               command definitions, Docker orchestration, SSE bridge
│   │   ├── watcher/  diff/  pipeline/  parser/  router/  filter/  graph/  crud/
│   │   └── analysers/csharp/     C# analyser (standalone binary)
│   ├── lightrag/                 LightRAG fork (Python, Docker image source)
│   └── mcp/                      Python MCP server (Docker, :9622)
├── scripts/setup.{sh,ps1}        one-shot installers
├── scripts/dev/                  diagnostics + runtime-test.sh smoke test
├── Dockerfile, docker-compose.yml
├── README.md, DOCUMENTATION.md, docs/
└── _dev_docs/                    private working docs (gitignored)
```

## 9. Ports & services

| Service | Port | Notes |
|---------|------|-------|
| LightRAG API + WebUI | 9621 | `GET /health` for liveness |
| MCP SSE | 9622 | `GET /sse` + `POST /messages/?session_id=…` |
| Ollama | 11434 | host, not Docker |

---

Back to [README](../README.md) · [DOCUMENTATION.md](../DOCUMENTATION.md)
