"""
Unit tests for MCP server functionality.
"""

import asyncio
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from coderag_mcp import server as server_module
from coderag_mcp.client import (
    LightRAGAPIError,
    LightRAGConnectionError,
    LightRAGError,
    LightRAGValidationError,
)


class TestServerModuleImports:
    """Test that server module exports expected functions."""

    def test_query_function_exists(self):
        """`query` is CLI-only but still exists as a callable function."""
        assert hasattr(server_module, "query")
        assert callable(server_module.query)

    def test_get_related_files_function_exists(self):
        assert hasattr(server_module, "get_related_files")
        assert callable(server_module.get_related_files)

    def test_get_file_dependencies_function_exists(self):
        assert hasattr(server_module, "get_file_dependencies")
        assert callable(server_module.get_file_dependencies)

    def test_file_dependency_cycles_function_exists(self):
        assert hasattr(server_module, "file_dependency_cycles")
        assert callable(server_module.file_dependency_cycles)

    def test_get_file_stats_function_exists(self):
        assert hasattr(server_module, "get_file_stats")
        assert callable(server_module.get_file_stats)

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
    """Test that mcp_server has tools registered (no specific name assertions).

    Per repo convention: the tool surface is intentionally fluid — what is
    exposed to agents is a curatorial decision, not a test contract.
    """

    @pytest.mark.asyncio
    async def test_tool_count_is_positive(self):
        tools = await server_module.mcp_server.list_tools()
        assert len(tools) > 0

    @pytest.mark.asyncio
    async def test_dead_code_tool_registered(self):
        tools = await server_module.mcp_server.list_tools()
        names = {t.name for t in tools}
        assert "dead_code" in names


class TestQueryCodebaseTool:
    """`query_codebase` (MCP name) is LLM-free vector search via /query/data.

    The tool must default to mode="mix" so the query is matched against ALL
    vector stores (entities, relationships, chunks); no LLM-backed tool may be
    registered.
    """

    def test_default_mode_is_mix(self):
        import inspect

        sig = inspect.signature(server_module.query_data)
        assert sig.parameters["mode"].default == "mix"

    def test_tool_docstring_documents_vector_search(self):
        tools_list = None

        async def _get():
            return await server_module.mcp_server.list_tools()

        tools_list = asyncio.run(_get())
        by_name = {t.name: t for t in tools_list}
        assert "no llm" in by_name["query_codebase"].description.lower()
        assert "vector" in by_name["query_codebase"].description.lower()

    @pytest.mark.asyncio
    async def test_delegates_to_client_query_data_with_mode(self):
        mock_client = MagicMock()
        mock_client.query_data = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {"status": "success", "data": {}, "metadata": {}}
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.query_data("how does auth work", mode="mix")
        mock_client.query_data.assert_awaited_once_with(
            "how does auth work", mode="mix"
        )
        assert result["status"] == "success"

    @pytest.mark.asyncio
    async def test_llm_backed_query_tool_not_registered(self):
        """The LLM natural-language answer tool stays reserved for the CLI."""
        tools = await server_module.mcp_server.list_tools()
        names = {t.name for t in tools}
        assert "query" not in names
        assert "get_related_files" not in names


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
