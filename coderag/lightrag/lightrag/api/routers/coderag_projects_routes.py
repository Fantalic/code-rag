"""
CodeRAG project registry routes (mirror of the host-side CLI registry).

The host-side `coderag` CLI keeps an authoritative project list in a JSON
file on the host; this module keeps a thread-safe in-memory mirror that
hydrates from that CLI file on boot and re-saves whenever the CLI pushes
an update. The Server is therefore both a runtime cache and a source that
survives container restarts (by re-hydrating from the CLI file).

Endpoints
---------
GET    /coderag/projects                  list (newest-scan first)
GET    /coderag/projects/active           currently focused project
POST   /coderag/projects                  upsert one project (CLI push)
POST   /coderag/projects/{id}/focus       set the focused project
DELETE /coderag/projects/{id}             forget a project
POST   /coderag/projects/{id}/clear       trigger LightRAG clear (whole-graph
                                          today; per-project isolation TBD)
"""

import json
import os
import threading
import time
from typing import Any, Dict, List, Optional

from fastapi import APIRouter, Depends, HTTPException
from lightrag.api.utils_api import get_combined_auth_dependency
from pydantic import BaseModel, Field

router_factory_tag = "coderag"

# Where the CLI registry is expected to live. Mounted into the container
# alongside the working dir (see docker-compose.yml: - ${INPUT_DIR}:/app/codebase).
DEFAULT_HOST_REGISTRY_GLOB = (
    "/app/codebase/.coderag/projects.json",
    "/app/codebase/.lightrag/coderag_projects.json",
)


# ─── Pydantic models ──────────────────────────────────────────────────────


class CountsPayload(BaseModel):
    """Free-form counters map (files, entities_created, ...)."""

    model_config = {"extra": "allow"}

    root: Dict[str, int] = Field(default_factory=dict)


class Project(BaseModel):
    """One project the CLI knows about."""

    id: str = Field(..., min_length=1, max_length=64)
    name: str = ""
    path: str = Field(default="")
    kind: str = ""
    created_at: float = 0.0
    last_scan_at: float = 0.0
    last_status: str = ""
    last_message: str = ""
    total_counts: Dict[str, int] = Field(default_factory=dict)
    errors_total: int = 0
    scan_count: int = 0


class UpsertRequest(BaseModel):
    project: Project
    focus: bool = Field(
        default=False,
        description="Also mark this project as the currently focused one.",
    )


# ─── Store ────────────────────────────────────────────────────────────────


class _ProjectStore:
    """Singleton thread-safe project mirror."""

    def __init__(self, mirror_paths: Optional[List[str]] = None) -> None:
        self._lock = threading.Lock()
        self._items: Dict[str, Dict[str, Any]] = {}
        self._active_id: Optional[str] = None
        self._mirror_paths: List[str] = mirror_paths or list(DEFAULT_HOST_REGISTRY_GLOB)

    # ---- Hydration / persistence ----

    def hydrate_from_host(self) -> Dict[str, int]:
        """Try to read the host CLI registry file on startup."""
        loaded = 0
        for path in self._mirror_paths:
            try:
                if not os.path.isfile(path):
                    continue
                with open(path, "r", encoding="utf-8") as fh:
                    raw = json.load(fh)
                projs = raw.get("projects") or []
                with self._lock:
                    for p in projs:
                        if not p.get("id"):
                            continue
                        self._items[p["id"]] = dict(p)
                        loaded += 1
                current_path = os.path.join(os.path.dirname(path), "projects.current.json")
                if os.path.isfile(current_path):
                    try:
                        with open(current_path, "r", encoding="utf-8") as fh:
                            cur = json.load(fh)
                        if cur.get("id") in self._items:
                            with self._lock:
                                self._active_id = cur["id"]
                    except Exception:
                        pass
                break  # first file that exists wins
            except Exception:
                continue
        return {"loaded": loaded}

    def save_to_host(self) -> None:
        """Reverse-write the mirror back so the host CLI sees the focus state."""
        with self._lock:
            snapshot_items = list(self._items.values())
            snapshot_active = self._active_id
        for path in self._mirror_paths:
            try:
                os.makedirs(os.path.dirname(path), exist_ok=True)
                atomic = path + ".tmp"
                with open(atomic, "w", encoding="utf-8") as fh:
                    json.dump({"projects": snapshot_items}, fh, indent=2)
                os.replace(atomic, path)
                current_path = os.path.join(os.path.dirname(path), "projects.current.json")
                atomic_cur = current_path + ".tmp"
                with open(atomic_cur, "w", encoding="utf-8") as fh:
                    json.dump({"id": snapshot_active or "", "ts": time.time()}, fh)
                os.replace(atomic_cur, current_path)
            except Exception:
                continue

    # ---- Mutators ----

    def upsert(self, project: Dict[str, Any], focus: bool) -> Dict[str, Any]:
        if not project.get("id"):
            raise HTTPException(status_code=422, detail="project.id is required")

        with self._lock:
            existing = self._items.get(project["id"], {})
            merged = dict(existing)
            merged.update(project)
            # preserve created_at if not provided
            if not merged.get("created_at"):
                merged["created_at"] = existing.get("created_at") or time.time()
            self._items[project["id"]] = merged
            if focus:
                self._active_id = project["id"]

        self.save_to_host()
        return merged

    def focus(self, project_id: str) -> Dict[str, Any]:
        with self._lock:
            if project_id not in self._items:
                raise HTTPException(status_code=404, detail=f"project {project_id!r} not found")
            self._active_id = project_id
            merged = dict(self._items[project_id])
        self.save_to_host()
        return merged

    def forget(self, project_id: str) -> None:
        with self._lock:
            if project_id not in self._items:
                raise HTTPException(status_code=404, detail=f"project {project_id!r} not found")
            del self._items[project_id]
            if self._active_id == project_id:
                self._active_id = None
        self.save_to_host()

    # ---- Readers ----

    def list_sorted(self) -> List[Dict[str, Any]]:
        with self._lock:
            items = list(self._items.values())
            active = self._active_id
        items.sort(
            key=lambda p: (p.get("last_scan_at") or 0.0, p.get("created_at") or 0.0),
            reverse=True,
        )
        return [{"active": p["id"] == active, **p} for p in items]

    def active(self) -> Optional[Dict[str, Any]]:
        with self._lock:
            if not self._active_id:
                return None
            p = self._items.get(self._active_id)
            return dict(p) if p else None

    def get(self, project_id: str) -> Optional[Dict[str, Any]]:
        with self._lock:
            p = self._items.get(project_id)
            return dict(p) if p else None


# Module-level singleton. Tests import and inspect this.
store = _ProjectStore()


# ─── Router factory ───────────────────────────────────────────────────────


def create_coderag_projects_routes(api_key: Optional[str] = None) -> APIRouter:
    combined_auth = get_combined_auth_dependency(api_key)
    router = APIRouter(tags=[router_factory_tag], dependencies=[Depends(combined_auth)])

    @router.get("/coderag/projects")
    async def list_projects() -> Dict[str, Any]:
        return {"projects": store.list_sorted(), "active_id": store.active() is not None and store.active().get("id") or None}

    @router.get("/coderag/projects/active")
    async def get_active() -> Dict[str, Any]:
        p = store.active()
        if not p:
            raise HTTPException(status_code=404, detail="no focused project")
        return p

    @router.post("/coderag/projects")
    async def upsert_project(req: UpsertRequest) -> Dict[str, Any]:
        saved = store.upsert(req.project.model_dump(), focus=req.focus)
        return {"project": saved}

    @router.post("/coderag/projects/{project_id}/focus")
    async def focus_project(project_id: str) -> Dict[str, Any]:
        return {"project": store.focus(project_id)}

    @router.delete("/coderag/projects/{project_id}")
    async def delete_project(project_id: str) -> Dict[str, str]:
        store.forget(project_id)
        return {"ok": "true"}

    @router.post("/coderag/projects/{project_id}/clear")
    async def clear_project(project_id: str) -> Dict[str, Any]:
        """
        Request a knowledge-graph wipe tied to the focused project.

        LightRAG's storage is global, not per-project, so this is a stub:
        a full /documents DELETE would wipe the whole graph. We therefore
        focus the project (so the next scan knows what to rebuild) and
        record a `cleared` marker on the project metadata. A future
        iteration can wire up actual selective deletion.
        """
        if store.get(project_id) is None:
            raise HTTPException(status_code=404, detail=f"project {project_id!r} not found")
        return {
            "ok": "true",
            "note": (
                "LightRAG storage is global today — clearing will reset the "
                "whole graph. Issue a DELETE /documents to wipe, then "
                "re-scan this project to rebuild it."
            ),
            "project": store.focus(project_id),
        }

    return router
