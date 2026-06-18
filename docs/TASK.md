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
