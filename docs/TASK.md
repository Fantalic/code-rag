# 🔥 Phase 11 — Efficient Incremental Graph Updates

## The Problem

Right now, the watchdog detects file changes but does the dumb thing:

```
File changes → re-read entire file → re-embed everything → replace in graph
```

This is wasteful and slow. For a real codebase, you'd re-process thousands of lines just because one function signature changed.

## What We Want

```
File changes → parse the file → compare with existing graph entities →
update/create/delete only the entities and relations that actually changed →
no redundant embeddings → minimal API calls to LightRAG
```

## Steps

### 11.1 — Inventory current watchdog flow
- [ ] Read `watchdog/main.py` — what does it do on file change?
- [ ] Read LightRAG insert/update API — what granularity does it support?
- [ ] Read MCP `update_relations` tool — how does the C# analyser pipeline work?
- [ ] Document the full current pipeline

### 11.2 — Entity-level tracking
- [ ] Design: how to map file paths → existing graph entities
- [ ] If LightRAG supports entity-level updates: expose them via MCP
- [ ] If not: implement a diff layer between watchdog and LightRAG

### 11.3 — Diff-based updates
- [ ] For each changed file: parse, extract current entities, compare with graph state
- [ ] Generate minimal update payload (only changed relations)
- [ ] Batch API calls to avoid N+1

### 11.4 — Avoid redundant work
- [ ] Track file hashes to skip unchanged files entirely
- [ ] Skip embedding for entities that haven't changed

### 11.5 — Stress test
- [ ] Watch a real project (this repo?), measure graph drift
- [ ] Verify: change one method → only that method's relations update

## Verification

- `pytest tests\` → 63 passed
- `pytest daniel-lightrag-mcp\tests\` → 163 passed
- Manual: edit a file, watch the graph update without full rebuild

## Key Files
- `watchdog/main.py` — starting point
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/server.py` — MCP `update_relations` tool
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/models.py` — entity/relation models
- `daniel-lightrag-mcp/analysers/` — C# analyser runner
