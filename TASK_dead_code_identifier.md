# TASK: Dead Code Identifier Tool

## Status

Proposed. Not implemented. Replaces `file_stats` (dropped from MCP surface
on 2026-08-04).

## Motivation

`file_stats` (nodes / fan-in / fan-out per file) was dropped because it is
a developer-side analytics tool, not something agents need to answer
questions about the codebase.

Agents need a different capability: **"find symbols that are dead"** so
they can suggest cleanup or explain what code is safe to remove. This is
the natural replacement.

## Use case

> "Find functions / classes / properties that are never called, never
> referenced, and not exported."

## Heuristic (file-graph level)

An entity `E` is **dead** if:

1. `E` has **zero incoming relations** in the LightRAG knowledge graph
   (no other entity references it).
2. `E` has **non-public visibility** (`private` or `internal`).
3. `E` is **not in its file's `# EXPORTS:`** header list (when available).

Edge cases (NOT dead even if no incoming relations):

- Public API entry points (`Main`, public top-level classes).
- Extension methods (`this X y`) — check the receiver type, not the
  method itself.
- Abstract methods — must be overridden; not dead.
- Interface members — interface methods are required to be implemented.

## Proposed MCP tool

```python
@mcp_server.tool(name="dead_code")
async def find_dead_code(
    min_age_days: int = 0,        # 0 = all; >0 = filter unchanged entities
    entity_type: str = "",        # "" = all; "method", "class", etc.
) -> dict:
```

### Response model

```python
class DeadCodeEntity(BaseModel):
    name: str                     # "TestApp.OrderService.Unused"
    type: str                     # "METHOD" / "CLASS" / "PROPERTY"
    file_path: str
    line_no: int
    visibility: str               # "private" / "internal" / "public"
    reason: str                   # "no incoming relations" / "not exported" / "private and unused"

class DeadCodeResponse(BaseModel):
    entities: list[DeadCodeEntity]
    total: int
```

## Open questions (resolve before implementation)

### 1. Sourcing `# EXPORTS:` headers

Currently NOT in the graph. The walker writes the `description` field with
a `[private]` / `[public]` / `[internal]` suffix but no file-level export
list.

Options:

- **A**: Analyser-level emit of `# EXPORTS:` headers as a file-level entity
  with `type=EXPORTS`. Requires walker change in Go.
- **B**: Parse `description` for `public` modifier and count. Cheap.
- **C**: Skip the export check entirely; only flag `private`/`internal`
  with no incoming relations. Minimum viable.

### 2. Visibility extraction

Today the `description` contains a visibility suffix like
`"OrderService.Pay(order) -> RETURNS -> bool [public]"`.

A parser is needed:

- Strip trailing `[visibility]` from `description`.
- Return both `description` and `visibility` as separate fields.
- Helper lives in `walker_common.go` (or per-language if conventions
  differ; e.g. Java has `package-private`, Python has `_` prefix).

### 3. Implementation layer

- **Go (analyser)**: emit `visibility` as a separate field on each entity;
  queryable directly from LightRAG. Fast, type-safe, requires a re-scan.
- **Python (MCP)**: compute at query time by walking entities + relations.
  No schema change, slower per-call.
- **Recommendation**: Go side for `visibility` field (one-time walker
  change); Python side for the dead-code query (no scanner re-run needed
  to start using it).

### 4. Age filtering

`min_age_days=0` returns all dead code. `min_age_days=30` returns only
entities not modified in 30 days.

Requires file modification time → not currently in the graph. Either:

- Add `mtime` to file-level entity (walker change).
- Skip the age filter in v1; revisit later.

## Phased rollout

### v1 (minimum viable)

- New analyser field: `visibility` on every entity.
- New MCP tool: `dead_code(entity_type="")`.
- Heuristic: no incoming relations AND `visibility` in {private,
  internal}.
- File path / line number from existing entity metadata.

### v2

- Add `# EXPORTS:` header parsing (analyser change).
- `dead_code` checks export list.
- More accurate reasoning strings ("not in EXPORTS", "private and
  unused", "internal and unreferenced").

### v3

- Add `mtime` field to file-level entity.
- `min_age_days` filter functional.

## Future work

- Per-language support: Java, Python, TypeScript analysers will each need
  visibility helpers.
- Dead-code hotspot visualisation (per-folder aggregation) — outside MCP
  scope; could be a CLI report later.
- Integration with refactoring suggestions (rename, extract, inline) —
  out of scope.

## Files touched (when implemented)

- `coderag/cli/analysers/csharp/walker.go`: emit `visibility` field on
  entity struct.
- `coderag/cli/analysers/csharp/walker_common.go`: shared visibility
  helper.
- `coderag/cli/analysers/csharp/walker_test.go`: visibility parser tests.
- `coderag/mcp/src/coderag_mcp/models.py`: `DeadCodeEntity` +
  `DeadCodeResponse` Pydantic models.
- `coderag/mcp/src/coderag_mcp/client.py`: new method
  `find_dead_code(min_age_days, entity_type)`.
- `coderag/mcp/src/coderag_mcp/server.py`: new
  `@mcp_server.tool(name="dead_code")` wrapper with one-sentence
  docstring + example.
- `coderag/mcp/tests/test_server.py`: tool registration test.
- `coderag/mcp/tests/test_client.py`: `find_dead_code` happy path +
  empty case.
- `coderag/mcp/tests/test_integration.py`: end-to-end probe against
  LightRAG with `dead_code {}`.
- `scripts/dev/runtime-test.sh`: probe new tool.
- `scripts/dev/diag_stack.sh`: include in stack dump.

## Doc updates (when implemented)

- `README.md`: tool table row.
- `docs/USAGE.md`: example "find dead code in src/".
- `coderag --agent-help`: one-sentence description.
