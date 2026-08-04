# codeRAG for Visual Studio / .NET Developers

> Index your .NET solution into a queryable knowledge graph, then ask AI tools questions
> about it. This guide assumes a Windows + Visual Studio
> setup, but the workflow is identical on macOS + VS Code or JetBrains Rider.

---

## 1. What you get

- Your solution's classes, interfaces, methods, properties, inheritance, and call
  graphs are indexed **statically** from source.
- A live file watcher keeps the graph in sync as you save.
- Any MCP-compatible AI extension (Copilot Agent, Cursor, Claude Desktop, …) can answer
  "what calls this?", "where is this interface implemented?", "is there a cycle?"
  without reading your source files one by one.
- A WebUI at `http://localhost:9621` for visual exploration.

## 2. Install

Follow [INSTALL.md](INSTALL.md). On Windows the one-liner is:

```powershell
cd code-rag
.\scripts\setup.ps1
```

This builds `coderag.exe` and the C# analyser (`csharp_analyser.exe`), pulls the
Ollama models, and pre-builds the Docker image. You do **not** need the .NET SDK on
the machine for codeRAG — the analyser is a self-contained Go binary.

## 3. Index your solution

Open a PowerShell **in your solution's parent folder** (where the `.sln` lives):

```powershell
coderag scan .\MySolution
# or from anywhere:
coderag scan C:\src\MySolution
```

Requirements:

- The folder must contain (or sit inside) a `.sln`. codeRAG searches up to 10
  directory levels up.
- Only `.cs` files belonging to projects in the solution are analysed.
- The C# analyser runs at **solution level**: when any file changes, the solution is
  re-analysed and the diff engine updates only what changed.

After the scan:

```powershell
coderag graph files                 # every file + entity counts
coderag graph deps .\src\Services\OrderService.cs
coderag graph cycles                # circular dependencies between files
coderag webui                       # open the visual browser
```

## 4. Use it from Copilot Agent / AI extensions

Any editor extension that supports MCP command servers can talk to codeRAG:

| Client | How |
|--------|-----|
| **VS Code + Copilot Agent / Cursor** | Add an MCP server: command `coderag mcp C:\src\MySolution` |
| **Visual Studio (via VS Code bridge)** | Run the VS Code MCP setup, or use the REST API directly |
| **Claude Desktop** | `claude_desktop_config.json` → command `coderag mcp C:\src\MySolution` |

Full wiring examples → **[MCP_CLIENTS.md](MCP_CLIENTS.md)**.

Example questions your assistant can now answer instantly:

> - "What does `OrderService.PlaceOrder` call?"
> - "Which files reference `IOrderRepository`?"
> - "Show me the class hierarchy under `Domain`."
> - "Is `OrderService.cs` part of any circular dependency?"
> - "What's been unused for more than 30 days?" (the `dead_code` tool with `min_age_days=30`)

## 5. Live watching

Keep it running while you code:

```powershell
coderag start C:\src\MySolution
```

Saves, renames, moves, and deletes are picked up in seconds (fsnotify + a 30 s
`git diff HEAD` poll). Stop with `Ctrl+C` — Docker keeps running, so your graph
stays available.

## 6. What gets indexed

For each solution, the graph contains:

| Entity type | Examples |
|-------------|----------|
| NAMESPACE | `MyApp.Domain`, `MyApp.Api` |
| CLASS / INTERFACE / STRUCT / ENUM | `OrderService`, `IOrderRepository`, `Point`, `OrderStatus` |
| METHOD | `OrderService.PlaceOrder()` |
| PROPERTY | `Order.Total`, `OrderService._order` |

Relations include `inheritance`, `implements`, `calls`, `property has`,
`namespace contains`, `depends_on`, and cross-file references. Visibility
(`public`/`private`/`internal`/`protected`) is stored per symbol, which powers
the `dead_code` tool.

## 7. Gotchas on Windows

- **Paths:** the graph stores Windows paths with backslashes. When filtering by file
  in tools, use a short substring (`OrderService.cs`) — it matches robustly either way.
- **Docker Desktop** must be running before `coderag start` / `coderag scan`.
- **PowerShell + the watcher:** leave the `coderag start` window open. Closing it stops
  the watcher (Docker stays up).
- **Ollama:** install from ollama.com, it adds itself to PATH, and must be running
  (`ollama serve`) for `coderag query` / embeddings. Graph scanning itself doesn't
  need the LLM, but embeddings are used when the graph is written — keep Ollama on.

---

Next: **[USAGE.md](USAGE.md)** · **[MCP_CLIENTS.md](MCP_CLIENTS.md)** · back to [README](../README.md)
