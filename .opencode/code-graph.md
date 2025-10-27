# CodeGraph: Codebase Knowledge Graph System

## Overview

The CodeGraph system transforms codebases into knowledge graphs similar to LightRAG's entity-relationship model. It maps code entities (classes, functions, variables, modules) as nodes and their relationships (inheritance, calls, imports, dependencies) as edges, creating a comprehensive graph representation for enhanced code understanding and navigation.

## Graph Representation Model

### Entity Types (Nodes)
- **Module/Package**: Top-level containers (files, directories)
- **Class**: Object-oriented types and structures
- **Function/Method**: Callable entities and procedures
- **Variable/Constant**: Data entities and configuration
- **Interface/Protocol**: Abstract contracts and specifications
- **Type/Enum**: Data type definitions
- **Import/Dependency**: External references

### Relationship Types (Edges)
- **INHERITS**: Class inheritance relationships
- **IMPLEMENTS**: Interface implementation
- **CALLS**: Function/method invocation
- **IMPORTS**: Module/package dependencies
- **CONTAINS**: Containment relationships (class contains method)
- **REFERENCES**: Variable/type references
- **OVERRIDES**: Method overriding
- **EXTENDS**: Extension relationships
- **USES**: General usage dependencies
- **DEFINES**: Definition relationships

### Entity Attributes
- **name**: Identifier name
- **type**: Entity type classification
- **location**: File path and line numbers
- **visibility**: Access modifiers (public, private, protected)
- **language**: Programming language
- **signature**: Function signatures, type annotations
- **docstring**: Documentation strings
- **complexity**: Cyclomatic complexity (for functions)

## Agent Instructions for Systematic Codebase Scanning

### Complete Project Scanning
For comprehensive analysis of entire codebase, see detailed instructions in [scan-complete.md](scan-complete.md)

**Objective**: Build a comprehensive knowledge graph of the entire codebase through systematic 6-task approach covering project structure, modules, classes, functions, cross-references, and graph consolidation.

### Targeted Entry-Point Scanning  
For focused analysis from specific keywords/entry points, see detailed instructions in [scan-targeted.md](scan-targeted.md)

**Objective**: Build targeted knowledge graph from specific starting points through 6-task approach covering entry point identification, dependency mapping, transitive discovery, reverse analysis, context enrichment, and subgraph integration.

## Implementation Guidelines

### Context Window Management
- **File Batching**: Process 10-15 files per task to avoid context limits
- **Relationship Chunking**: Handle relationships in semantic groups
- **Progressive Refinement**: Build graph incrementally with validation steps
- **Checkpointing**: Save intermediate results between tasks

### Language-Specific Adaptations
- **Python**: Focus on modules, classes, decorators, imports
- **JavaScript/TypeScript**: Emphasize modules, exports, prototypes, interfaces
- **Java/C#**: Highlight packages, classes, interfaces, annotations
- **C/C++**: Track headers, includes, function definitions, structs

### Output Format
```json
{
  "entities": [
    {
      "id": "unique_identifier",
      "name": "entity_name",
      "type": "class|function|variable|module",
      "location": "file_path:line_number",
      "attributes": {
        "visibility": "public|private|protected",
        "signature": "function_signature",
        "docstring": "documentation",
        "language": "programming_language"
      }
    }
  ],
  "relationships": [
    {
      "source": "entity_id",
      "target": "entity_id", 
      "type": "CALLS|INHERITS|IMPORTS|CONTAINS",
      "attributes": {
        "strength": 0.8,
        "context": "relationship_context"
      }
    }
  ],
  "metadata": {
    "scan_type": "complete|targeted",
    "timestamp": "iso_timestamp",
    "entry_points": ["keyword1", "keyword2"],
    "files_processed": 150,
    "entities_found": 500,
    "relationships_mapped": 800
  }
}
```

### Quality Assurance
- **Relationship Validation**: Verify all relationships reference valid entities
- **Completeness Check**: Ensure all entry points have been traced
- **Consistency Verification**: Check for naming conflicts and ambiguities
- **Performance Metrics**: Track processing time and memory usage per task

This systematic approach ensures comprehensive codebase analysis while maintaining manageable context windows and providing actionable knowledge graph representations for code understanding and navigation.