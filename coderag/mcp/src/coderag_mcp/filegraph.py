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


def _edge_endpoints(edge: dict[str, Any]) -> tuple[str, str]:
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


def build_file_graph(
    nodes: list[dict[str, Any]], edges: list[dict[str, Any]]
) -> FileGraph:
    """Build a FileGraph from raw LightRAG graph nodes and edges."""
    fg = FileGraph()
    for node in nodes:
        node_id = _node_id(node)
        file_path = _node_file_path(node)
        if not node_id or not file_path:
            continue
        fg.entity_file[node_id] = file_path
        fg.file_entities[file_path].append(node_id)

    for edge in edges:
        src, tgt = _edge_endpoints(edge)
        file_src = fg.entity_file.get(src)
        file_tgt = fg.entity_file.get(tgt)
        if not file_src or not file_tgt or file_src == file_tgt:
            continue
        if file_tgt not in fg.refs[file_src]:
            fg.refs[file_src].append(file_tgt)
        if file_src not in fg.deps[file_tgt]:
            fg.deps[file_tgt].append(file_src)

    fg.files = sorted(fg.file_entities.keys())
    return fg
