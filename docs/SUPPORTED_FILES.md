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

The only registered analyser today is **C#**:

| Extension | Analyser | Notes |
|-----------|----------|-------|
| `.cs` | `c_sharp` (Go native analyser) | Requires a `.sln` in (or above) the scanned directory |

**Excluded even for `.cs`:** anything under a `bin/` or `obj/` folder (build output,
generated code) — the router filters these paths before analysis.

## 3. C# analysis coverage

For a solution, the analyser extracts:

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

Relation kinds: `inheritance extends`, `implements interface`, `method calls`,
`property has`, `namespace contains`, `depends_on references`, `contains nested`,
`has_note annotation` (notes). Visibility (`[public]`/`[private]`/`[internal]`/
`[protected]`) is recorded per symbol and powers the `dead_code` tool.

**Limits:**

- The walker is a Go regex state machine — it understands common C# idioms but is
  **not** a full compiler front-end. Exotic syntax may be missed.
- Solution-level analysis requires a `.sln`; a lone `.cs` file with no solution is
  not picked up by `coderag scan`.
- Cross-file calls are resolved from a global symbol index built per scan.

## 4. Everything else (Path B, optional)

Markdown, YAML, `.txt`, JSON, config files, and any other text can be ingested through
LightRAG's LLM pipeline (`/documents/text`). This is **opt-in in practice**: the watcher
filters to supported extensions (`router.IsSupported`) before it processes anything, so
non-C# files are ignored by the watcher. The pipeline itself *does* contain a
`fallbackToLLM` path — anything without an analyser can be sent through the LLM
pipeline — but it's only exercised when a non-C# file is fed explicitly (e.g. via the
REST API).

Because Path B stores the file's chunked content in the graph, watch what you index:
prose files may contain secrets or PII. See [SECURITY.md](SECURITY.md).

## 5. How the watcher decides

```
File event
  → router.IsSupported(file)?
      ├─ yes (.cs, not bin/obj) → Path A (C# analyser)
      └─ no → ignored by the watcher (Path B only on explicit feed)
```

Adding a new language later is a router entry + analyser + parser — the pipeline is
language-agnostic (pipe-delimited analyser output).

---

Back to [README](../README.md) · [DOCUMENTATION.md](../DOCUMENTATION.md)
