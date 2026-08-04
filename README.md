<div align="center">

# 🧠 codeRAG

### *Your codebase, visualized. Your knowledge graph, alive.*

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Python](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python)](https://python.org)
[![MCP](https://img.shields.io/badge/MCP-18%20tools-blueviolet)](https://modelcontextprotocol.io)
[![Docker](https://img.shields.io/badge/Docker-required-2496ED?logo=docker)](https://docker.com)
[![Ollama](https://img.shields.io/badge/Ollama-required-black)](https://ollama.com)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Turn your project into a live, queryable knowledge graph.**
100% local. No cloud APIs. No API keys. AI agents explore your code via MCP.

</div>

---

## What it is

codeRAG watches your project folder and builds a **live semantic knowledge graph** — classes, methods, interfaces, call graphs, and dependencies — all indexed and instantly explorable. When you change a file, only the affected entities update.

- 🌐 **WebUI** — visual graph exploration at `http://localhost:9621`
- 🤖 **MCP** — 18 curated tools for any MCP-compatible AI agent
- 💻 **CLI** — direct control with `coderag` (single Go binary)

---

## Who is this for

- **.NET devs on Visual Studio** — index your solution without burning tokens; query it from Copilot Agent or any AI extension. → **[docs/VS.md](docs/VS.md)**
- **AI-agent users** — give your agent structured, deterministic knowledge of your codebase. Works with opencode, Claude Desktop, Cursor, Windsurf, and any other MCP client. → **[docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md)**
- **Code archaeologists** — explore a large codebase visually with the bundled WebUI.

---

## How it works

Four pieces, all running locally:

1. **C# → knowledge graph (no LLM).** A native Go static analyser parses your `.csproj`/`.cs` files, extracts entities and relations deterministically, and pushes them straight into LightRAG via HTTP CRUD. No tokens burned, no hallucinations — what you see is what your code says.
2. **Other files → optional LLM ingest.** Markdown, YAML, `.txt`, and anything else can be ingested through LightRAG's standard pipeline. This part does use Ollama (bge-m3 for embeddings, qwen3-coder for extraction) — but it's opt-in and used only when you feed non-code files.
3. **Live watcher.** `coderag start` watches your project with three signals: an initial scan, an `fsnotify` watcher for save events, and a periodic `git diff HEAD` poll to catch anything fsnotify misses (e.g. branch switches). Renames, moves, and deletes cascade — affected files are re-indexed automatically.
4. **MCP for AI agents.** Two transport modes: HTTP/SSE at `:9622` (for Docker-side MCP clients) and a `coderag mcp <dir>` stdio bridge for AI agents that don't speak SSE natively (opencode, Claude Desktop, Cursor, …).

---

## Example conversations

What an agent experience looks like once your project is indexed:

> **You:** What does `OrderService` depend on?
> **Agent:** *calls `node_details(view="details")` → `node_details(view="neighbors")`*
> OrderService depends on 4 entities: `OrderRepository`, `PaymentGateway`, `EventBus`, and the `ILogger<OrderService>` interface. The two heaviest edges are to `PaymentGateway` (3 calls) and `OrderRepository` (2 calls).

> **You:** If I rename `PlaceOrder`, which files break?
> **Agent:** *calls `node_details(view="relations")` → `get_nodes(file_path=...)` per result*
> 3 files reference `OrderService.PlaceOrder`: `OrdersController.cs`, `CheckoutFlow.cs`, and the test file `OrderServiceTests.cs`. Start with the controller — it has 2 call sites.

> **You:** Is there a circular dependency between `Order` and `OrderService`?
> **Agent:** *calls `file_dependency_cycles`*
> No cycles in the project. Closest one is in a different area: `Notifications` → `Templates` → `Notifications` (cycle length 2).

---

## Requirements

| Tool | Min version | Notes |
|------|-------------|-------|
| Go | 1.25+ | Only needed to build the CLI; not needed at runtime |
| Docker | 24+ | Runs LightRAG + MCP server |
| Ollama | 0.3+ | Local LLM + embeddings (runs on host, not in Docker) |
| OS | Linux, macOS, Windows (10/11 + WSL2 or Git Bash) | |

Required Ollama models (pulled automatically by the installer):

- `qwen3-coder:480b-cloud` — code LLM (used for non-code ingest only)
- `bge-m3:latest` — embeddings

GPU is optional but recommended for large projects.

---

## Install

```bash
git clone https://github.com/Fantalic/code-rag.git
cd code-rag

# Linux / macOS
bash scripts/setup.sh

# Windows (PowerShell)
.\scripts\setup.ps1
```

The setup script installs missing prerequisites, pulls Ollama models, builds the `coderag` CLI (~20 MB static binary), builds the C# analyser, and pre-builds the Docker image.

**Manual step-by-step install, troubleshooting, and an offline mode** → **[docs/INSTALL.md](docs/INSTALL.md)**

---

## Quick start

```bash
coderag start ./myproject      # Docker + live file watcher
coderag scan  ./myproject      # one-time index
coderag graph files            # peek at the graph (3 files, 10 entities, …)
coderag webui                  # open browser at http://localhost:9621
coderag mcp   ./myproject      # start MCP bridge for your AI agent
```

For a full smoke test (down → build → up → scan → CLI + MCP):

```bash
sh scripts/dev/runtime-test.sh
sh scripts/dev/runtime-test.sh --clean   # wipe graph first
```

---

## MCP tools (18)

The MCP server exposes 18 curated tools. The **graph + file tools** are fast and deterministic — no LLM round-trip — and that's what your AI agent should reach for first. Only `query` (natural-language answer) goes through Ollama, and it's intentionally not exposed to agents — it's available only via the `coderag` CLI.

| Agent intent | Tool |
|---|---|
| "Does X exist?" | `check_entity_exists` |
| "Find entities matching X" | `get_nodes` (filter by entity_type, name, file_path, connected_to) |
| "Tell me about X" | `node_details` (view: details, relations, neighbors) |
| "Codebase overview" | `get_knowledge_graph` |
| "File ↔ entity bridge" | `get_related_files` |
| "File-level views" | `get_file_stats`, `get_file_dependencies`, `file_dependency_cycles` |
| "Structured retrieval" | `query_data` |
| "Annotate" | `add_note_to_entity` |

Wiring instructions for opencode, Claude Desktop, Cursor, Windsurf, and any other MCP client → **[docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md)**

---

## Architecture

```
┌───────────────────────── HOST MACHINE ─────────────────────────┐
│                                                                 │
│  ┌─────────────┐  stdio    ┌─────────────────────────┐          │
│  │  AI Agent   │◄─────────►│  coderag CLI  (Go)     │          │
│  │  opencode / │    MCP    │  • file watcher         │          │
│  │  Claude /   │           │  • C# analyser          │          │
│  │  Cursor / … │           │  • Docker orchestration │          │
│  └─────────────┘           │  • MCP stdio ↔ SSE      │          │
│                            └──────────┬──────────────┘          │
│                                       │                         │
│  ┌────────────────────┐               │                         │
│  │  Ollama (native)   │◄──────────────┘  HTTP :11434           │
│  │  qwen3-coder       │   bge-m3 embeddings + LLM              │
│  │  bge-m3            │                                      │
│  └────────────────────┘                                      │
└─────────────────────────────────┬───────────────────────────────┘
                          HTTP :9621    SSE :9622
┌─────────────────────────────────┼───────────────────────────────┐
│  DOCKER                         │                               │
│  ┌────────────────────┐ HTTP    │ ┌────────────────────┐        │
│  │  LightRAG API      │◄────────┤ │  MCP Server (Py)   │        │
│  │  + WebUI (React)   │         │ │  FastMCP / SSE     │        │
│  │  :9621             │         │ │  :9622  18 tools   │        │
│  └────────────────────┘         │ └────────────────────┘        │
└─────────────────────────────────┴───────────────────────────────┘
```

Ollama runs natively on the host (not in Docker) so the C# code path stays 100% LLM-free. The MCP bridge converts stdio → SSE so any local AI agent can talk to a server inside Docker.

---

## Built with

- **Native Go static analyser for C#** — `tree-sitter`-style parsing directly into the graph, no LLM round-trip.
- **Git-aware + fsnotify hybrid watcher** — survives branch switches, renames, and large refactors; cascade-deletes affected files.
- **MCP over HTTP/SSE + stdio bridge** — the agent talks stdio, the server speaks SSE; `coderag mcp` is the protocol translator.
- **Two ingestion paths** — deterministic for code, LLM-assisted only for prose; you pay tokens only when you ingest prose.
- **Single-binary CLI + Docker orchestration** — one `coderag` binary drives the whole stack; the Docker side stays focused on LightRAG itself.
- **Idempotent relation diffing** — re-indexing the same file is a no-op; entity/relation IDs are content-derived.

---

## 📚 Documentation

| Doc | What it covers |
|-----|----------------|
| [docs/INSTALL.md](docs/INSTALL.md) | Manual install, troubleshooting, offline mode |
| [docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md) | Wire codeRAG into opencode, Claude Desktop, Cursor, Windsurf, … |
| [docs/VS.md](docs/VS.md) | Visual Studio / Microsoft-stack developers |
| [docs/USAGE.md](docs/USAGE.md) | Full CLI reference, all flags, day-to-day workflow |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Repo layout, data flow, components |
| [docs/SECURITY.md](docs/SECURITY.md) | What gets redacted before reaching the graph |
| [docs/SUPPORTED_FILES.md](docs/SUPPORTED_FILES.md) | File types, analysers, ingestion paths |

---

## Status

**Status:** `experimental` 🧪 · **License:** MIT

Built on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and [modelcontextprotocol](https://github.com/modelcontextprotocol/python-sdk) (MIT).
