import os
import sys
import time
import threading
import queue
from pathlib import Path
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

# CONFIGURABLES
INPUT_DIR = os.environ.get("INPUT_DIR", "./rag_input")  # directory to watch
DEBOUNCE_SECONDS = float(os.environ.get("DEBOUNCE_SECONDS", "0.5"))  # debounce per path
MAX_WORKERS = int(os.environ.get("WATCHER_WORKERS", "4"))

# Ensure INPUT_DIR exists
Path(INPUT_DIR).mkdir(parents=True, exist_ok=True)

# Ingestion function placeholder
def ingest_file(file_path: str, change_type: str):
    """
    Implement your LightRAG ingestion logic here.
    - file_path: path to the file that changed
    - change_type: 'created', 'modified', 'deleted', 'moved'
    This function must be idempotent:
      - For created/modified: upsert embeddings and update graph
      - For deleted: remove embeddings and prune graph edges
    """
    # Replace with your actual ingestion call, e.g.:
    # rag.ingest_file(file_path, change_type=change_type)
    print(f"[ingest] {change_type} {file_path}")

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
        try:
            ingest_file(path, change_type)
        except Exception as e:
            print(f"[error] ingestion failed for {path}: {e}", file=sys.stderr)
        finally:
            work_queue.task_done()

# Event handler
class LightRAGEventHandler(FileSystemEventHandler):
    def on_created(self, event):
        if event.is_directory:
            return
        path = os.path.abspath(event.src_path)
        debouncer.schedule(path, self._handle, path, "created")

    def on_modified(self, event):
        if event.is_directory:
            return
        path = os.path.abspath(event.src_path)
        debouncer.schedule(path, self._handle, path, "modified")

    def on_deleted(self, event):
        if event.is_directory:
            return
        path = os.path.abspath(event.src_path)
        # Deletions should be processed immediately, not debounced
        work_queue.put((path, "deleted"))

    def on_moved(self, event):
        if event.is_directory:
            return
        # Treat as: delete old path, then create new path
        old_path = os.path.abspath(event.src_path)
        new_path = os

