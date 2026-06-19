"""File change watcher that syncs changes to LightRAG knowledge graph.

Pipelines file changes through:
  file extension -> analyser -> diff engine -> security filter -> graph CRUD
Falls back to old POST /documents/text for unsupported file types.
"""

from __future__ import annotations

import hashlib
import logging
import os
import queue
import threading
import time
from pathlib import Path

import httpx
from file_router import get_analyser, is_supported
from graph_crud import (
    create_entity,
    create_relation,
    delete_entity,
    delete_relation,
    get_all_entities,
    update_entity,
    update_relation,
)
from graph_diff import diff_entities, diff_relations
from security_filter import filter_entity_data, filter_relation_data

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer

# CONFIGURABLES
INPUT_DIR = os.environ.get("INPUT_DIR", "./rag_input")
DEBOUNCE_SECONDS = float(os.environ.get("DEBOUNCE_SECONDS", "0.5"))
MAX_WORKERS = int(os.environ.get("WATCHER_WORKERS", "4"))
LIGHTRAG_URL = os.environ.get("LIGHTRAG_URL", "http://localhost:9621").rstrip("/")
LIGHTRAG_API_KEY = os.environ.get("LIGHTRAG_API_KEY", None)

# Ensure INPUT_DIR exists
Path(INPUT_DIR).mkdir(parents=True, exist_ok=True)

logging.basicConfig(
    level=logging.DEBUG, format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# File hash cache: file_path -> MD5 hex digest
FILE_HASHES: dict[str, str] = {}

# Re-ingestion counter for versioned file_source
REINGEST_COUNTER: dict[str, int] = {}


def _file_hash(file_path: str) -> str | None:
    """Return MD5 hex digest of file content, or None if unreadable."""
    try:
        with open(file_path, "rb") as f:
            return hashlib.md5(f.read()).hexdigest()
    except Exception as e:
        logger.error(f"Cannot hash {file_path}: {e}")
        return None


def _next_file_source(file_path: str, is_new: bool) -> str:
    """Generate a file_source that bypasses LightRAG's dedup on re-ingest."""
    if is_new:
        return file_path
    count = REINGEST_COUNTER.get(file_path, 0) + 1
    REINGEST_COUNTER[file_path] = count
    return f"{file_path}#v{count}"


# ---------------------------------------------------------------------------
# Pipeline: run analysis for a supported file
# ---------------------------------------------------------------------------


def _find_solution_file(file_path: str) -> str | None:
    """Walk up from file_path looking for a .sln file."""
    current = Path(file_path).resolve().parent
    for _ in range(10):
        sln_files = list(current.glob("*.sln"))
        if sln_files:
            return str(sln_files[0])
        parent = current.parent
        if parent == current:
            break
        current = parent
    return None


def _analyse_file(
    file_path: str, extension: str
) -> tuple[list[dict], list[dict]] | None:
    """Run analysis on a file, returning (entities, relations).

    Returns None if analysis cannot be performed (falls back to old pipeline).
    """
    analyser_name = get_analyser(extension)
    if analyser_name is None:
        return None

    if analyser_name == "c_sharp":
        from watchdog.analyser_parsers import parse_csharp_output

        sln = _find_solution_file(file_path)
        if sln is None:
            logger.warning(f"No .sln found for {file_path}, cannot run C# analyser")
            return None

        try:
            from daniel_lightrag_mcp.analyser_runner import run_analysis

            results = run_analysis(sln)
            if not results:
                logger.warning(f"C# analyser produced no output files for {sln}")
                return None

            entities, relations = parse_csharp_output(results)
            logger.info(
                f"C# analyser extracted {len(entities)} entities, "
                f"{len(relations)} relations from {sln}"
            )
            return entities, relations

        except ImportError:
            logger.warning(
                "daniel_lightrag_mcp not available. "
                "Install it with: pip install -e daniel-lightrag-mcp"
            )
            return None
        except Exception as e:
            logger.error(f"C# analyser failed for {file_path}: {e}")
            return None

    return None


def _run_pipeline(file_path: str, change_type: str) -> bool:
    """Run the full analysis pipeline for a supported file.

    Returns True if pipeline was applied, False if it should fall back.
    """
    import os as _os

    _, ext = _os.path.splitext(file_path)

    if not is_supported(file_path):
        return False

    result = _analyse_file(file_path, ext)
    if result is None:
        return False

    new_entities, new_relations = result

    # Load current graph state for diffing
    current_entities = get_all_entities()
    current_entity_map: dict[str, dict] = {}
    for ent in current_entities:
        name = ent.get("entity_name") or ent.get("entity_id") or ""
        if name:
            current_entity_map[name] = ent

    # Build old relations set from graph
    current_relation_set: set[tuple[str, str, dict]] = set()
    try:
        resp = httpx.get(
            f"{LIGHTRAG_URL}/graphs",
            params={"label": "*", "max_depth": 2, "max_nodes": 10000},
            headers=_headers(),
            timeout=30,
        )
        if resp.is_success:
            graph_data = resp.json()
            for edge in graph_data.get("edges", []):
                src = edge.get("source", edge.get("src_id", ""))
                tgt = edge.get("target", edge.get("tgt_id", ""))
                if src and tgt:
                    current_relation_set.add((src, tgt, edge))
    except Exception as e:
        logger.warning(f"Could not load current relations: {e}")

    # Build new relation set
    new_relation_set: set[tuple[str, str, dict]] = set()
    for rel in new_relations:
        src = rel.get("source", rel.get("source_entity", ""))
        tgt = rel.get("target", rel.get("target_entity", ""))
        if src and tgt:
            skip_keys = {"source", "target", "source_entity", "target_entity"}
            rel_data = {k: v for k, v in rel.items() if k not in skip_keys}
            new_relation_set.add((src, tgt, rel_data))

    # Diff
    new_entity_map: dict[str, dict] = {}
    for ent in new_entities:
        name = ent.get("entity_name") or ent.get("entity_id") or ""
        if name:
            new_entity_map[name] = ent

    entity_actions = diff_entities(current_entity_map, new_entity_map)
    relation_actions = diff_relations(current_relation_set, new_relation_set)

    # Apply actions in order: DELETE first, then CREATE/UPDATE
    applied_entity_count = 0
    applied_relation_count = 0

    for action in entity_actions:
        try:
            if action.kind == "DELETE":
                delete_entity(action.entity_name)
                applied_entity_count += 1
            elif action.kind == "CREATE":
                safe_data = filter_entity_data(action.data)
                create_entity(
                    entity_name=action.entity_name,
                    description=safe_data.get("description", ""),
                    entity_type=safe_data.get("entity_type", "UNKNOWN"),
                    source_id=safe_data.get("source_id", file_path),
                    file_path=file_path,
                )
                applied_entity_count += 1
            elif action.kind == "UPDATE":
                safe_data = filter_entity_data(action.data)
                update_entity(
                    entity_name=action.entity_name,
                    updated_data=safe_data,
                )
                applied_entity_count += 1
        except Exception as e:
            logger.error(f"Entity action failed: {action} - {e}")

    for action in relation_actions:
        try:
            if action.kind == "DELETE":
                delete_relation(action.source, action.target)
                applied_relation_count += 1
            elif action.kind == "CREATE":
                safe_data = filter_relation_data(action.data)
                create_relation(
                    source_entity=action.source,
                    target_entity=action.target,
                    description=safe_data.get("description", ""),
                    keywords=safe_data.get("keywords", ""),
                    weight=float(safe_data.get("weight", 1.0)),
                    source_id=file_path,
                    file_path=file_path,
                )
                applied_relation_count += 1
            elif action.kind == "UPDATE":
                safe_data = filter_relation_data(action.data)
                update_relation(
                    source_entity=action.source,
                    target_entity=action.target,
                    updated_data=safe_data,
                )
                applied_relation_count += 1
        except Exception as e:
            logger.error(f"Relation action failed: {action} - {e}")

    logger.info(
        f"Pipeline applied: {applied_entity_count} entity actions, "
        f"{applied_relation_count} relation actions for {file_path}"
    )
    return True


def _headers() -> dict[str, str]:
    headers: dict[str, str] = {}
    if LIGHTRAG_API_KEY:
        headers["X-API-Key"] = LIGHTRAG_API_KEY
    return headers


# ---------------------------------------------------------------------------
# Main ingestion entry point
# ---------------------------------------------------------------------------


def ingest_file(file_path: str, change_type: str):
    headers = _headers()

    if change_type in ("created", "modified"):
        new_hash = _file_hash(file_path)
        if new_hash is None:
            return
        prev_hash = FILE_HASHES.get(file_path)
        if prev_hash is not None and new_hash == prev_hash:
            logger.debug(f"Unchanged, skipping: {file_path}")
            return

        # Try pipeline first
        pipeline_applied = _run_pipeline(file_path, change_type)
        if pipeline_applied:
            FILE_HASHES[file_path] = new_hash
            return

        # Fallback: old-style ingest via /documents/text
        try:
            with open(file_path, "r", encoding="utf-8", errors="replace") as f:
                content = f.read()
        except Exception as e:
            logger.error(f"Cannot read {file_path}: {e}")
            return

        is_new = prev_hash is None
        file_source = _next_file_source(file_path, is_new)

        resp = httpx.post(
            f"{LIGHTRAG_URL}/documents/text",
            json={"text": content, "file_source": file_source},
            headers=headers,
            timeout=30,
        )
        if resp.is_success:
            logger.info(
                f"Ingested {change_type}: {file_path} (source={file_source})"
            )
            FILE_HASHES[file_path] = new_hash
        else:
            logger.error(
                f"Failed to ingest {file_path}: {resp.status_code} {resp.text}"
            )

    elif change_type in ("deleted",):
        resp = httpx.delete(
            f"{LIGHTRAG_URL}/documents",
            json={"file_source": file_path},
            headers=headers,
            timeout=30,
        )
        if resp.is_success:
            logger.info(f"Deleted: {file_path}")
            FILE_HASHES.pop(file_path, None)
            REINGEST_COUNTER.pop(file_path, None)
        else:
            logger.warning(f"Delete {file_path}: {resp.status_code} {resp.text}")


# ---------------------------------------------------------------------------
# Debounce manager
# ---------------------------------------------------------------------------


class DebounceManager:
    def __init__(self, debounce_seconds: float):
        self.debounce = debounce_seconds
        self.lock = threading.Lock()
        self.timers = {}

    def schedule(self, path: str, callback, *args, **kwargs):
        with self.lock:
            if path in self.timers:
                self.timers[path].cancel()
            timer = threading.Timer(self.debounce, callback, args=args, kwargs=kwargs)
            self.timers[path] = timer
            timer.start()

    def cancel(self, path: str):
        with self.lock:
            if path in self.timers:
                self.timers[path].cancel()
                del self.timers[path]


debouncer = DebounceManager(DEBOUNCE_SECONDS)

# Worker pool
work_queue: queue.Queue[tuple[str, str]] = queue.Queue()
stop_event = threading.Event()


def worker():
    while not stop_event.is_set():
        try:
            path, change_type = work_queue.get(timeout=0.2)
        except queue.Empty:
            continue
        logger.debug(f"Worker processing {change_type}: {path}")
        try:
            ingest_file(path, change_type)
        except Exception as e:
            logger.error(f"Ingestion failed for {path}: {e}")
        finally:
            work_queue.task_done()


# Event handler
class LightRAGEventHandler(FileSystemEventHandler):
    def _handle(self, path: str, change_type: str) -> None:
        logger.debug(f"Scheduling ingestion for {change_type}: {path}")
        work_queue.put((path, change_type))

    def on_created(self, event):
        if event.is_directory:
            return
        path = str(os.path.abspath(event.src_path))
        logger.debug(f"File created event: {path}")
        debouncer.schedule(
            path, lambda: self._handle(path, "created"), path, "created"
        )

    def on_modified(self, event):
        if event.is_directory:
            return
        path = str(os.path.abspath(event.src_path))
        logger.debug(f"File modified event: {path}")
        debouncer.schedule(
            path, lambda: self._handle(path, "modified"), path, "modified"
        )

    def on_deleted(self, event):
        if event.is_directory:
            return
        path = str(os.path.abspath(event.src_path))
        logger.debug(f"File deleted event: {path}")
        work_queue.put((path, "deleted"))

    def on_moved(self, event):
        if event.is_directory:
            return
        old_path = str(os.path.abspath(event.src_path))
        new_path = str(os.path.abspath(event.dest_path))
        logger.debug(f"File moved from {old_path} to {new_path}")
        work_queue.put((old_path, "deleted"))
        debouncer.schedule(
            new_path,
            lambda: self._handle(new_path, "created"),
            new_path,
            "created",
        )


if __name__ == "__main__":
    logger.info(f"Starting {MAX_WORKERS} worker threads")
    threads = []
    for _ in range(MAX_WORKERS):
        t = threading.Thread(target=worker, daemon=True)
        t.start()
        threads.append(t)

    event_handler = LightRAGEventHandler()
    observer = Observer()
    observer.schedule(event_handler, INPUT_DIR, recursive=True)
    observer.start()

    logger.info(f"Watching directory: {INPUT_DIR} (press Ctrl+C to stop)")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        logger.info("Received interrupt, stopping...")
        observer.stop()
        stop_event.set()
    observer.join()
    for t in threads:
        t.join()
    logger.info("Shutdown complete")
