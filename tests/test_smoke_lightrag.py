import pytest

pytest.importorskip("lightrag")


class TestLightRAGCore:
    def test_import_and_version(self):
        import lightrag
        assert lightrag.__version__ == "1.4.9.4"

    def test_embedding_func_class(self):
        from lightrag.utils import EmbeddingFunc
        ef = EmbeddingFunc(
            embedding_dim=768,
            func=lambda x: None,
            max_token_size=512,
        )
        assert ef.embedding_dim == 768
        assert ef.max_token_size == 512

    def test_can_instantiate(self, temp_working_dir):
        from lightrag import LightRAG
        import numpy as np

        async def dummy_embed(texts):
            return np.array([[0.0] * 768 for _ in texts])

        from lightrag.utils import EmbeddingFunc
        embedding_func = EmbeddingFunc(
            embedding_dim=768,
            func=dummy_embed,
            max_token_size=512,
        )

        rag = LightRAG(
            working_dir=temp_working_dir,
            llm_model_func=lambda *a, **kw: "ok",
            embedding_func=embedding_func,
            enable_llm_cache=False,
        )
        assert rag is not None
        assert rag.working_dir == temp_working_dir


class TestLightRAGAPI:
    def test_import_create_app(self):
        import sys
        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            from lightrag.api.lightrag_server import create_app, get_application
            assert callable(create_app)
            assert callable(get_application)
        finally:
            sys.argv = old_argv

    def test_global_args_has_defaults(self):
        import sys
        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            from lightrag.api.config import get_global_args
            args = get_global_args()
            assert hasattr(args, "host")
            assert hasattr(args, "port")
            assert hasattr(args, "llm_binding")
            assert hasattr(args, "embedding_binding")
        finally:
            sys.argv = old_argv

    def test_config_module_imports(self):
        import sys
        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            from lightrag.api.config import (
                update_uvicorn_mode_config,
                get_default_host,
            )
            assert callable(update_uvicorn_mode_config)
            assert callable(get_default_host)
        finally:
            sys.argv = old_argv

    def test_utils_api_imports(self):
        import sys
        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            from lightrag.api.utils_api import (
                get_combined_auth_dependency,
                display_splash_screen,
            )
            assert callable(get_combined_auth_dependency)
            assert callable(display_splash_screen)
        finally:
            sys.argv = old_argv

    def test_routers_import(self):
        import sys
        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            from lightrag.api.routers.document_routes import DocumentManager
            from lightrag.api.routers.query_routes import create_query_routes
            assert DocumentManager is not None
            assert callable(create_query_routes)
        finally:
            sys.argv = old_argv

    def test_types_import(self):
        import sys
        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            from lightrag.types import GPTKeywordExtractionFormat
            assert GPTKeywordExtractionFormat is not None
        finally:
            sys.argv = old_argv
