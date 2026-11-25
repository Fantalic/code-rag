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
        assert list(sig.parameters) == ["query", "page", "per_page"]

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


class TestPaginationDeadCode:
    """Test pagination for the dead_code tool."""

    @pytest.mark.asyncio
    async def test_default_page_returns_100(self):
        from pydantic import BaseModel

        class Entity(BaseModel):
            name: str
            type: str
            file_path: str
            line_no: int
            visibility: str = ""
            reason: str = ""
            in_exports: bool = False

        class Resp(BaseModel):
            entities: list[Entity]
            total: int
            filters: dict = {}

        entities = [
            Entity(
                name=f"DeadEntity{i}",
                type="METHOD",
                file_path=f"file_{i // 10}.cs",
                line_no=i * 10,
                visibility="private",
                reason="private",
            )
            for i in range(250)
        ]
        mock_client = MagicMock()
        mock_client.find_dead_code = AsyncMock(
            return_value=Resp(entities=entities, total=250, filters={})
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.find_dead_code()
        assert len(result["entities"]) == 100
        assert result["total"] == 250
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 250
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2

    @pytest.mark.asyncio
    async def test_page_3_returns_50(self):
        from pydantic import BaseModel

        class Entity(BaseModel):
            name: str
            type: str
            file_path: str
            line_no: int
            visibility: str = ""
            reason: str = ""
            in_exports: bool = False

        class Resp(BaseModel):
            entities: list[Entity]
            total: int
            filters: dict = {}

        entities = [
            Entity(
                name=f"DeadEntity{i}",
                type="METHOD",
                file_path=f"file_{i // 10}.cs",
                line_no=i * 10,
                visibility="private",
                reason="private",
            )
            for i in range(250)
        ]
        mock_client = MagicMock()
        mock_client.find_dead_code = AsyncMock(
            return_value=Resp(entities=entities, total=250, filters={})
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.find_dead_code(page=3)
        assert len(result["entities"]) == 50
        assert result["total"] == 250
        assert result["pagination"]["page"] == 3
        assert result["pagination"]["has_more"] is False
        assert result["pagination"]["next_page"] is None

    @pytest.mark.asyncio
    async def test_invalid_page_returns_error(self):
        mock_client = MagicMock()
        mock_client.find_dead_code = AsyncMock(
            return_value=MagicMock(entities=[], total=0, filters={})
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.find_dead_code(page=0)
        assert result.get("isError") is True

    @pytest.mark.asyncio
    async def test_invalid_per_page_returns_error(self):
        mock_client = MagicMock()
        mock_client.find_dead_code = AsyncMock(
            return_value=MagicMock(entities=[], total=0, filters={})
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.find_dead_code(per_page=5000)
        assert result.get("isError") is True


class TestPaginationWhatChanged:
    """Test pagination for the what_changed tool."""

    @pytest.mark.asyncio
    async def test_default_page_returns_100(self):
        mock_client = MagicMock()
        mock_client.get_knowledge_graph = AsyncMock(return_value=MagicMock(nodes=[]))
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.what_changed()
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 0

    @pytest.mark.asyncio
    async def test_150_changes_returns_100_on_page_1(self):
        import time

        mock_client = MagicMock()
        changelog_nodes = []
        base_time = time.time()
        for i in range(150):
            node = MagicMock()
            node.get.return_value = {}
            node.get.side_effect = lambda key, default=None: {
                "properties": {
                    "entity_type": "CHANGELOG",
                    "description": f'{{"changed_at": "{base_time - i * 60}"}}',
                }
            }.get(key, default)
            changelog_nodes.append(node)
        mock_client.get_knowledge_graph = AsyncMock(
            return_value=MagicMock(nodes=changelog_nodes)
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.what_changed()
        assert len(result["changes"]) == 100
        assert result["total"] == 150
        assert result["pagination"]["total"] == 150
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2

    @pytest.mark.asyncio
    async def test_150_changes_page_2_returns_50(self):
        import time

        mock_client = MagicMock()
        changelog_nodes = []
        base_time = time.time()
        for i in range(150):
            node = MagicMock()
            node.get.return_value = {}
            node.get.side_effect = lambda key, default=None: {
                "properties": {
                    "entity_type": "CHANGELOG",
                    "description": f'{{"changed_at": "{base_time - i * 60}"}}',
                }
            }.get(key, default)
            changelog_nodes.append(node)
        mock_client.get_knowledge_graph = AsyncMock(
            return_value=MagicMock(nodes=changelog_nodes)
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.what_changed(page=2)
        assert len(result["changes"]) == 50
        assert result["total"] == 150
        assert result["pagination"]["has_more"] is False
        assert result["pagination"]["next_page"] is None

    @pytest.mark.asyncio
    async def test_invalid_page_returns_error(self):
        mock_client = MagicMock()
        mock_client.get_knowledge_graph = AsyncMock(return_value=MagicMock(nodes=[]))
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.what_changed(page=0)
        assert result.get("isError") is True


class TestPaginationCycles:
    """Test pagination for the detect_circular_dependencies tool."""

    @pytest.mark.asyncio
    async def test_120_cycles_returns_100(self):
        from coderag_mcp.models import FileCyclesResponse

        cycles = [[f"file_{i}.cs", f"file_{(i + 1) % 120}.cs"] for i in range(120)]
        mock_client = MagicMock()
        mock_client.file_dependency_cycles = AsyncMock(
            return_value=FileCyclesResponse(cycles=cycles, count=120)
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.file_dependency_cycles()
        assert len(result["cycles"]) == 100
        assert result["count"] == 120
        assert result["pagination"]["total"] == 120
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2

    @pytest.mark.asyncio
    async def test_page_2_returns_20(self):
        from coderag_mcp.models import FileCyclesResponse

        cycles = [[f"file_{i}.cs", f"file_{(i + 1) % 120}.cs"] for i in range(120)]
        mock_client = MagicMock()
        mock_client.file_dependency_cycles = AsyncMock(
            return_value=FileCyclesResponse(cycles=cycles, count=120)
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.file_dependency_cycles(page=2)
        assert len(result["cycles"]) == 20
        assert result["count"] == 120
        assert result["pagination"]["has_more"] is False
        assert result["pagination"]["next_page"] is None

    @pytest.mark.asyncio
    async def test_invalid_per_page_returns_error(self):
        from coderag_mcp.models import FileCyclesResponse

        mock_client = MagicMock()
        mock_client.file_dependency_cycles = AsyncMock(
            return_value=FileCyclesResponse(cycles=[], count=0)
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.file_dependency_cycles(per_page=5000)
        assert result.get("isError") is True


class TestPaginationFileDependencies:
    """Test pagination for the file_dependencies tool."""

    @pytest.mark.asyncio
    async def test_130_entities_returns_100(self):
        from coderag_mcp.models import FileDependenciesResponse

        entities = [f"Entity_{i}" for i in range(130)]
        mock_client = MagicMock()
        mock_client.get_file_dependencies = AsyncMock(
            return_value=FileDependenciesResponse(
                file_path="test.cs",
                references=[f"ref_{i}" for i in range(50)],
                dependents=[f"dep_{i}" for i in range(50)],
                entities=entities,
                not_found=False,
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_file_dependencies("test.cs")
        assert len(result["entities"]) == 100
        assert result["pagination"]["total"] == 130
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2
        assert result["file_path"] == "test.cs"
        assert result["not_found"] is False
        assert len(result["references"]) == 50
        assert len(result["dependents"]) == 50

    @pytest.mark.asyncio
    async def test_page_2_returns_30(self):
        from coderag_mcp.models import FileDependenciesResponse

        entities = [f"Entity_{i}" for i in range(130)]
        mock_client = MagicMock()
        mock_client.get_file_dependencies = AsyncMock(
            return_value=FileDependenciesResponse(
                file_path="test.cs",
                references=[f"ref_{i}" for i in range(50)],
                dependents=[f"dep_{i}" for i in range(50)],
                entities=entities,
                not_found=False,
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_file_dependencies("test.cs", page=2)
        assert len(result["entities"]) == 30
        assert result["pagination"]["page"] == 2
        assert result["pagination"]["has_more"] is False
        assert result["pagination"]["next_page"] is None
        assert result["file_path"] == "test.cs"
        assert result["not_found"] is False

    @pytest.mark.asyncio
    async def test_not_found_preserved(self):
        from coderag_mcp.models import FileDependenciesResponse

        mock_client = MagicMock()
        mock_client.get_file_dependencies = AsyncMock(
            return_value=FileDependenciesResponse(
                file_path="missing.cs", not_found=True
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_file_dependencies("missing.cs")
        assert result["not_found"] is True
        assert result["file_path"] == "missing.cs"

    @pytest.mark.asyncio
    async def test_invalid_page_returns_error(self):
        from coderag_mcp.models import FileDependenciesResponse

        mock_client = MagicMock()
        mock_client.get_file_dependencies = AsyncMock(
            return_value=FileDependenciesResponse(file_path="test.cs")
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_file_dependencies("test.cs", page=0)
        assert result.get("isError") is True


class TestPaginationGetNodes:
    """Test page/per_page pagination for the get_nodes tool."""

    @pytest.mark.asyncio
    async def test_default_page_returns_100_with_pagination(self):
        from pydantic import BaseModel

        class Entity(BaseModel):
            entity_type: str
            entity_name: str
            file_path: str
            start_line: int = 0
            end_line: int = 0
            description: str = ""

        all_entities = [
            Entity(
                entity_type="CLASS", entity_name=f"Entity{i}", file_path=f"f{i//10}.cs"
            )
            for i in range(250)
        ]

        def _get_nodes_side_effect(*args, **kwargs):
            offset = kwargs.get("offset", 0)
            limit = kwargs.get("limit", 100)
            sliced = all_entities[offset : offset + limit]
            return MagicMock(
                model_dump=lambda: {
                    "entities": sliced,
                    "total": 250,
                    "returned": len(sliced),
                    "filters": {},
                }
            )

        mock_client = MagicMock()
        mock_client.get_nodes = AsyncMock(side_effect=_get_nodes_side_effect)
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_nodes()
        assert len(result["entities"]) == 100
        assert result["total"] == 250
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 250
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2

    @pytest.mark.asyncio
    async def test_page_2_per_page_50_derives_offset(self):
        from pydantic import BaseModel

        class Entity(BaseModel):
            entity_type: str
            entity_name: str
            file_path: str
            start_line: int = 0
            end_line: int = 0
            description: str = ""

        mock_client = MagicMock()
        entities = [
            Entity(
                entity_type="CLASS", entity_name=f"Entity{i}", file_path=f"f{i//10}.cs"
            )
            for i in range(250)
        ]
        mock_client.get_nodes = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entities": entities,
                    "total": 250,
                    "returned": 250,
                    "filters": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_nodes(page=2, per_page=50)
        assert result["pagination"]["page"] == 2
        assert result["pagination"]["per_page"] == 50
        mock_client.get_nodes.assert_awaited_once()
        call_kwargs = mock_client.get_nodes.call_args.kwargs
        assert call_kwargs["offset"] == 50
        assert call_kwargs["limit"] == 50

    @pytest.mark.asyncio
    async def test_legacy_limit_offset_still_works(self):
        from pydantic import BaseModel

        class Entity(BaseModel):
            entity_type: str
            entity_name: str
            file_path: str
            start_line: int = 0
            end_line: int = 0
            description: str = ""

        mock_client = MagicMock()
        entities = [
            Entity(
                entity_type="CLASS", entity_name=f"Entity{i}", file_path=f"f{i//10}.cs"
            )
            for i in range(250)
        ]
        mock_client.get_nodes = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entities": entities,
                    "total": 250,
                    "returned": 250,
                    "filters": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_nodes(limit=25, offset=50)
        assert result["pagination"]["per_page"] == 25
        assert result["pagination"]["page"] == 3
        mock_client.get_nodes.assert_awaited_once()
        call_kwargs = mock_client.get_nodes.call_args.kwargs
        assert call_kwargs["offset"] == 50
        assert call_kwargs["limit"] == 25

    @pytest.mark.asyncio
    async def test_invalid_page_returns_error(self):
        mock_client = MagicMock()
        mock_client.get_nodes = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entities": [],
                    "total": 0,
                    "returned": 0,
                    "filters": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_nodes(page=0)
        assert result.get("isError") is True

    @pytest.mark.asyncio
    async def test_invalid_per_page_returns_error(self):
        mock_client = MagicMock()
        mock_client.get_nodes = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entities": [],
                    "total": 0,
                    "returned": 0,
                    "filters": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_nodes(per_page=5000)
        assert result.get("isError") is True


class TestPaginationCodebaseOverview:
    """Test page/per_page pagination for the codebase_overview tool."""

    @pytest.mark.asyncio
    async def test_graph_view_defaults_to_100_per_page(self):
        from coderag_mcp.client import GraphResponse

        nodes = [
            {
                "id": f"n{i}",
                "name": f"Node{i}",
                "properties": {"entity_type": "CLASS", "entity_name": f"Node{i}"},
            }
            for i in range(250)
        ]
        mock_client = MagicMock()
        mock_client.get_knowledge_graph = AsyncMock(
            return_value=GraphResponse(nodes=nodes, edges=[])
        )
        mock_client.paginate_graph = MagicMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "mode": "graph",
                    "nodes": nodes[:100],
                    "edges": [],
                    "total_nodes": 250,
                    "total_edges": 0,
                    "offset": 0,
                    "limit": 100,
                    "truncated": True,
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_knowledge_graph(view="graph")
        assert "pagination" in result
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 250
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2
        assert result["total_nodes"] == 250
        assert result["mode"] == "graph"

    @pytest.mark.asyncio
    async def test_summary_view_includes_pagination(self):
        from pydantic import BaseModel

        class Summary(BaseModel):
            mode: str = "summary"
            max_depth: int = 1
            total_nodes: int = 0
            total_edges: int = 0
            nodes_by_type: dict = {}
            edges_by_keyword: dict = {}
            files_indexed: int = 0

        from coderag_mcp.client import GraphResponse

        nodes = [
            {
                "id": f"n{i}",
                "name": f"Node{i}",
                "properties": {"entity_type": "CLASS", "entity_name": f"Node{i}"},
            }
            for i in range(250)
        ]
        mock_client = MagicMock()
        mock_client.get_knowledge_graph = AsyncMock(
            return_value=GraphResponse(nodes=nodes, edges=[])
        )
        mock_client.summarize_graph = MagicMock(
            return_value=Summary(
                mode="summary",
                max_depth=1,
                total_nodes=250,
                total_edges=0,
                nodes_by_type={"CLASS": 250},
                edges_by_keyword={},
                files_indexed=10,
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_knowledge_graph(view="summary")
        assert "pagination" in result
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 250
        assert result["mode"] == "summary"
        assert result["total_nodes"] == 250

    @pytest.mark.asyncio
    async def test_graph_view_legacy_limit_offset(self):
        from coderag_mcp.client import GraphResponse

        nodes = [
            {
                "id": f"n{i}",
                "name": f"Node{i}",
                "properties": {"entity_type": "CLASS", "entity_name": f"Node{i}"},
            }
            for i in range(250)
        ]
        mock_client = MagicMock()
        mock_client.get_knowledge_graph = AsyncMock(
            return_value=GraphResponse(nodes=nodes, edges=[])
        )
        mock_client.paginate_graph = MagicMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "mode": "graph",
                    "nodes": nodes[50:75],
                    "edges": [],
                    "total_nodes": 250,
                    "total_edges": 0,
                    "offset": 50,
                    "limit": 25,
                    "truncated": True,
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_knowledge_graph(
                view="graph", limit=25, offset=50
            )
        assert result["pagination"]["per_page"] == 25
        assert result["pagination"]["page"] == 3
        mock_client.paginate_graph.assert_called_once()
        call_kwargs = mock_client.paginate_graph.call_args.kwargs
        assert call_kwargs["offset"] == 50
        assert call_kwargs["limit"] == 25

    @pytest.mark.asyncio
    async def test_invalid_page_returns_error(self):
        mock_client = MagicMock()
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_knowledge_graph(view="summary", page=0)
        assert result.get("isError") is True

    @pytest.mark.asyncio
    async def test_invalid_per_page_returns_error(self):
        mock_client = MagicMock()
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_knowledge_graph(
                view="summary", per_page=5000
            )
        assert result.get("isError") is True


class TestPaginationNodeDetails:
    """Test page/per_page pagination for the node_details tool."""

    @pytest.mark.asyncio
    async def test_details_view_default_returns_100_with_pagination(self):
        mock_client = MagicMock()
        mock_client.get_node_details = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entity": {"entity_name": "TestClass"},
                    "relations": [f"rel{i}" for i in range(250)],
                    "incoming_relations": [f"inc{i}" for i in range(250)],
                    "outgoing_relations": [f"out{i}" for i in range(250)],
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_node_details(entity_name="TestClass")
        assert len(result["relations"]) == 100
        assert len(result["incoming_relations"]) == 100
        assert len(result["outgoing_relations"]) == 100
        assert "entity" in result
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 250
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2

    @pytest.mark.asyncio
    async def test_details_view_page_2_per_page_50_returns_correct_slice(self):
        mock_client = MagicMock()
        mock_client.get_node_details = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entity": {"entity_name": "TestClass"},
                    "relations": [f"rel{i}" for i in range(250)],
                    "incoming_relations": [f"inc{i}" for i in range(250)],
                    "outgoing_relations": [f"out{i}" for i in range(250)],
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_node_details(
                entity_name="TestClass", page=2, per_page=50
            )
        assert len(result["relations"]) == 50
        assert result["relations"][0] == "rel50"
        assert result["relations"][-1] == "rel99"
        assert result["pagination"]["page"] == 2
        assert result["pagination"]["per_page"] == 50
        assert result["pagination"]["total"] == 250
        assert result["pagination"]["total_pages"] == 5
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 3

    @pytest.mark.asyncio
    async def test_neighbors_view_page_1_preserves_total_neighbors(self):
        mock_client = MagicMock()
        mock_client.get_node_details = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entity_name": "TestClass",
                    "neighbors": [f"neighbor{i}" for i in range(200)],
                    "incoming_relations": [f"inc{i}" for i in range(50)],
                    "outgoing_relations": [f"out{i}" for i in range(50)],
                    "total_neighbors": 200,
                    "total_incoming": 50,
                    "total_outgoing": 50,
                    "truncated": True,
                    "limit": 100,
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_node_details(
                entity_name="TestClass", view="neighbors"
            )
        assert len(result["neighbors"]) == 100
        assert result["total_neighbors"] == 200
        assert result["total_incoming"] == 50
        assert result["total_outgoing"] == 50
        assert result["truncated"] is True
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 200
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2

    @pytest.mark.asyncio
    async def test_neighbors_view_page_2_passes_sufficient_client_limit(self):
        mock_client = MagicMock()
        mock_client.get_node_details = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entity_name": "TestClass",
                    "neighbors": [f"neighbor{i}" for i in range(200)],
                    "incoming_relations": [f"inc{i}" for i in range(50)],
                    "outgoing_relations": [f"out{i}" for i in range(50)],
                    "total_neighbors": 200,
                    "total_incoming": 50,
                    "total_outgoing": 50,
                    "truncated": True,
                    "limit": 200,
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_node_details(
                entity_name="TestClass", view="neighbors", page=2
            )
        call_kwargs = mock_client.get_node_details.call_args.kwargs
        assert call_kwargs["limit"] == 200
        assert len(result["neighbors"]) == 100
        assert result["neighbors"][0] == "neighbor100"
        assert result["total_neighbors"] == 200
        assert result["pagination"]["page"] == 2
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 200
        assert result["pagination"]["total_pages"] == 2
        assert result["pagination"]["has_more"] is False
        assert result["pagination"]["next_page"] is None

    @pytest.mark.asyncio
    async def test_legacy_limit_25_still_works_with_pagination(self):
        mock_client = MagicMock()
        mock_client.get_node_details = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "entity": {"entity_name": "TestClass"},
                    "relations": [f"rel{i}" for i in range(250)],
                    "incoming_relations": [f"inc{i}" for i in range(250)],
                    "outgoing_relations": [f"out{i}" for i in range(250)],
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_node_details(
                entity_name="TestClass", limit=25
            )
        assert len(result["relations"]) == 25
        assert result["pagination"]["per_page"] == 25
        assert result["pagination"]["page"] == 1
        call_kwargs = mock_client.get_node_details.call_args.kwargs
        assert call_kwargs["limit"] == 25

    @pytest.mark.asyncio
    async def test_invalid_page_returns_error(self):
        mock_client = MagicMock()
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_node_details(
                entity_name="TestClass", page=0
            )
        assert result.get("isError") is True

    @pytest.mark.asyncio
    async def test_invalid_per_page_returns_error(self):
        mock_client = MagicMock()
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.get_node_details(
                entity_name="TestClass", per_page=5000
            )
        assert result.get("isError") is True


class TestPaginationQueryCodebase:
    """Test page/per_page pagination for the query_codebase tool."""

    @pytest.mark.asyncio
    async def test_250_entities_default_returns_100(self):
        mock_client = MagicMock()
        entities = [{"entity_name": f"Entity{i}", "type": "CLASS"} for i in range(250)]
        relationships = [{"source": f"S{i}", "target": f"T{i}"} for i in range(250)]
        chunks = [{"content": f"chunk{i}"} for i in range(250)]
        references = [{"name": f"ref{i}"} for i in range(250)]
        mock_client.query_data = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "status": "success",
                    "message": "ok",
                    "data": {
                        "entities": entities,
                        "relationships": relationships,
                        "chunks": chunks,
                        "references": references,
                    },
                    "metadata": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.query_data("how does auth work")
        assert len(result["data"]["entities"]) == 100
        assert len(result["data"]["relationships"]) == 100
        assert len(result["data"]["chunks"]) == 100
        assert len(result["data"]["references"]) == 100
        assert result["status"] == "success"
        assert result["pagination"]["page"] == 1
        assert result["pagination"]["per_page"] == 100
        assert result["pagination"]["total"] == 250
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 2

    @pytest.mark.asyncio
    async def test_page_3_per_page_50(self):
        mock_client = MagicMock()
        entities = [{"entity_name": f"Entity{i}", "type": "CLASS"} for i in range(250)]
        relationships = [{"source": f"S{i}", "target": f"T{i}"} for i in range(250)]
        chunks = [{"content": f"chunk{i}"} for i in range(250)]
        references = [{"name": f"ref{i}"} for i in range(250)]
        mock_client.query_data = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "status": "success",
                    "data": {
                        "entities": entities,
                        "relationships": relationships,
                        "chunks": chunks,
                        "references": references,
                    },
                    "metadata": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.query_data(
                "how does auth work", page=3, per_page=50
            )
        assert len(result["data"]["entities"]) == 50
        assert result["data"]["entities"][0]["entity_name"] == "Entity100"
        assert result["pagination"]["page"] == 3
        assert result["pagination"]["per_page"] == 50
        assert result["pagination"]["total"] == 250
        assert result["pagination"]["total_pages"] == 5
        assert result["pagination"]["has_more"] is True
        assert result["pagination"]["next_page"] == 4

    @pytest.mark.asyncio
    async def test_only_chunks_no_entities_key_works(self):
        mock_client = MagicMock()
        chunks = [{"content": f"chunk{i}"} for i in range(50)]
        mock_client.query_data = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "status": "success",
                    "data": {
                        "chunks": chunks,
                    },
                    "metadata": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.query_data("prose search")
        assert len(result["data"]["chunks"]) == 50
        assert result["pagination"]["total"] == 0
        assert "entities" not in result["data"] or result["data"]["entities"] == []
        assert "relationships" not in result["data"] or (
            "relationships" in result["data"]
            and isinstance(result["data"]["relationships"], list)
        )

    @pytest.mark.asyncio
    async def test_invalid_page_returns_error(self):
        mock_client = MagicMock()
        mock_client.query_data = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "status": "success",
                    "data": {"entities": [], "relationships": []},
                    "metadata": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.query_data("search", page=0)
        assert result.get("isError") is True

    @pytest.mark.asyncio
    async def test_invalid_per_page_returns_error(self):
        mock_client = MagicMock()
        mock_client.query_data = AsyncMock(
            return_value=MagicMock(
                model_dump=lambda: {
                    "status": "success",
                    "data": {"entities": [], "relationships": []},
                    "metadata": {},
                }
            )
        )
        with patch.object(
            server_module, "_get_lightrag_client", return_value=mock_client
        ):
            result = await server_module.query_data("search", per_page=5000)
        assert result.get("isError") is True
