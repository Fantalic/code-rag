#!/usr/bin/env python3
"""
Test script to verify the regression: why edges have empty properties.source/target.

The Go code reads edges like:
  {"id":"TestApp.Order-TestApp.OrderService._order","source":"","target":"",...}
with the direction stored in "properties.source" and "properties.target".

This script demonstrates the correct logic for extracting the direction.
"""

import json


def _edge_endpoints_original_buggy(raw_edge: dict) -> tuple[str, str]:
    """The current buggy approach from Go code - only looks at root source/target."""
    src = raw_edge.get("source") or ""
    tgt = raw_edge.get("target") or ""
    return str(src), str(tgt)


def _edge_endpoints_correct(raw_edge: dict) -> tuple[str, str]:
    """The correct approach: look at properties.source/target first."""
    src = raw_edge.get("properties", {}).get("source") or ""
    tgt = raw_edge.get("properties", {}).get("target") or ""
    if src and tgt:
        return str(src), str(tgt)
    # Fallback to root source/target only if properties lacks them
    src2 = raw_edge.get("source") or ""
    tgt2 = raw_edge.get("target") or ""
    return str(src2), str(tgt2)


EDGE_EXAMPLES = [
    {"id": "TestApp.Order-TestApp.OrderService._order", "source": "", "target": "", "properties": {"source": "TestApp.Order", "target": "TestApp.OrderService._order"}},
    {"id": "TestApp.OrderService._order-TestApp.OrderService", "source": "", "target": "", "properties": {"source": "TestApp.OrderService", "target": "TestApp.OrderService._order"}},
    {"id": "TestApp.OrderService.PlaceOrder-TestApp.Program.Main", "source": "TestApp.OrderService.PlaceOrder", "target": "TestApp.Program.Main", "properties": {}},
]


if __name__ == "__main__":
    print("Edge endpoint extraction test")
    print("=" * 60)

    for i, edge in enumerate(EDGE_EXAMPLES):
        print(f"\nEdge {i + 1}:")
        print(f"  Raw: {json.dumps(edge, indent=4)}")

        buggy_src, buggy_tgt = _edge_endpoints_original_buggy(edge)
        print(f"  Buggy (root-only):     {buggy_src} -> {buggy_tgt}")

        correct_src, correct_tgt = _edge_endpoints_correct(edge)
        print(f"  Correct (props-first): {correct_src} -> {correct_tgt}")

    print("\n" + "=" * 60)
    print("\nImpact: The buggy extraction results in empty source/target,")
    print("causing derived file graph to skip these edges, leading to")
    print("incorrect references/dependents relationships.")
