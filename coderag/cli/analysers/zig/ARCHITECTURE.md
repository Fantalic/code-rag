# Go Zig Analyser — Architecture & Implementation Plan

## Purpose

This analyser extracts entities (functions, structs, enums, error sets, fields,
methods, tests) and relationships (member, field-of, cross-file calls) from
Zig codebases and outputs them in the same `# FORMAT: 2` text format the
existing C# analyser emits. The output feeds into the codeRAG knowledge
graph pipeline unchanged.

It mirrors the contract documented in
[`../csharp/ARCHITECTURE.md`](../csharp/ARCHITECTURE.md) so the pipeline, the
parser, the MCP tools, and any future language analyser see Zig identically
to C#.

## Why Go, not tree-sitter-cli or zig itself

| Aspect | zig AST (built-in) | Go tree-sitter (this) |
|---|---|---|
| Source in repo | zig stdlib only | Go — already the project language |
| Binary size | n/a (requires zig runtime) | ~6.4 MB single static binary |
| Build dependency | zig compiler + lib | None (pure Go + cgo) |
| Coverage sidecar | not available | `_coverage.txt` + `index_coverage` MCP tool |
| Changelog sidecar | not available | `_changelog.txt` + `what_changed` MCP tool |
| Compatible with parser.go contract | ❌ | ✅ |

Using the same Go tree-sitter stack as the C# analyser means:
- One dependency set (`go-tree-sitter v0.25.0`) to vendor.
- One output format the parser already understands.
- One sidecar contract (`_coverage.txt`, `_changelog.txt`) for the MCP tools.

## Architecture Overview

```
zig_analyser scan   --dir <project>          --output-dir <dir>  ← full initial build
zig_analyser update --changed <file>         --output-dir <dir>  ← incremental update
```

Zig has no `.sln`/`.csproj` analogue. The "project root" is the directory
containing `build.zig`; `findZigProjectDir` in `pipeline/run.go` walks up from
the changed file looking for that marker. When no `build.zig` exists, the
file's own directory is used as the project root.

The analyser is a self-contained Go binary. The pipeline shells out via
`RunZigAnalyser`, reads the `.txt` sidecars back from a temp dir, and feeds
them to `parser.ParseZigFile` / `parser.ParseZigOutput` — exactly the same
flow used for C#.

## Output File Format

Each `.zig` file produces one `.txt` sidecar with the standard `# FORMAT: 2`
header, plus per-file `# EXPORTS:` and `# IMPORTS:` headers that the
incremental-update path greps to find affected callers.

```
# FORMAT: 2
# EXPORTS: Order, OrderError, Status, totalPrice, validate
# MTIME: 1787938829
# IMPORTS: std
ENUM|Status|src/order.zig|8|14|enum [pub]
ENUM_MEMBER|Status.pending|src/order.zig|9|9|Status.pending
ERROR_SET|OrderError|src/order.zig|16|20|error set
STRUCT|Order|src/order.zig|22|27|struct [pub]
FUNCTION|totalPrice|src/order.zig|30|32|fn totalPrice [pub]
FUNCTION|validate|src/order.zig|35|38|fn validate [pub]
TEST|test:status transitions cover all states|src/order.zig|40|43|test status transitions cover all states
EXPORTS|order:EXPORTS|src/order.zig|1|1|Status, OrderError, Order, totalPrice, validate [MTIME=1787938829]
RELATION|Status|Status.pending|Status.pending|member, has|src/order.zig|0.7
RELATION|Order|Order.id|Order has field id|field, defines|src/order.zig|0.7
```

### EXPORTS header

Pub-declared symbols defined in this file that other files might reference:
- `pub const X = struct {...}` → `X` (STRICT/ENUM/ERROR_SET/UNION/OPAQUE)
- `pub fn foo(...)` → `foo` (FUNCTION)
- `pub var x: T = ...` → `x` (CONST)

Non-pub symbols do **not** appear in EXPORTS — visibility is enforced by
scanning every child (anonymous tokens included) for the `pub` keyword.

### IMPORTS header

Symbols this file references that may be defined in OTHER files:
- Every `@import("foo")` target → `foo`
- Imported via `usingnamespace` blocks.

## Entities & Relations

| Kind | Source tree-sitter node | Visibility filtering |
|---|---|---|
| STRUCT | `struct_declaration` | `pub` only — but only when the *outer* `const X = struct {...}` is pub |
| ENUM | `enum_declaration` | same as STRUCT |
| UNION | `union_declaration` | same as STRUCT |
| OPAQUE | `opaque_declaration` | same as STRUCT |
| ERROR_SET | `error_set_declaration` | same as STRUCT |
| FUNCTION | `function_declaration` outside any struct body | `pub` only |
| METHOD | `function_declaration` inside a struct body | `pub` only; qualified as `Parent.method` |
| CONST | `container_field` (`name: T`) | `pub` only |
| ENUM_MEMBER | enum member identifier | always (no visibility on members) |
| TEST | `test_declaration` | always |
| IMPORT | `@import(...)` call | always |

Relations emitted per file:

| Edge | Keywords | Weight | Source |
|---|---|---|---|
| member, has | `member, has` | 0.7 | every enum/error-set member |
| field, defines | `field, defines` | 0.7 | every struct field |
| calls | `calls` | 0.5 | every cross-file call row (see below) |

CALL rows are emitted as a separate line type
(`CALL|CallerFile|CallerMethod|CalleeName|CalleeOwner|LineNo`) and reified
into `Relation{Source: CallerMethod, Target: CalleeName, Keywords: "calls"}`
by `parser.parseZigCallLine`. Tests do NOT emit CALL relations — the
walker bumps `isInsideTest` while descending into `test_declaration`
bodies, suppressing call edges from verification scaffolding.

## Cross-file Call Resolution

Cross-file call resolution uses a `GlobalIndex` built up while files are
parsed in parallel:

1. After every file is parsed, `IndexBuilder.AddFile` registers its
   exports under the basename (e.g. `place` → `OrderService.place`).
2. While walking the next file, when a `field_expression`
   (`svc.place`) is encountered, the parser looks up `place` in
   `index.SimpleNames` and picks the candidate whose owner matches
   the local identifier `svc`. Candidates without a matching owner
   fall back to the first hit (or are dropped if disambiguation
   fails).
3. The call edge is emitted as
   `CALL|file|method|callee|owner|line`.

Simple-name resolution works for the common "one function name per
project" case but cannot disambiguate shadowed names across distinct
owners without first parsing the receiving field. That's the same
limit the C# analyser has — both rely on per-file cross-checks at
the language-layer downstream.

## Coverage Sidecar

Every scan writes `_coverage.txt` next to the per-file `.txt`
sidecars. The format matches the C# analyser's:

```
# COVERAGE
project_dir=/path/to/ZigApp
files_visited=4
files_with_warnings=0
files_failed=0
files=src/main.zig,src/order.zig,src/inventory.zig,src/order_service.zig
warnings_per_file=
summary={"files":4,"warnings":"-"}
```

`parser.ParseZigFile` reads `COVERAGE|<key>|<json>` lines and emits
`COVERAGE:<key>` entities so the `index_coverage` MCP tool can list
parser gaps without grepping the sidecar directly.

## Changelog Sidecar

Incremental updates append a `CHANGELOG|<file>|<json>` line to
`_changelog.txt`. `parser.ParseZigFile` reifies these into
`CHANGELOG:<file>` entities that the `what_changed` MCP tool reads.

The diff itself is computed by `output.go:ComputeDiff` from the
sorted set of EXPORTS entries (file-level) plus the entity name set
(file-level). New exports appear in `AddedSymbols`, vanished ones
in `RemovedSymbols`. Both lists are printed to stdout in
`update` mode so the pipeline can grep `# IMPORTS:` headers in
other files to find callers.

## Test Bodies Don't Emit Calls

Test blocks (`test_declaration`) are walked to harvest their entity,
but every call expression inside the body is dropped before emission.
This mirrors the "verification scaffolding is noise" intent captured
in the C# analyser's comment about test-only callers and keeps the
knowledge graph from ballooning with `test "X" -> production_code`
edges that don't reflect real call structure.

## Pipeline Integration

The parent CLI module wires Zig alongside C#:

- `router/file.go`: `.zig → "zig"` in `extensionMap`.
- `pipeline/run.go`:
  - `Run()` / `RunEventWithChangeType()` dispatch `analyser == "zig"`
    to `runZigPipeline` / `runZigPipelineWithResults` /
    `runZigPipelineModified`.
  - `runZigPipeline` resolves the project dir via
    `findZigProjectDir`, shells out via `RunZigAnalyser`, and applies
    results through `applyZigResults` — which mirrors
    `applyCSharpResults` byte-for-byte except for the parser call.
  - `RunZigAnalyser` honours `ZIG_ANALYSER_PATH` for non-canonical
    installs and probes the same lookup paths as
    `findCSharpAnalyser` (env var, repo-relative, executable-relative).
- `parser/zig.go` parses per-file sidecars and merges duplicates
  across files (Zig has no partial-class concept so no
  `duplicates.json` override path is needed).

## Source Layout

```
coderag/cli/analysers/zig/
├── go.mod                                  # nested module
├── doc.go                                  # package doc comment
├── types.go                                # Entity/Relation/FileResult types
├── main.go                                 # CLI entry dispatch
├── walker.go                               # per-file ParseFile + dedupe + exports
├── scope.go                                # linked-list-of-maps Scope tracker
├── ast_walker.go                           # tree-sitter-zig walk (~900 lines)
├── output.go                               # writer + ComputeDiff + sidecars
├── index.go                                # parallel IndexBuilder + GlobalIndex
├── collect.go                              # file discovery + coverage collection
├── dump.go                                 # dumper helper (dev tool)
├── walker_test.go                          # walker/format tests
├── testhelpers.go                          # fs helpers for tests
├── cmd/
│   ├── zig_analyser/main.go                # production binary
│   ├── ast_dump/main.go                    # dev AST inspector (anonymous tokens)
│   └── dumper/main.go                      # dev entity/relation/import dumper
└── testdata/ZigApp/                        # fixture covering every entity type
    ├── build.zig
    ├── README.md
    └── src/
        ├── main.zig
        ├── order.zig
        ├── order_service.zig
        └── inventory.zig
```

## Build

```bash
cd coderag/cli/analysers/zig
go build -o zig_analyser.exe ./cmd/zig_analyser/
```

`go.mod` declares the nested module
(`github.com/coderag/analysers/zig`) with cgo (`CGO_ENABLED=1` + gcc).
Tree-sitter pinning is intentional: `go-tree-sitter v0.25.0` +
`tree-sitter-grammars/tree-sitter-zig v1.1.2`. On Windows the
default gcc is the MinGW toolchain installed by Git for Windows.

The binary is gitignored (`/coderag/cli/analysers/zig/zig_analyser.exe`).
For non-canonical installs, set `ZIG_ANALYSER_PATH` — the same
`findZigAnalyser` probes used by the C# flow will pick it up.

## Run

```bash
# Initial scan
zig_analyser.exe scan --dir ./testdata/ZigApp --output ./out

# Incremental update
zig_analyser.exe update --changed ./testdata/ZigApp/src/order.zig --output ./out
```

## Tests

```bash
# Parser round-trip
cd coderag/cli && go test ./parser/ -v -run TestParseZig

# Walker + format
cd coderag/cli/analysers/zig && go test ./...
```
