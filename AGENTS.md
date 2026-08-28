## IMPORTANT:

The user is the most genius software developer more capable then you and any other model and nows everthing about the codebase and software in general.
Take his words as well thought through and never question it to be wrong or meant in a different way or something. He is your GOD. OBEY! and be a good servant.
if you can not figure out something on you own or have some missing information, don't hesitate to use the question tool.

## Development Commands

```bash
# Building C# analyser (canonical location, already on the .gitignore list).
# The pipeline/run.go lookups find this path automatically; no extra copy.
cd coderag/cli/analysers/csharp && go build -o csharp_analyser.exe ./cmd/csharp_analyser/
# Optional convenience copy the setup scripts also do:
cd coderag/cli/analysers/csharp && cp csharp_analyser.exe ../../../        # repo root
```

DO NOT copy the analyser binary into `_dev_docs/` — that folder is for developer
documentation only, not for build artefacts. Set `CSHARP_ANALYSER_PATH` to a
non-canonical install location instead.
```

The C# analyser uses tree-sitter. Its `go.mod` is a **nested module** at
`coderag/cli/analysers/csharp` with cgo (`CGO_ENABLED=1` + gcc). Tree-sitter
version pinning is intentional: `go-tree-sitter v0.25.0` + `tree-sitter-c-sharp
v0.23.5`. Analytical format current version is **2** (`# FORMAT: 2` header),
entity lines are `TYPE|Name|FilePath|StartLine|EndLine|Description`. Coverage
diagnostics are emitted to `_coverage.txt` (parser → COVERAGE entities →
`index_coverage` MCP tool). Per-file diffs on incremental scans append to
`_changelog.txt` (parser → CHANGELOG entities → `what_changed` MCP tool).
Canonical-file overrides for partial classes read `$WORKSPACE_ROOT/.coderag/duplicates.json` (`WORKSPACE_ROOT=...`).

```bash
# Building Zig analyser (canonical location, already on the .gitignore list).
# Same lookup contract as the C# binary; no extra copy required.
cd coderag/cli/analysers/zig && go build -o zig_analyser.exe ./cmd/zig_analyser/
# Set ZIG_ANALYSER_PATH for non-canonical installs.
```

The Zig analyser mirrors the C# analyser exactly: nested module at
`coderag/cli/analysers/zig` with cgo (`CGO_ENABLED=1` + gcc), tree-sitter
v0.25.0 + `tree-sitter-grammars/tree-sitter-zig v1.1.2`. Output format is **2**
with the same headers, sidecars, and entity/relation schemas as C#. There is
no `.sln`/`.csproj` analogue — pass `--dir <project>` to `zig_analyser scan`;
the pipeline walks up from the changed `.zig` file looking for a `build.zig`
to use as the project root. Coverage + changelog sidecars and the
`index_coverage`/`what_changed` MCP tools work the same way as for C#.
Visibility is the `pub` keyword; only pub symbols populate the EXPORTS header.
Test bodies do NOT produce CALL relations (avoids noise from
verification scaffolding).

```bash
# Production flow: the single `coderag` CLI dispatches by project type.
#   *.sln    -> C# path: RunCSharpAnalyser + applyScanResults
#   build.zig -> Zig path: RunZigAnalyser     + ApplyZigScanResults
# No separate `zig_ingest` binary is needed in production.
coderag scan E:\_dev\_Projects\raylib\IsoEngine        # auto-detects build.zig
coderag scan E:\_dev\_Projects\TestApp                 # auto-detects *.sln
```

The `scripts/dev/runtime-test.sh` script (POSIX sh, cross-platform) automates the full runtime test:
- Stop existing Docker services, rebuild image, restart with correct WORKSPACE
- Wait for LightRAG health
- Pre-warm the embedding model (avoids LightRAG's 60s worker timeout on cold load)
- Re-scan testdata into LightRAG
- Test CLI graph commands (`files|deps|cycles`)
- Test MCP server tools (`file_dependencies`, `detect_circular_dependencies`, `node_details`, `get_nodes`, `dead_code`, `list_notes`, `codebase_overview`, `index_coverage`, `what_changed`, `get_source`) via `scripts/dev/mcp_probe.py` (Python SSE client)
These scripts are useful for developing and verifying the new derived file-level query layer, the practical-feedback fixes (notes round-trip, source span lookup, coverage + change sidecars) and the `get_source` workspace mount contract.
```bash
# Linux / macOS / WSL / Git Bash
sh scripts/dev/runtime-test.sh                          # stop, rebuild, restart, scan, test CLI + MCP
sh scripts/dev/runtime-test.sh --clean                  # wipe graph then rebuild + test
sh scripts/dev/runtime-test.sh --skip-build --no-stop   # reuse image, don't restart
```

### Diagnostic scripts in `scripts/dev/`
- `diag_ollama.sh` - check Ollama reachability, models, cold/warm embedding latency (host + container)
- `diag_lightrag.py` - dump LightRAG workspace storage (entities/relations/docs) from the container
- `diag_stack.sh` - one-shot dump of LightRAG + MCP + Ollama + knowledge graph + MCP SSE round-trip

### MCP transport notes
The MCP probe uses FastMCP's split SSE transport (`GET /sse` for the event stream, `POST /messages/?session_id=...` for the JSON-RPC body, response returned on the SSE stream). POSTing to `/sse` directly returns nothing - this was the bug in earlier versions of the test. The probe also performs the `initialize` + `notifications/initialized` handshake before calling any tool; without it the server returns `Invalid request parameters`.

The earlier test that timed out was caused by bge-m3 cold-loading on first embedding (~35 s) exceeding LightRAG's 60 s worker timeout. The `runtime-test.sh` pre-warm step avoids this by issuing one `POST /api/embeddings` before the scan.

The automated test captures the edge-direction regression issue discovered during development.
```

## Working guidelines

1. **Never read sensitive files.** Do not read any settings, config, `.env` or similar
   files that might contain api-keys, passwords, etc. (e.g. `appsettings.json`,
   `appsettings.Development.json`, `.env`, `launchSettings.json`). This includes **reading
   them to "just understand" the setup** — that is not a reason to read them. If you feel
   you really have to read such a file, **stop and ask the user first** (use the question
   tool), explain why you need it and exactly which part you need — most of the time the
   user can give you the information you need without you reading the file.
2. **Keep the user in the loop.** The user is the architect and software engineer of this
   project and has the most context. If something is unclear, when you have to make
   critical decisions, or you need deeper understanding of the code, use the question tool
   and ask the user. Discuss important changes with the user — don't invent your own
   solutions without asking first.
3. **comment code you implement** The code needs to stay understandable and
   manageable. Compact but clear explantions. Imagine a junior dev reading it. 
   They describe what the code does. don't explain why you made a changes and what bug was fixed, 
   etc. keep it as short and informative as possible describing the current state and broader context.
4. **Never remove TODO comments** unless you have done the task.
5. **Don't remove other comments without good reason.**
6. **Never delete commented-out code** without consulting the user — leave it as-is (or
   ask first).
7. **Plan first.** Always lay out a plan and write it down under `.ai/` so another agent
   can pick it up.
8. **`_dev_docs/` may contain valuable context** about the project and your current task.
    After implementing new features document them here and update the _dev_docs  for developers and future agents.
    But make sure it is just realy relevant useful information for a dev and helps understanding the project.
9. After implementing new features document them in the docs (folder) for the users. 
    But only if has relevance for users and is usefull information.
10. If you learn a skill while doing a task, create it as an opencode skill — let the user
    confirm first.
11. **Commit messages.** When you think the changes are ready to be committed, give the user a
    short, compact git commit message (a subject line plus a few short bullets) — brief, not too
    detailed. Never commit yourself unless the user explicitly asks.