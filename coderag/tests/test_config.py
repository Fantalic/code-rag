"""Test the config module — no side effects, lazy loading."""

import pytest

pytest.importorskip("lightrag")


class TestConfigModule:
    def test_import_without_argv(self):
        """Importing config should not require sys.argv hack."""
        import sys
        old_argv = sys.argv
        sys.argv = ["test_runner"]
        try:
            from lightrag.api.config import get_global_args
            args = get_global_args()
            assert args.host == "0.0.0.0"
            assert args.port == 9621
            assert args.llm_binding == "ollama"
        finally:
            sys.argv = old_argv

    def test_get_global_args_is_lazy(self):
        """get_global_args should return same instance on repeated calls."""
        import sys
        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            from lightrag.api.config import get_global_args
            args1 = get_global_args()
            args2 = get_global_args()
            assert args1 is args2
        finally:
            sys.argv = old_argv

    def test_get_env_value(self):
        """get_env_value should return default when env var not set."""
        from lightrag.api.config import get_env_value
        import os

        # Ensure not set
        os.environ.pop("_TEST_LIGHTRAG_UNSET", None)
        val = get_env_value("_TEST_LIGHTRAG_UNSET", "default_val", str)
        assert val == "default_val"

    def test_get_env_value_int(self):
        """get_env_value should parse int types."""
        from lightrag.api.config import get_env_value
        import os

        os.environ["_TEST_LIGHTRAG_INT"] = "42"
        try:
            val = get_env_value("_TEST_LIGHTRAG_INT", 0, int)
            assert val == 42
            assert isinstance(val, int)
        finally:
            os.environ.pop("_TEST_LIGHTRAG_INT", None)
