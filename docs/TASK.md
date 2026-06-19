# Current Task: Phase 7 — Analyser Integration into MCP Server

## Objective
Wire the C# analyser (`CSharpAnalyzerTool.exe`) into the MCP server so the `update_relations` tool:
1. Runs the analyser on a given C# project
2. Ingests the analyser's structured output into LightRAG
3. Returns results to the AI agent

## Background
The C# analyser uses Roslyn to parse source code at the compiler level and produces clean structured text summaries (classes, methods, return types). This is the RIGHT input for LightRAG — not raw `.cs` files. The 0.5b LLM can then easily extract entities from simple text like:
```
FILE: Calculator.cs
  CLASS: Calculator
    METHOD: Add -> RETURNS -> int
```

## Steps

### 7.1 — Create analyser_runner.py
- [ ] Module in `daniel-lightrag-mcp/src/daniel_lightrag_mcp/`
- [ ] Resolves `CSharpAnalyzerTool.exe` path relative to package
- [ ] Takes project path → finds `.sln` file
- [ ] Runs analyser via `subprocess` into output directory
- [ ] Filters output: skips `_index.txt`, `AssemblyInfo`, `GlobalUsings`, `AssemblyAttributes`
- [ ] Returns list of generated file paths with metadata

### 7.2 — Wire into client.py:update_relations()
- [ ] Read each analyser output `.txt` file
- [ ] POST to LightRAG `/documents/text` with proper `file_source`
- [ ] Return structured results (files ingested, track IDs, errors)

### 7.3 — Update server.py tool description
- [ ] Update MCP `update_relations` tool with meaningful description

### 7.4 — Test end-to-end
- [ ] Run analyser on `test-csharp-project/TestCodebase.sln`
- [ ] Ingest output into LightRAG
- [ ] Query the knowledge graph
- [ ] Verify entities extracted correctly from analyser output

### 7.5 — Commit
- [ ] `git add` only intended files (no `.env`, `rag_storage`, `relations_output`, `venv`)
- [ ] `git commit` with descriptive message
