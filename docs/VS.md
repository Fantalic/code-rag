# codeRAG for Visual Studio / .NET Developers

> A step-by-step guide for Windows developers who work in Visual Studio and want to index their .NET solution into a queryable knowledge graph.

---

## Prerequisites

Before you start, make sure the following are installed and running:

| Tool | Version | Check in PowerShell |
|---|---|---|
| **Visual Studio 2022** | Any edition (Community works) | `devenv /?` |
| **Docker Desktop** | 24+ | `docker --version` |
| **Ollama** | 0.3+ | `ollama --version` |
| **Git for Windows** | Any recent | `git --version` |
| **PowerShell 7+** | Recommended | `pwsh --version` |

> **Important:** Docker Desktop and Ollama must be **running** before you start codeRAG. Ollama does not auto-start on boot by default.

---

## Step 1: Clone & Build

Open **PowerShell** (not CMD) and run:

```powershell
# Clone the repo
git clone https://github.com/Fantalic/code-rag.git
cd code-rag

# Build coderag.exe, the C# analyser, Docker image, and pull Ollama models
.\scripts\setup.ps1
```

This takes 2–5 minutes. You will see:
- `coderag.exe` built in the project root
- `csharp_analyser.exe` built (self-contained Go binary — no .NET SDK needed for codeRAG itself)
- Docker image `coderag:latest` built
- Ollama model `bge-m3:latest` pulled

Verify the build:

```powershell
.\coderag.exe --version
```

---

## Step 2: Add coderag to your PATH (optional but recommended)

So you can run `coderag` from any solution folder:

```powershell
# Get the full path to the code-rag folder
$codeRagPath = (Get-Location).Path

# Add to user PATH permanently
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$codeRagPath", "User")
```

Restart PowerShell after this.

---

## Step 3: Index your solution

Open PowerShell **in your solution's root folder** (where the `.sln` file lives):

```powershell
cd C:\src\MySolution

# Start the Docker services and file watcher
coderag start .

# In a second PowerShell window, index the solution
coderag scan .
```

> **What happens:**
> - `coderag start` spins up LightRAG in Docker and starts the live file watcher.
> - `coderag scan` runs the Go C# analyser on your `.sln`, extracts all classes, interfaces, methods, properties, inheritance, and call graphs, then pushes them into the graph.

You will see output like:

```
Analyzing solution: MySolution.sln
Found 12 projects, 847 .cs files
Extracted 3,421 entities, 8,912 relations
Import complete. Graph ready.
```

---

## Step 4: Explore your graph

While `coderag start` is still running, open another PowerShell window:

```powershell
cd C:\src\MySolution

# See indexed files and entity counts
coderag graph files

# Check dependencies for a specific file
coderag graph deps .\src\Services\OrderService.cs

# Detect circular dependencies
coderag graph cycles

# Open the visual graph explorer in your browser
coderag webui
```

The WebUI opens at `http://localhost:9621`. You can browse entities, see relations, and search by name.

---

## Step 5: Connect your AI assistant (MCP)

### Option A: GitHub Copilot Agent in Visual Studio

Visual Studio 2022 17.14+ supports MCP servers via the Copilot Agent extension.

1. In VS, go to **Tools → Options → GitHub → Copilot → Agent**
2. Add a new MCP server:
   - **Name:** `codeRAG`
   - **Command:** `coderag mcp C:\src\MySolution`
   - **Type:** `stdio`
3. Restart the Copilot Agent panel

Now ask Copilot:

> *"What does `OrderService.PlaceOrder` call?"*
> *"Which files reference `IOrderRepository`?"*
> *"Show me the class hierarchy under `Domain`."*
> *"Is `OrderService.cs` part of any circular dependency?"*

### Option B: Claude Desktop (sidecar)

1. Open Claude Desktop → **Settings → Developer → Edit Config**
2. Edit `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "coderag": {
      "command": "coderag",
      "args": ["mcp", "C:\\src\\MySolution"]
    }
  }
}
```

3. Restart Claude Desktop

### Option C: Cursor / VS Code

See [`MCP_CLIENTS.md`](MCP_CLIENTS.md) for Cursor, Windsurf, and VS Code-specific wiring.

---

## Step 6: Natural-language queries (optional)

If you pulled an LLM model (not just the embedding model), you can ask plain English:

```powershell
coderag query "How does the checkout flow work?"
coderag query "Explain the dependency chain from OrderController to the database"
```

Set a model before setup, or pull one manually:

```powershell
ollama pull phi3.5:latest
$env:LLM_MODEL = "phi3.5:latest"
coderag query "What is the architecture of this solution?"
```

---

## Step 7: Live workflow (day-to-day)

Keep `coderag start` running in a dedicated PowerShell window while you code in Visual Studio.

```powershell
# Terminal 1 — keep this running all day
coderag start C:\src\MySolution

# Terminal 2 — run commands as needed
coderag scan .          # re-index after major refactors
coderag graph cycles    # check for new circular deps
coderag query "..."     # ask questions
```

What gets synced automatically:
- ✅ File saves in VS
- ✅ File renames / moves
- ✅ File deletions (cascading cleanup in the graph)
- ✅ Git commits / branch switches (30-second polling)

What needs a manual re-scan:
- ⚠️ New `.csproj` added to the solution
- ⚠️ Major solution restructuring

Stop the watcher with `Ctrl+C` in Terminal 1. Docker keeps running, so your graph stays queryable.

---

## Windows-Specific Tips

| Situation | Solution |
|---|---|
| **"Docker not running" error** | Start Docker Desktop from the Start Menu. Wait until the whale icon stops animating. |
| **"Ollama not found"** | Ollama must be running: `ollama serve` in a terminal, or launch from Start Menu. |
| **Paths with spaces** | Quote them: `coderag scan "C:\My Projects\Solution"` |
| **PowerShell execution policy blocks setup** | Run: `Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser` |
| **Graph seems stale after big refactor** | Run `coderag scan .` again — the diff engine updates only what changed. |
| **Want to start fresh** | `coderag stop` then `coderag start .` — this wipes and rebuilds the graph container. |

---

## What gets indexed

| Entity type | Examples |
|---|---|
| `NAMESPACE` | `MyApp.Domain`, `MyApp.Api` |
| `CLASS` / `INTERFACE` / `STRUCT` / `ENUM` | `OrderService`, `IOrderRepository`, `Point`, `OrderStatus` |
| `METHOD` | `OrderService.PlaceOrder()` |
| `PROPERTY` | `Order.Total`, `OrderService._order` |

Relations: `inheritance`, `implements`, `calls`, `property_has`, `namespace_contains`, `depends_on`, cross-file references. Visibility (`public`/`private`/`internal`/`protected`) is stored per symbol.

---

## Quick Reference

```powershell
# Start services + watcher (run once, keep open)
coderag start C:\src\MySolution

# Index / re-index
coderag scan .

# Explore
coderag graph files
coderag graph deps .\Path\To\File.cs
coderag graph cycles
coderag webui

# AI bridge
coderag mcp .

# Ask questions (needs LLM)
coderag query "How does X work?"

# Stop everything
coderag stop
```

---

## Next Steps

- Full CLI reference → [`USAGE.md`](USAGE.md)
- MCP client wiring for more editors → [`MCP_CLIENTS.md`](MCP_CLIENTS.md)
- Architecture & data flow → [`ARCHITECTURE.md`](ARCHITECTURE.md)
- Security & redaction → [`SECURITY.md`](SECURITY.md)

---

<p align="center">
  <sub>MIT License © <a href="https://github.com/Fantalic">Fantalic</a></sub>
</p>
