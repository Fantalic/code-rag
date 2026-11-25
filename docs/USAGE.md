# codeRAG — Usage & CLI Reference

> The complete day-to-day workflow and every command, flag, and exit behavior of the
> `coderag` binary. Install first → [INSTALL.md](INSTALL.md).

---

## 1. Command overview

| Command | Purpose |
|---------|---------|
| [`coderag start [dir]`](#coderag-start) | Start Docker + file watcher (foreground) |
| [`coderag scan [dir]`](#coderag-scan) | One-time re-index of a supported project (.sln, build.zig, or Python) |
| [`coderag mcp [dir]`](#coderag-mcp) | Start the MCP stdio↔SSE bridge for AI agents |
| [`coderag graph files/deps/cycles`](#coderag-graph) | File-level derived views of the graph |
| [`coderag query "<text>"`](#coderag-query) | Natural-language question (LLM) |
| [`coderag watch [dir]`](#coderag-watch) | File watcher only (no Docker orchestration) |
| [`coderag webui`](#coderag-webui) | Open the browser UI |
| [`coderag status`](#coderag-status) | Health of API, MCP, workspace |
| [`coderag stop`](#coderag-stop) | `docker compose down` |

Global flags (apply to every command):

| Flag | Meaning |
|------|---------|
| `--quiet` | Suppress informational output; show only errors and results |
| `--version` | Print the build version |
| `--agent-help` | Print a compact CLI guide for AI agents (commands, MCP tools, docs) and exit |

If `<dir>` is omitted, the current directory is used. All paths are resolved to
absolute paths before use.

---

## 2. coderag start

```bash
coderag start ./myproject
```

1. Ensures Docker services are healthy for the project (starts them if not; the
   workspace is derived from the directory name).
2. Runs an initial git-based sync (changed/deleted files vs `HEAD`).
3. Starts a live watcher in the foreground.
4. On `Ctrl+C`: stops the watcher; **Docker keeps running**.

The workspace name is the directory's base name (spaces → underscores). It must
match between runs — `coderag` auto-restarts the stack if the container is on a
different workspace.

## 3. coderag scan

```bash
coderag scan ./myproject          # index (creates/updates/deletes what changed)
coderag scan ./myproject --clean  # wipe graph + analyser output, then index
```

`--clean` / `-c` clears the project's analyser output and calls LightRAG's
`DELETE /documents` before scanning. Use it when entities look stale or you changed
the analyser.

What a scan does:

1. Detect the project type and root:
   - `.sln` → C# analyser
   - `build.zig` → Zig analyser (walk up from the changed file)
   - Python sources → Python `ast` analyser
2. Run the matching analyser → `.coderag/analyser_output/*.txt`.
3. Parse format-2 output into entities + relations (including source spans and sidecars).
4. Diff against the current graph (content-derived IDs → re-scans are idempotent).
5. Apply creates/updates/deletes via a single batched import.
6. Print per-language counts, then `Scan complete. Knowledge graph now contains N entities`.

## 4. coderag mcp

```bash
coderag mcp ./myproject
```

The connection point for MCP agents:

1. Ensure MCP SSE is healthy at `:9622` (start Docker if needed, wait ≤ 60 s).
2. Start the file watcher in the background (so the graph stays live).
3. Bridge stdin/stdout (JSON-RPC) ↔ SSE. Blocking; agents control its lifecycle.

Run it in the foreground of the session your agent launches — opencode, Claude
Desktop, Cursor, etc. run this exact command for you. See [MCP_CLIENTS.md](MCP_CLIENTS.md).

## 5. coderag graph

File-level views, derived on demand from the knowledge graph (no extra storage).

```bash
coderag graph files                          # FILE  NODES  FAN-IN  FAN-OUT
coderag graph deps ./src/Services/OrderService.cs   # references + dependents + entities
coderag graph cycles                         # cycles or "No dependency cycles found."
```

Common flags:

| Flag | Meaning |
|------|---------|
| `-w, --workspace <name>` | Query a specific workspace (default: currently active) |
| `-u, --url <url>` | LightRAG API URL (default `http://localhost:9621`, env `LIGHTRAG_URL`) |

`graph deps` resolves partial file names (substring match) — `coderag graph deps
OrderService.cs` works even from a different directory. Cycles are computed with
Tarjan SCC over the file graph and printed as `A -> B -> A`.

## 6. coderag query

```bash
coderag query "How does order checkout work?"
```

The **only LLM-backed command** — sends the question to LightRAG's `hybrid` mode,
which retrieves graph context and lets Ollama compose an answer. Needs:

- The configured `LLM_MODEL` (default `phi3.5:latest`) served by your Ollama — either
  a local model you pulled, or an Ollama cloud model such as `qwen3-coder:480b-cloud`
  (no download; works out of the box with free tokens). Change models with
  `LLM_MODEL` env var — see [INSTALL.md](INSTALL.md#2-ollama-models).
- Ollama reachable from the Docker container (see [INSTALL.md](INSTALL.md#6-configuration)).

Structured retrieval (entities/relations/chunks/references) is available to
agents via the `query_codebase` MCP tool instead.

## 7. coderag watch

```bash
coderag watch ./myproject                    # watcher only (no Docker orchestration)
coderag watch --debounce 1.0 ./myproject     # 1 s debounce (default 0.5)
coderag watch --workers 8 ./myproject        # 8 workers (default 4)
coderag watch --url http://other:9621 ./x    # different LightRAG instance
```

| Flag | Default | Env | Meaning |
|------|---------|-----|---------|
| `-d, --debounce` | `0.5` | `DEBOUNCE_SECONDS` | seconds to wait after a file event |
| `-w, --workers` | `4` | `WATCHER_WORKERS` | worker pool size |
| `-u, --url` | `http://localhost:9621` | `LIGHTRAG_URL` | LightRAG API base URL |

Use `watch` when you manage Docker yourself and just want the watcher.

## 8. coderag webui

```bash
coderag webui
```

Opens `http://localhost:9621` in your default browser. Fails with a hint if LightRAG
isn't running (`coderag start <dir>` first).

## 9. coderag status

```bash
coderag status
#   codeRAG v1.x
#   LightRAG API:   running
#   MCP SSE:        running
#   Workspace:      myproject
#   WebUI:          http://localhost:9621
```

## 10. coderag stop

```bash
coderag stop
```

Runs `docker compose down` on the repo's compose file (found via `findComposeFile`).
The graph data in the `rag_data` volume is preserved.

---

## 11. Day-to-day workflow

**Setup once:**

```bash
coderag scan ./MySolution
```

**While coding (option A — watcher + WebUI):**

```bash
coderag start ./MySolution
coderag webui          # in another terminal
```

**While coding (option B — ask your agent):**

```bash
coderag mcp ./MySolution   # launched for you by opencode / Claude Desktop / Cursor
```

**Weekly hygiene:**

```bash
coderag graph cycles                     # find architecture smells
coderag scan ./MySolution --clean        # reconcile the graph after big refactors
```

---

## 12. Direct HTTP API

MCP tools are wrappers over the REST API on `:9621` — scripts can call it directly:

| MCP tool | HTTP call |
|----------|-----------|
| `exists_in_codebase` | `GET /graph/entity/exists?name=NAME` |
| `get_nodes` / `node_details` / `codebase_overview` | `GET /graphs?label=*&max_depth=N&max_nodes=10000` + client-side filter |
| `file_dependencies` / `detect_circular_dependencies` / `dead_code` | `GET /graphs?label=*` → derived client-side |
| `query_codebase` | `POST /query/data` `{"query": "...", "mode": "hybrid"}` |
| `attach_note_to_node` | `POST /graph/entity/create` (+ relation) |

Example:

```bash
curl "http://localhost:9621/graph/entity/exists?name=TestApp.OrderService"
```

## 13. Exit codes & logging

- Most commands exit non-zero with an explanatory message on failure (e.g. missing
  directory, no `.sln`, LightRAG start timeout after 120 s).
- `mcp` and `start` are long-running; they only return on signal or error.
- Docker startup logs are captured to `<project>/.coderag/docker-logs/` after a
  successful start.

---

Next: **[MCP_CLIENTS.md](MCP_CLIENTS.md)** · **[VS.md](VS.md)** · back to [README](../README.md)
