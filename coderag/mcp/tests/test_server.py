"""
Unit tests for MCP server functionality.
"""

import pytest
from unittest.mock import AsyncMock, patch, MagicMock

from coderag_mcp.client import (
    LightRAGError,
    LightRAGConnectionError,
    LightRAGValidationError,
    LightRAGAPIError,
)
from coderag_mcp import server as server_module


class TestServerModuleImports:
    """Test that server module exports expected functions."""

    def test_query_function_exists(self):
        assert hasattr(server_module, "query")
        assert callable(server_module.query)

    def test_get_related_files_function_exists(self):
        assert hasattr(server_module, "get_related_files")
        assert callable(server_module.get_related_files)

    def test_check_entity_exists_function_exists(self):
        assert hasattr(server_module, "check_entity_exists")
        assert callable(server_module.check_entity_exists)

    def test_get_knowledge_graph_function_exists(self):
        assert hasattr(server_module, "get_knowledge_graph")
        assert callable(server_module.get_knowledge_graph)

    def test_get_graph_labels_function_exists(self):
        assert hasattr(server_module, "get_graph_labels")
        assert callable(server_module.get_graph_labels)

    def test_mcp_server_exists(self):
        assert hasattr(server_module, "mcp_server")
        assert server_module.mcp_server is not None

    def test_serialize_function_exists(self):
        assert hasattr(server_module, "_serialize")
        assert callable(server_module._serialize)

    def test_error_function_exists(self):
        assert hasattr(server_module, "_error")
        assert callable(server_module._error)


class TestSerialize:
    """Test _serialize helper."""

    def test_serialize_pydantic_model(self):
        from pydantic import BaseModel

        class DummyModel(BaseModel):
            key: str

        result = server_module._serialize(DummyModel(key="value"))
        assert result == {"key": "value"}

    def test_serialize_list(self):
        result = server_module._serialize([1, 2, 3])
        assert result == {"results": [1, 2, 3]}

    def test_serialize_str(self):
        result = server_module._serialize("hello")
        assert result == {"result": "hello"}


class TestErrorHelper:
    """Test _error helper."""

    def test_error_returns_dict(self):
        result = server_module._error("test_tool", ValueError("test error"))
        assert isinstance(result, dict)
        assert result.get("isError") is True


class TestServerToolNames:
    """Test that all expected tools are registered on mcp_server."""

    @pytest.mark.asyncio
    async def test_tool_count(self):
        tools = await server_module.mcp_server.list_tools()
        assert len(tools) > 0

    @pytest.mark.asyncio
    async def test_expected_tools_present(self):
        tools = await server_module.mcp_server.list_tools()
        tool_names = {t.name for t in tools}
        expected = [
            "query", "get_related_files", "check_entity_exists",
            "get_knowledge_graph", "get_graph_labels", "add_note_to_entity",
            "query_data", "get_codebase_stats", "get_entity_neighbors",
            "get_entity_details", "list_entities_by_type", "get_entities_in_file",
            "search_entities", "get_relation_detail", "get_method_signature",
            "find_related_files", "get_interface_methods",
        ]
        for name in expected:
            assert name in tool_names, f"Tool {name} not found in registered tools"


class TestClientErrorClasses:
    """Test LightRAG client error hierarchy."""

    def test_lightrag_error_base(self):
        err = LightRAGError("test")
        assert str(err) == "test"

    def test_lightrag_connection_error(self):
        err = LightRAGConnectionError("connection failed")
        assert isinstance(err, LightRAGError)

    def test_lightrag_validation_error(self):
        err = LightRAGValidationError("validation failed")
        assert isinstance(err, LightRAGError)

    def test_lightrag_api_error(self):
        err = LightRAGAPIError("api failed", status_code=400)
        assert isinstance(err, LightRAGError)