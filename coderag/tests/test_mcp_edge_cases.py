"""Test MCP client edge cases — errors, env vars, streaming."""

import pytest

pytest.importorskip("daniel_lightrag_mcp")


class TestMCPEdgeCases:
    @pytest.mark.asyncio
    async def test_connection_refused(self):
        from daniel_lightrag_mcp.client import LightRAGClient, LightRAGConnectionError

        client = LightRAGClient(base_url="http://localhost:1")
        try:
            with pytest.raises(LightRAGConnectionError):
                await client.get_health()
        finally:
            await client.client.aclose()

    @pytest.mark.asyncio
    async def test_empty_query_raises_validation(self):
        from daniel_lightrag_mcp.client import LightRAGClient, LightRAGValidationError
        import httpx
        from httpx import ASGITransport
        import sys

        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            import lightrag.api.lightrag_server as server_mod
            server_mod.check_frontend_build = lambda: None
            import ascii_colors
            ascii_colors.ASCIIColors.green = lambda t, **k: None
            import lightrag.llm.ollama as ollama_mod
            import numpy as np

            async def _mock_llm(prompt, system_prompt=None, history_messages=None, **kwargs):
                return "mock response"

            async def _mock_embed(texts, embed_model, **kwargs):
                return np.array([[0.1] * 1024 for _ in texts])

            ollama_mod.ollama_model_complete = _mock_llm
            ollama_mod.ollama_embed = _mock_embed

            from lightrag.api.lightrag_server import get_application
            app = get_application()

            client = LightRAGClient(base_url="http://test")
            await client.client.aclose()
            client.client = httpx.AsyncClient(
                transport=ASGITransport(app=app),
                base_url="http://test",
            )

            with pytest.raises(LightRAGValidationError):
                await client.query_text("")
        finally:
            sys.argv = old_argv

    @pytest.mark.asyncio
    async def test_invalid_mode_raises_validation(self):
        from daniel_lightrag_mcp.client import LightRAGClient, LightRAGValidationError
        import httpx
        from httpx import ASGITransport
        import sys

        old_argv = sys.argv
        sys.argv = ["lightrag-server"]
        try:
            import lightrag.api.lightrag_server as server_mod
            server_mod.check_frontend_build = lambda: None
            import ascii_colors
            ascii_colors.ASCIIColors.green = lambda t, **k: None
            import lightrag.llm.ollama as ollama_mod
            import numpy as np

            async def _mock_llm(prompt, system_prompt=None, history_messages=None, **kwargs):
                return "mock response"

            async def _mock_embed(texts, embed_model, **kwargs):
                return np.array([[0.1] * 1024 for _ in texts])

            ollama_mod.ollama_model_complete = _mock_llm
            ollama_mod.ollama_embed = _mock_embed

            from lightrag.api.lightrag_server import get_application
            app = get_application()

            client = LightRAGClient(base_url="http://test")
            await client.client.aclose()
            client.client = httpx.AsyncClient(
                transport=ASGITransport(app=app),
                base_url="http://test",
            )

            with pytest.raises(LightRAGValidationError):
                await client.query_text("hello", mode="invalid_mode")
        finally:
            sys.argv = old_argv


class TestMCPEnvVarUnification:
    def test_lightrag_url_is_used(self, monkeypatch):
        import os
        monkeypatch.setenv("LIGHTRAG_URL", "http://custom:9999")
        monkeypatch.delenv("LIGHTRAG_BASE_URL", raising=False)

        from daniel_lightrag_mcp.server import _get_lightrag_url
        assert _get_lightrag_url() == "http://custom:9999"

    def test_lightrag_base_url_fallback(self, monkeypatch):
        import os
        monkeypatch.delenv("LIGHTRAG_URL", raising=False)
        monkeypatch.setenv("LIGHTRAG_BASE_URL", "http://fallback:1111")

        from daniel_lightrag_mcp.server import _get_lightrag_url
        assert _get_lightrag_url() == "http://fallback:1111"
