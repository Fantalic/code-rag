# Supported Files & Ingestion Paths

> Which files codeRAG understands, how each is ingested, and what the limits are.

---

## 1. Two ingestion paths

| Path | Trigger | LLM? | Where it lands |
|------|---------|------|----------------|
| **A — Direct graph CRUD** | extension has an analyser (`IsSupported`) | ❌ never | entities + relations via `/graph/…` CRUD (batched) |
| **B — LightRAG pipeline** | everything else you feed explicitly | ✅ yes (Ollama) | chunked content + extracted entities via `/documents/text` |

Path A is **deterministic** — the graph is a faithful, static picture of your code.
Path B is **semantic** — the LLM reads prose and extracts meaning.

## 2. Currently supported extensions

| Extension | Analyser | Project root | Notes |
|-----------|----------|--------------|-------|
| `.cs` | C# Go/tree-sitter analyser | `.sln` | Requires a `.sln` in or above the scanned directory |
| `.zig` | Zig Go/tree-sitter analyser | `build.zig` | The pipeline walks up from the changed `.zig` file to `build.zig` |
| `.py` | Python stdlib `ast` analyser | any folder | No third-party Python packages; `PYTHON_PATH` must find a Python 3 interpreter |

Build output and generated folders (`bin/`, `obj/`, `.coderag/`, `.git/`) are filtered before analysis.

## 3. Per-language coverage

### C#

| Entity type | Example |
|-------------|---------|
| NAMESPACE | `MyApp.Domain` |
| CLASS | `OrderService` |
| INTERFACE | `IOrderRepository` |
| STRUCT | `Point` |
| ENUM | `OrderStatus` |
| METHOD | `OrderService.PlaceOrder` |
| PROPERTY | `Order.Total`, `OrderService._order` |
| EXPORTS (file metadata) | `OrderService.cs:EXPORTS` |

Relations include inheritance, interface implementation, method calls, property
access, namespace containment, file references, and nested containment. Visibility
is recorded per symbol and powers the `dead_code` tool.

### Zig

The Zig analyser mirrors the C# format-2 output: entities, relations, source spans,
`COVERAGE` and `CHANGELOG` sidecars, and `EXPORTS` headers for `pub` symbols. Test
bodies do not emit `CALL` relations, which keeps the file graph readable.

### Python

The Python analyser uses only the standard library `ast` module. It extracts:

| Entity type | Example |
|-------------|---------|
| CLASS | `UserService` |
| FUNCTION | `load_config` |
| METHOD | `UserService.save` |
| PROPERTY | `User.name` |
| CONST / VAR | `MAX_RETRIES`, `settings` |
| IMPORT | `taiji.settings` |

It writes format-2 sidecars under `.coderag/analyser_output/`, including source
spans and the same `_coverage.txt` / `_changelog.txt` sidecars used by C# and Zig.
`relate` and `call` rows feed the derived file graph, so `graph files`, `graph deps`,
and `graph cycles` work on Python projects too.

**Limits:**

- Python analyser output is static `ast` data — it does not execute imports or code.
- A `.sln`, `build.zig`, or Python source tree must be discoverable from the scan root.
- Cross-file calls are resolved from a per-scan symbol index.

## 4. Everything else (Path B, optional)

Markdown, YAML, `.txt`, JSON, config files, and any other text can be ingested through
LightRAG's LLM pipeline (`/documents/text`). This is **opt-in in practice**: the watcher
filters to supported extensions (`router.IsSupported`) before it processes anything, so
unsupported files are ignored by the watcher. The pipeline itself contains a
`fallbackToLLM` path — anything without an analyser can be sent through the LLM
pipeline — but it is only exercised when such a file is fed explicitly (e.g. via the
REST API).

Because Path B stores the file's chunked content in the graph, watch what you index:
prose files may contain secrets or PII. See [SECURITY.md](SECURITY.md).

## 5. How the watcher decides

```
File event
  → router.IsSupported(file)?
      ├─ .cs   → C# analyser (.sln root)
      ├─ .zig  → Zig analyser (build.zig root)
      ├─ .py   → Python analyser (stdlib ast)
      └─ else  → ignored by the watcher (Path B only on explicit feed)
```

Adding a new language later is a router entry + analyser + parser — the pipeline is
language-agnostic (pipe-delimited analyser output).

---

Back to [README](../README.md) · [DOCUMENTATION.md](../DOCUMENTATION.md)
