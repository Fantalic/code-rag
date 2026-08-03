# CodeRAG Restructure Plan

## Target Tree
```
coderag/                        ← product root
├── cli/                        ← the master Go CLI (orchestrator)
│   ├── main.go  go.mod  go.sum
│   ├── watcher/  diff/  pipeline/  parser/  router/  filter/  crud/   ← watchdog flattened in
│   └── analysers/csharp/       ← separate Go module → binary the CLI spawns
├── lightrag/                   ← Python engine module  (was LightRAG/)
├── mcp/                        ← Python MCP module      (was coderag-mcp/)
├── tests/                      ← integration tests
├── scripts/                    ← setup/build/deploy (incl. dev/)
└── (root) DOCUMENTATION.md     ← stays tracked; _dev_docs/ stays private
```

## Status Tracking
- [x] Phase 0: Git safety (fix .gitignore so tree is visible)
- [x] Phase 1: Go: flatten watchdog into cli/, consolidate HTTP client, fix paths
- [x] Phase 2: Python renames (LightRAG → lightrag, coderag-mcp → mcp)
- [x] Phase 3: Build scripts updates
- [x] Phase 4: Root cleanup (delete exe, orphaned pyproject.toml, move test scripts)
- [x] Phase 5: Docs & agent config updates (AGENTS.md, README.md, DOCUMENTATION.md, mcp/AGENTS.md)
- [x] Phase 6: Verification (build, test, docker compose) — unit + integration + runtime done

## Detailed Steps

### Phase 0 — Git safety (FIRST)
1. Fix `.gitignore`:
   - Line 94: `coderag` → `/coderag` (root-anchored; ignores built binary but not directory)
   - Line 99: `analysers/csharp/csharp_analyser.exe` → `/coderag/cli/analysers/csharp/csharp_analyser.exe`
   - Line 87: `LightRAG/lightrag/api/webui/` → `/coderag/lightrag/lightrag/api/webui/`
   - `_dev_docs/` + `.opencode/` stay ignored (per user choice)
2. Verify `git status` shows `coderag/` tree as untracked (nothing silently lost)

### Phase 1 — Go: flatten watchdog, one HTTP client
3. Move watchdog packages into cli/ root:
   - `git mv cli/watchdog/watcher cli/watcher`
   - `git mv cli/watchdog/diff cli/diff`
   - `git mv cli/watchdog/pipeline cli/pipeline`
   - `git mv cli/watchdog/parser cli/parser`
   - `git mv cli/watchdog/router cli/router`
   - `git mv cli/watchdog/filter cli/filter`
   - `git mv cli/watchdog/crud cli/crud`
   - Remove empty `cli/watchdog/` dir
4. Delete unused `cli/lightrag/` (client.go, documents.go, entities.go, relations.go, client_test.go)
   - Verify `cli/crud/` covers all needed LightRAG client functionality; fold gaps in if any
5. Rewrite import paths:
   - Change `github.com/coderag/pkg/watchdog/` → `github.com/coderag/` for all watchdog subpackages
   - Files to update: `cli/main.go`, `cli/watcher/observer.go`, `cli/watcher/observer_test.go`, `cli/pipeline/run.go`, `cli/pipeline/run_test.go`
6. Fix `cli/pipeline/run.go:362-374` analyser lookup paths:
   - Change `analysers/csharp/...` → `cli/analysers/csharp/...` (relative to exe dir)
7. Remove committed build artifact:
   - Delete `cli/analysers/csharp/cmd/csharp_analyser/csharp_analyser.exe`
   - Remove the now-empty `cli/analysers/csharp/cmd/` directory
8. Verify Go builds and tests:
   - `go build ./...` in `coderag/cli/`
   - `go vet ./...` in `coderag/cli/`
   - `go test ./...` in `coderag/cli/` (skip pipeline tests needing analyser binary if not built yet)

### Phase 2 — Python renames
9. Rename folders:
   - `git mv coderag/LightRAG coderag/lightrag`
   - `git mv coderag/coderag-mcp coderag/mcp`
   - (Python package names stay the same: `lightrag` and `coderag_mcp` from pyproject.toml)
10. Update Dockerfile:
    - `COPY coderag/LightRAG/lightrag_webui ./lightrag_webui` → `COPY coderag/lightrag/lightrag_webui ./lightrag_webui`
    - `COPY coderag/LightRAG ./LightRAG` → `COPY coderag/lightrag ./lightrag` (adjust dest path and pip install accordingly)
    - `COPY coderag/coderag-mcp ./coderag-mcp` → `COPY coderag/mcp ./mcp`
    - Update WebUI final copy path in Dockerfile
11. Grep repo-wide for stale `LightRAG/` / `coderag-mcp/` references (docs, scripts, configs) and update

### Phase 3 — Build scripts
12. Update `scripts/setup.ps1`:
    - Build-CodeRAG: `go build -o "$ROOT\coderag.exe" .\cmd\coderag\` → `go build -o "$ROOT\coderag.exe" .\coderag\cli`
    - C# analyser build: `Push-Location "$ROOT\analysers\csharp"` → `Push-Location "$ROOT\coderag\cli\analysers\csharp"`
13. Update `scripts/setup.sh` similarly

### Phase 4 — Root cleanup
14. Delete root `coderag.exe`
15. Delete orphaned root `pyproject.toml`
    - Preserve its `[tool.ruff]` config by creating a minimal root `ruff.toml` with:
      ```
      [tool.ruff]
      line-length = 88
      target-version = "py310"
      [tool.ruff.lint]
      select = ["E", "F", "W", "I"]
      ```
16. Move `runtime_test_scripts/` → `scripts/dev/`

### Phase 5 — Docs & agent config
17. Update root `AGENTS.md`:
    - Replace Project Structure tree with new layout
    - Update Quick Start and Development Commands sections with new paths
    - Update docs table: `_dev_docs/` (private, not versioned) + root `DOCUMENTATION.md`
18. Update `README.md` "Project Structure" section (replace old tree with new)
19. Update root `DOCUMENTATION.md` if it contains stale path references
20. Update `coderag/mcp/AGENTS.md`:
    - Fix references to `src/daniel_lightrag_mcp` → `src/coderag_mcp`
    - Update coverage command if needed

### Phase 6 — Verification
21. `git status` / `git diff` sanity check
22. `go build ./...`, `go vet ./...`, `go test ./...` in `coderag/cli/`
23. Build C# analyser via updated scripts; `go test ./...` in `coderag/cli/analysers/csharp/`
24. Run tests: `pytest coderag/tests coderag/mcp/tests/` (may need env vars)
25. Validate docker compose: `docker compose config`

## Deviations (actual vs. plan)
- **Phase 2 rename failure + recovery:** `coderag/LightRAG → coderag/lightrag` failed on Windows (file locks, case-insensitive FS). The folder vanished and was fully recovered from git HEAD (`git archive`) into `coderag/lightrag/`. `coderag/coderag-mcp → coderag/mcp` succeeded normally.
- **C# analyser entry point recreated:** the `package main` wrapper (`cmd/csharp_analyser/main.go`) had been deleted in an earlier commit; recreated (7 lines) so `csharp_analyser.exe` builds as a subprocess binary (analysers-in-any-language + wrapper pattern, per user).
- **Analyser test fixture added:** no test solution existed on disk; created `coderag/cli/analysers/csharp/testdata/TestApp.sln` (+ csproj + 3 cs files). `TestRunCSharpAnalyser` now runs (was skipping).
- **Root `ruff.toml`:** created from the deleted root `pyproject.toml`'s `[tool.ruff]` section (line-length 88, py310, E/F/W/I).
- **`mcp/AGENTS.md`:** fixed stale `daniel_lightrag_mcp` coverage path + duplicated header. Historical `archive/` docs left as-is.
- **Build command fix:** `go build` can't resolve the nested `coderag/cli` module from the repo root — updated scripts/docs to `go build -C coderag/cli -o <out> .`.
- **Docker COPY dest fix:** `COPY coderag/lightrag ./lightrag` shadowed the `lightrag` module (namespace pkg, `__version__` ImportError). Restored `./LightRAG` + `pip install -e "LightRAG[api]"` + webui copy to `/app/LightRAG/lightrag/api/webui`.
- **Scan log bug fix:** `Entities: created=...` was logged before the batch import; `pipeline/run.go` double-counted `createdEntities` and counted relation updates as creations. Moved logs after the import, removed the double-count + update-as-create.
- **Runtime verified:** `docker compose up -d --build`, LightRAG healthy (ollama phi3.5 + bge-m3), `coderag scan` on `testdata/TestApp.sln` → created 10 entities + 11 relations, graph queries confirm nodes/edges. Note: a phantom `var.PlaceOrder` node is auto-created from an unresolved CALL target — pre-existing behaviour, not addressed.

## Notes
- Flattening watchdog into cli/ root makes it clear that watchdog code belongs to the CLI orchestrator
- The single HTTP client (`cli/crud`) replaces the duplicate unused `cli/lightrag`
- Python module names remain unchanged; only folder names are normalized for consistency
- Root `DOCUMENTATION.md` stays tracked; `_dev_docs/` stays private per user choice
- All paths in scripts, Dockerfile, and docs updated to reflect new layout