# Complete Project Scanning Tasks

**Objective**: Build a comprehensive knowledge graph of the entire codebase

## Multi-Task Approach

### Task 1.1: Project Structure Discovery

**Purpose**: Map the overall project organization and file hierarchy

**Instructions**:
- Scan directory structure using glob patterns (`**/*.py`, `**/*.js`, `**/*.java`, etc.)
- Identify all source files by language extension
- Map project hierarchy and module organization
- Create CONTAINS relationships for directory/file structure
- Identify configuration files, documentation, and build files
- Note package managers and dependency files (package.json, requirements.txt, pom.xml)

**Output**: Project structure map with file inventory
```json
{
  "project_root": "/path/to/project",
  "directories": ["src/", "tests/", "docs/"],
  "source_files": ["src/main.py", "src/utils.py"],
  "config_files": ["package.json", "setup.py"],
  "total_files": 150
}
```

### Task 1.2: Module-Level Analysis

**Purpose**: Extract high-level module dependencies and top-level definitions

**Instructions** (Process in batches of 10-15 files):
- For each source file batch:
  * Extract imports/dependencies at file top
  * Identify top-level definitions (classes, functions, constants, exports)
  * Map module-level relationships
  * Create IMPORTS and DEFINES relationships
  * Note module docstrings and metadata
  * Track external vs internal dependencies

**Output**: Module dependency graph
```json
{
  "modules": [
    {
      "name": "src/database.py",
      "imports": ["sqlite3", "src.config"],
      "exports": ["Database", "connect"],
      "dependencies": ["external", "internal"]
    }
  ]
}
```

### Task 1.3: Class/Type Analysis

**Purpose**: Build inheritance hierarchies and type relationships

**Instructions** (Process classes in batches):
- For each class/type entity:
  * Extract inheritance hierarchy (superclasses, base classes)
  * Identify implemented interfaces or protocols
  * Map inner classes and nested types
  * Create INHERITS and IMPLEMENTS relationships
  * Note class decorators, annotations, or attributes
  * Track abstract classes and concrete implementations

**Output**: Class hierarchy graph
```json
{
  "classes": [
    {
      "name": "DatabaseConnection",
      "inherits_from": ["BaseConnection"],
      "implements": ["IConnectable"],
      "inner_classes": ["ConnectionState"],
      "is_abstract": false
    }
  ]
}
```

### Task 1.4: Function/Method Analysis

**Purpose**: Map function call relationships and method behaviors

**Instructions** (Process in logical groups):
- For each function/method:
  * Extract function signatures and parameters
  * Identify function calls within body (both internal and external)
  * Map method overrides and implementations
  * Create CALLS and OVERRIDES relationships
  * Note decorators, annotations, and return types
  * Track async/sync patterns and error handling

**Output**: Call graph with method relationships
```json
{
  "functions": [
    {
      "name": "process_data",
      "calls": ["validate_input", "transform", "save_result"],
      "overrides": "BaseProcessor.process_data",
      "parameters": ["data", "options"],
      "return_type": "ProcessResult"
    }
  ]
}
```

### Task 1.5: Cross-Reference Analysis

**Purpose**: Map variable usage and data flow across modules

**Instructions**:
- Identify variable/constant references across modules
- Map type usage and instantiation patterns
- Track data flow between functions and classes
- Create REFERENCES and USES relationships
- Note global variables, singletons, and shared state
- Identify configuration and environment variable usage

**Output**: Cross-reference relationship map
```json
{
  "references": [
    {
      "entity": "CONFIG_PATH",
      "referenced_by": ["main.py", "config.py", "tests/test_config.py"],
      "type": "constant",
      "scope": "global"
    }
  ]
}
```

### Task 1.6: Graph Consolidation

**Purpose**: Merge all analysis results into unified knowledge graph

**Instructions**:
- Merge all relationship maps into unified graph structure
- Resolve entity name conflicts and aliases (imports with different names)
- Calculate graph metrics (centrality, clustering, complexity)
- Validate relationship integrity and consistency
- Generate summary statistics and insights
- Create final consolidated knowledge graph

**Output**: Complete codebase knowledge graph
```json
{
  "entities": [...],
  "relationships": [...],
  "metrics": {
    "total_entities": 500,
    "total_relationships": 1200,
    "most_central_entities": ["Database", "UserManager"],
    "complexity_hotspots": ["data_processor.py:45-120"]
  },
  "metadata": {
    "scan_completed": "2024-01-01T12:00:00Z",
    "files_analyzed": 150,
    "languages_detected": ["python", "javascript"]
  }
}
```

## Quality Checks

- **Completeness**: Ensure all source files have been processed
- **Consistency**: Verify entity IDs are unique and relationships are valid
- **Coverage**: Check that major architectural components are represented
- **Performance**: Monitor memory usage and processing time per task