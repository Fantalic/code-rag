# Wiring MCP Clients

> Connect your AI agent to codeRAG's knowledge graph. codeRAG exposes **10 deterministic
> MCP tools** over SSE (`:9622`) in Docker; the `coderag mcp <dir>` command bridges them
> to stdio for desktop agents.

---

## 1. The transport model

```
Your agent (stdio MCP client)
        │
        ▼
coderag mcp <project-dir>      ← Go binary on the host, stdio↔SSE bridge
        │  (also starts the file watcher in the background)
        ▼  SSE
coderag-mcp (Docker :9622)     ← FastMCP server, 10 tools
        ▼  HTTP
lightrag-server (Docker :9621) ← the actual knowledge graph
```

`coderag mcp` does three things at once:

1. Ensures Docker services are healthy (starts them if needed).
2. Starts the file watcher in the background, so the graph is always fresh.
3. Bridges stdio JSON-RPC → SSE and back, so any stdio MCP client "just works".

---

## 2. opencode

opencode runs a command as an MCP server. In your `opencode.json` (or global config):

```json
{
  "mcpServers": {
    "coderag": {
      "type": "stdio",
      "command": "coderag",
      "args": ["mcp", "C:\\path\\to\\myproject"]
    }
  }
}
```

opencode launches `coderag mcp <dir>`, which starts the services, starts the watcher,
and serves the 10 tools. Restart opencode (or reconnect MCP) after editing the config.

## 3. Claude Desktop

In `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "coderag": {
      "command": "coderag",
      "args": ["mcp", "/path/to/myproject"]
    }
  }
}
```

Restart Claude Desktop. Ask it to call the tools by intent (e.g. "use
`node_details` on `OrderService`"). Claude Desktop connects over stdio, so the
`coderag mcp` bridge is exactly what it needs.

## 4. Cursor

Cursor supports MCP servers from its settings:

1. Open **Cursor Settings → MCP → Add new MCP server**.
2. Type: **command**.
3. Command:

   ```
   coderag mcp C:\path\to\myproject
   ```

Cursor launches the command and lists the tools in the MCP panel. Note: Cursor
works best with a small number of tools — codeRAG's 10 tools fit comfortably.

## 5. Windsurf

Windsurf also supports command-based MCP servers:

1. **Settings → MCP → Add Server**.
2. Command:

   ```
   coderag mcp C:\path\to\myproject
   ```

If a client needs SSE instead of stdio, point it at `http://localhost:9622` directly —
the Docker MCP server speaks the MCP SSE transport natively. Only use this when the
client runs on the same machine as Docker (it will not reach `:9622` from a remote box
unless you publish the port).

---

## 6. The tools your agent gets

| Tool | Best for |
|------|----------|
| `exists_in_codebase` | cheap existence checks by qualified name |
| `get_nodes` | lists/filters of entities (type, name, file, connection) |
| `node_details` | deep dive on one symbol (details / relations / neighbors) |
| `codebase_overview` | orienting yourself in an unknown repo |
| `file_dependencies` | "what touches this file, and what does it touch?" |
| `detect_circular_dependencies` | architecture smell detection |
| `dead_code` | cleanup candidates (unreferenced + non-exported) |
| `query_codebase` | semantic retrieval (entities/relations/chunks/references) |
| `attach_note_to_node` | leave guidance for future agents |
| `get_notes` | read guidance left by previous agents |

### Agent guidance (paste this into your agent if it won't discover the tools)

> The codebase is indexed as a knowledge graph. Entity names are fully qualified
> (`MyApp.OrderService.PlaceOrder`). For anything about a specific symbol use
> `node_details`; to enumerate symbols use `get_nodes`; for file-level questions use
> `file_dependencies` / `detect_circular_dependencies`. `file_path` filters match
> substrings — pass `OrderService.cs`, not a full path. `dead_code` finds unreferenced
> symbols that are private/internal or not exported. Prefer these deterministic tools
> over reading files directly.

---

## 7. No MCP client? Use the REST API

The MCP tools are thin wrappers over the LightRAG HTTP API on `:9621`. Scripts and
CLIs can call it directly — e.g. `curl http://localhost:9621/graph/entity/exists?name=Foo`.
See **[USAGE.md](USAGE.md#direct-http-api)** for the mapping.

---

Next: **[USAGE.md](USAGE.md)** · **[VS.md](VS.md)** · back to [README](../README.md)
