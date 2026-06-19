"""Parser for C# analyser text output into structured entities and relations.

The C# analyser (CSharpAnalyzerTool.exe) outputs one .txt file per class.
Format per file:
    FILE: <relative-path>
    [empty line]
    CLASS: <ClassName>
      METHOD: <name> -> RETURNS -> <returntype>
      METHOD: <name> -> RETURNS -> <returntype>
    [empty line]
    PROJECT REFERENCES:
      - <ProjectName>
      - <ProjectName>
"""

from __future__ import annotations

import re
from typing import Any


def parse_csharp_file(
    filename: str, content: str
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    """Parse a single C# analyser output file into entities and relations.

    Args:
        filename: The .txt output filename (e.g. "TestLib_Calculator.txt").
        content: The text content of the analyser output file.

    Returns:
        (entities, relations) where each is a list of dicts suitable for
        graph_diff.diff_entities / diff_relations.
    """
    entities: list[dict[str, Any]] = []
    relations: list[dict[str, Any]] = []

    lines = content.splitlines()

    file_path = ""
    class_name = ""
    methods: list[str] = []
    project_refs: list[str] = []
    current_section = ""

    for line in lines:
        stripped = line.strip()

        if stripped.startswith("FILE:"):
            file_path = stripped[5:].strip()

        elif stripped.startswith("CLASS:"):
            class_name = stripped[6:].strip()

        elif stripped.startswith("METHOD:"):
            method_match = re.match(
                r"METHOD:\s*(\w+)\s*->\s*RETURNS\s*->\s*(.+)", stripped
            )
            if method_match:
                methods.append(method_match.group(1))

        elif stripped.startswith("PROJECT REFERENCES:"):
            current_section = "refs"

        elif stripped.startswith("- ") and current_section == "refs":
            project_refs.append(stripped[2:].strip())

    if not class_name:
        return [], []

    entity_data = {
        "entity_type": "CLASS",
        "description": f"Class {class_name} with {len(methods)} method(s): "
        + ", ".join(methods),
        "source_id": filename,
        "file_path": file_path,
        "methods": methods,
    }
    entities.append({
        "entity_name": class_name,
        **entity_data,
    })

    for method in methods:
        relations.append({
            "source": class_name,
            "target": method,
            "description": f"{class_name}.{method}()",
            "keywords": "method, defines",
            "weight": 1.0,
        })

    for ref in project_refs:
        relations.append({
            "source": class_name,
            "target": ref,
            "description": f"{class_name} references project {ref}",
            "keywords": "project_reference, dependency",
            "weight": 1.0,
        })

    return entities, relations


def parse_csharp_output(
    results: list[tuple[str, str]]
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    """Parse all C# analyser output files into a combined entity/relation set.

    Args:
        results: List of (filename, content) tuples from run_analysis().

    Returns:
        (all_entities, all_relations) deduplicated by name/key.
    """
    entity_map: dict[str, dict[str, Any]] = {}
    relation_set: list[dict[str, Any]] = []

    for filename, content in results:
        entities, relations = parse_csharp_file(filename, content)

        for ent in entities:
            name = ent.get("entity_name", "")
            if name and name not in entity_map:
                entity_map[name] = ent

        relation_set.extend(relations)

    unique_relations: list[dict[str, Any]] = []
    seen_rel_keys: set[str] = set()
    for rel in relation_set:
        key = (rel.get("source", ""), rel.get("target", ""))
        if key not in seen_rel_keys:
            seen_rel_keys.add(key)
            unique_relations.append(rel)

    return list(entity_map.values()), unique_relations

