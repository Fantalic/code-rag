"""
Unit tests for LightRAG client functionality.
"""

from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from coderag_mcp.client import (
    LightRAGAPIError,
    LightRAGAuthError,
    LightRAGClient,
    LightRAGConnectionError,
    LightRAGError,
    LightRAGServerError,
    LightRAGTimeoutError,
    LightRAGValidationError,
)
from coderag_mcp.models import GraphResponse


class TestLightRAGClientInitialization:
    """Test LightRAG client initialization."""

    def test_client_initialization_default(self):
        client = LightRAGClient()
        assert client.base_url == "http://localhost:9621"
        assert client.api_key is None
        assert client.timeout == 30.0
        assert client.client is not None

    def test_client_initialization_custom(self):
        client = LightRAGClient(
            base_url="http://custom:8080",
            api_key="test_key",
            timeout=60.0,
        )
        assert client.base_url == "http://custom:8080"
        assert client.api_key == "test_key"
        assert client.timeout == 60.0

    async def test_client_context_manager(self):
        async with LightRAGClient() as client:
            assert client.client is not None


class TestErrorClasses:
    """Test LightRAG client error hierarchy."""

    def test_lightrag_error_base(self):
        err = LightRAGError("test message")
        assert str(err) == "test message"

    def test_lightrag_connection_error(self):
        err = LightRAGConnectionError("connection failed")
        assert isinstance(err, LightRAGError)

    def test_lightrag_auth_error(self):
        err = LightRAGAuthError("auth failed")
        assert isinstance(err, LightRAGError)

    def test_lightrag_validation_error(self):
        err = LightRAGValidationError("validation failed")
        assert isinstance(err, LightRAGError)

    def test_lightrag_api_error(self):
        err = LightRAGAPIError("api error", status_code=500)
        assert isinstance(err, LightRAGError)
        assert err.status_code == 500

    def test_lightrag_timeout_error(self):
        err = LightRAGTimeoutError("request timed out")
        assert isinstance(err, LightRAGError)

    def test_lightrag_server_error(self):
        err = LightRAGServerError("server error", status_code=503)
        assert isinstance(err, LightRAGError)


class TestClientMethodsExist:
    """Test that all expected client methods exist."""

    def test_query_text_exists(self):
        assert hasattr(LightRAGClient, "query_text")
        assert callable(getattr(LightRAGClient, "query_text"))

    def test_get_knowledge_graph_exists(self):
        assert hasattr(LightRAGClient, "get_knowledge_graph")

    def test_get_graph_labels_exists(self):
        assert hasattr(LightRAGClient, "get_graph_labels")

    def test_check_entity_exists_exists(self):
        assert hasattr(LightRAGClient, "check_entity_exists")

    def test_create_entity_exists(self):
        assert hasattr(LightRAGClient, "create_entity")

    def test_update_entity_exists(self):
        assert hasattr(LightRAGClient, "update_entity")

    def test_insert_text_exists(self):
        assert hasattr(LightRAGClient, "insert_text")

    def test_update_relations_exists(self):
        assert hasattr(LightRAGClient, "update_relations")

    def test_query_data_exists(self):
        assert hasattr(LightRAGClient, "query_data")

    def test_get_entity_neighbors_exists(self):
        assert hasattr(LightRAGClient, "get_entity_neighbors")

    def test_get_entity_details_exists(self):
        assert hasattr(LightRAGClient, "get_entity_details")

    def test_list_entities_by_type_exists(self):
        assert hasattr(LightRAGClient, "list_entities_by_type")

    def test_get_nodes_exists(self):
        assert hasattr(LightRAGClient, "get_nodes")

    def test_get_entities_in_file_exists(self):
        assert hasattr(LightRAGClient, "get_entities_in_file")

    def test_search_entities_exists(self):
        assert hasattr(LightRAGClient, "search_entities")

    def test_get_relation_detail_exists(self):
        assert hasattr(LightRAGClient, "get_relation_detail")

    def test_find_related_files_exists(self):
        assert hasattr(LightRAGClient, "find_related_files")

    def test_get_interface_methods_exists(self):
        assert hasattr(LightRAGClient, "get_interface_methods")

    def test_get_related_files_exists(self):
        assert hasattr(LightRAGClient, "get_related_files")


class TestMakeRequestErrorHandling:
    """Test _make_request error handling."""

    @pytest.mark.asyncio
    async def test_connection_error_on_connect(self):
        client = LightRAGClient()
        client.client.post = AsyncMock(
            side_effect=httpx.ConnectError("connection refused")
        )

        with pytest.raises(LightRAGConnectionError, match="Connection failed"):
            await client._make_request("POST", "/ingest")

    @pytest.mark.asyncio
    async def test_timeout_error_on_timeout(self):
        client = LightRAGClient()
        client.client.post = AsyncMock(side_effect=httpx.TimeoutException("timeout"))

        with pytest.raises(LightRAGTimeoutError, match="Request timeout"):
            await client._make_request("POST", "/ingest")

    @pytest.mark.asyncio
    async def test_http_status_error(self):
        client = LightRAGClient()

        mock_response = MagicMock()
        mock_response.status_code = 500
        mock_response.text = "server error"
        mock_response.raise_for_status = MagicMock(
            side_effect=httpx.HTTPStatusError(
                "server error", request=MagicMock(), response=mock_response
            )
        )
        client.client.post = AsyncMock(return_value=mock_response)

        with pytest.raises(LightRAGServerError):
            await client._make_request("POST", "/ingest")


class TestFindDeadCode:
    """Test the find_dead_code heuristic against mock graphs."""

    @staticmethod
    def _make_graph(nodes, edges):
        return GraphResponse(nodes=nodes, edges=edges)

    @pytest.mark.asyncio
    async def test_finds_private_method_no_incoming(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.Foo",
                    "name": "Ns.Foo",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Ns.Foo",
                        "file_path": "Ns/Foo.cs",
                        "description": "class Foo",
                    },
                },
                {
                    "id": "Ns.Foo.Unused",
                    "name": "Ns.Foo.Unused",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.Foo.Unused",
                        "file_path": "Ns/Foo.cs",
                        "line_no": 10,
                        "description": "Ns.Foo.Unused() -> RETURNS -> void [private]",
                    },
                },
                {
                    "id": "Ns:EXPORTS",
                    "name": "Ns:EXPORTS",
                    "properties": {
                        "entity_type": "EXPORTS",
                        "file_path": "Ns/Foo.cs",
                        "description": "Ns.Foo [MTIME=1700000000]",
                    },
                },
            ],
            edges=[
                {
                    "source": "Ns.Foo",
                    "target": "Ns.Foo.Unused",
                    "properties": {"keywords": "method, defines"},
                }
            ],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)

        resp = await client.find_dead_code()
        names = {e.name for e in resp.entities}
        assert "Ns.Foo.Unused" in names
        assert resp.entities[0].reason == "private"
        assert resp.entities[0].in_exports is False
        assert resp.total == 1

    @pytest.mark.asyncio
    async def test_keeps_public_symbol_in_exports(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.Foo",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Ns.Foo",
                        "file_path": "Ns/Foo.cs",
                    },
                },
                {
                    "id": "Ns.Foo.PublicUsed",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.Foo.PublicUsed",
                        "file_path": "Ns/Foo.cs",
                        "description": "Ns.Foo.PublicUsed() -> RETURNS -> void [public]",
                    },
                },
                {
                    "id": "Ns:EXPORTS",
                    "properties": {
                        "entity_type": "EXPORTS",
                        "file_path": "Ns/Foo.cs",
                        "description": "Ns.Foo, Ns.Foo.PublicUsed",
                    },
                },
            ],
            edges=[
                {
                    "source": "Ns.Foo",
                    "target": "Ns.Foo.PublicUsed",
                    "properties": {"keywords": "method, defines"},
                }
            ],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code()
        assert resp.total == 0

    @pytest.mark.asyncio
    async def test_flags_public_symbol_not_in_exports(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.Foo",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Ns.Foo",
                        "file_path": "Ns/Foo.cs",
                    },
                },
                {
                    "id": "Ns.Foo.PublicDead",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.Foo.PublicDead",
                        "file_path": "Ns/Foo.cs",
                        "description": "Ns.Foo.PublicDead() -> RETURNS -> void [public]",
                    },
                },
                {
                    "id": "Ns:EXPORTS",
                    "properties": {
                        "entity_type": "EXPORTS",
                        "file_path": "Ns/Foo.cs",
                        "description": "Ns.Foo",
                    },
                },
            ],
            edges=[
                {
                    "source": "Ns.Foo",
                    "target": "Ns.Foo.PublicDead",
                    "properties": {"keywords": "method, defines"},
                }
            ],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code()
        assert resp.total == 1
        assert resp.entities[0].reason == "not_exported"

    @pytest.mark.asyncio
    async def test_skips_entities_with_incoming_edges(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.Foo",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Ns.Foo",
                        "file_path": "Ns/Foo.cs",
                    },
                },
                {
                    "id": "Ns.Foo.M",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.Foo.M",
                        "file_path": "Ns/Foo.cs",
                        "description": "Ns.Foo.M() -> RETURNS -> void [private]",
                    },
                },
                {
                    "id": "Ns.Bar",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Ns.Bar",
                        "file_path": "Ns/Bar.cs",
                    },
                },
            ],
            edges=[
                {
                    "source": "Ns.Foo",
                    "target": "Ns.Foo.M",
                    "properties": {"keywords": "method, defines"},
                },
                {
                    "source": "Ns.Bar",
                    "target": "Ns.Foo.M",
                    "properties": {"keywords": "calls"},
                },
            ],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code()
        assert resp.total == 0

    @pytest.mark.asyncio
    async def test_entity_type_filter(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.Foo",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Ns.Foo",
                        "file_path": "Ns/Foo.cs",
                    },
                },
                {
                    "id": "Ns.Foo.M",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.Foo.M",
                        "file_path": "Ns/Foo.cs",
                        "description": "Ns.Foo.M() [private]",
                    },
                },
                {
                    "id": "Ns:EXPORTS",
                    "properties": {
                        "entity_type": "EXPORTS",
                        "file_path": "Ns/Foo.cs",
                        "description": "",
                    },
                },
            ],
            edges=[
                {
                    "source": "Ns.Foo",
                    "target": "Ns.Foo.M",
                    "properties": {"keywords": "method, defines"},
                }
            ],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code(entity_type="CLASS")
        # Class is no-visibility + not in exports; with empty visibility
        # we exclude. So total == 0.
        assert resp.total == 0
        resp = await client.find_dead_code(entity_type="METHOD")
        assert resp.total == 1

    @pytest.mark.asyncio
    async def test_min_age_days_filter_excludes_recent_files(self):
        import time as _t

        client = LightRAGClient()
        recent_mtime = int(_t.time()) - 60  # 1 min old
        old_mtime = int(_t.time()) - 60 * 86400  # 60 days old
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.Recent",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.Recent",
                        "file_path": "Ns/Recent.cs",
                        "description": "Ns.Recent() [private]",
                    },
                },
                {
                    "id": "Ns.Old",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.Old",
                        "file_path": "Ns/Old.cs",
                        "description": "Ns.Old() [private]",
                    },
                },
                {
                    "id": "Recent:EXPORTS",
                    "properties": {
                        "entity_type": "EXPORTS",
                        "file_path": "Ns/Recent.cs",
                        "description": f" [MTIME={recent_mtime}]",
                    },
                },
                {
                    "id": "Old:EXPORTS",
                    "properties": {
                        "entity_type": "EXPORTS",
                        "file_path": "Ns/Old.cs",
                        "description": f" [MTIME={old_mtime}]",
                    },
                },
            ],
            edges=[],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code(min_age_days=30)
        names = {e.name for e in resp.entities}
        assert "Ns.Old" in names
        assert "Ns.Recent" not in names

    @pytest.mark.asyncio
    async def test_empty_graph_returns_empty(self):
        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(
            return_value=self._make_graph(nodes=[], edges=[])
        )
        resp = await client.find_dead_code()
        assert resp.total == 0
        assert resp.entities == []

    @pytest.mark.asyncio
    async def test_unknown_visibility_excluded(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.NoVis",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.NoVis",
                        "file_path": "Ns/NoVis.cs",
                        "description": "Ns.NoVis() -> RETURNS -> void",  # no [vis]
                    },
                },
            ],
            edges=[],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code()
        assert resp.total == 0

    @pytest.mark.asyncio
    async def test_skips_extension_method(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.StringExt",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.StringExt.Repeat",
                        "file_path": "Ns/StringExt.cs",
                        "description": "Ns.StringExt.Repeat(this string s, int n) -> RETURNS -> string [public]",
                    },
                },
            ],
            edges=[],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code()
        assert resp.total == 0

    @pytest.mark.asyncio
    async def test_skips_interface_member(self):
        client = LightRAGClient()
        graph = self._make_graph(
            nodes=[
                {
                    "id": "Ns.IFoo",
                    "properties": {
                        "entity_type": "INTERFACE",
                        "entity_name": "Ns.IFoo",
                        "file_path": "Ns/IFoo.cs",
                    },
                },
                {
                    "id": "Ns.IFoo.M",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.IFoo.M",
                        "file_path": "Ns/IFoo.cs",
                        "description": "Ns.IFoo.M() [public]",
                    },
                },
            ],
            edges=[],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code()
        assert resp.total == 0
