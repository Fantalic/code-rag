"""Unit tests for the CodeRAG scan-status routes (no server needed)."""

import sys

# Importing the router pulls in lightrag.api.auth, which parses global CLI args
# at import time — pytest's argv would crash argparse. Parse with clean argv,
# exactly like conftest's mock_lightrag_server fixture does.
_argv_backup = sys.argv
sys.argv = [_argv_backup[0]]
try:
    from lightrag.api.routers.coderag_status_routes import (
        create_coderag_status_routes,
        store,
    )
finally:
    sys.argv = _argv_backup

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient


@pytest.fixture()
def client():
    """Fresh FastAPI app mounting only the status router."""
    app = FastAPI()
    app.include_router(create_coderag_status_routes())
    with TestClient(app) as c:
        yield c
    store.clear()


def _ev(run_id="run-1", stage="analyse", **kw):
    return {"run_id": run_id, "stage": stage, "message": kw.pop("message", ""), **kw}


def test_snapshot_initially_idle(client):
    res = client.get("/coderag/status")
    assert res.status_code == 200
    data = res.json()
    assert data["state"] == "idle"
    assert data["run_id"] is None
    assert data["events"] == []
    assert "server_time" in data


def test_event_upsert_merges_counts(client):
    r1 = client.post(
        "/coderag/status/event",
        json=_ev(stage="scan", message="scan started", counts={"files": 0}),
    )
    assert r1.status_code == 200
    snap = r1.json()
    assert snap["run_id"] == "run-1"
    assert snap["state"] == "scanning"

    r2 = client.post(
        "/coderag/status/event",
        json=_ev(stage="analyse", detail="A.cs", counts={"files": 7}),
    )
    snap = r2.json()
    assert snap["stage"] == "analyse"
    assert snap["detail"] == "A.cs"
    assert snap["counts"]["files"] == 7

    # GET returns the same snapshot shape, events newest first
    got = client.get("/coderag/status").json()
    assert got["counts"] == {"files": 7}
    assert [e["stage"] for e in got["events"]] == ["analyse", "scan"]


def test_new_run_id_resets_snapshot(client):
    client.post("/coderag/status/event", json=_ev(run_id="run-a", counts={"files": 9}))
    snap = client.post(
        "/coderag/status/event", json=_ev(run_id="run-b", stage="hash")
    ).json()
    assert snap["run_id"] == "run-b"
    # old run's counters must not leak into the new run
    assert snap["counts"] == {}


def test_error_level_sets_state_error(client):
    snap = client.post(
        "/coderag/status/event",
        json=_ev(level="error", message="analyser crashed"),
    ).json()
    assert snap["state"] == "error"


def test_explicit_state_override_wins(client):
    snap = client.post(
        "/coderag/status/event",
        json=_ev(state="idle", message="watcher stopped"),
    ).json()
    assert snap["state"] == "idle"


def test_batch_events_applied_in_order(client):
    batch = [
        _ev(stage="scan", message="start"),
        _ev(stage="crud", counts={"relations": 3}),
        _ev(stage="scan", state="idle", message="done"),
    ]
    snap = client.post("/coderag/status/events", json=batch).json()
    assert snap["state"] == "idle"
    assert snap["counts"]["relations"] == 3
    assert len(snap["events"]) == 3


def test_invalid_stage_rejected(client):
    res = client.post("/coderag/status/event", json=_ev(stage="bogus"))
    assert res.status_code == 422


def test_invalid_level_rejected(client):
    res = client.post("/coderag/status/event", json=_ev(level="fatal"))
    assert res.status_code == 422


def test_clear_resets_store(client):
    client.post("/coderag/status/event", json=_ev())
    assert client.delete("/coderag/status").status_code == 200
    data = client.get("/coderag/status").json()
    assert data["state"] == "idle"
    assert data["events"] == []


def test_ring_buffer_bounded():
    """The event ring must never grow past MAX_EVENTS (500)."""
    assert store._events.maxlen == store.MAX_EVENTS
