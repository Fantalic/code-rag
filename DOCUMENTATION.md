# codeRAG — User Documentation

> The complete guide to installing, running, and using codeRAG: what it does, how to
> set it up, how to work with it day-to-day, and how to fix things when they break.
>
> Quick start → [README.md](README.md) · Full CLI reference → [docs/USAGE.md](docs/USAGE.md)
> · Install details → [docs/INSTALL.md](docs/INSTALL.md) · Agent wiring → [docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md)

---

## 1. What codeRAG does

codeRAG turns a source-code folder into a **live, queryable knowledge graph**:

- **Entities** — namespaces, classes, interfaces, structs, enums, methods, properties.
- **Relations** — inheritance, interface implementation, method calls, property
  ownership, namespace containment, cross-file dependencies.

Everything runs **locally by default** — no cloud APIs, no API keys. C# code is analysed by a
native Go static analyser: the graph is deterministic and faithful to your source. An LLM
(Ollama) is only used for optional, opt-in features:
natural-language queries (`coderag query`) and ingestion of non-C# prose files. The
LLM can be a local model or, if you want stronger answers, an Ollama **cloud** model
with a small free-token allowance (see §3.1).

Once indexed, you can:

- **Browse the WebUI** at `http://localhost:9621` — visual graph exploration.
- **Ask an AI agent** — 10 MCP tools let opencode/Claude/Cursor reason over the graph.
- **Use the CLI** — `coderag graph files|deps|cycles`, `coderag query`, `coderag scan`.
- **Keep it current** — the watcher re-indexes files as you save them.

---

## 2. Concepts

### Entities and relations

The graph stores *symbols*, not text. An entity is a named symbol (`TestApp.OrderService`),
a relation is a typed edge between two entities (`TestApp.OrderService.PlaceOrder --calls-->
TestApp.OrderRepository.Find`). Every entity records which file it came from, its
description, and — for C# — its visibility (`public`/`private`/`internal`/`protected`).

### Two ingestion paths

| Path | Files | LLM? | Used for |
|------|-------|------|----------|
| **A — Direct graph CRUD** | `.cs` (via the C# analyser) | ❌ never | C# solutions with a `.sln` |
| **B — LightRAG pipeline** | everything else | ✅ opt-in | markdown, yaml, txt, … |

Path A is deterministic and free. Path B sends file contents through LightRAG's
LLM pipeline (chunking → entity extraction → embeddings) and only runs when you
explicitly feed it non-C# files.

### Storage per project

Each project keeps its data in a `.coderag/` folder next to your code:

```
myproject/
├── .coderag/
│   ├── rag_storage/            # the knowledge graph + vector data
│   ├── analyser_output/        # C# analyser output (.txt, debuggable)
│   └── docker-logs/            # captured service logs after startup
└── src/                        # your code
```

codeRAG finds this folder by searching upward from the working directory; the active
workspace is reported by `coderag status`.

---

## 3. Install

### 3.1 Requirements

| Tool | Min version | Needed for |
|------|-------------|------------|
| Go | 1.25+ | Building the CLI (not needed at runtime) |
| Docker (Desktop on Win/mac) | 24+ | Running LightRAG + MCP services |
| Ollama | 0.3+ | Local LLM + embeddings |
| A `.NET` project | any | A solution file (`.sln`) for C# indexing |

Two kinds of models:

- **`bge-m3:latest`** — local **embeddings** model, required for the graph to work.
  The setup script pulls it (override via `EMBEDDING_MODEL`).
- **A code LLM** — used only by `coderag query` and prose ingestion. Two options:

  | Model | Kind | Notes |
  |-------|------|-------|
  | `phi3.5:latest` (default) | local | Small, pulls easily, works fully offline |
  | `qwen3-coder:480b-cloud` | **cloud** | Served by Ollama from the cloud — **nothing is downloaded**, works immediately through your local Ollama with a small free-token allowance. Needs internet + an Ollama account. |

  The default `LLM_MODEL` in `docker-compose.yml` is `phi3.5:latest` because it is a
  real local model; `qwen3-coder:480b-cloud` is a cloud model, so it is *not* pulled
  like a local one. Switch models at any time with the `LLM_MODEL` env var.

> If you don't plan to use `coderag query` or prose ingestion, `bge-m3` alone is enough
> to keep the graph tools working — the C# analyser does not call an LLM.

### 3.2 Automated setup

```bash
git clone https://github.com/Fantalic/code-rag.git
cd code-rag

# Linux / macOS
bash scripts/setup.sh

# Windows (PowerShell, run from the repo root)
.\scripts\setup.ps1
```

The script: installs missing prerequisites → pulls Ollama models → builds the `coderag`
CLI + C# analyser → pre-builds the Docker image → adds the repo to your `PATH`.

The model pull honors your env vars:

```bash
# default: embeddings only — MCP usage needs no code LLM
bash scripts/setup.sh

# explicit embedding model + a code LLM for 'coderag query'
EMBEDDING_MODEL=bge-m3:latest LLM_MODEL=qwen3-coder:480b-cloud bash scripts/setup.sh
```

`EMBEDDING_MODEL` defaults to `bge-m3:latest`. `LLM_MODEL` is pulled only when set —
by default the setup script pulls **no query model** (the MCP tools don't need one).

### 3.3 Manual setup

Build the CLI and analyser, then start Docker once:

```bash
# CLI (repo root)
go build -C coderag/cli -o coderag .            # Linux/macOS
go build -C coderag/cli -o coderag.exe .        # Windows

# C# analyser (only needed if you index C#)
cd coderag/cli/analysers/csharp
go build -o csharp_analyser ./cmd/csharp_analyser/   # + .exe on Windows

# Docker images (first run of coderag start does this automatically)
docker compose build
```

Manual install, offline mode, and troubleshooting → **[docs/INSTALL.md](docs/INSTALL.md)**.

### 3.4 Verify the install

```bash
coderag status
#   codeRAG v...
#   LightRAG API:   running        ← http://localhost:9621
#   MCP SSE:        running        ← http://localhost:9622
#   Workspace:      myproject
```

If services show `not running`, run `coderag start <your project>` once — it starts the
Docker stack automatically.

---

## 4. First index

Point codeRAG at a C# solution:

```bash
coderag scan ./myproject          # needs a .sln (searches up to 10 levels up)
coderag scan ./myproject --clean  # wipe existing graph + analyser output first
```

What happens:

1. codeRAG starts the Docker services for your project (first time only).
2. The C# analyser parses the whole solution → `.coderag/analyser_output/*.txt`.
3. Output is parsed into entities + relations, diffed against the current graph, and
   written via a single batched import.
4. `Scan complete. Knowledge graph now contains N entities`.

Try it out:

```bash
coderag graph files                 # per-file stats (nodes, fan-in, fan-out)
coderag graph deps OrderService.cs  # refs + dependents for one file
coderag graph cycles                # circular file dependencies
coderag webui                       # open the visual browser
coderag query "How does order checkout work?"   # natural-language answer (LLM)
```

> `coderag query` is the only command that calls an LLM, and it needs the configured
> `LLM_MODEL` (default `phi3.5:latest`) reachable from the Docker container — either a
> local model you pulled, or a cloud model registered in your Ollama. See [docs/USAGE.md](docs/USAGE.md#coderag-query)
> if it times out.

---

## 5. Day-to-day use

### Live watching

```bash
coderag start ./myproject
```

Starts the Docker stack and a file watcher in the foreground. Save a `.cs` file and the
affected entities update within seconds. Three signals keep it accurate:

1. **Initial sync** — git-based incremental scan at startup.
2. **fsnotify** — real-time events for saves, renames, moves.
3. **Git poll** — every 30 s, catches branch switches and anything fsnotify missed.

Deletions **cascade**: removing a file also cleans up entities/relations that referenced
it and re-analyses the affected files. Press `Ctrl+C` to stop (Docker stays up).

### MCP for your AI agent

```bash
coderag mcp ./myproject
```

Starts the stdio↔SSE bridge your agent connects to (see [docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md)
for wiring opencode, Claude Desktop, Cursor, Windsurf). It also starts the watcher in the
background, so the graph stays fresh while you chat.

### Managing services

```bash
coderag status    # health of API, MCP, workspace
coderag stop      # docker compose down
coderag watch ./myproject            # watcher only, no Docker orchestration
coderag watch --debounce 1.0 ./x     # tune debounce (default 0.5s)
coderag watch --workers 8 ./x        # tune worker pool (default 4)
```

---

## 6. MCP tools reference (15)

Every tool is deterministic. Responses use a common envelope:
`{"tool": "<name>", "ok": true, "result": {...}, "error": null}`.

| Tool | Arguments | What it returns |
|------|-----------|-----------------|
| `exists_in_codebase` | `entity_name` | True/false by qualified name (`MyApp.OrderService.PlaceOrder`) |
| `get_nodes` | `entity_type`, `name`, `file_path` (with `file_match` = `contains`\|`exact`), `connected_to`, `limit=100`, `offset=0` | Filtered nodes (filters AND-combine, paginated) |
| `node_details` | `entity_name`, `view` (`details`\|`relations`\|`neighbors`), `relation_type`, `depth=2`, `limit` | `details` = node + incoming/outgoing · `relations` = just edges (optionally filtered) · `neighbors` = 1-hop entities + connecting relations (capped at `limit`) |
| `codebase_overview` | `view` (`summary`\|`graph`), `limit`, `offset` | `summary` = type counts + totals · `graph` = paginated nodes + edges (capped) |
| `file_dependencies` | `file_path` | Files that reference this file and files it depends on |
| `detect_circular_dependencies` | — | File dependency cycles (Tarjan SCC) |
| `dead_code` | `min_age_days=0`, `entity_type` | Unreferenced + non-exported symbols; optional age filter, type filter (`CLASS`/`METHOD`/`PROPERTY`) |
| `query_codebase` | `query`, `mode="hybrid"` | Structured retrieval: entities, relations, chunks, file references (chunk matches only exist for prose ingested via LLM — code lives in the graph) |
| `attach_note_to_node` | `entity_name`, `note_text` | Attaches a note (creates a NOTE node + edge) |
| `get_notes` | `node_name` | All notes on a node, most recent first (direction-tolerant) |
| `index_coverage` | — | Per-project scan coverage — file counts, reference counts, warnings per project — so silent scan gaps become visible |
| `what_changed` | `hours=24`, `since=<iso or epoch>` | Symbol-level diffs captured by watch-mode re-scans (added/removed per file within the time window) |
| `get_source` | `entity_name` *or* `file_path`+`start_line`+`end_line`, `max_lines` | Reads exact source lines using indexed spans. **Requires `SOURCE_ROOT`** (mount the analyzed workspace into the MCP container). Refuses paths that escape `SOURCE_ROOT`. |
| `add_note` | `entity_name`, `note_text` | Convenience synonym for `attach_note_to_node` |
| `list_notes` | `entity_name` | Convenience synonym for `get_notes` |

**Notes for agents:**

- Names are **fully qualified** (`TestApp.OrderService.PlaceOrder`), except where a tool
  says otherwise.
- `file_path` filtering in `get_nodes` defaults to **substring** matching — pass
  `OrderService.cs`, not a full Windows path. Pass `file_match="exact"` when ambiguity
  matters (e.g. `Snake` vs `Snake1`/`Snake2`).
- Every `EntityInfo` carries `start_line` / `end_line` (when known) — drive
  `get_source` off them for precise reads.
- `dead_code` reports **unreferenced** symbols that are also `private`/`internal` or
  *not in their file's export list*. Public methods are always exported, so expect
  mostly properties and non-exported classes.
- `get_source` returns a clean `LookupError` when `SOURCE_ROOT` is unset — bind
  the analyzed repo into the **mcp** container (`docker-compose.yml`) and set
  `SOURCE_ROOT=/workspace` (default) to enable it. The tool refuses any path that
  escapes `SOURCE_ROOT`, so a misconfigured mount cannot exfiltrate host files.

---

## 7. Configuration

Environment variables (all optional; defaults shown):

| Variable | Default | Purpose |
|----------|---------|---------|
| `LIGHTRAG_URL` | `http://localhost:9621` | LightRAG API address (used by CLI + MCP) |
| `OLLAMA_HOST` | `http://host.docker.internal:11434` | Ollama endpoint reachable from Docker (override on Linux host networking with `http://localhost:11434`) |
| `WORKSPACE` | — | Forces the active workspace for `docker compose up` |
| `LLM_MODEL` | `phi3.5:latest` | Code LLM for `coderag query` / prose ingestion. Use any local model you pulled (e.g. `qwen3-coder` local variants) or an Ollama cloud model (e.g. `qwen3-coder:480b-cloud`, free tokens) |
| `EMBEDDING_MODEL` | `bge-m3:latest` | Embedding model |
| `EMBEDDING_DIM` | `1024` | Embedding dimension (must match the model) |
| `DEBOUNCE_SECONDS` | `0.5` | Per-file watcher debounce |
| `WATCHER_WORKERS` | `4` | Watcher worker pool size |
| `LIGHTRAG_TIMEOUT` | `30.0` | MCP → LightRAG request timeout (seconds) |
| `LIGHTRAG_API_KEY` | unset | Optional auth header; unused in local mode |
| `SOURCE_ROOT` | unset | In-container absolute path to the analyzed workspace; enables the MCP `get_source` tool. Bind-mount the repo into the `mcp` service and set `SOURCE_ROOT=/workspace` so agents can read exact source lines. |
| `WORKSPACE_ROOT` | unset | CLI hint pointing at the on-disk project root, used to locate `<project>/.coderag/duplicates.json` for canonical-file overrides (see [Canonical-file conflicts](#canonical-file-conflicts)) |

> **Model consistency:** set `LLM_MODEL` to a model your Ollama can actually serve —
> a local model you pulled (e.g. `phi3.5:latest`) or a cloud model registered in your
> Ollama account (e.g. `qwen3-coder:480b-cloud`). Switching models is just an env var:
> `export LLM_MODEL=qwen3-coder:480b-cloud` before `coderag start`, or in the repo `.env`.

---

## 7a. Canonical-file conflicts

Projects that use **partial classes** (or otherwise emit the same fully-qualified
class from multiple files) end up with multiple `CLASS` entities holding the same
name. The pipeline picks a canonical file by default using a "filename without
extension matches the class name" heuristic. When the heuristic isn't enough (or
picks the wrong partial), drop a small overrides file:

```jsonc
// .coderag/duplicates.json
{
    "TestApp.OrderService": "src/Services/OrderService.Primary.cs",
    "TestApp.Money":        "src/Shared/Money.cs"
}
```

The CLI looks in `$WORKSPACE_ROOT/.coderag/duplicates.json` first, then falls
back to `~/.coderag/duplicates.json` for global preferences (per-project wins).
Keys are fully-qualified class names; values are absolute paths that must match
one of the candidate files from the scan.

---

## 8. Security

The watcher runs a **security filter after analysis and before graph insertion**.
Matched values are replaced with `[REDACTED]`:

- API keys (`sk-…`, `pk-…`, `ghp_…`, `AKIA…`), auth tokens, JWTs
- Passwords / secrets (`password=`, `secret=`, `token=`, …)
- Private keys (`-----BEGIN … PRIVATE KEY-----`)
- PII — emails, IPv4/IPv6 addresses
- Internal/UNC paths (`\\server\share`, `//internal/`)
- Values of env vars named `*SECRET*` / `*TOKEN*`

See **[docs/SECURITY.md](docs/SECURITY.md)** for the full details.

---

## 9. Troubleshooting quick reference

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `coderag scan` finds no `.sln` | Project has no solution file | Point at a folder containing (or below) a `.sln` |
| `coderag query` times out | Ollama not reachable from Docker | Check `OLLAMA_HOST`; run `scripts/dev/diag_ollama.sh` |
| First embedding is slow (~35 s) | `bge-m3` cold load | Pre-warm: `curl -X POST localhost:11434/api/embeddings -d '{"model":"bge-m3","input":"hi"}'` |
| Services show `not running` | Docker not started | Start Docker Desktop, then `coderag start` |
| Stale/wrong entities | Old workspace cached | `coderag scan --clean`, or `coderag stop` then restart |

Full troubleshooting + offline mode → **[docs/INSTALL.md](docs/INSTALL.md)**.

---

## 10. Where things live

| Component | Address / path |
|-----------|----------------|
| LightRAG API + WebUI | `http://localhost:9621` |
| MCP SSE | `http://localhost:9622` |
| Ollama | `http://localhost:11434` (host) |
| Project graph data | `<project>/.coderag/rag_storage` |
| Analyser output | `<project>/.coderag/analyser_output` |

---

## 11. Project status

- **Status:** `experimental` 🧪
- **Ingestion:** C# (`.sln`) via the native Go analyser; other file types via the
  optional LLM pipeline. Supported file types → **[docs/SUPPORTED_FILES.md](docs/SUPPORTED_FILES.md)**.
- **Architecture:** host-side Go CLI + Docker-hosted Python services →
  **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**.
- **License:** MIT. Built on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and
  [modelcontextprotocol](https://github.com/modelcontextprotocol/python-sdk) (MIT).
