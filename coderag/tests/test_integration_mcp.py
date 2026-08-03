import pytest
import httpx
from httpx import ASGITransport


pytest.importorskip("daniel_lightrag_mcp")


class TestMCPClientIntegration:
    """Real MCP client against mocked LightRAG server."""

    @pytest.mark.asyncio
    async def test_health(self, mock_lightrag_server, lifespan):
        from daniel_lightrag_mcp.client import LightRAGClient

        client = LightRAGClient(base_url="http://test")
        await client.client.aclose()
        client.client = httpx.AsyncClient(
            transport=ASGITransport(app=mock_lightrag_server),
            base_url="http://test",
        )

        result = await client.get_health()
        assert result.status == "healthy"

    @pytest.mark.asyncio
    async def test_insert_text(self, mock_lightrag_server, lifespan):
        from daniel_lightrag_mcp.client import LightRAGClient

        client = LightRAGClient(base_url="http://test")
        await client.client.aclose()
        client.client = httpx.AsyncClient(
            transport=ASGITransport(app=mock_lightrag_server),
            base_url="http://test",
        )

        insert_resp = await client.insert_text("Hello world")
        assert insert_resp.status in ("success", "duplicated")

    @pytest.mark.asyncio
    async def test_query(self, mock_lightrag_server, lifespan):
        from daniel_lightrag_mcp.client import LightRAGClient

        client = LightRAGClient(base_url="http://test")
        await client.client.aclose()
        client.client = httpx.AsyncClient(
            transport=ASGITransport(app=mock_lightrag_server),
            base_url="http://test",
        )

        query_resp = await client.query_text("test query")
        assert query_resp.response

    @pytest.mark.asyncio
    async def test_pipeline_status(self, mock_lightrag_server, lifespan):
        from daniel_lightrag_mcp.client import LightRAGClient

        client = LightRAGClient(base_url="http://test")
        await client.client.aclose()
        client.client = httpx.AsyncClient(
            transport=ASGITransport(app=mock_lightrag_server),
            base_url="http://test",
        )

        status = await client.get_pipeline_status()
        assert status.busy is not None
