"""Derive file-level views from the knowledge graph without storing file nodes.

Every entity node carries a source ``file_path`` and every edge references
entity ids, so ``entity -> file`` is a complete mapping. File dependencies are
computed on demand from the relation graph.
"""

from __future__ import annotations

from collections import defaultdict
from typing import Any


def _node_id(node: dict[str, Any]) -> str:
    node_id = node.get("id") or ""
    if node_id:
        return str(node_id)
    props = node.get("properties") or {}
    return str(props.get("entity_id") or "")


def _node_file_path(node: dict[str, Any]) -> str:
    props = node.get("properties") or {}
    return str(props.get("file_path") or "")


def edge_endpoints(edge: dict[str, Any]) -> tuple[str, str]:
    """Return the true source/target entity ids of an edge.

    LightRAG swaps ``source``/``target`` lexicographically on read-back, so the
    original direction is read from ``properties.source``/``properties.target``
    when present.
    """
    props = edge.get("properties") or {}
    src = props.get("source") or ""
    tgt = props.get("target") or ""
    if src and tgt:
        return str(src), str(tgt)
    return str(edge.get("source") or ""), str(edge.get("target") or "")


class FileGraph:
    """Files, their entities, and derived cross-file reference edges."""

    def __init__(self) -> None:
        self.files: list[str] = []
        self.entity_file: dict[str, str] = {}
        self.file_entities: dict[str, list[str]] = defaultdict(list)
        self.refs: dict[str, list[str]] = defaultdict(list)
        self.deps: dict[str, list[str]] = defaultdict(list)

    def resolve_file(self, arg: str) -> str | None:
        """Resolve a user-supplied file path, basename, or substring."""
        if arg in self.file_entities:
            return arg
        base = arg.replace("\\", "/").rsplit("/", 1)[-1]
        for f in self.files:
            if f.replace("\\", "/").rsplit("/", 1)[-1] == base:
                return f
        matches = [f for f in self.files if arg in f]
        if len(matches) == 1:
            return matches[0]
        return None

    def entities(self, file: str) -> list[str]:
        return sorted(self.file_entities.get(file, []))

    def references(self, file: str) -> list[str]:
        return sorted(self.refs.get(file, []))

    def dependents(self, file: str) -> list[str]:
        return sorted(self.deps.get(file, []))

    def stats(self) -> list[dict[str, Any]]:
        return [
            {
                "file_path": f,
                "nodes": len(self.file_entities.get(f, [])),
                "fan_in": len(self.deps.get(f, [])),
                "fan_out": len(self.refs.get(f, [])),
            }
            for f in sorted(self.files)
        ]

    def cycles(self) -> list[list[str]]:
        """Dependency cycles via Tarjan's strongly connected components."""
        index: dict[str, int] = {}
        low: dict[str, int] = {}
        on_stack: dict[str, bool] = {}
        stack: list[str] = []
        counter = 0
        sccs: list[list[str]] = []

        def strongconnect(v: str) -> None:
            nonlocal counter
            counter += 1
            index[v] = counter
            low[v] = counter
            stack.append(v)
            on_stack[v] = True

            for w in self.refs.get(v, []):
                if w not in index:
                    strongconnect(w)
                    low[v] = min(low[v], low[w])
                elif on_stack.get(w, False):
                    low[v] = min(low[v], index[w])

            if low[v] == index[v]:
                component: list[str] = []
                while True:
                    w = stack.pop()
                    on_stack[w] = False
                    component.append(w)
                    if w == v:
                        break
                sccs.append(component)

        for f in self.files:
            if f not in index:
                strongconnect(f)

        cycles: list[list[str]] = []
        for comp in sccs:
            if len(comp) > 1:
                cycles.append(sorted(comp))
        return sorted(cycles, key=lambda c: c[0])


def _resolve_import_file(
    importer_file: str, import_path: str, known_files: list[str]
) -> str | None:
    """Resolve a Zig `@import("…")` target to a known project file.

    The import target is relative to the importing file's directory (Zig
    resolves import paths relative to the importing file) and usually
    omits the `.zig` extension. Returns the canonical known file path on
    an exact match, falling back to a unique basename match.
    """
    importer_dir = importer_file.replace("\\", "/").rsplit("/", 1)[0]
    target = import_path.strip().strip('"')
    if not target:
        return None
    # Skip std-lib / package imports ("std", "raylib", "raygui") that
    # never resolve to a project file.
    if "/" not in target and "." not in target:
        return None
    if not target.endswith(".zig"):
        target += ".zig"
    combined = target if target.startswith("/") else f"{importer_dir}/{target}"
    # Normalize "." and ".." segments without filesystem access.
    segments: list[str] = []
    for part in combined.split("/"):
        if part in ("", "."):
            continue
        if part == "..":
            if segments:
                segments.pop()
            continue
        segments.append(part)
    norm = "/".join(segments)

    known_norm = {f.replace("\\", "/"): f for f in known_files}
    if norm in known_norm:
        return known_norm[norm]
    # Fallback: unique basename match (handles path-prefix differences).
    base = norm.rsplit("/", 1)[-1]
    candidates = [
        f for f in known_files if f.replace("\\", "/").rsplit("/", 1)[-1] == base
    ]
    if len(candidates) == 1:
        return candidates[0]
    return None


def build_file_graph(
    nodes: list[dict[str, Any]], edges: list[dict[str, Any]]
) -> FileGraph:
    """Build a FileGraph from raw LightRAG graph nodes and edges.

    Cross-file references come from two sources:
      1. relation edges between entities in different files, and
      2. IMPORT entities (Zig `@import` / `usingnamespace`), which are
         nodes rather than edges — resolved via `_resolve_import_file`.
    """
    fg = FileGraph()
    importer_by_name: dict[str, str] = {}
    for node in nodes:
        node_id = _node_id(node)
        file_path = _node_file_path(node)
        if not node_id or not file_path:
            continue
        fg.entity_file[node_id] = file_path
        fg.file_entities[file_path].append(node_id)
        props = node.get("properties") or {}
        if (props.get("entity_type") or "").upper() == "IMPORT":
            importer_by_name[node_id] = file_path

    fg.files = sorted(fg.file_entities.keys())

    def _add_ref(file_src: str, file_tgt: str) -> None:
        if not file_src or not file_tgt or file_src == file_tgt:
            return
        if file_tgt not in fg.refs[file_src]:
            fg.refs[file_src].append(file_tgt)
        if file_src not in fg.deps[file_tgt]:
            fg.deps[file_tgt].append(file_src)

    for edge in edges:
        src, tgt = edge_endpoints(edge)
        _add_ref(fg.entity_file.get(src, ""), fg.entity_file.get(tgt, ""))

    # Resolve IMPORT entities into file references. The import name is
    # stored as "import:<path>"; the entity's own file_path is the
    # importing file.
    for name, importer_file in importer_by_name.items():
        target = name
        if target.lower().startswith("import:"):
            target = target[len("import:") :]
        target_file = _resolve_import_file(importer_file, target, fg.files)
        if target_file:
            _add_ref(importer_file, target_file)

    return fg
