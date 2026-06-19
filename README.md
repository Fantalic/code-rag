# 🔥 CodeLightRAG — An AI That *Sees* Your Codebase Grow

> Stop serializing your whole repo into a prompt. Give your AI a live knowledge graph that breathes with every commit.

CodeLightRAG watches your filesystem, builds a **semantic knowledge graph** of entities, classes, functions, and their relationships — and exposes it to AI agents via 22 [Model Context Protocol](https://modelcontextprotocol.io) tools. No more "please read these 400 files before answering."

```
[You code] ──► [Watchdog] ──► [LightRAG Graph] ◄──► [MCP Server] ◄──► [AI Agent]
```

---

## 🚀 What Makes This Different

Most "AI for code" tools dump raw files into context. CodeLightRAG does something harder:

| Approach | What happens |
|----------|-------------|
| ❌ Naive RAG | Chunk every file → embed → retrieve snippets. No structure. |
| ✅ **CodeLightRAG** | Parse code into **entities** (classes, functions) + **relations** (inherits, calls, imports) → store as a knowledge graph → agents query relationships, not text. |

**The result**: An AI agent can ask "what does `handleRequest` depend on?" and get back a graph traversal instead of 12 irrelevant code blocks.

---

## ✨ Features That Actually Matter

- 🔍 **22 MCP tools** — health, insert, query, knowledge graph, document management, system tools
- 🤖 **Agent-native** — speaks MCP, the protocol Claude Desktop, Cursor, and others understand natively
- ⚡ **Live indexing** — watchdog sees your `git checkout` and updates the graph automatically
- 🧩 **Multi-language C# analyser** — extracts classes, methods, inheritance, deps from .NET projects
- 🐳 **Docker-ready** — LightRAG + MCP server deploy in containers
- 🕸️ **Web UI** — React 19 interface for visual graph exploration (113 built assets)
- 🧪 **226 passing tests** — because "it works on my machine" isn't a strategy

---

## ⚡ Quick Start

```powershell
# One command — venv, deps, everything
.\scripts\setup.ps1

# Three terminals, one system:
lightrag-server          # Terminal 1: RAG API on :9621
python -m daniel_lightrag_mcp   # Terminal 2: MCP bridge
python watchdog\main.py         # Terminal 3: file watcher
```

Or smash them all at once:
```powershell
.\scripts\start_all.ps1
```

---

## 🧪 Where We Are

**Status**: `experimental` 🧪 · **Build**: `226 tests passing` ✅

| Suite | Tests |
|-------|-------|
| Root integration | 63 ✅ |
| MCP server | 163 ✅ |
| Lint (ruff) | ... we'll get there 😅 |

---

## 🗺️ What's Next

The big technical meat of this project — and the reason it exists — is **efficient incremental graph updates**. Current state:

- ✅ **Watchdog detects file changes** (create, modify, delete)
- ✅ **LightRAG stores entities + relations** in a queryable graph
- ❌ **Watchdog only re-ingests whole files** — no diff-based relation updates
- ❌ **No entity-level granularity** — changing one method re-parses the entire file

**Next phase**: wire the watchdog to the analyser pipeline so that when you change a single function signature, only the affected relations get updated in the graph. No full rebuilds. No redundant embeddings.

If that sounds like a fun systems problem — it is. That's the part I'm building right now.

---

## 🔧 Stack

| Layer | What |
|-------|------|
| **RAG Engine** | LightRAG (forked) — FastAPI, Pydantic V2, asyncio |
| **Agent Protocol** | MCP Server — 22 tools, stdio transport |
| **File Watch** | `watchdog` Python library |
| **Analyser** | C# .NET 9 (compiled binary) |
| **Storage** | LightRAG graph store (pluggable backends) |
| **Tests** | pytest + pytest-asyncio — 226 passing |
| **Python** | 3.10+ (yes, 3.8 is finally gone) |
| **Container** | Docker Compose — LightRAG + MCP |

---

## 📁 Structure

```
code-lightrag/
├── LightRAG/                  ← RAG engine (forked)
│   ├── lightrag/              ← Python core
│   ├── lightrag-api/          ← FastAPI service
│   └── lightrag_webui/        ← React 19 UI
├── daniel-lightrag-mcp/       ← MCP server + code analysers
├── watchdog/                  ← File change watcher
├── scripts/                   │  Dev tooling
├── tests/                     │  63 integration tests
└── docs/                      │  Plans, context, decisions
```

---

## 📜 License

MIT. This monorepo builds on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and [daniel-lightrag-mcp](https://github.com/desimpkins/daniel-lightrag-mcp) (MIT) — both excellent projects that made this possible.
