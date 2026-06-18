import os
import sys
import time
import threading
import queue
from pathlib import Path
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler
import logging
import httpx

# CONFIGURABLES
INPUT_DIR = os.environ.get("INPUT_DIR", "./rag_input")  # directory to watch
DEBOUNCE_SECONDS = float(os.environ.get("DEBOUNCE_SECONDS", "0.5"))  # debounce per path
MAX_WORKERS = int(os.environ.get("WATCHER_WORKERS", "4"))
LIGHTRAG_URL = os.environ.get("LIGHTRAG_URL", "http://localhost:9621").rstrip("/")
LIGHTRAG_API_KEY = os.environ.get("LIGHTRAG_API_KEY", None)

# Ensure INPUT_DIR exists
Path(INPUT_DIR).mkdir(parents=True, exist_ok=True)

logging.basicConfig(level=logging.DEBUG, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)


def ingest_file(file_path: str, change_type: str):
    headers = {}
    if LIGHTRAG_API_KEY:
        headers["X-API-Key"] = LIGHTRAG_API_KEY

    if change_type in ("created", "modified"):
        try:
            with open(file_path, "r", encoding="utf-8", errors="replace") as f:
                content = f.read()
        except Exception as e:
            logger.error(f"Cannot read {file_path}: {e}")
            return

        resp = httpx.post(
            f"{LIGHTRAG_URL}/documents/text",
            json={"text": content, "file_source": file_path},
            headers=headers,
            timeout=30,
        )
        if resp.is_success:
            logger.info(f"Ingested {change_type}: {file_path}")
        else:
            logger.error(f"Failed to ingest {file_path}: {resp.status_code} {resp.text}")

    elif change_type in ("deleted",):
        resp = httpx.delete(
            f"{LIGHTRAG_URL}/documents",
            json={"file_source": file_path},
            headers=headers,
            timeout=30,
        )
        if resp.is_success:
            logger.info(f"Deleted: {file_path}")
        else:
            logger.warning(f"Delete {file_path}: {resp.status_code} {resp.text}")

# Debounce manager
class DebounceManager:
    def __init__(self, debounce_seconds: float):
        self.debounce = debounce_seconds
        self.lock = threading.Lock()
        self.timers = {}  # path -> Timer

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
work_queue = queue.Queue()
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
        debouncer.schedule(path, lambda: self._handle(path, "created"), path, "created")

    def on_modified(self, event):
        if event.is_directory:
            return
        path = str(os.path.abspath(event.src_path))
        logger.debug(f"File modified event: {path}")
        debouncer.schedule(path, lambda: self._handle(path, "modified"), path, "modified")

    def on_deleted(self, event):
        if event.is_directory:
            return
        path = str(os.path.abspath(event.src_path))
        logger.debug(f"File deleted event: {path}")
        # Deletions should be processed immediately, not debounced
        work_queue.put((path, "deleted"))

    def on_moved(self, event):
        if event.is_directory:
            return
        # Treat as: delete old path, then create new path
        old_path = str(os.path.abspath(event.src_path))
        new_path = str(os.path.abspath(event.dest_path))
        logger.debug(f"File moved from {old_path} to {new_path}")
        work_queue.put((old_path, "deleted"))
        debouncer.schedule(new_path, lambda: self._handle(new_path, "created"), new_path, "created")

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
