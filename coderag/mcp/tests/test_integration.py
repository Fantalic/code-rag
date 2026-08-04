"""
Integration tests for the coderag MCP server and client together.
"""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from coderag_mcp.client import (
    LightRAGAPIError,
    LightRAGClient,
    LightRAGConnectionError,
    LightRAGError,
    LightRAGValidationError,
)
from coderag_mcp.server import (
    _error,
    _serialize,
    add_note_to_entity,
    check_entity_exists,
    get_graph_labels,
    get_knowledge_graph,
    get_node_details,
    get_related_files,
    mcp_server,
    query_data,
)


class TestClientModule:
    """Test that client module imports work correctly."""

    def test_lightrag_client_exists(self):
        assert LightRAGClient is not None

    def test_lightrag_client_instantiation(self):
        client = LightRAGClient(base_url="http://localhost:9621")
        assert client.base_url == "http://localhost:9621"

    def test_error_hierarchy(self):
        assert issubclass(LightRAGConnectionError, LightRAGError)
        assert issubclass(LightRAGValidationError, LightRAGError)
        assert issubclass(LightRAGAPIError, LightRAGError)


class TestServerFunctions:
    """Test that server tool functions are properly defined."""

    def test_check_entity_exists_is_coroutine(self):
        import inspect

        assert inspect.iscoroutinefunction(check_entity_exists)

    def test_get_knowledge_graph_is_coroutine(self):
        import inspect

        assert inspect.iscoroutinefunction(get_knowledge_graph)

    def test_get_graph_labels_is_coroutine(self):
        import inspect

        assert inspect.iscoroutinefunction(get_graph_labels)

    def test_get_node_details_is_coroutine(self):
        import inspect

        assert inspect.iscoroutinefunction(get_node_details)


class TestMcpServerRegistration:
    """Test that tools are properly registered on the FastMCP server."""

    @pytest.mark.asyncio
    async def test_list_tools_returns_tools(self):
        tools = await mcp_server.list_tools()
        assert len(tools) > 0

    @pytest.mark.asyncio
    async def test_all_tools_have_schemas(self):
        """Sanity check: every registered tool has a name, description, and inputSchema."""
        tools = await mcp_server.list_tools()
        for tool in tools:
            assert tool.name is not None
            assert tool.description is not None
            assert hasattr(tool, "inputSchema")
            assert isinstance(tool.inputSchema, dict)

    @pytest.mark.asyncio
    async def test_tool_schemas_have_required_fields(self):
        tools = await mcp_server.list_tools()
        for tool in tools:
            assert tool.name is not None
            assert tool.description is not None
            assert hasattr(tool, "inputSchema")
            assert isinstance(tool.inputSchema, dict)


class TestHelpers:
    """Test helper functions."""

    def test_serialize_pydantic_model(self):
        from pydantic import BaseModel

        class DummyModel(BaseModel):
            key: str

        result = _serialize(DummyModel(key="value"))
        assert result == {"key": "value"}

    def test_serialize_list(self):
        result = _serialize([1, 2, 3])
        assert result == {"results": [1, 2, 3]}

    def test_serialize_str(self):
        result = _serialize("hello")
        assert result == {"result": "hello"}

    def test_error_returns_error_dict(self):
        result = _error("test_tool", ValueError("test error"))
        assert isinstance(result, dict)
        assert result.get("isError") is True


class TestClientErrorClasses:
    """Test LightRAG client error hierarchy."""

    def test_lightrag_error_base(self):
        err = LightRAGError("test message")
        assert str(err) == "test message"

    def test_lightrag_connection_error(self):
        err = LightRAGConnectionError("connection failed")
        assert isinstance(err, LightRAGError)

    def test_lightrag_validation_error(self):
        err = LightRAGValidationError("validation failed")
        assert isinstance(err, LightRAGError)

    def test_lightrag_api_error(self):
        err = LightRAGAPIError("api error", status_code=500)
        assert isinstance(err, LightRAGError)
        assert err.status_code == 500
