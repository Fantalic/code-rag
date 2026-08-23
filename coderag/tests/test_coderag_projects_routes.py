"""Unit tests for the CodeRAG projects routes (no server needed)."""

import sys

# Importing the router pulls in lightrag.api.auth, which parses global CLI args
# at import time — pytest's argv would crash argparse. Parse with clean argv
# exactly like conftest's mock_lightrag_server fixture does.
_argv_backup = sys.argv
sys.argv = [_argv_backup[0]]
try:
    from lightrag.api.routers.coderag_projects_routes import (
        create_coderag_projects_routes,
        store as projects_store,
    )
finally:
    sys.argv = _argv_backup

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient


@pytest.fixture()
def client(tmp_path, monkeypatch):
    """Fresh FastAPI app mounting only the projects router; mirror redirected to tmp."""
    mirror_paths = [str(tmp_path / "projects.json")]
    monkeypatch.setattr(projects_store, "_mirror_paths", mirror_paths, raising=False)
    projects_store._items.clear()  # type: ignore[attr-defined]
    projects_store._active_id = None  # type: ignore[attr-defined]
    app = FastAPI()
    app.include_router(create_coderag_projects_routes())
    with TestClient(app) as c:
        yield c
    projects_store._items.clear()  # type: ignore[attr-defined]
    projects_store._active_id = None  # type: ignore[attr-defined]


def _proj(pid="p1", **kw):
    base = {"id": pid, "name": kw.get("name", pid), "path": kw.get("path", f"/tmp/{pid}"), "kind": "csharp"}
    base.update(kw)
    return base


def test_list_initially_empty(client):
    res = client.get("/coderag/projects")
    assert res.status_code == 200
    body = res.json()
    assert body["projects"] == []
    assert body["active_id"] is None


def test_active_404_when_no_focus(client):
    res = client.get("/coderag/projects/active")
    assert res.status_code == 404


def test_upsert_then_list_then_active(client):
    payload = {"project": _proj("p1", total_counts={"files": 7}), "focus": True}
    res = client.post("/coderag/projects", json=payload)
    assert res.status_code == 200
    assert res.json()["project"]["total_counts"]["files"] == 7

    active = client.get("/coderag/projects/active")
    assert active.status_code == 200
    assert active.json()["id"] == "p1"


def test_focus_then_active_changes(client):
    client.post("/coderag/projects", json={"project": _proj("p1")})
    client.post("/coderag/projects", json={"project": _proj("p2")})
    client.post("/coderag/projects/p2/focus")
    active = client.get("/coderag/projects/active")
    assert active.json()["id"] == "p2"


def test_focus_unknown_404(client):
    res = client.post("/coderag/projects/ghost/focus")
    assert res.status_code == 404


def test_delete_removes_project(client):
    client.post("/coderag/projects", json={"project": _proj("p1"), "focus": True})
    client.delete("/coderag/projects/p1")
    assert client.get("/coderag/projects").json()["projects"] == []
    # active should be cleared
    assert client.get("/coderag/projects/active").status_code == 404


def test_persistence_roundtrip(tmp_path):
    """Hydrate from a disk file written by the host-CLI registry format."""
    import json
    host_file = tmp_path / "projects.json"
    host_file.write_text(
        json.dumps({"projects": [_proj("disk-1", path=r"E:\seed")]}, indent=2)
    )
    projects_store._mirror_paths = [str(host_file)]  # type: ignore[attr-defined]
    projects_store._items.clear()  # type: ignore[attr-defined]
    projects_store._active_id = None  # type: ignore[attr-defined]
    loaded = projects_store.hydrate_from_host()
    assert loaded["loaded"] == 1
    assert "disk-1" in projects_store._items  # type: ignore[attr-defined]


def test_clear_returns_notice(client):
    client.post("/coderag/projects", json={"project": _proj("p1"), "focus": True})
    res = client.post("/coderag/projects/p1/clear")
    assert res.status_code == 200
    assert "lightrag storage is global" in res.json()["note"].lower()


def test_list_orders_by_last_scan_at(client):
    client.post("/coderag/projects", json={"project": _proj("old", last_scan_at=100.0)})
    client.post("/coderag/projects", json={"project": _proj("new", last_scan_at=200.0)})
    res = client.get("/coderag/projects").json()
    ids = [p["id"] for p in res["projects"]]
    assert ids == ["new", "old"]
