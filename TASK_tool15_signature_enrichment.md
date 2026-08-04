# Task: Tool 15 signature enrichment + analyser fix

**Status**: ✅ Complete (committed pending).

## Why
The MCP tool `get_method_signature` (tool 15) extracted nothing useful:
the C# analyser wrote method descriptions as `Foo() -> RETURNS -> void`
(empty `()`) because the param string was never spliced in. The MCP
regex then found the empty `()` and reported zero parameters for every
method.

For non-method entity types (CLASS, INTERFACE, STRUCT, ENUM, PROPERTY)
the C# analyser description is just `"class MyApp.OrderService"` — no
structured info worth extracting.

**Decision**: enrich the C# analyser to include parameters in all method
entity + relation descriptions, then **delete** `get_method_signature`
entirely (no Python function, no MCP wrapper, no model, no client
method, no test). The signature info becomes available through
`node_details` and `node_relations` — fewer tools, richer graph, more
future-proof for other languages.

**Tool count after this task**: 16 (was 17). One MCP tool permanently
removed.

## What was done

### C# analyser — `coderag/cli/analysers/csharp/walker.go`

Added 4 helpers:
- `balancedClose(s, openChar, closeChar byte) int` — find matching close bracket
- `findTopLevelChar(s string, c byte) int` — find first char at depth 0
- `splitParams(s string) []string` — balanced comma split respecting `()`, `<>`, `[]`
- `parseParamDecl(s string) (typeName, paramName string, ok bool)` — strips `[Attr]` attributes, leading modifiers (`out`, `ref`, `in`, `params`, `this`), trailing `= default`, then extracts `Type Name`

Enriched 4 description sites:
- Method entity description (was `Foo() -> RETURNS -> bool`)
  → `Foo(Order order, int qty) -> RETURNS -> bool`
- Class → Method relation (HAS_METHOD)
  → includes full signature in description
- Method → ReturnType relation
  → includes signature: `Foo(Order order) -> RETURNS -> Type`
- Method → ParamType relation
  → includes signature: `Foo(Order order) uses Type param name`

Rewrote param parsing loop using the new helpers; correctly resolves
`out`/`ref`/`params`/`this` modifiers and `[Attr]` attributes.

### Testdata — `coderag/cli/analysers/csharp/testdata/TestApp/ComplexMethods.cs`

New file `ComplexMethods.cs` with a `TestApp.ComplexMethods` class
exercising 8 edge cases:

| Method | Edge case covered |
|---|---|
| `TryParse(string input, out int result)` | `out` keyword |
| `Swap(ref int a, ref int b)` | `ref` keyword (2 params) |
| `Log(params string[] messages)` | `params` keyword + arrays |
| `Process(Func<int, bool> predicate, int seed)` | Lambda delegate type |
| `Send(string to, int priority = 0, bool ccSender = true)` | Default parameter values |
| `Save([NotNull] string key, [NotNull] string value)` | Attributes on params |
| `ExtendTo(this string s, int times)` | Extension method (`this`) |
| `GetAsync(int id, CancellationToken ct = default)` | Async + `Task<T>` + default |

Auto-included via SDK-style `TestApp.csproj` (ImplicitUsings=enable).

Expected output regenerated at:
`testdata/.coderag/analyser_output/TestApp__ComplexMethods.cs.txt`

Existing testdata (`OrderService.cs`) shows the enrichment too — was
`PlaceOrder() -> RETURNS -> void`, now `PlaceOrder(Order order) -> RETURNS -> void`.

### MCP — `coderag/mcp/src/coderag_mcp/`

Deleted entirely (no commented stubs):
- `server.py` — `get_method_signature` async function removed
- `client.py` — `async def get_method_signature` method + `MethodSignatureResponse` import removed
- `models.py` — `MethodSignatureResponse` class removed
- `tests/test_client.py` — `test_get_method_signature_exists` removed

The MCP server no longer exposes `get_method_signature`. The Python
function does not exist on `LightRAGClient`. The model class is gone.

### Scripts

- `scripts/dev/runtime-test.sh` — updated probe calls to new MCP tool names:
  `get_file_stats` → `file_stats`,
  `get_file_dependencies` → `file_dependencies`,
  `file_dependency_cycles` → `detect_circular_dependencies`.
- `scripts/dev/diag_stack.sh` — same update for `file_stats`.

## Verification

- `go test ./coderag/cli/analysers/csharp/...` — all 12 new tests pass:
  - `TestSplitParamsBalanced` (8 cases)
  - `TestParseParamDecl` (19 cases incl. `out`, `ref`, `params`, `this`,
    `[Attr]`, `= default`, generics, tuples, nullable, arrays)
  - `TestParseMethodWithOutKeyword`
  - `TestParseMethodWithRefKeyword`
  - `TestParseMethodWithParamsKeyword`
  - `TestParseMethodWithLambdaDelegateType`
  - `TestParseMethodWithTupleType` (splitParams unit)
  - `TestParseMethodWithAttributeOnParam`
  - `TestParseMethodWithDefaultValue`
  - `TestParseMethodNoParams`
  - `TestParseMethodRelationsWithNamespace` (depends_on + namespace)
  - `TestParseComplexMethodsFixture` (parses actual testdata file)
- `pytest tests/` — 146 tests pass (was 147; one removed test).
- `black && isort` — clean.
- `sh scripts/dev/runtime-test.sh --clean` — full Docker rebuild + scan
  + CLI probes + MCP probes, all green.
- Live MCP probe via `mcp_probe.py node_details` on
  `TestApp.ComplexMethods.TryParse` returns:
  `description = "TestApp.ComplexMethods.TryParse(string input, out int result) -> RETURNS -> bool [public]"`
  and a relation with description
  `"TestApp.ComplexMethods.TryParse(string input, out int result)"`.

## Files changed

| File | Change |
|---|---|
| `coderag/cli/analysers/csharp/walker.go` | +4 helpers, +4 description enrichments, rewrite param parsing |
| `coderag/cli/analysers/csharp/walker_test.go` | +12 new tests |
| `coderag/cli/analysers/csharp/testdata/TestApp/ComplexMethods.cs` | new testdata file (8 edge cases) |
| `coderag/cli/analysers/csharp/testdata/.coderag/analyser_output/TestApp__ComplexMethods.cs.txt` | regenerated expected output |
| `coderag/cli/analysers/csharp/testdata/.coderag/analyser_output/TestApp__OrderService.cs.txt` | regenerated (PlaceOrder params now in description) |
| `coderag/mcp/src/coderag_mcp/server.py` | `get_method_signature` async fn deleted |
| `coderag/mcp/src/coderag_mcp/client.py` | `get_method_signature` method + import deleted |
| `coderag/mcp/src/coderag_mcp/models.py` | `MethodSignatureResponse` class deleted |
| `coderag/mcp/tests/test_client.py` | `test_get_method_signature_exists` removed |
| `scripts/dev/runtime-test.sh` | 3 MCP probe calls renamed to new tool names |
| `scripts/dev/diag_stack.sh` | 1 MCP probe call renamed |
| `_dev_docs/analysers/csharp/csharp_analyser.exe` | rebuilt binary |

## Known limitations

- **Tuple types** like `(int, int) coords`: the `methodRegex` in
  `walker.go:15` uses `[^)]*` for the params capture, which truncates at
  the first `)` inside a tuple type. The unit test `TestSplitParamsBalanced`
  proves the helper handles tuples correctly; the upstream regex still
  doesn't capture them. Fixing the regex to handle balanced parens in
  Go regex (no recursion available) is a separate task — defer or use a
  manual line scanner.
- **Async modifier** appears to parse correctly (`GetAsync(int id, CancellationToken ct = default) -> RETURNS -> Task<Order>`),
  but the call-detection still misidentifies `match(item)` inside `Find<T>` body
  as a separate METHOD entity. Out of scope; pre-existing limitation.

## Out of scope (deferred)

- Enriching CLASS / INTERFACE / STRUCT / ENUM entity descriptions with
  base type, parent interfaces, member lists.
- Fixing the tuple-type regex limitation.
- Adding `coderag --agent-help` flag.
- Updating README.md — happens after all tool renames are locked.
- Fixing the false-positive METHOD entity detection in `Find<T>` body.

## Future-proofing

- `docs/SUPPORTED_FILES.md` should mention: "Method signatures appear
  in the entity description and in HAS_METHOD / DEPENDS_ON relation
  descriptions. There is no dedicated `get_method_signature` MCP tool
  because the info is fully available via `node_details` and
  `node_relations`."
- Future analysers (Java, Python, TypeScript) should follow the same
  convention: parameter lists always included in method entity +
  relation descriptions.
- Same `splitParams` / `parseParamDecl` helper pair should be lifted to
  a shared package so other analysers can reuse it.

## Step 1 — walker.go

### 1a — Add four private helpers (near top of file, after imports/types)

```go
// balancedClose returns the index of the matching close bracket for
// s[start] == openChar (which must be at position 0). Returns -1 if
// no match (unbalanced input).
func balancedClose(s string, openChar, closeChar byte) int {
    depth := 0
    for i := 0; i < len(s); i++ {
        switch s[i] {
        case openChar:
            depth++
        case closeChar:
            depth--
            if depth == 0 {
                return i
            }
        }
    }
    return -1
}

// findTopLevelChar returns the index of the first occurrence of c at
// depth 0 (outside any (), <>, or []). Returns -1 if not found.
func findTopLevelChar(s string, c byte) int {
    depth := 0
    for i := 0; i < len(s); i++ {
        switch s[i] {
        case '(', '<', '[':
            depth++
        case ')', '>', ']':
            if depth > 0 {
                depth--
            }
        case c:
            if depth == 0 {
                return i
            }
        }
    }
    return -1
}

// splitParams splits a C# parameter list by commas at depth 0.
// Respects (), <>, and [] so generic and tuple types stay intact.
func splitParams(s string) []string {
    var result []string
    start := 0
    depth := 0
    for i := 0; i < len(s); i++ {
        switch s[i] {
        case '(', '<', '[':
            depth++
        case ')', '>', ']':
            if depth > 0 {
                depth--
            }
        case ',':
            if depth == 0 {
                result = append(result, strings.TrimSpace(s[start:i]))
                start = i + 1
            }
        }
    }
    if start <= len(s) {
        rest := strings.TrimSpace(s[start:])
        if rest != "" {
            result = append(result, rest)
        }
    }
    return result
}

// parseParamDecl extracts (type, name) from one C# parameter declaration.
// Strips leading [Attr] attributes, leading modifiers (out, ref, in,
// params, this) and trailing "= default" values.
// Returns ok=false if a name cannot be extracted.
func parseParamDecl(s string) (typeName, paramName string, ok bool) {
    s = strings.TrimSpace(s)
    if s == "" {
        return "", "", false
    }
    // Strip leading [Attr1, Attr2] blocks
    for strings.HasPrefix(s, "[") {
        end := balancedClose(s, '[', ']')
        if end < 0 {
            return "", "", false
        }
        s = strings.TrimSpace(s[end+1:])
    }
    // Strip leading modifier
    lower := strings.ToLower(s)
    for _, mod := range []string{"out ", "ref ", "in ", "params ", "this "} {
        if strings.HasPrefix(lower, mod) {
            s = strings.TrimSpace(s[len(mod):])
            lower = strings.ToLower(s)
            break
        }
    }
    // Strip trailing "= default" value
    if eq := findTopLevelChar(s, '='); eq >= 0 {
        s = strings.TrimSpace(s[:eq])
    }
    if strings.TrimSpace(s) == "" {
        return "", "", false
    }
    // Find last identifier (the param name)
    re := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*\s*$`)
    loc := re.FindStringIndex(s)
    if loc == nil {
        return "", "", false
    }
    paramName = s[loc[0]:loc[1]]
    typeName = strings.TrimSpace(s[:loc[0]])
    if typeName == "" {
        return "", "", false
    }
    return typeName, paramName, true
}
```

`regexp` is already imported in walker.go.

### 1b — Enrich method entity description (current lines 379–382)

Replace:
```go
desc := fmt.Sprintf("%s.%s()%s", cls, methodName, visSuffix)
if returnType != "" {
    desc = fmt.Sprintf("%s.%s() -> RETURNS -> %s%s", cls, methodName, returnType, visSuffix)
}
```

With:
```go
paramStr := "()"
if params != "" {
    paramStr = "(" + params + ")"
}
desc := fmt.Sprintf("%s.%s%s%s", cls, methodName, paramStr, visSuffix)
if returnType != "" {
    desc = fmt.Sprintf("%s.%s%s -> RETURNS -> %s%s", cls, methodName, paramStr, returnType, visSuffix)
}
```

### 1c — Enrich Class→Method relation description (line 395)

Replace `Description: fmt.Sprintf("%s.%s()", cls, methodName),`
With    `Description: fmt.Sprintf("%s.%s%s", cls, methodName, paramStr),`

### 1d — Enrich Method→ReturnType relation description (line 407)

Replace `Description: fmt.Sprintf("%s.%s() -> RETURNS -> %s", cls, methodName, resolvedType),`
With    `Description: fmt.Sprintf("%s.%s%s -> RETURNS -> %s", cls, methodName, paramStr, resolvedType),`

### 1e — Enrich Method→ParamType relation description (line 439)

Replace `Description: fmt.Sprintf("%s.%s() uses %s param %s", cls, methodName, resolvedType, paramName),`
With    `Description: fmt.Sprintf("%s.%s%s uses %s param %s", cls, methodName, paramStr, resolvedType, paramName),`

### 1f — Rewrite param parsing loop (lines 420–447)

Replace:
```go
if params != "" {
    paramPairs := strings.Split(params, ",")
    for _, pair := range paramPairs {
        pair = strings.TrimSpace(pair)
        if pair == "" {
            continue
        }
        parts := strings.Fields(pair)
        if len(parts) >= 2 {
            paramType := parts[0]
            paramName := parts[len(parts)-1]
            resolvedType := p.resolveType(paramType)
            p.scopeVars[len(p.scopeVars)-1][paramName] = resolvedType
            p.imports = append(p.imports, paramType)
            if p.index != nil && resolvedType != paramType {
                if _, ok := p.index.Classes[resolvedType]; ok {
                    p.addRelation(Relation{
                        Source:      qualifiedName,
                        Target:      resolvedType,
                        Description: fmt.Sprintf("%s.%s() uses %s param %s", cls, methodName, resolvedType, paramName),
                        Keywords:    "depends_on",
                        Weight:      0.3,
                    })
                }
            }
        }
    }
}
```

With:
```go
if params != "" {
    for _, pair := range splitParams(params) {
        paramType, paramName, ok := parseParamDecl(pair)
        if !ok || paramName == "" {
            continue
        }
        resolvedType := p.resolveType(paramType)
        p.scopeVars[len(p.scopeVars)-1][paramName] = resolvedType
        p.imports = append(p.imports, paramType)
        if p.index != nil && resolvedType != paramType {
            if _, ok := p.index.Classes[resolvedType]; ok {
                p.addRelation(Relation{
                    Source:      qualifiedName,
                    Target:      resolvedType,
                    Description: fmt.Sprintf("%s.%s%s uses %s param %s", cls, methodName, paramStr, resolvedType, paramName),
                    Keywords:    "depends_on",
                    Weight:      0.3,
                })
            }
        }
    }
}
```

## Step 2 — walker_test.go: add 8 tests

| Test | Source | What it asserts |
|---|---|---|
| `TestParseMethodWithOutKeyword` | `class C { bool TryParse(string s, out int result) { return true; } }` | depends_on → `int` (not `out`); description contains `out int result` |
| `TestParseMethodWithRefKeyword` | `class C { void Swap(ref int a, ref int b) { } }` | 2 depends_on params, both target `int`; description contains `ref int a, ref int b` |
| `TestParseMethodWithParamsKeyword` | `class C { void Log(params string[] msgs) { } }` | depends_on → `string[]` (not `params`); description contains `params string[] msgs` |
| `TestParseMethodWithLambdaDelegateType` | `class C { void Process(Func<int, bool> predicate) { } }` | description contains `Func<int, bool> predicate`; no wrong `depends_on` for `Func<` |
| `TestParseMethodWithTupleType` | `class C { void Use((int, int) coords) { } }` | description contains `(int, int) coords` (NOT `(int)`); splitParams correctly nested |
| `TestParseMethodWithAttributeOnParam` | `class C { void Foo([NotNull] string s) { } }` | description contains `[NotNull] string s`; depends_on → `string` |
| `TestParseMethodWithDefaultValue` | `class C { void Bar(int x = 5, string label = "hi") { } }` | description contains `int x = 5, string label = "hi"`; 2 depends_on params, types `int` + `string` |
| `TestParseMethodNoParams` | `class C { void Baz() { } }` | description is `C.Baz() -> RETURNS -> void` (unchanged behavior) |

Index `index.Classes` should contain entries for `int`, `string`, `string[]`, `(int, int)` as appropriate. For tuple/array types that aren't real classes, no depends_on relation is emitted (this is correct — only emits relations for types actually defined in the codebase).

For tests that need a class to exist for the depends_on target to resolve, seed the index with `index.Classes["int"] = &ClassInfo{Name: "int"}`, etc. Use `C.Int` as the target class for out/ref tests so the relation actually emits.

Helper function for tests:
```go
func parseMethodContent(t *testing.T, src string) *FileResult {
    t.Helper()
    tmpFile := filepath.Join(t.TempDir(), "test.cs")
    if err := os.WriteFile(tmpFile, []byte(src), 0644); err != nil {
        t.Fatal(err)
    }
    index := NewGlobalIndex()
    result, err := ParseFile(tmpFile, index)
    if err != nil {
        t.Fatal(err)
    }
    return result
}

func methodEntity(t *testing.T, r *FileResult) Entity {
    t.Helper()
    for _, e := range r.Entities {
        if e.Type == EntityMethod {
            return e
        }
    }
    t.Fatal("no METHOD entity found")
    return Entity{}
}
```

## Step 3 — server.py: comment out get_method_signature

`coderag/mcp/src/coderag_mcp/server.py` line 396:

Replace `@mcp_server.tool()` decorator (line 396) with `# @mcp_server.tool()`.
Add a `raise NotImplementedError(...)` to the body and update the docstring.

Pattern: follow the same shape used for `query`, `get_related_files`, `search_entities`.

```python
# @mcp_server.tool()
async def get_method_signature(entity_name: str) -> dict:
    """Not exposed as MCP tool. Method signatures are now in entity + relation descriptions; use `node_details` or `node_relations` to retrieve them."""
    raise NotImplementedError(
        "get_method_signature is not exposed. Method signatures live in the "
        "node description and the HAS_METHOD / DEPENDS_ON relation descriptions. "
        "Use node_details or node_relations."
    )
```

`tests/test_client.py::test_get_method_signature_exists` stays as-is
(Python method still exists on `LightRAGClient`).

## Step 4 — Rebuild, redeploy, regen testdata

```bash
cd coderag/cli/analysers/csharp
go build -o csharp_analyser.exe ./cmd/csharp_analyser
cd ../../..
copy coderag\cli\analysers\csharp\csharp_analyser.exe _dev_docs\analysers\csharp\
```

Then regenerate testdata by running the analyser against the
`testdata/TestApp.sln` (the existing `runtime-test.sh --clean` will do
this end-to-end):

```bash
sh scripts/dev/runtime-test.sh --clean
```

The `TestApp__OrderService.cs.txt` line for `PlaceOrder` will change from
`METHOD|...|...|14|TestApp.OrderService.PlaceOrder() -> RETURNS -> void [public]`
to
`METHOD|...|...|14|TestApp.OrderService.PlaceOrder(Order order) -> RETURNS -> void [public]`.

## Step 5 — Verify

```bash
# Go tests
cd coderag/cli/analysers/csharp && go test ./... && cd ../../..

# Python lint + tests
cd coderag/mcp
python -m black --check src/ tests/
python -m isort --check-only src/ tests/ --profile black
python -m mypy src/ --python-version 3.10 --disallow-untyped-defs --warn-return-any
python -m pytest tests/ -q
```

Expectations:
- `go test`: all old + 8 new tests pass.
- `black`, `isort`: clean.
- `mypy`: same 28 pre-existing errors as before, no new ones.
- `pytest`: 147 tests still pass.

## Step 6 — End-to-end

After `runtime-test.sh --clean` rebuilds LightRAG with the new analyser:

```bash
python scripts/dev/mcp_probe.py node_details TestApp.OrderService.PlaceOrder
# Expect: description contains "PlaceOrder(Order order) -> RETURNS -> void [public]"

python scripts/dev/mcp_probe.py node_relations TestApp.OrderService method
# Expect: relation descriptions contain signatures

python scripts/dev/mcp_probe.py get_method_signature TestApp.OrderService.PlaceOrder
# Expect: error response (NotImplementedError); tool not in list_tools()
```

## Out of scope (do NOT do in this task)

- Enriching CLASS / INTERFACE / STRUCT / ENUM entity descriptions with
  base type, parent interfaces, or member lists.
- Updating the MCP `MethodSignatureResponse` model — it's still
  internal and unused from MCP now.
- Adding `coderag --agent-help` flag.
- Updating README.md — happens after all tool renames are locked.

## Open follow-ups

- `docs/SUPPORTED_FILES.md` should mention: "Method signatures appear
  in the entity description and in HAS_METHOD / DEPENDS_ON relation
  descriptions. There is no dedicated `get_method_signature` MCP tool
  because the info is fully available via `node_details` and
  `node_relations`."
- Future analysers (Java, Python, TypeScript) should follow the same
  convention: parameter lists always included in method entity +
  relation descriptions.
