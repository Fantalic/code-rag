# Task — Go C# Analyser Implementation

## Goal

Replace the external .NET 9 `CSharpAnalyzerTool.exe` with a self-contained Go C# analyser using tree-sitter AST parsing + cross-file call graph resolution.

Output files (`.txt`) are the single source of truth. The pipeline orchestrates incremental updates by reading `# EXPORTS:` / `# IMPORTS:` headers and re-analysing affected files.

## Steps

| # | Step | Files | Status |
|---|------|-------|--------|
| 1 | Module + CLI | `go.mod`, `main.go` | ✅ Done |
| 2 | .sln parser | `sln.go` | ✅ Done |
| 3 | .csproj parser | `csproj.go` | ✅ Done |
| 4 | AST walker (entities) | `walker.go` | ✅ Done |
| 5 | Symbol index | `index.go` | ✅ Done |
| 6 | Local scope tracker | `scope.go` | ✅ Done |
| 7 | Cross-file resolver | `resolver.go` | ✅ Done (in `index.go`) |
| 8 | Output writer + diff | `output.go` | ✅ Done |
| 9 | Incremental update mode | `main.go` (update cmd) | ✅ Done |
| 10 | Wire pipeline integration | update pipeline, verify end-to-end | ⏳ Pending |
| 11 | Test against test-csharp-project | compare output with expected format | ⏳ Pending |

## Build & Test

```bash
cd analysers/csharp
go build -o csharp_analyser.exe ./cmd/csharp_analyser

# Full scan
./csharp_analyser.exe scan --solution ../../test-csharp-project/TestCSharpProject.sln --output ./output

# Incremental update
./csharp_analyser.exe update --changed ../../test-csharp-project/Foo.cs --output ./output
```

## Notes
- The analyser uses a Go-native state-machine parser (not tree-sitter) for zero CGo dependencies and cross-compilation.
- Expression-bodied methods (`=>`) are handled as inline (no `{` scope), so `p.inMethod` is never set.
- Class scope tracking uses `classDepth` (brace depth at class declaration) instead of checking for `braceDepth == 0`, which fails when classes are nested inside namespaces.

## Progress

2026-07-28: All 9 initial implementation steps complete. Steps 10-11 pending wire-up and end-to-end testing.

---

## Proposed Implementation Order

1. ✅ **Module skeleton** (`go.mod`, `main.go`, `doc.go`) — CLI flags, error handling
2. ✅ **Solution discovery** (`sln.go`, `csproj.go`) — parse `.sln` → get `.csproj` list → get `.cs` list
3. ✅ **Output format** (`output.go`) — write the output files, diff computation, symbol header tracking
4. ✅ **AST walker** (`walker.go`) — state-machine parser for CLASS, METHOD, PROPERTY, INTERFACE
5. ✅ **Index + resolution** (`index.go`, `scope.go`) — cross-file call graph
6. ✅ **Wire up `scan` mode** — full solution scan
7. ✅ **Wire up `update` mode** — incremental single-file update with stdout diff
8. **End-to-end test** against test-csharp-project
