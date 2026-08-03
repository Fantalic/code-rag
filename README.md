<div align="center">

# 🧠 codeRAG

### *Your codebase, visualized. Your knowledge graph, alive.*

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Python](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python)](https://python.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react)](https://react.dev)
[![Tests](https://img.shields.io/badge/tests-226%20passing-brightgreen)](https://github.com/Fantalic/code-rag)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](https://docker.com)

**Turn your project into a live, queryable knowledge graph.**  
AI agents explore your code via MCP. You explore it in a beautiful WebUI.  
No cloud APIs. No API keys. 100% local with Ollama.

</div>

---

## 🎯 What It Does

codeRAG watches your project folder and builds a **live semantic knowledge graph** — classes, methods, interfaces, their inheritance chains, call graphs, and dependencies — all indexed and instantly explorable.

When you change a file, only the affected entities update. No full rebuilds. No wasted embeddings.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────────┐     ┌─────────────┐
│ Your Code   │────►│ coderag CLI │────►│  LightRAG API   │────►│  Knowledge  │
│  (C# .sln)  │     │  (Go binary)│     │  (FastAPI)      │     │   Graph     │
└─────────────┘     └─────────────┘     └─────────────────┘     └──────┬──────┘
                                                                       │
┌─────────────┐     ┌─────────────┐     ┌─────────────────┐            │
│  AI Agent   │◄────│ MCP Server  │◄────│  coderag bridge │◄───────────┘
│  (opencode) │     │  (Python)   │     │  (stdio↔SSE)    │
└─────────────┘     └─────────────┘     └─────────────────┘
```

**Three ways to interact:**
1. 🌐 **WebUI** — Visual graph exploration at `http://localhost:9621`
2. 🤖 **MCP Tools** — AI agents query the graph via 17+ structured tools
3. 💻 **CLI** — Direct control with 8 commands

---

## ✨ What Makes It Special

### 🌐 Interactive WebUI — See Your Code as a Graph
A React 19 + TypeScript interface served directly by LightRAG. Browse entities, explore relationships, and navigate your codebase visually. No config needed — just run `coderag webui`.

> *"Finally, I can see how `Calculator` inherits from `BaseCalculator` and who calls `AddItem()` — without grepping through 50 files."*

### ⚡ Zero-LLM C# Analysis — Fast & Deterministic
For `.cs` files, codeRAG uses a **native Go analyser** that extracts classes, methods, interfaces, enums, structs, inheritance, and call references — then writes them directly to the graph via CRUD. **No LLM involved.** No tokens burned. No nondeterminism.

| Extracted | Relation Types |
|-----------|---------------|
| `NAMESPACE`, `CLASS`, `METHOD`, `PROPERTY` | `contains`, `defines`, `belongs_to` |
| `INTERFACE`, `STRUCT`, `ENUM` | `implements`, `extends`, `calls` |

### 👁️ Live File Watching — Git-Aware & Smart
The watcher uses **three complementary mechanisms**:
- **Initial sync** — MD5 hash map + `git diff` to only process changed files
- **Git watcher** — Polls `git diff HEAD` every 30s for committed changes
- **FSNotify fallback** — Real-time filesystem events when git isn't available

With **cascade-aware deletion**: remove a file, and codeRAG automatically re-analyses all files that referenced its entities.

### 🛡️ Security Filter — Secrets Never Touch the Graph
Before any data enters the knowledge graph, it passes through a security filter that redacts:
- API keys (`sk-...`, `ghp_...`, `AKIA...`)
- Auth tokens & JWTs
- Passwords & secrets
- Private keys (RSA, EC, OPENSSH)
- PII (emails, IPs)
- Internal network paths

### 🧩 Dual Ingestion Paths
| Path | Files | Speed | LLM? |
|------|-------|-------|------|
| **A — Direct Graph CRUD** | `.cs` (C#) | ⚡ Instant | ❌ No |
| **B — LLM Pipeline** | `.md`, `.yaml`, `.json`, etc. | 🐢 Slower | ✅ Yes |

Unsupported file types gracefully fall back to Path B. Both paths are always available.

---

## 🚀 Quick Start

### Prerequisites
- [Go](https://go.dev/dl/) 1.25+
- [Docker](https://docs.docker.com/engine/install/)
- [Ollama](https://ollama.com/) running locally
  - Required models: `qwen3-coder:480b-cloud` (or any code-capable model), `bge-m3:latest`

### One-Command Setup

```bash
git clone https://github.com/Fantalic/code-rag.git
cd code-rag

# Linux / macOS
bash scripts/setup.sh

# Windows (PowerShell)
.\scripts\setup.ps1
```

This installs missing prerequisites, pulls Ollama models, builds the `coderag` CLI (~20MB static binary), builds the C# analyser, and pre-builds Docker images.

### Start Exploring Your Project

```bash
# 1. Start Docker services + live file watcher
coderag start ./my-dotnet-project

# 2. Scan your C# solution into the knowledge graph
coderag scan ./my-dotnet-project

# 3. Open the WebUI in your browser
coderag webui
```

🎉 That's it. Your codebase is now a live knowledge graph at `http://localhost:9621`.

---

## 📖 CLI Reference

```bash
# Start everything (Docker + watcher) — blocks until Ctrl+C
coderag start ./myproject

# Scan a .NET solution into the graph (one-time indexing)
coderag scan ./myproject

# Re-scan from scratch (clears existing data first)
coderag scan --clean ./myproject

# Open the WebUI in your default browser
coderag webui

# File watcher only (no Docker orchestration)
coderag watch ./myproject

# MCP bridge mode for AI agents (e.g. opencode)
coderag mcp ./myproject

# Check what's running
coderag status

# Stop all Docker services
coderag stop

# Quick graph query (basic — see MCP tools for serious exploration)
coderag query "What classes are in this project?"
```

### Global Flags
| Flag | Description |
|------|-------------|
| `--quiet` | Suppress info logs, show only errors and results |
| `--version` | Print version and exit |

### Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `LIGHTRAG_URL` | `http://localhost:9621` | LightRAG API endpoint |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama endpoint (only supported LLM backend) |
| `DEBOUNCE_SECONDS` | `0.5` | Per-file debounce delay |
| `WATCHER_WORKERS` | `4` | Thread pool size |

---

## 🤖 MCP Tools — Power Your AI Agent

Connect codeRAG to any MCP-compatible client (e.g. [opencode](https://opencode.ai)) and give your AI agent structured access to your codebase.

> **💡 Pro tip:** The graph traversal and exploration tools are the stars here. They are fast, reliable, and give your agent precise structural understanding of your code.

### 🔍 Graph Exploration *(The Good Stuff)*
| Tool | What It Does | Status |
|------|-------------|--------|
| `get_knowledge_graph` | Full graph with nodes + edges. Use `max_depth`: `1`=types, `2`=+members, `3`=all | ✅ Excellent |
| `get_entity_details` | Full entity info with all relations. `depth`: `1`=entity, `2`=structural, `3`=full | ✅ Excellent |
| `get_entity_neighbors` | 1-hop neighbors — inheritance, implementation, calls | ✅ Excellent |
| `check_entity_exists` | Fast boolean check by entity name | ✅ Excellent |
| `search_entities` | Regex search on names and descriptions | ✅ Excellent |
| `get_graph_labels` | List all entity/relation types in the graph | ✅ Excellent |
| `get_relation_detail` | All relations of a specific type for an entity | ✅ Excellent |
| `get_method_signature` | Parameters, return type, etc. | ✅ Excellent |
| `get_interface_methods` | Interface methods + implementing classes | ✅ Excellent |
| `find_related_files` | Files referencing a given entity | ✅ Excellent |

### 📝 Document Management
| Tool | What It Does |
|------|-------------|
| `insert_text` | Insert raw text into the LLM pipeline |
| `insert_texts` | Batch text insertion |
| `upload_document` | Upload a file for LLM processing |
| `scan_documents` | Trigger document scan |
| `get_documents` / `get_documents_paginated` | List indexed documents |
| `delete_document` | Remove a document |

### 🔎 Natural Language Query *(Experimental)*
| Tool | What It Does | Status |
|------|-------------|--------|
| `query_text` | LLM-powered natural language query | ⚠️ POST `/query` may timeout — Ollama connectivity |
| `query_text_stream` | Streaming version of above | ⚠️ Same limitation |

> **Recommendation:** Use the graph exploration tools above for reliable, instant results. The NL query tools are handy for open-ended questions but depend on Ollama responsiveness.

### 🏥 System & Status
| Tool | What It Does |
|------|-------------|
| `get_pipeline_status` | Current pipeline state |
| `get_track_status` | Tracking status |
| `get_document_status_counts` | Document counts by status |
| `get_health` | Service health check |

### Example: Agent Conversation
```
User: "What does the Calculator class depend on?"
Agent: [calls get_entity_details on "Calculator"]
Agent: "Calculator depends on IOperation (interface) and extends BaseCalculator 
        (class). It has 4 methods: Add, Subtract, Multiply, Divide."

User: "Who calls the Add method?"
Agent: [calls get_entity_neighbors on "Calculator.Add"]
Agent: "Add is called by ShoppingCart.CalculateTotal and TaxEngine.ApplyTax."
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  HOST MACHINE                                                               │
│  ┌─────────────────┐  stdio  ┌─────────────────────────────────────────┐   │
│  │   AI Agent      │◄───────►│  coderag CLI  (Go static binary, ~20MB) │   │
│  │  (opencode/etc) │   MCP   │  • File watcher (fsnotify + git)        │   │
│  └─────────────────┘         │  • C# analyser (native subprocess)      │   │
│                              │  • Docker orchestration                 │   │
│                              │  • MCP stdio ↔ SSE bridge               │   │
│                              └──────────────────┬──────────────────────┘   │
│                                                 │                          │
│                              HTTP :9621    SSE :9622                       │
└─────────────────────────────────────────────────┼──────────────────────────┘
                                                  │
┌─────────────────────────────────────────────────┼──────────────────────────┐
│  DOCKER / PODMAN                                │                          │
│  ┌────────────────────┐  HTTP  ┌────────────────┐│                          │
│  │  LightRAG API      │◄───────│  MCP Server    ││                          │
│  │  (Python FastAPI)  │        │  (Python)      ││                          │
│  │  :9621             │        │  :9622         ││                          │
│  │  • Graph DB        │        │  • 17+ tools   ││                          │
│  │  • Vector DB       │        │  • SSE transport│                          │
│  │  • WebUI (React 19)│        └────────────────┘│                          │
│  └────────────────────┘                         │                          │
│                              ┌────────────────────┐                        │
│                              │  Ollama (optional) │                        │
│                              │  :11434            │                        │
│                              │  LLM + embeddings  │                        │
│                              └────────────────────┘                        │
└────────────────────────────────────────────────────────────────────────────┘
```

**Why the host/Docker split?**
- `coderag` runs on the host for low-latency file watching and native subprocess spawning
- Python services run in Docker for isolation, reproducibility, and resource management
- The MCP bridge connects host-side agents to Docker-side MCP server via lightweight SSE

---

## 📁 Project Structure

```
code-rag/
├── coderag/                      ← All owned code
│   ├── cli/                      ← Go CLI (orchestrator)
│   │   ├── main.go               ← Entry point
│   │   ├── go.mod
│   │   ├── watcher/              ← File watcher
│   │   ├── diff/                 ← Entity/relation diff engine
│   │   ├── pipeline/             ← Pipeline orchestration
│   │   ├── parser/               ← Analyser output parser
│   │   ├── router/               ← File extension → analyser mapping
│   │   ├── filter/               ← Security filter (PII/keys/secrets)
│   │   ├── crud/                 ← LightRAG HTTP CRUD client
│   │   └── analysers/csharp/     ← C# Go native analyser
│   ├── lightrag/                 ← Python RAG engine (Docker)
│   │   ├── lightrag/             ← Core Python package
│   │   ├── lightrag-api/         ← FastAPI service
│   │   └── lightrag_webui/       ← React 19 + TypeScript UI
│   ├── mcp/                      ← Python MCP server (Docker)
│   └── tests/                    ← Integration tests
├── scripts/                      ← Setup & build scripts
├── docker-compose.yml
├── Dockerfile
└── _dev_docs/                    ← Private working docs (gitignored)
```

---

## 🧪 Testing

```bash
# Root integration tests (74)
pytest tests/ -v

# MCP server tests (163)
pytest daniel-lightrag-mcp/tests/ -v

# All Python tests
pytest tests/ daniel-lightrag-mcp/tests/ -v   # 226 total

# Go build check
go build ./...
```

---

## 🛡️ Security

All analyser output is filtered **after extraction, before graph insertion**:

| Category | Patterns Redacted |
|----------|-------------------|
| API keys | `sk-...`, `pk-...`, `ghp_...`, `AKIA...` |
| Auth tokens | `Bearer ...`, JWT (`eyJ...`), `token=` |
| Passwords | `password=`, `passwd=`, `PASS=`, `secret=` |
| Private keys | `-----BEGIN ... PRIVATE KEY-----` |
| PII | Emails, IPv4/IPv6 addresses |
| Internal paths | `\\server\share`, `//internal/` |

Matched values are replaced with `[REDACTED]` before reaching the graph.

---

## 📋 Supported File Types

| Extension | Analyser | Ingestion Path |
|-----------|----------|---------------|
| `.cs` | C# Go native analyser | **Path A** — Direct CRUD (no LLM) |
| `.py`, `.js`, `.ts`, `.go`, `.rs` | Planned | Path B — LLM fallback |
| `.md`, `.yaml`, `.json`, `.txt` | — | Path B — LLM fallback |

---

## 🚦 Status

**Status:** `experimental` 🧪 · **Build:** `226 tests passing` ✅ · **License:** MIT

Built on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and [daniel-lightrag-mcp](https://github.com/desimpkins/daniel-lightrag-mcp) (MIT).
