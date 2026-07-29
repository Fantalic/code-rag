# Sub-Agent Prompt: Explore dotnet-console-games via MCP Tools

## Target Project
`E:\_dev\_Projects\dotnet-console-games`

## Goal
Navigate the dotnet-console-games codebase using LightRAG MCP tools, understand the game architecture, and implement a small feature for one of the games.

## How to Use MCP Tools

**Connect via HTTP REST API at `http://localhost:9621`.** Do NOT use MCP SSE protocol — use direct HTTP calls.

| MCP Tool | HTTP Endpoint | Method |
|---|---|---|
| `check_entity_exists` | `/graph/entity/exists?name=NAME` | GET |
| `get_knowledge_graph` | `/graphs?label=*&max_depth=3&max_nodes=1000` | GET |
| `get_graph_labels` | `/graph/label/list` | GET |
| `get_entity_details` | `/graphs?label=*&max_depth=3&max_nodes=1000` + filter | GET |
| `get_entity_neighbors` | `/graphs?label=*&max_depth=3&max_nodes=1000` + filter | GET |
| `query` | `/query` | POST |
| `query_data` | `/query/data` | POST |
| `get_codebase_stats` | `/graph/label/list` + `/graphs?label=*&max_depth=3` | GET + GET |
| `list_entities_by_type` | `/graphs?label=*&max_depth=3&max_nodes=1000` + filter | GET |
| `search_entities` | `/graphs?label=*&max_depth=3&max_nodes=1000` + regex | GET |
| `get_entities_in_file` | `/graphs?label=*&max_depth=3&max_nodes=1000` + filter | GET |
| `get_related_files` | `/query/data` | POST |
| `get_relation_detail` | `/graphs?label=*&max_depth=3&max_nodes=1000` + filter | GET |
| `get_method_signature` | `/graphs?label=*&max_depth=3&max_nodes=1000` + search | GET |
| `find_related_files` | `/graphs?label=*&max_depth=3&max_nodes=1000` + match | GET |
| `get_interface_methods` | `/graphs?label=*&max_depth=3&max_nodes=1000` + filter | GET |
| `add_note_to_entity` | `/graph/entity/create` + `/graph/relation/create` | POST |

Use `curl.exe` or Python httpx to make requests. For POST with JSON body, use `-H "Content-Type: application/json" -d '{...}'`.

## Exploration Steps

### Step 1: Graph Overview
- GET `/graph/label/list` — list entity/relation types
- GET `/graphs?label=*&max_depth=1&max_nodes=10000` — count total entities/relations
- Record the counts and entity types present

### Step 2: Entity Existence Checks
Check if these game-related entities exist in the graph (use exact qualified names):
- Any game class (e.g., `Website.Games.Fighter.Fighter`, `Projects.Console_Monsters.Program`, etc.)
- Use `GET /graph/entity/exists?name=NAME` for each
- Try searching with `GET /graphs?label=*&max_depth=3&max_nodes=10000` and grep/filter for game-related names

### Step 3: Find Game Projects
Use search or full graph dump to identify:
- All unique game project namespaces
- All class names that look like game entry points (Program.cs classes)
- All game-related method entities

### Step 4: Pick a Game to Modify
Choose one game that has enough complexity to modify. Recommended candidates:
- **Hangman** — simple state machine, good for adding features
- **Tic Tac Toe** — small, easy to understand
- **Minesweeper** — moderate complexity
- **Snake** — classic game, good for a feature
- **PacMan** — complex, has many interactions

### Step 5: Understand the Chosen Game
For the chosen game:
1. Find its entity in the graph
2. Get all neighbors (what methods does it call, what inherits from what)
3. Get entities in its source file
4. Get method signatures for key methods
5. Query natural language: "What does the Hangman game do?" (POST `/query`)
6. Query structured data for the game (POST `/query/data`)

### Step 6: Propose and Implement a Feature
Based on your exploration, implement a small feature. Examples:
- Add a score tracker to Hangman
- Add a hint system to Snake
- Add difficulty levels to Tetris
- Add a timer to Minesweeper
- Add a high-score feature to any game

The feature should be small enough to implement in 1-2 method changes.

### Step 7: Report Findings
Return a structured report with:
1. Graph overview (entity counts, types)
2. Game entities found and selected
3. Game architecture summary (classes, methods, relations)
4. Feature proposed and rationale
5. Implementation plan

## Important Notes

- Query endpoints POST `/query` and POST `/query/data` may be slow — wait up to 30 seconds for a response
- Windows paths use backslashes in the graph — normalize with `.replace("\\", "/")` when comparing
- If an entity name has special characters (spaces, dots), try different formats
- The graph may have 300+ entities — use `max_nodes=10000` to get all data
- Focus on `GET /graphs` endpoint — it returns all nodes and edges for client-side filtering