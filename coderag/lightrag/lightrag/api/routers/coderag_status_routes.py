"""
CodeRAG scan-status routes.

The host-side `coderag` CLI pushes pipeline events here (POST /coderag/status/event)
so the WebUI can show the scan/watch process live (GET /coderag/status polling).

State is kept in memory only: it resets when the server restarts, which is fine —
the CLI reports fresh runs continuously while it works.
"""

import threading
import time
from collections import deque
from typing import Any, Dict, List, Optional

from fastapi import APIRouter, Depends, HTTPException
from lightrag.api.utils_api import get_combined_auth_dependency
from pydantic import BaseModel, Field

router_factory_tag = "coderag"

# Pipeline stages the CLI reports (kept in sync with coderag/cli).
VALID_STAGES = {
    "watch",      # watcher lifecycle (started/stopped)
    "debounce",   # fs event debounced / queued
    "hash",       # hash check (skip unchanged)
    "route",      # extension -> analyser routing
    "analyse",    # analyser subprocess running/done
    "filter",     # security filter pass
    "diff",       # graph diff computed
    "crud",       # entity/relation CRUD + batch-import
    "llm",        # Path-B LLM fallback ingest
    "scan",       # whole-scan lifecycle (start/end/summary)
}

VALID_LEVELS = {"info", "warn", "error"}
VALID_STATES = {"idle", "scanning", "error"}


class StatusEvent(BaseModel):
    """One progress event pushed by the host-side coderag CLI."""

    run_id: str = Field(..., description="Groups all events of one scan/watch burst")
    stage: str = Field(..., description="Pipeline stage (see VALID_STAGES)")
    level: str = Field(default="info", description="info | warn | error")
    message: str = Field(default="", description="Human-readable progress line")
    detail: str = Field(default="", description="Optional focus object, e.g. current file")
    state: Optional[str] = Field(
        default=None, description="Optional run-state override (scanning/error)"
    )
    ts: Optional[float] = Field(default=None, description="Host unix timestamp (seconds)")
    counts: Optional[Dict[str, int]] = Field(
        default=None,
        description="Cumulative counters, keys merged into the run snapshot "
        "(files, entities_created, entities_updated, entities_deleted, relations)",
    )


class _RunState:
    """Mutable snapshot of the latest/latest-finished scan run."""

    __slots__ = (
        "run_id",
        "state",
        "stage",
        "message",
        "detail",
        "started_at",
        "updated_at",
        "counts",
    )

    def __init__(self) -> None:
        self.run_id: Optional[str] = None
        self.state: str = "idle"
        self.stage: str = ""
        self.message: str = ""
        self.detail: str = ""
        self.started_at: float = 0.0
        self.updated_at: float = 0.0
        self.counts: Dict[str, int] = {}


class _StatusStore:
    """Thread-safe in-memory store: one run snapshot + bounded event ring."""

    MAX_EVENTS = 500

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._run = _RunState()
        self._events: deque = deque(maxlen=self.MAX_EVENTS)

    def apply_event(self, ev: StatusEvent) -> Dict[str, Any]:
        """Merge one pushed event into the snapshot and ring buffer."""
        now = time.time()
        ts = ev.ts if ev.ts is not None else now

        stored = {
            "ts": ts,
            "run_id": ev.run_id,
            "stage": ev.stage,
            "level": ev.level,
            "message": ev.message,
            "detail": ev.detail,
        }

        with self._lock:
            # New run id -> start a fresh snapshot.
            if self._run.run_id != ev.run_id:
                self._run = _RunState()
                self._run.run_id = ev.run_id
                self._run.started_at = ts
                self._run.state = "scanning"

            self._run.updated_at = now
            self._run.stage = ev.stage
            self._run.message = ev.message
            self._run.detail = ev.detail

            if ev.state in VALID_STATES:
                self._run.state = ev.state
            elif ev.level == "error":
                self._run.state = "error"

            if ev.counts:
                self._run.counts.update(ev.counts)

            self._events.append(stored)

            return {
                **self._snapshot_locked(),
                "server_time": now,
                "events": list(self.recent_events_locked(50)),
            }

    def snapshot(self, event_limit: int = 50) -> Dict[str, Any]:
        with self._lock:
            return {
                **self._snapshot_locked(),
                "server_time": time.time(),
                "events": list(self.recent_events_locked(event_limit)),
            }

    def clear(self) -> None:
        with self._lock:
            self._run = _RunState()
            self._events.clear()

    # Callers must hold self._lock.
    def _snapshot_locked(self) -> Dict[str, Any]:
        return {
            "run_id": self._run.run_id,
            "state": self._run.state,
            "stage": self._run.stage,
            "message": self._run.message,
            "detail": self._run.detail,
            "started_at": self._run.started_at,
            "updated_at": self._run.updated_at,
            "counts": dict(self._run.counts),
        }

    # Callers must hold self._lock.
    def recent_events_locked(self, limit: int) -> List[Dict[str, Any]]:
        """Newest-first view over the ring buffer."""
        if limit <= 0 or not self._events:
            return []
        take = list(self._events)[-limit:]
        take.reverse()
        return take


# Module-level singleton so tests can import and inspect it.
store = _StatusStore()


def create_coderag_status_routes(api_key: Optional[str] = None) -> APIRouter:
    combined_auth = get_combined_auth_dependency(api_key)
    router = APIRouter(tags=[router_factory_tag], dependencies=[Depends(combined_auth)])

    @router.post("/coderag/status/event")
    async def push_status_event(event: StatusEvent) -> Dict[str, Any]:
        """Ingest one progress event pushed by the coderag CLI."""
        if event.stage not in VALID_STAGES:
            raise HTTPException(status_code=422, detail=f"unknown stage '{event.stage}'")
        if event.level not in VALID_LEVELS:
            raise HTTPException(status_code=422, detail=f"unknown level '{event.level}'")
        return store.apply_event(event)

    @router.post("/coderag/status/events")
    async def push_status_events(events: List[StatusEvent]) -> Dict[str, Any]:
        """Batch variant of /coderag/status/event (applied in order)."""
        result: Dict[str, Any] = {}
        for ev in events:
            if ev.stage not in VALID_STAGES:
                raise HTTPException(status_code=422, detail=f"unknown stage '{ev.stage}'")
            if ev.level not in VALID_LEVELS:
                raise HTTPException(status_code=422, detail=f"unknown level '{ev.level}'")
            result = store.apply_event(ev)
        return result

    @router.get("/coderag/status")
    async def get_status() -> Dict[str, Any]:
        """Snapshot polled by the WebUI Scan tab."""
        return store.snapshot()

    @router.delete("/coderag/status")
    async def clear_status() -> Dict[str, str]:
        """Reset state (e.g. after a --clean scan)."""
        store.clear()
        return {"ok": "true"}

    return router
