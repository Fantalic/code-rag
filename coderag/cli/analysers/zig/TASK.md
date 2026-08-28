# Task — Go Zig Analyser Implementation

## Goal

Add a self-contained Go Zig analyser that uses tree-sitter-zig + cross-file
call graph resolution and emits the same `# FORMAT: 2` text output the C#
analyser already produces. Pipeline, parser, and MCP tools work for both
languages without forking.

Output files (`.txt`) are the single source of truth. The pipeline
orchestrates incremental updates by reading `# EXPORTS:` / `# IMPORTS:`
headers and re-analysing affected files.

## Steps

| # | Step | Files | Status |
|---|------|-------|--------|
| 1 | Module + CLI (`scan`/`update`) | `go.mod`, `main.go`, `cmd/zig_analyser/main.go` | ✅ Done |
| 2 | Output format (`OutputFormatVersion = 2`, EXPORTS, IMPORTS, MTIME, sidecars) | `output.go` | ✅ Done |
| 3 | AST walker (entities: STRUCT/ENUM/UNION/OPAQUE/ERROR_SET/FUNCTION/CONST/ENUM_MEMBER/TEST/IMPORT) | `ast_walker.go` | ✅ Done |
| 4 | Scope tracker | `scope.go` | ✅ Done |
| 5 | Symbol index + cross-file resolver | `index.go` | ✅ Done |
| 6 | Per-file ParseFile + dedupe + emitFileMetadata | `walker.go` | ✅ Done |
| 7 | Incremental update mode (stdout ADDED/REMOVED_SYMBOL) | `main.go` | ✅ Done |
| 8 | Zig test project fixture (every entity type) | `testdata/ZigApp/` | ✅ Done |
| 9 | Parser for Zig output (`parser.ParseZigFile` / `ParseZigOutput`) | `coderag/cli/parser/zig.go` | ✅ Done |
| 10 | Wire router (`router/file.go`) and pipeline (`run.go`: `runZigPipeline`, `RunZigAnalyser`, `findZigAnalyser`, `findZigProjectDir`) | `coderag/cli/{router,pipeline}/...` | ✅ Done |
| 11 | .gitignore update | `.gitignore` | ✅ Done |
| 12 | AGENTS.md update | `AGENTS.md` | ✅ Done |
| 13 | Parser tests (line-format + dedup) | `parser/zig_test.go` | ✅ Done |
| 14 | Walker/format tests | `walker_test.go`, `testhelpers.go` | ✅ Done |
| 15 | End-to-end smoke test against `testdata/ZigApp` | manual | ✅ Done |
| 16 | End-to-end test through pipeline (LightRAG round-trip) | manual via `scripts/dev/runtime-test.sh` | ⏳ Pending |
| 17 | Arch docs | `ARCHITECTURE.md`, `TASK.md` | ✅ Done |

## Build & Test

```bash
# Build
cd coderag/cli/analysers/zig
go build -o zig_analyser.exe ./cmd/zig_analyser/

# Full scan
./zig_analyser.exe scan --dir ./testdata/ZigApp --output ./out

# Incremental update
./zig_analyser.exe update --changed ./testdata/ZigApp/src/order.zig --output ./out

# Tests
cd ../..            # coderag/cli
go test ./parser/ -v -run TestParseZig
cd ../analysers/zig
go test ./...
```

## Notes

- The analyser uses tree-sitter-zig (Go bindings) via `cgo`. Same
  constraint as the C# analyser — `gcc` must be on PATH (MinGW on
  Windows, gcc on Linux/macOS).
- Zig grammar quirks that drove design:
  - `pub`, `const`, `fn` are **anonymous TOKEN children**, not named
    nodes. Visibility detection walks `n.Child(i)` for **all** children.
  - For `pub const X = struct {...};` the identifier X is a **direct
    sibling** of the inline type — no `_variable_declaration_header`
    wrapper exists.
  - `struct_declaration`/`enum_declaration` body is **NOT** wrapped
    in a `body` field — children are direct; we walk the node itself.
  - `expression_statement`, `return_expression`, `defer_statement`
    need explicit `walkExpression` handling so call sites buried
    inside `try_expression` etc. are not dropped.
  - `walkVariableRHS` skips inline type children to avoid walking
    function bodies twice (which would emit duplicate CALL rows).
- Test bodies do NOT produce CALL relations — `isInsideTest` is
  bumped while descending into `test_declaration` bodies.
- Free-function heuristic: every `fn` inside a struct body becomes
  `Parent.method` (qualified), regardless of whether it takes
  `self`. No first-param detection — accepted limitation.
- The Zig analyser has no `.sln`/`.csproj` equivalent. The pipeline
  walks upward from the changed file looking for a `build.zig`
  marker (or falls back to the file's directory).

## Progress

2026-08-28: All implementation + parser + walker + smoke test steps
complete. Step 16 (end-to-end LightRAG round-trip) pending — requires
the Docker stack to be up. Run `sh scripts/dev/runtime-test.sh` to
exercise the full path.
