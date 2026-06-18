# Current Task: Phase 2 — Strip & Simplify Analysis

## Objective
Survey the entire monorepo codebase to identify dead code, orphaned scripts, duplicate/config files, and unused dependencies. **Do NOT remove anything** — only document findings. All removals require user approval via the `question` tool.

## Scope
All three sub-projects:
- `LightRAG/` (core engine + API + WebUI)
- `daniel-lightrag-mcp/` (MCP server + analysers)
- `watchdog/` (file watcher)

Plus root-level files.

## Investigation Checklist

### 1. Orphaned Scripts & Entry Points
Find every `if __name__ == "__main__":` block. List each script and its purpose. Flag any that:
- Are never called from any workflow
- Are superseded by another script (e.g. setup.ps1 vs manual install instructions)
- Reference removed/deleted modules

### 2. Duplicate Configuration
Look for:
- Multiple `.env.example`, `config.ini`, `config.py` files with overlapping settings
- Duplicate environment variable names with different defaults
- Multiple requirements files (`requirements.txt`, `pyproject.toml`, `setup.py`, etc.)
- CI configs that reference stale workflows

### 3. Unused / Dead Code
- Python imports that are never used (run `ruff check --select=F401 .` or manual scan)
- Classes/functions with no callers (search for definitions and cross-reference)
- Routes/endpoints in the API that are never hit
- Deprecated compatibility shims (Pydantic v1 → v2 transition helpers left behind)

### 4. Stale Build Artifacts
Files that are generated, not source:
- `*.egg-info/`, `__pycache__/` (already gitignored — check if any are tracked)
- `.gitignore` patterns that no longer apply
- Old `PLAN.md` at root (was moved to `docs/PLAN.md` — already deleted)

### 5. NPM / Frontend Residue
- `lightrag_webui/` is a full React 19 + TypeScript app. Is it used? Can it be stripped for API-only mode?
- Any `package.json`, `node_modules/`, `bun.lock` at unexpected locations

### 6. Dependencies
- `requirements.txt` vs `setup.py` vs `pyproject.toml` — overlapping?
- Dependencies imported but never used
- Test-only deps mixed with runtime deps

## Output Format
For each finding, document:

```
## [Category] File:Path
- **Type**: orphaned / duplicate / dead / stale / unused-dep
- **Risk**: low / medium / high (how likely to break something if removed)
- **Evidence**: grep output, import trace, or git log showing last use
- **Recommendation**: remove / keep / merge / defer
```

## Workflow
1. Survey using glob + grep + ruff
2. Write findings to a new section in `docs/TASK.md` (appended below)
3. For each candidate removal, use the `question` tool to ask the user
4. Only remove after explicit approval
5. After each removal, commit with message referencing the decision
6. Update `docs/TASK.md` with what was done

## Rules
- Never remove anything without asking first
- Never remove something you don't fully understand
- If unsure about a file, mark it as "uncertain — needs human review"
- Keep edge cases minimal — better to flag too much than too little

---

## Survey Findings

### 1. Large Binary Artifacts Tracked in Git (~35 MB)
**Path**: `daniel-lightrag-mcp/analysers/c_sharp/`
- **State**: 150 files, ~35 MB of compiled .NET binaries (`.exe`, `.dll`, `.pdb`, `.deps.json`, etc.) committed to git
- **Source**: The C# analyser is pre-built — no `.cs` source or `.csproj` exists in the repo
- **Git status**: Tracked (no gitignore rule applies)
- **Orphaned**: The MCP server never calls this tool; no Python code references it
- **Recommendation**: Either (a) remove from git and distribute separately, or (b) add source code and build from source
- **Risk**: Low — only the e2e test uses pre-generated output, not the tool itself

### 2. `requirements-offline-*.txt` — Duplicates `pyproject.toml`
**Path**: `LightRAG/requirements-offline.txt`, `requirements-offline-storage.txt`, `requirements-offline-llm.txt`, `requirements-offline-docs.txt`
- **State**: 4 files that duplicate the exact dependency lists already in `pyproject.toml [project.optional-dependencies]`
- **Risk**: Medium — version bumps must be updated in two places
- **Recommendation**: Remove the `.txt` files; they are acknowledged as redundant ("Recommended: Use `pip install lightrag-hku[offline]`")

### 3. `setup.py` — Legacy Shim
**Path**: `LightRAG/setup.py`
- **State**: 4-line no-op stub that calls `setup()` with no args. Comment says "for backward compatibility."
- **Recommendation**: Can be removed once `pip` compatibility with only `pyproject.toml` is verified
- **Risk**: Low — some ancient tools require `setup.py` present

### 4. `AGStorage` — References Non-Existent Module
**Path**: `LightRAG/lightrag/kg/__init__.py:56-60, 98`
- **State**: `STORAGE_ENV_REQUIREMENTS["AGEStorage"]` and `STORAGES["AGEStorage"]` both reference `.kg.age_impl` which **does not exist**
- **Risk**: High — configuring AGEStorage as graph backend causes `ModuleNotFoundError`
- **Recommendation**: Remove the dead entries from the maps

### 5. `ChromaVectorDBStorage` — Partially Commented Out
**Path**: `LightRAG/lightrag/kg/__init__.py:29, 69, 95`
- **State**: Removed from `STORAGE_IMPLEMENTATIONS` (commented out) but still present in `STORAGE_ENV_REQUIREMENTS` and `STORAGES` maps. The actual module was moved to `deprecated/chroma_impl.py`
- **Risk**: Medium — could cause confusion but won't crash (commented out from the active map)
- **Recommendation**: Clean up dead entries from all three maps

### 6. `.cs` and `.rel` File Extensions Broken
**Path**: `LightRAG/lightrag/api/routers/document_routes.py:980-982`
- **State**: String concatenation bug — `".c" ".cs" ".rel"` becomes `".c.cs.rel"`, never matching actual `.cs` or `.rel` files
- **Risk**: Low — C# files silently get "Unsupported file type" instead of being processed
- **Recommendation**: Fix the bug (add commas)

### 7. `tools/lightrag_visualizer/` — Orphaned GUI Tool
**Path**: `LightRAG/lightrag/tools/lightrag_visualizer/`
- **State**: Full ImGui/Tkinter graph visualizer, never imported anywhere in production code
- **Risk**: Low — standalone tool, adds ~1 MB to repo
- **Recommendation**: Keep (it's a standalone utility) or move out of core package

### 8. `tools/check_initialization.py` — Orphaned Diagnostic Tool
**Path**: `LightRAG/lightrag/tools/check_initialization.py`
- **State**: 180-line diagnostic script, never imported anywhere
- **Recommendation**: Keep (useful for debugging)

### 9. Unreferenced LLM Modules (7 files)
**Paths**: `LightRAG/lightrag/llm/{lmdeploy,hf,nvidia_openai,siliconcloud,zhipu,anthropic,llama_index_impl}.py`
- **State**: Each only referenced in example scripts, not wired into production code
- **Risk**: Low — they work if imported directly
- **Recommendation**: Keep (part of upstream LightRAG ecosystem)

### 10. Example Scripts (22 files)
**Paths**: `LightRAG/examples/` + `LightRAG/examples/unofficial-sample/`
- **State**: Upstream demo scripts for various LLM backends
- **Risk**: Very low — not imported by production code
- **Recommendation**: Keep (useful reference)

### 11. `reproduce/` Scripts (5 files)
**Paths**: `LightRAG/reproduce/`
- **State**: Upstream benchmark/evaluation scripts
- **Recommendation**: Keep

### 12. `lightrag_webui/` — Frontend React App
**Path**: `LightRAG/lightrag_webui/`
- **State**: Full React 19 app with 4 config files, 1 lock file (bun.lock), source code
- **Integration**: Server checks for its build output at startup
- **Risk**: Low — not built yet (no `index.html` in `api/webui/`)
- **Recommendation**: Keep (part of the product vision)

### 13. Gitignore Duplication
**Paths**: `.gitignore` (root), `LightRAG/.gitignore`, `daniel-lightrag-mcp/.gitignore`, `LightRAG/lightrag/api/.gitignore`, `LightRAG/lightrag_webui/.gitignore`, `watchdog/.gitignore`
- **State**: 6 gitignore files, root and LightRAG ones overlap heavily
- **Decision**: ✅ Consolidate into root `.gitignore`, remove sub-directory ones

### 14. `docker-compose.yml` References Missing Files
**Path**: `LightRAG/docker-compose.yml`
- **State**: Expects `./config.ini` and `./.env` as bind-mounts — neither exists in repo
- **Risk**: Low — user creates these at runtime
- **Decision**: ⏸️ Defer (user docs improvement, not strip/simplify)

### 15. `DEFAULT_WOKERS` Typo
**Path**: `LightRAG/lightrag/constants.py:10`
- **State**: `DEFAULT_WOKERS = 2` (should be `DEFAULT_WORKERS`)
- **Decision**: ✅ Fix typo

### 16. K8s Deploy Scripts (11 files)
**Path**: `LightRAG/k8s-deploy/`
- **State**: Kubernetes deployment scripts for production deploy
- **Decision**: Keep (upstream infrastructure)

## Removals / Fixes (ordered by execution)

1. ✅ Fix `.cs`/`.rel` string concatenation bug in `document_routes.py` (add commas)
2. ✅ Fix `DEFAULT_WOKERS` typo in `constants.py`
3. ✅ Clean up `AGEStorage` + `ChromaVectorDBStorage` dead entries in `kg/__init__.py`
4. ✅ Remove `LightRAG/requirements-offline-*.txt` (4 files)
5. ✅ Remove `LightRAG/setup.py`
6. ✅ Consolidate gitignore files (6 → 1 root .gitignore)
