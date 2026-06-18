# Targeted Entry-Point Scanning Tasks

**Objective**: Build targeted knowledge graph from specific starting points

## Multi-Task Approach

### Task 2.1: Entry Point Identification

**Purpose**: Locate and classify target keywords/entities in the codebase

**Instructions**:
- Search for target keywords/entities using grep/ripgrep with patterns
- Identify exact locations and definitions (file paths, line numbers)
- Classify entity types (class, function, variable, module, interface)
- Prioritize entry points by relevance and centrality
- Filter out false positives (comments, strings, unrelated matches)
- Group related entry points for efficient processing

**Output**: Prioritized list of entry points with locations
```json
{
  "entry_points": [
    {
      "keyword": "DatabaseManager",
      "type": "class",
      "locations": [
        {"file": "src/database.py", "line": 25, "definition": true},
        {"file": "src/main.py", "line": 15, "usage": true}
      ],
      "priority": "high"
    }
  ]
}
```

### Task 2.2: Direct Dependency Mapping

**Purpose**: Map immediate relationships for each entry point

**Instructions** (Per entry point):
- For each entry point:
  * Extract immediate dependencies (direct calls, imports, references)
  * Map direct relationships within same file
  * Identify parent/container entities (class containing method)
  * Note immediate inheritance or implementation relationships
  * Track direct usage patterns and instantiations
  * Limit scope to single-hop relationships

**Output**: First-degree relationship map
```json
{
  "direct_dependencies": [
    {
      "source": "DatabaseManager",
      "relationships": [
        {"type": "CALLS", "target": "connect_to_db"},
        {"type": "INHERITS", "target": "BaseManager"},
        {"type": "USES", "target": "ConnectionPool"}
      ]
    }
  ]
}
```

### Task 2.3: Transitive Relationship Discovery

**Purpose**: Expand relationships through breadth-first exploration

**Instructions** (Breadth-first expansion):
- From direct dependencies, expand to second-degree relationships
- Follow call chains and inheritance hierarchies
- Map cross-file dependencies and imports
- Limit depth to prevent context overflow (max 3-4 levels)
- Track relationship paths and connection strength
- Stop expansion at framework/library boundaries

**Output**: Multi-degree relationship subgraph
```json
{
  "transitive_relationships": [
    {
      "path": ["DatabaseManager", "connect_to_db", "create_connection"],
      "depth": 2,
      "relationship_chain": ["CALLS", "CALLS"],
      "end_points": ["create_connection"]
    }
  ]
}
```

### Task 2.4: Reverse Dependency Analysis

**Purpose**: Find entities that depend on the entry points

**Instructions**:
- Search for entities that reference/use the entry points
- Identify callers, inheritors, and importers
- Map "who depends on this" relationships
- Track usage patterns across different modules
- Note test files and documentation references
- Identify critical dependency paths

**Output**: Reverse dependency map
```json
{
  "reverse_dependencies": [
    {
      "target": "DatabaseManager",
      "used_by": [
        {"entity": "UserService", "relationship": "INSTANTIATES"},
        {"entity": "DataProcessor", "relationship": "CALLS"},
        {"entity": "TestDatabase", "relationship": "TESTS"}
      ]
    }
  ]
}
```

### Task 2.5: Context Enrichment

**Purpose**: Add semantic context and metadata to the subgraph

**Instructions**:
- Add semantic context (docstrings, comments, type hints)
- Extract type information and function signatures
- Identify design patterns and architectural relationships
- Note performance-critical paths or bottlenecks
- Add business logic context from comments/documentation
- Classify relationship importance and frequency

**Output**: Enriched subgraph with contextual information
```json
{
  "enriched_entities": [
    {
      "name": "DatabaseManager",
      "context": {
        "docstring": "Manages database connections and transactions",
        "design_pattern": "Singleton",
        "performance_critical": true,
        "business_context": "Core data access layer"
      }
    }
  ]
}
```

### Task 2.6: Subgraph Integration

**Purpose**: Merge all entry point subgraphs into unified focused graph

**Instructions**:
- Merge all entry point subgraphs
- Identify overlapping entities and relationships
- Resolve conflicts and deduplicate nodes/edges
- Calculate subgraph metrics and centrality scores
- Create unified focused knowledge graph
- Generate insights about component interactions

**Output**: Targeted knowledge graph for specified keywords/entry points
```json
{
  "focused_graph": {
    "entities": [...],
    "relationships": [...],
    "entry_points": ["DatabaseManager", "UserService"],
    "subgraph_metrics": {
      "total_entities": 45,
      "max_depth": 3,
      "central_entities": ["DatabaseManager"],
      "cluster_density": 0.7
    }
  }
}
```

## Quality Checks

- **Entry Point Coverage**: Verify all specified keywords/entities found
- **Relationship Integrity**: Ensure all mapped relationships are valid
- **Depth Consistency**: Check that expansion depth limits are respected
- **Context Accuracy**: Validate that semantic context is correctly extracted