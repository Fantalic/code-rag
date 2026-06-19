# 🗺️ CodeLightRAG — Where We're Going

## The Big Idea

Make a **live, minimal-work knowledge graph** for codebases. When you save a file, the graph updates — only the changed parts, no full rebuilds, no wasted tokens.

## What We Have ✅

All foundation:
- LightRAG + MCP server + watchdog working together as a monorepo
- 22 MCP tools for AI agents to query the graph
- C# analyser pipeline (extracts entities + relations)
- 226 passing tests (63 root + 163 MCP)
- Pydantic V2, Python 3.10+ types
- Docker Compose deploy
- CI workflow

## The Hard Part — Next Up 🔥

### Phase 11 — Efficient Incremental Graph Updates

The actual hard problem. Watchdog detects changes, but right now it only re-ingests whole files. We need **entity-level granularity**.

| Step | Status |
|------|--------|
| 11.1 — Analyse what changes per file event | ⬜ |
| 11.2 — Map file paths → existing graph entities | ⬜ |
| 11.3 — Diff-based relation updates (create/modify/delete) | ⬜ |
| 11.4 — Avoid redundant embeddings on unchanged entities | ⬜ |
| 11.5 — Stress test: watch a real project, measure graph drift | ⬜ |

### Phase 12 — Beyond C# (maybe)

Language-agnostic analyser? YAML? Python self-analysis? TBD after Phase 11 proves the pattern.

## What We're Not Doing

- ❌ PyPI publishing (not useful as a library)
- ❌ Premature corporate docs (CHANGELOG, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY — deleted, not ready for them yet)
- ❌ Perfection — this is an **experimental fun project** that ships
- ❌ GitHub CI workflows / issue templates — premature, deleted
