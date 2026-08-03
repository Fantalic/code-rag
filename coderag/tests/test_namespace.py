"""Test the codelightrag unified namespace package."""

import sys
import os

# Add repo root so codelightrag/ is importable (for non-editable test runs)
_repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if _repo_root not in sys.path:
    sys.path.insert(0, _repo_root)

import pytest

pytest.importorskip("lightrag")


class TestCodeLightRAGNamespace:
    def test_import_namespace(self):
        from codelightrag import LightRAG
        assert LightRAG is not None

    def test_version(self):
        from codelightrag import __version__
        assert isinstance(__version__, str)
        assert __version__ == "0.1.1"

    def test_mcp_client_optional(self):
        from codelightrag import LightRAGClient
        try:
            import daniel_lightrag_mcp  # noqa: F401
            assert LightRAGClient is not None
        except ImportError:
            assert LightRAGClient is None

    def test_can_instantiate_lightrag(self, temp_working_dir):
        from codelightrag import LightRAG

        async def dummy_llm(*a, **kw):
            return "ok"

        def dummy_embed(texts, **kw):
            import numpy as np
            return np.array([[0.1] * 1024 for _ in texts])
        dummy_embed.embedding_dim = 1024

        rag = LightRAG(
            working_dir=temp_working_dir,
            llm_model_func=dummy_llm,
            embedding_func=dummy_embed,
            enable_llm_cache=False,
        )
        assert rag is not None
        assert rag.working_dir == temp_working_dir
