"""
Unit tests for the derived file-level graph (filegraph.py).
"""

import pytest

from coderag_mcp.filegraph import FileGraph, build_file_graph


def _node(entity_id: str, file_path: str) -> dict:
    return {
        "id": entity_id,
        "name": entity_id,
        "properties": {
            "entity_id": entity_id,
            "entity_name": entity_id,
            "file_path": file_path,
        },
    }


def _edge(src: str, tgt: str) -> dict:
    return {
        "id": f"{src}->{tgt}",
        "source": src,
        "target": tgt,
        "properties": {"source": src, "target": tgt},
    }


NODES = [
    _node("Order", "Order.cs"),
    _node("OrderService", "OrderService.cs"),
    _node("OrderController", "Program.cs"),
]
EDGES = [
    _edge("OrderController", "OrderService"),
    _edge("OrderService", "Order"),
]


def test_build_refs_and_dependents():
    fg = build_file_graph(NODES, EDGES)
    assert fg.files == ["Order.cs", "OrderService.cs", "Program.cs"]
    assert fg.references("Program.cs") == ["OrderService.cs"]
    assert fg.dependents("Program.cs") == []
    assert fg.references("OrderService.cs") == ["Order.cs"]
    assert fg.dependents("OrderService.cs") == ["Program.cs"]
    assert fg.references("Order.cs") == []
    assert fg.dependents("Order.cs") == ["OrderService.cs"]


def test_direction_from_properties():
    swapped = [
        {
            "id": "OrderService->OrderController",
            "source": "OrderService",
            "target": "OrderController",
            "properties": {"source": "OrderController", "target": "OrderService"},
        }
    ]
    fg = build_file_graph(NODES, swapped)
    assert fg.references("Program.cs") == ["OrderService.cs"]
    assert fg.dependents("OrderService.cs") == ["Program.cs"]


def test_skips_same_file_and_dangling():
    same_file_edge = _edge("OrderService", "OrderRepository")
    nodes = NODES + [_node("OrderRepository", "OrderService.cs")]
    edges = EDGES + [same_file_edge, _edge("Order", "GhostEntity")]
    fg = build_file_graph(nodes, edges)
    assert fg.references("OrderService.cs") == ["Order.cs"]
    assert fg.dependents("OrderService.cs") == ["Program.cs"]


def test_cycles():
    a, b, c = "A.cs", "B.cs", "C.cs"
    nodes = [_node("A", a), _node("B", b), _node("C", c)]
    edges = [
        _edge("A", "B"),
        _edge("B", "C"),
        _edge("C", "A"),
    ]
    fg = build_file_graph(nodes, edges)
    assert fg.cycles() == [[a, b, c]]


def test_no_cycles():
    nodes = [_node("Order", "Order.cs"), _node("OrderService", "OrderService.cs")]
    edges = [_edge("OrderService", "Order")]
    fg = build_file_graph(nodes, edges)
    assert fg.cycles() == []


def test_stats():
    fg = build_file_graph(NODES, EDGES)
    stats = fg.stats()
    by_path = {s["file_path"]: s for s in stats}
    assert by_path["Program.cs"]["nodes"] == 1
    assert by_path["Program.cs"]["fan_out"] == 1
    assert by_path["Program.cs"]["fan_in"] == 0
    assert by_path["OrderService.cs"]["fan_in"] == 1
    assert by_path["OrderService.cs"]["fan_out"] == 1


def test_resolve_file():
    fg = build_file_graph(NODES, EDGES)
    assert fg.resolve_file("OrderService.cs") == "OrderService.cs"
    assert fg.resolve_file("OrderService") == "OrderService.cs"
    assert fg.resolve_file("Service.cs") == "OrderService.cs"
    assert fg.resolve_file("Missing.cs") is None


def _import_node(importer: str, target: str) -> dict:
    return {
        "id": f"import:{target}",
        "name": f"import:{target}",
        "properties": {
            "entity_id": f"import:{target}",
            "entity_type": "IMPORT",
            "description": f'@import("{target}")',
            "file_path": importer,
        },
    }


def test_import_entities_become_file_refs():
    # Zig @import statements are IMPORT entities, not edges; the file
    # graph must resolve them into file -> file references.
    nodes = [
        _node("World", "src/world.zig"),
        _node("VoxelModel", "src/voxel_model.zig"),
        _node("AtomSim", "src/atom/atom_sim.zig"),
        _import_node("src/atom/atom_sim.zig", "../world.zig"),
        _import_node("src/atom/atom_sim.zig", "world.zig"),
        _import_node("src/atom/atom_sim.zig", "../voxel_model.zig"),
        _import_node("src/atom/atom_sim.zig", "std"),
    ]
    fg = build_file_graph(nodes, [])
    # std (package, no path/extension) and the mismatched "world.zig"
    # (relative to src/atom, which is not a real file) do not resolve.
    assert fg.references("src/atom/atom_sim.zig") == [
        "src/voxel_model.zig",
        "src/world.zig",
    ]
    assert fg.dependents("src/world.zig") == ["src/atom/atom_sim.zig"]
    assert fg.dependents("src/voxel_model.zig") == ["src/atom/atom_sim.zig"]
