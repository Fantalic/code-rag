## Development Commands

```bash
# Building C# analyser
cd coderag/cli/analysers/csharp && go build -o csharp_analyser.exe ./cmd/csharp_analyser/ && cd ../../..
echo "Deploy C# analyser (Windows)"
copy coderag\cli\analysers\csharp\csharp_analyser.exe _dev_docs\analysers\csharp\
```

The `scripts/dev/runtime-test.sh` script (POSIX sh, cross-platform) automates the full runtime test:
- Stop existing Docker services, rebuild image, restart with correct WORKSPACE
- Wait for LightRAG health
- Pre-warm the embedding model (avoids LightRAG's 60s worker timeout on cold load)
- Re-scan testdata into LightRAG
- Test CLI graph commands (`files|deps|cycles`)
- Test MCP server tools (`get_file_*`) via `scripts/dev/mcp_probe.py` (Python SSE client)
These scripts are useful for developing and verifying the new derived file-level query layer.
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