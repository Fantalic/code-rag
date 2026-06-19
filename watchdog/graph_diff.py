"""Diff engine — compares old entity state vs new and produces a list of CRUD actions.

Optimised for the common case: most files change infrequently.
"""

from __future__ import annotations

import logging
from typing import Any, Literal

logger = logging.getLogger(__name__)

ActionKind = Literal["CREATE", "UPDATE", "DELETE"]


class EntityAction:
    """A single action to perform against the graph."""

    def __init__(
        self,
        kind: ActionKind,
        entity_name: str,
        data: dict[str, Any] | None = None,
    ):
        self.kind = kind
        self.entity_name = entity_name
        self.data = data or {}

    def __repr__(self) -> str:
        return f"EntityAction({self.kind}, {self.entity_name!r})"


class RelationAction:
    """A single relation action to perform against the graph."""

    def __init__(
        self,
        kind: ActionKind,
        source: str,
        target: str,
        data: dict[str, Any] | None = None,
    ):
        self.kind = kind
        self.source = source
        self.target = target
        self.data = data or {}

    def __repr__(self) -> str:
        return f"RelationAction({self.kind}, {self.source!r} -> {self.target!r})"


def diff_entities(
    old_entities: dict[str, dict[str, Any]],
    new_entities: dict[str, dict[str, Any]],
) -> list[EntityAction]:
    """Compare old and new entity sets, returning the necessary actions.

    Args:
        old_entities: Entity name -> data dict (from the graph).
        new_entities: Entity name -> data dict (from the analyser output).

    Returns:
        List of EntityAction in order: DELETEs first, then CREATEs, then UPDATEs.
        This minimises conflict windows in the graph DB.
    """
    actions: list[EntityAction] = []
    old_names = set(old_entities)
    new_names = set(new_entities)

    # Entities that only exist in the old set -> DELETE
    for name in old_names - new_names:
        actions.append(EntityAction("DELETE", name))

    # Entities that only exist in the new set -> CREATE
    for name in new_names - old_names:
        actions.append(EntityAction("CREATE", name, new_entities[name]))

    # Entities in both sets -> UPDATE if data changed
    for name in old_names & new_names:
        old_data = old_entities[name]
        new_data = new_entities[name]
        if _data_changed(old_data, new_data):
            actions.append(EntityAction("UPDATE", name, new_data))

    return actions


def diff_relations(
    old_relations: set[tuple[str, str, dict[str, Any]]],
    new_relations: set[tuple[str, str, dict[str, Any]]],
) -> list[RelationAction]:
    """Compare old and new relation sets, returning the necessary actions.

    Args:
        old_relations: Set of (source, target, data) tuples (from the graph).
        new_relations: Set of (source, target, data) tuples (from the analyser).

    Returns:
        List of RelationAction.
    """
    actions: list[RelationAction] = []

    old_keys = {(s, t) for s, t, _ in old_relations}
    new_keys = {(s, t) for s, t, _ in new_relations}

    # Relations only in old -> DELETE
    for src, tgt in old_keys - new_keys:
        actions.append(RelationAction("DELETE", src, tgt))

    # Relations only in new -> CREATE
    for src, tgt in new_keys - old_keys:
        data = _relation_data(new_relations, src, tgt)
        actions.append(RelationAction("CREATE", src, tgt, data))

    # Relations in both -> UPDATE if data changed
    for src, tgt in old_keys & new_keys:
        old_data = _relation_data(old_relations, src, tgt)
        new_data = _relation_data(new_relations, src, tgt)
        if _data_changed(old_data, new_data):
            actions.append(RelationAction("UPDATE", src, tgt, new_data))

    return actions


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------


def _data_changed(a: dict[str, Any], b: dict[str, Any]) -> bool:
    """Check if two data dicts differ meaningfully.

    Ignores server-managed fields: created_at, source_id, entity_id.
    """
    IGNORED_KEYS = {"created_at", "source_id", "entity_id", "src_id", "tgt_id"}
    filtered_a = {k: v for k, v in a.items() if k not in IGNORED_KEYS}
    filtered_b = {k: v for k, v in b.items() if k not in IGNORED_KEYS}
    return filtered_a != filtered_b


def _relation_data(
    relations: set[tuple[str, str, dict[str, Any]]],
    src: str,
    tgt: str,
) -> dict[str, Any]:
    """Extract data dict for a specific relation from a set."""
    for s, t, d in relations:
        if s == src and t == tgt:
            return d
    return {}
