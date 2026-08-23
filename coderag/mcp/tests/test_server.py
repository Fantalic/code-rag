"""
Unit tests for MCP server functionality.
"""

import asyncio
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from coderag_mcp import server as server_module
from coderag_mcp.client import (
    LightRAGAPIError,
    LightRAGClient,
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

    The tool takes only a query — it always searches ALL vector stores
    (mode "mix": entities, relationships, chunks); no LLM-backed tool may be
    registered.
    """

    def test_tool_has_no_mode_parameter(self):
        """Agents get no mode knob: search always covers all vector stores."""
        import inspect

        sig = inspect.signature(server_module.query_data)
        assert "mode" not in sig.parameters
        assert list(sig.parameters) == ["query"]

    def test_tool_docstring_documents_vector_search(self):
        tools_list = None

        async def _get():
            return await server_module.mcp_server.list_tools()

        tools_list = asyncio.run(_get())
        by_name = {t.name: t for t in tools_list}
        assert "no llm" in by_name["query_codebase"].description.lower()
        assert "vector" in by_name["query_codebase"].description.lower()

    @pytest.mark.asyncio
    async def test_delegates_to_client_query_data_with_mix_mode(self):
        mock_client = MagicMock()
        mock_client.query_data = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {"status": "success", "data": {}, "metadata": {}}
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.query_data("how does auth work")
        mock_client.query_data.assert_awaited_once_with("how does auth work")
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


class TestListNotesDirection:
    """Report F8: attach creates annotated --HAS_NOTE--> note; get_notes must
    accept that direction (and legacy note --HAS_NOTE--> annotated)."""

    @staticmethod
    def _graph(src, tgt):
        from coderag_mcp.client import GraphResponse

        return GraphResponse(
            nodes=[
                {
                    "id": "Ns.Foo",
                    "name": "Ns.Foo",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Ns.Foo",
                        "file_path": "f.cs",
                    },
                },
                {
                    "id": "note:123:abc",
                    "name": "note:123:abc",
                    "properties": {
                        "entity_type": "NOTE",
                        "entity_name": "note:123:abc",
                        "description": "[agent annotation @ 2026-08-23T00:00:00Z] hi",
                    },
                },
            ],
            edges=[
                {
                    "source": src,
                    "target": tgt,
                    "properties": {"keywords": "has_note,annotation"},
                }
            ],
        )

    @pytest.mark.asyncio
    async def test_lists_notes_for_annotated_node(self):
        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(
            return_value=self._graph("Ns.Foo", "note:123:abc")
        )
        notes = await server_module._list_notes_for_node(client, "Ns.Foo")
        assert len(notes) == 1
        assert notes[0]["note_entity_name"] == "note:123:abc"

    @pytest.mark.asyncio
    async def test_lists_notes_legacy_direction(self):
        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(
            return_value=self._graph("note:123:abc", "Ns.Foo")
        )
        notes = await server_module._list_notes_for_node(client, "Ns.Foo")
        assert len(notes) == 1

    @pytest.mark.asyncio
    async def test_no_notes_for_unrelated_node(self):
        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(
            return_value=self._graph("Ns.Foo", "note:123:abc")
        )
        notes = await server_module._list_notes_for_node(client, "Other.Node")
        assert notes == []
