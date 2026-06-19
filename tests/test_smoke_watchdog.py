import pytest
import sys
import os

pytest.importorskip("watchdog.observers")

WATCHDOG_DIR = os.path.join(os.path.dirname(__file__), "..", "watchdog")
WATCHDOG_FILE = os.path.join(WATCHDOG_DIR, "main.py")


@pytest.fixture(scope="session")
def watchdog_mod():
    import importlib.util

    sys.path.insert(0, WATCHDOG_DIR)
    spec = importlib.util.spec_from_file_location(
        "watchdog_main", WATCHDOG_FILE
    )
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class TestWatchdogImports:
    def test_file_exists(self):
        assert os.path.isfile(WATCHDOG_FILE)

    def test_syntax(self):
        with open(WATCHDOG_FILE, encoding="utf-8") as f:
            source = f.read()
        compile(source, WATCHDOG_FILE, "exec")


class TestWatchdogComponents:
    def test_ingest_file(self, watchdog_mod):
        assert callable(watchdog_mod.ingest_file)

    def test_debounce_manager(self, watchdog_mod):
        dm = watchdog_mod.DebounceManager(0.5)
        assert dm.debounce == 0.5
        assert dm.timers == {}
        assert dm.lock is not None

    def test_debounce_schedule_and_cancel(self, watchdog_mod):
        called = []
        dm = watchdog_mod.DebounceManager(0.1)
        dm.schedule("/tmp/a", lambda: called.append(1))
        assert "/tmp/a" in dm.timers
        dm.cancel("/tmp/a")
        assert "/tmp/a" not in dm.timers

    def test_lightrag_event_handler(self, watchdog_mod):
        handler = watchdog_mod.LightRAGEventHandler()
        assert hasattr(handler, "on_created")
        assert hasattr(handler, "on_modified")
        assert hasattr(handler, "on_deleted")
        assert hasattr(handler, "on_moved")

    def test_worker_is_callable(self, watchdog_mod):
        assert callable(watchdog_mod.worker)

    def test_constants(self, watchdog_mod):
        assert watchdog_mod.INPUT_DIR == "./rag_input"
        assert watchdog_mod.DEBOUNCE_SECONDS == 0.5
        assert watchdog_mod.MAX_WORKERS == 4
