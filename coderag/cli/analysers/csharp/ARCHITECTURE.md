# Go C# Analyser — Architecture & Implementation Plan

## Purpose

This analyser extracts entities (classes, methods, properties) and relationships
(inheritance, implementation, calls) from C# codebases and outputs them in a
structured text format that feeds into the codeRAG knowledge graph pipeline.

It replaces the previous `CSharpAnalyzerTool.exe` (a .NET 9 Roslyn binary whose
source was not in this repo) with a self-contained Go module.

## Why Go, not C#+Roslyn

| Aspect | Roslyn .exe (old) | Go tree-sitter (new) |
|---|---|---|
| Language in project | C# .NET 9 — second runtime | Go — already the project language |
| Binary size | ~126 MB (145 DLLs) | ~15 MB single static binary |
| Build dependency | .NET 9 SDK installed | None (pure Go) |
| Source in repo | ❌ | ✅ |
| Cross-file call graph | ✅ semantic resolution | ✅ custom index-based resolution |
| AST quality | ✅ full Roslyn | ✅ tree-sitter C# grammar |

## Architecture Overview

The analyser is a CLI tool with two modes:

```
csharp_analyser scan --sln <path> --output-dir <dir>       ← full initial build
csharp_analyser update --changed <file> --output-dir <dir> ← incremental update
```

Output files (`.txt`) are the **single source of truth**. No persistent cache, no
database. The pipeline orchestrates incremental updates by:

1. Calling `update` for the changed file
2. Reading the diff from the analyser's stdout
3. Finding affected files by scanning output headers
4. Calling `update` for each affected file
5. Repeating until no more ripple

This pattern is **language-agnostic** — Go, Python, Rust analysers all follow the
same contract.

## Output File Format

Each `.cs` file produces one `.txt` output file with a symbol header:

```
# EXPORTS: UserService, GetUser, FetchData
# IMPORTS: DatabaseContext, ILogger
FILE: src/Foo.cs
CLASS: UserService : BaseService, IUserRepo
METHOD: GetUser -> RETURNS -> User
METHOD: FetchData -> RETURNS -> List<User>
PROPERTY: Logger -> RETURNS -> ILogger
```

### EXPORTS header

Symbols defined in this file that other files might reference:
- Class names, interface names, struct names
- Public method names (qualified: `ClassName.MethodName`)
- Public property names

### IMPORTS header

Symbols this file references that are defined in OTHER files:
- Base types (`UserService : BaseService` → imports `BaseService`)
- Interface types
- Types used in method signatures and local variables

The pipeline reads only the first line (`# EXPORTS:`) to find callers when a
symbol changes — no full file scan needed.

## Incremental Update Flow

```
Git detects: Foo.cs modified
       │
       ▼
Pipeline: csharp_analyser update --changed Foo.cs --output-dir ./output
       │
       ▼
Analyser:
  1. Parse new Foo.cs → AST → entities + relations
  2. Read old output/Foo.cs.txt (if exists)
  3. Diff old vs new → report added/removed symbols
  4. Write new output/Foo.cs.txt
  5. Print to stdout:
     REMOVED_SYMBOL: SaveAll
     ADDED_SYMBOL:   BatchSave
       │
       ▼
Pipeline reads stdout → gets REMOVED_SYMBOL: SaveAll
       │
       ▼
Pipeline greps output/*.txt for "IMPORTS.*SaveAll":
  output/Bar.cs.txt:  "# IMPORTS: ..., SaveAll, ..."
  output/Baz.cs.txt:  "# IMPORTS: ..., SaveAll, ..."
       │
       ▼
Pipeline calls analyser for each affected file (concurrently in worker pool):
  csharp_analyser update --changed Bar.cs --output-dir ./output
  csharp_analyser update --changed Baz.cs --output-dir ./output
       │
       ▼
Bar.cs's new output may itself have different exports →
  → repeat until no more ripple effects detected
```

### Change Type Detection

The diff calculation (in the analyser) compares old output vs new output:

```
OLD output/Foo.cs.txt          NEW output/Foo.cs.txt
────────────────────────────   ────────────────────────
CLASS: Foo                     CLASS: Foo                ← same
METHOD: GetUser                METHOD: GetUser           ← same
METHOD: SaveAll                ─                         ← REMOVED
─                              METHOD: BatchSave         ← ADDED
```

The analyser emits `REMOVED_SYMBOL` / `ADDED_SYMBOL` lines for each difference.

### Ripple Termination

The pipeline tracks which files have already been updated in this ripple cycle to
prevent infinite loops (A changes → B changes → A changes → ...). Each file is
processed at most once per update cycle.

## How Cross-File Resolution Works

Roslyn resolves cross-file calls via a **full compilation** (parse all files →
build semantic model → resolve symbols). Go cannot do this, so we implement a
**two-phase index-based approach**:

### Phase 1 — Symbol Indexing

Walk every `.cs` file in the solution and build a global index:

```
GlobalIndex:
  classes:    map[ClassName] → { file, baseType, interfaces, methods[] }
  methods:    map[MethodName] → [{ className, file, params, returnType }]
  properties: map[PropName]  → [{ className, file, type }]
  types:      map[TypeName]  → { file, kind (class/interface/enum/struct) }
```

### Phase 2 — Local Scope Tracking

When parsing a method body, track visible names:

```go
type Scope struct {
    variables   map[string]TypeInfo   // local vars, params
    thisType    string                 // enclosing class
    usings      []string              // using directives
    namespace   string
}
```

This handles:
- `this.Method()` → resolve in same class
- `param.Method()` → resolve via param's type
- `someVar.Method()` → resolve via variable's declared type
- `Class.Method()` → static call resolution
- `Method()` — no qualifier → same-class fallback, then global search

### Phase 3 — Call Resolution

For each call site, resolve `calleeName` + `expressionType` against the global
index to find the target file. Output format:

```
METHOD: Foo -> CALLS -> Bar [LINK: src/OtherFile.cs:42]
```

Unresolved calls are omitted (not an error — some are external/NuGet).

## Analyser ↔ Pipeline Contract

### Analyser stdout (for `update` mode)

```
REMOVED_SYMBOL: <symbolName>
ADDED_SYMBOL:   <symbolName>
UNRESOLVED_CALL: <methodName>           ← calls the pipeline could not resolve
```

### Pipeline actions

| Analyser output | Pipeline response |
|---|---|
| `REMOVED_SYMBOL: X` | Grep all `# IMPORTS:` headers for `X`, schedule affected files for re-analysis |
| `ADDED_SYMBOL: X` | No action needed — callers will pick it up when they themselves change |
| `UNRESOLVED_CALL: X` | Log warning (could indicate missing dependency or NuGet) |

## Module Structure

```
analysers/csharp/
├── go.mod                ← module definition
├── ARCHITECTURE.md       ← this file
├── TASK.md               ← active task tracking
├── main.go               ← CLI entry: scan + update commands
├── sln.go                ← .sln parser (extract project paths)
├── csproj.go             ← .csproj parser (extract source files)
├── walker.go             ← tree-sitter AST visitors
├── index.go              ← global symbol index + relation resolution
├── scope.go              ← local scope tracker for method bodies
├── output.go             ← text output writer + diff computation
└── doc.go                ← package documentation
```

## Dependencies

- `github.com/smacker/go-tree-sitter` — C# grammar bindings
- `github.com/tree-sitter/tree-sitter-c-sharp` — C# grammar

## Building

```bash
cd analysers/csharp
go build -o csharp_analyser.exe .
```

## Integration Points

| Component | File | What changes |
|---|---|---|
| Pipeline | `coderag/cli/pipeline/run.go` | `findCSharpAnalyser()` paths unchanged (same binary name) |
| Parser | `coderag/cli/parser/csharp.go` | **No changes** — same output format |
| Router | `coderag/cli/router/file.go` | **No changes** — still maps `.cs` → `"c_sharp"` |
| Setup | `scripts/setup.ps1` | Remove .NET 9 dependency, `go build` instead |
| Docker | `Dockerfile` | Add `go build` step for analyser if needed in future |
| Gitignore | `.gitignore` | Add `coderag/cli/analysers/csharp/csharp_analyser.exe` |

## Multi-Language Architecture (Future)

The same pattern applies for Go, Python, Rust analysers:

```
┌──────────────────────────────────────────────────┐
│                   Pipeline                        │
│  (change detection → orchestration → ripple)      │
└───┬──────────┬──────────┬──────────┬─────────────┘
    │          │          │          │
    ▼          ▼          ▼          ▼
  csharp/    go/        python/    rust/
  analyser   analyser   analyser   analyser
  (Go)       (Go)       (Python)   (Rust)
```

Each analyser:
1. Takes `--changed <file> --output-dir <dir>`
2. Writes `<file>.txt` with `# EXPORTS:` / `# IMPORTS:` header
3. Prints `REMOVED_SYMBOL` / `ADDED_SYMBOL` to stdout

Pipeline logic (ripple detection, grep, dedup) is shared — written once.

## Accuracy vs Roslyn

| Feature | Roslyn | Go tree-sitter + index | Gap |
|---|---|---|---|
| Class detection | ✅ | ✅ | None |
| Method detection | ✅ | ✅ | None |
| Property detection | ✅ | ✅ | None |
| Inheritance (explicit) | ✅ | ✅ | None (`: BaseClass`) |
| Interface impl (explicit) | ✅ | ✅ | None (`: IInterface`) |
| Calls — same file | ✅ | ✅ | None |
| Calls — cross file, explicit type | ✅ | ✅ | `Service.Foo()` → index resolves |
| Calls — cross file, `var` | ✅ | ⚠️ | Can't resolve inferred types |
| Calls — cross file, generics | ✅ | ⚠️ | Can't resolve type params |
| Calls — NuGet/ext methods | ✅ | ❌ | Not indexed |
| Ambiguous inheritance | ✅ resolves | ⚠️ best-effort | Name-based matching |
