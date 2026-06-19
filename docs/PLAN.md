# 🗺️ CodeLightRAG — Where We're Going

## The Big Idea

A **live, minimal-work knowledge graph** for codebases. When you save a file, only the affected entities and relations update — no full rebuilds, no wasted tokens, no secrets leaked.

## What We Have ✅

- Monorepo: LightRAG + MCP server + watchdog working together
- 22 MCP tools for AI agents to query the knowledge graph
- C# analyser (Roslyn-based, compiled binary) — extracts classes, methods, inheritance, deps
- File-watch with debouncing + hash tracking (skips no-op events)
- 226 passing tests (63 root + 163 MCP)

## Phase 11 — Efficient Incremental Updates 🔥

### Architecture

```
[File changes] ──► [Watchdog] ──► [File-type router]
                                       │
              ┌────────────────────────┼────────────────────────┐
              ▼                        ▼                        ▼
      ┌──────────────┐       ┌──────────────┐        ┌──────────────┐
      │  C# analyser │       │  Zig analyser │        │  JSON/TXT    │
      │  (Roslyn)    │       │  (future)     │        │  analyser    │
      └──────┬───────┘       └──────┬────────┘        └──────┬───────┘
             │                      │                        │
             └──────────────────────┼────────────────────────┘
                                    ▼
                    ┌──────────────────────────────┐
                    │        Diff engine            │
                    │  old entities vs new entities  │
                    └──────────────┬───────────────┘
                                   ▼
                    ┌──────────────────────────────┐
                    │    Graph CRUD executor        │
                    │  create/update/delete entities│
                    │  & relations via LightRAG API │
                    └──────────────────────────────┘
                                   │
                                   ▼
                    ┌──────────────────────────────┐
                    │   Security filter (PII/keys)  │
                    │   Strip secrets before embed  │
                    └──────────────────────────────┘
```

### Current state

| Layer | Status |
|-------|--------|
| File hash tracking (skip unchanged) | ✅ Done (11.1) |
| Versioned re-ingest (bypass dedup) | ✅ Done (11.1) |
| C# analyser pipeline | ✅ Done (Phase 7) |
| File-type routing (ignore unsupported) | ⬜ Next |
| Wire watchdog → C# analyser on .cs changes | ⬜ Next |
| Graph CRUD (direct endpoints, no LLM) | ⬜ Next |
| Diff engine (old vs new entities) | ⬜ Next |
| Security filter (PII, keys, secrets) | ⬜ Next |
| Zig analysers (multi-language) | ⬜ Future |

### File type plan

| Extension | Analyser | Status |
|-----------|----------|--------|
| `.cs` | C# Roslyn (existing) | ✅ Working, needs graph CRUD |
| `.py`, `.js`, `.ts`, `.go`, `.rs` | Zig (future) | ⏸️ |
| `.md`, `.txt`, `.json`, `.yaml`, `.ini`, `.toml` | Zig (future) | ⏸️ |
| Everything else | Ignored | ✅ Default |

### Security

All analysers **must** filter before inserting into the graph:
- 🔴 API keys, tokens, passwords (`sk-...`, `ghp_...`, etc.)
- 🔴 Email addresses, usernames, personal names
- 🔴 IP addresses, internal URLs, server paths
- 🔴 Any `SECRETS`/`PASSWORDS`/`TOKENS` env var values

The security filter lives in the pipeline **after** extraction but **before** graph insertion. This prevents sensitive data from ever reaching the vector DB.

---

## What We're Not Doing (Yet)

- ❌ PyPI publishing
- ❌ Premature corporate docs
- ❌ Full-language support before the pipeline pattern is proven with C#
- ❌ Perfection — experimental project that ships
