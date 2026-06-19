"""HTTP client for LightRAG entity/relation CRUD endpoints.

Calls graph CRUD endpoints directly, bypassing the LLM document pipeline.
"""

from __future__ import annotations

import logging
import os
from typing import Any

import httpx

logger = logging.getLogger(__name__)

LIGHTRAG_URL = os.environ.get("LIGHTRAG_URL", "http://localhost:9621").rstrip("/")
LIGHTRAG_API_KEY = os.environ.get("LIGHTRAG_API_KEY", None)
REQUEST_TIMEOUT = float(os.environ.get("LIGHTRAG_TIMEOUT", "30.0"))


def _headers() -> dict[str, str]:
    headers: dict[str, str] = {}
    if LIGHTRAG_API_KEY:
        headers["X-API-Key"] = LIGHTRAG_API_KEY
    return headers


def _url(path: str) -> str:
    return f"{LIGHTRAG_URL}{path}"


# ---------------------------------------------------------------------------
# Entity CRUD
# ---------------------------------------------------------------------------


def check_entity_exists(entity_name: str) -> bool:
    """GET /graph/entity/exists?name=... -> {"exists": bool}"""
    try:
        resp = httpx.get(
            _url("/graph/entity/exists"),
            params={"name": entity_name},
            headers=_headers(),
            timeout=REQUEST_TIMEOUT,
        )
        resp.raise_for_status()
        return resp.json().get("exists", False)
    except Exception as e:
        logger.error(f"check_entity_exists({entity_name!r}) failed: {e}")
        return False


def create_entity(
    entity_name: str,
    description: str = "",
    entity_type: str = "UNKNOWN",
    source_id: str = "",
    file_path: str = "",
) -> dict[str, Any]:
    """POST /graph/entity/create

    Returns the response data dict, or raises on failure.
    """
    payload = {
        "entity_name": entity_name,
        "entity_data": {
            "description": description,
            "entity_type": entity_type,
            "source_id": source_id or "manual_creation",
            "file_path": file_path or "manual_creation",
        },
    }
    resp = httpx.post(
        _url("/graph/entity/create"),
        json=payload,
        headers=_headers(),
        timeout=REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    logger.info(f"Created entity {entity_name!r} ({entity_type})")
    return resp.json()


def update_entity(
    entity_name: str,
    updated_data: dict[str, Any],
    allow_rename: bool = False,
) -> dict[str, Any]:
    """POST /graph/entity/edit"""
    payload = {
        "entity_name": entity_name,
        "updated_data": updated_data,
        "allow_rename": allow_rename,
    }
    resp = httpx.post(
        _url("/graph/entity/edit"),
        json=payload,
        headers=_headers(),
        timeout=REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    logger.info(f"Updated entity {entity_name!r}")
    return resp.json()


def delete_entity(entity_name: str) -> dict[str, Any]:
    """DELETE /documents/delete_entity

    Returns the DeletionResult dict.
    """
    payload = {"entity_name": entity_name}
    resp = httpx.request(
        "DELETE",
        _url("/documents/delete_entity"),
        json=payload,
        headers=_headers(),
        timeout=REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    data = resp.json()
    status = data.get("status", "?")
    logger.info(f"Deleted entity {entity_name!r} -> status={status}")
    return data


# ---------------------------------------------------------------------------
# Relation CRUD
# ---------------------------------------------------------------------------


def create_relation(
    source_entity: str,
    target_entity: str,
    description: str = "",
    keywords: str = "",
    weight: float = 1.0,
    source_id: str = "",
    file_path: str = "",
) -> dict[str, Any]:
    """POST /graph/relation/create"""
    payload = {
        "source_entity": source_entity,
        "target_entity": target_entity,
        "relation_data": {
            "description": description,
            "keywords": keywords,
            "weight": weight,
            "source_id": source_id or "manual_creation",
            "file_path": file_path or "manual_creation",
        },
    }
    resp = httpx.post(
        _url("/graph/relation/create"),
        json=payload,
        headers=_headers(),
        timeout=REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    logger.info(f"Created relation {source_entity!r} -> {target_entity!r}")
    return resp.json()


def update_relation(
    source_entity: str,
    target_entity: str,
    updated_data: dict[str, Any],
) -> dict[str, Any]:
    """POST /graph/relation/edit"""
    payload = {
        "source_id": source_entity,
        "target_id": target_entity,
        "updated_data": updated_data,
    }
    resp = httpx.post(
        _url("/graph/relation/edit"),
        json=payload,
        headers=_headers(),
        timeout=REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    logger.info(f"Updated relation {source_entity!r} -> {target_entity!r}")
    return resp.json()


def delete_relation(source_entity: str, target_entity: str) -> dict[str, Any]:
    """DELETE /documents/delete_relation"""
    payload = {"source_entity": source_entity, "target_entity": target_entity}
    resp = httpx.request(
        "DELETE",
        _url("/documents/delete_relation"),
        json=payload,
        headers=_headers(),
        timeout=REQUEST_TIMEOUT,
    )
    resp.raise_for_status()
    data = resp.json()
    status = data.get("status", "?")
    logger.info(
        f"Deleted relation {source_entity!r} -> {target_entity!r} -> status={status}"
    )
    return data


# ---------------------------------------------------------------------------
# Graph reads
# ---------------------------------------------------------------------------


def get_all_entities() -> list[dict[str, Any]]:
    """GET /graphs?label=* returns the full knowledge graph.

    Returns the list of nodes (entities) or empty list on failure.
    """
    try:
        resp = httpx.get(
            _url("/graphs"),
            params={"label": "*", "max_depth": 1, "max_nodes": 10000},
            headers=_headers(),
            timeout=REQUEST_TIMEOUT,
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("nodes", [])
    except Exception as e:
        logger.error(f"get_all_entities() failed: {e}")
        return []
