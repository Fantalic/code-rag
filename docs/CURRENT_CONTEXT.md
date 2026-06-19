# Current Context

## Status
- **Phase**: 7 — Analyser Integration into MCP Server 🔄 WIP
- **Phases 1–6**: ✅ Complete (all committed)
- **Phase 6**: 39/39 tests pass, WebUI builds, docs updated

---

## What's Been Done

### Phases 1–6 ✅
- Monorepo unified (LightRAG + MCP + watchdog), Pydantic V2, tests pass, WebUI builds

### Phase 7 — Analyser Integration

**Critical discovery**: The C# analyser (`CSharpAnalyzerTool.exe`) uses Roslyn to parse C# code at the compiler level and outputs **structured plaintext summaries** (classes, methods, return types, project references). This structured output must be **fed into LightRAG** for entity extraction and knowledge graph building — NOT raw `.cs` source files.

**Previous mistake**: Our earlier server run fed raw C# source code into LightRAG. The 0.5b LLM produced gibberish because it was trying to parse raw code tokens instead of clean analyser output like:
```
FILE: TestLib\Calculator.cs

  CLASS: Calculator
    METHOD: Add -> RETURNS -> int
    METHOD: Subtract -> RETURNS -> int

PROJECT REFERENCES:
```

**Analyser verified working** — ran successfully:
```
CSharpAnalyzerTool.exe test-csharp-project\TestCodebase.sln
Loaded 2 projects.
Analyzing project: TestConsole
Analyzing project: TestLib
  -> Saved all relation files in E:\_dev\_Projects\code-lightrag\relations_output
```

**Output format**: Per-file `.txt` files in `relations_output/` with structure `{ProjectName}_{FileName}.txt` plus `_index.txt` global index. Auto-generated files (AssemblyInfo, GlobalUsings) are included but should be filtered.

**What remains**: Wire the analyser into the MCP server's `update_relations` tool so an AI agent can trigger: "run the C# analyser on this project → ingest output → build graph" in one MCP call. Watchdog integration (auto-trigger on C# file changes) is a future enhancement.

---

## Test Suite (39 passing)
```
tests/test_smoke_lightrag.py        —  9
tests/test_smoke_mcp.py             —  9
tests/test_smoke_watchdog.py        —  0 (skipped)
tests/test_integration_lightrag.py  — 12
tests/test_integration_mcp.py       —  4
tests/test_namespace.py             —  4
tests/test_pydantic_v2.py           —  4
tests/test_retry.py                 —  3
tests/test_config.py                —  4
tests/test_mcp_edge_cases.py        —  5
```

---

## Current Integration Work

### Goal
Implement `analyser_runner.py` — a Python module that:
1. Finds the C# analyser binary (`CSharpAnalyzerTool.exe`) relative to its own package location
2. Takes a project path (.sln file or directory containing .sln)
3. Runs the analyser via `subprocess` in a temp or specified output directory
4. Filters output files (skips auto-generated `obj/` artifacts, `_index.txt`)
5. Returns metadata about generated files

Then wire it into `client.py:update_relations()` to:
1. Run the analyser on the given project path
2. Read each output `.txt` file
3. POST each to LightRAG `/documents/text` for embedding + graph building
4. Return structured results

### Key design decisions
- Analyser output goes to `<project>/.rag/` directory (consistent, not temp)
- Filters out `AssemblyInfo`, `GlobalUsings`, `AssemblyAttributes` (auto-generated noise)
- Uses `insert_text` API endpoint (same as watchdog)
- Path to analyser binary resolved relative to `__file__` in the Python package

### Blockers
- .NET 9 runtime required for analyser (available: ✅ 9.0.17)
- Docker Hub blocked on this network (Dockerfiles written but untestable locally)
- 0.5b LLM too weak for entity extraction even from clean analyser output (need 1.5b+)

### Next Steps
1. ✅ Read all relevant code (server.py, client.py, watchdog, analyser output)
2. ⬜ Implement `analyser_runner.py`
3. ⬜ Wire into `client.py:update_relations()`
4. ⬜ Update `server.py` tool description
5. ⬜ Test end-to-end: run analyser → ingest → query
