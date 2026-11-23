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


class TestEntityResolution:
    """Shared resolver: exact match first, then unique qualified suffix."""

    @staticmethod
    def _make_graph(nodes, edges=None):
        return GraphResponse(nodes=nodes, edges=edges or [])

    @staticmethod
    def _node(name, etype="METHOD", fp="Ns/Snake.cs", **extra_props):
        props = {"entity_type": etype, "entity_name": name, "file_path": fp}
        props.update(extra_props)
        return {"id": name, "name": name, "properties": props}

    @pytest.mark.asyncio
    async def test_exact_match_wins(self):
        from coderag_mcp.client import LightRAGClient

        client = LightRAGClient()
        graph = self._make_graph([self._node("Website.Games.Snake.Snake.Tile")])
        client.get_knowledge_graph = AsyncMock(return_value=graph)

        resp = await client.get_node_details("Website.Games.Snake.Snake.Tile")
        assert resp.entity.name == "Website.Games.Snake.Snake.Tile"

    @pytest.mark.asyncio
    async def test_unique_suffix_resolves(self):
        """Report F3: exists said yes while node_details missed the same name."""
        from coderag_mcp.client import LightRAGClient

        client = LightRAGClient()
        graph = self._make_graph(
            [self._node("Website.Games.Snake.Snake.Tile", etype="PROPERTY")]
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)

        resp = await client.get_node_details("Snake.Tile")
        assert resp.entity.name == "Website.Games.Snake.Snake.Tile"

    @pytest.mark.asyncio
    async def test_ambiguous_suffix_raises(self):
        from coderag_mcp.client import LightRAGAPIError, LightRAGClient

        client = LightRAGClient()
        graph = self._make_graph(
            [
                self._node("A.Mod.Snake.Tile"),
                self._node("B.Other.Snake.Tile"),
            ]
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)

        with pytest.raises(LightRAGAPIError):
            await client.get_node_details("Snake.Tile")


class TestNeighborLimits:
    """Output guards on node_details(view='neighbors') for hub nodes."""

    @pytest.mark.asyncio
    async def test_neighbors_capped_with_true_totals(self):
        from coderag_mcp.client import LightRAGClient

        hub = {
            "id": "Hub",
            "name": "Hub",
            "properties": {
                "entity_type": "CLASS",
                "entity_name": "Hub",
                "file_path": "h.cs",
            },
        }
        nodes = [hub]
        edges = []
        for i in range(80):
            child = f"Child{i}"
            nodes.append(
                {
                    "id": child,
                    "name": child,
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": child,
                        "file_path": "c.cs",
                    },
                }
            )
            edges.append(
                {
                    "source": "Hub",
                    "target": child,
                    "properties": {"keywords": "inheritance, extends"},
                }
            )

        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(
            return_value=GraphResponse(nodes=nodes, edges=edges)
        )

        resp = await client.get_entity_neighbors("Hub", limit=10)
        assert len(resp.outgoing_relations) == 10
        assert resp.total_outgoing == 80
        assert resp.total_incoming == 0
        assert resp.truncated is True
        assert resp.limit == 10


class TestGetNodesFileMatch:
    """file_match='exact' avoids substring false positives (Snake~Snake1)."""

    @staticmethod
    def _graph():
        return GraphResponse(
            nodes=[
                {
                    "id": "n1",
                    "name": "n1",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Snake",
                        "file_path": "Games/Snake.cs",
                    },
                },
                {
                    "id": "n2",
                    "name": "n2",
                    "properties": {
                        "entity_type": "CLASS",
                        "entity_name": "Snake1",
                        "file_path": "Monsters/Snake1.cs",
                    },
                },
            ],
            edges=[],
        )

    @pytest.mark.asyncio
    async def test_contains_default_matches_related(self):
        from coderag_mcp.client import LightRAGClient

        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(return_value=self._graph())
        resp = await client.get_nodes(file_path="Snake")
        assert {e.name for e in resp.entities} == {"Snake", "Snake1"}

    @pytest.mark.asyncio
    async def test_exact_matches_only_requested_file(self):
        from coderag_mcp.client import LightRAGClient

        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(return_value=self._graph())
        resp = await client.get_nodes(file_path="games/snake.cs", file_match="exact")
        assert [e.name for e in resp.entities] == ["Snake"]

    @pytest.mark.asyncio
    async def test_invalid_file_match_rejected(self):
        from coderag_mcp.client import (
            LightRAGClient,
            LightRAGValidationError,
        )

        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(return_value=self._graph())
        with pytest.raises(LightRAGValidationError):
            await client.get_nodes(file_path="x", file_match="regex")


class TestSpanLifting:
    """start_line/end_line/canonical graph properties surface as fields."""

    @pytest.mark.asyncio
    async def test_spans_exposed_on_entities(self):
        from coderag_mcp.client import LightRAGClient

        graph = GraphResponse(
            nodes=[
                {
                    "id": "Ns.C.Run",
                    "name": "Ns.C.Run",
                    "properties": {
                        "entity_type": "METHOD",
                        "entity_name": "Ns.C.Run",
                        "file_path": "c.cs",
                        "start_line": 12,
                        "end_line": 48,
                        "canonical": False,
                    },
                }
            ],
            edges=[],
        )
        client = LightRAGClient()
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.get_nodes(entity_type="method")
        e = resp.entities[0]
        assert e.start_line == 12
        assert e.end_line == 48
        assert e.canonical is False


class TestGraphLanguage:
    """Language detection + language-aware type vocabularies."""

    def test_graph_language_detection(self):
        from coderag_mcp.client import _graph_language

        zig_nodes = [
            {"properties": {"entity_type": "FUNCTION"}},
            {"properties": {"entity_type": "STRUCT"}},
        ]
        assert _graph_language(zig_nodes) == "zig"
        csharp_nodes = [
            {"properties": {"entity_type": "CLASS"}},
            {"properties": {"entity_type": "METHOD"}},
            {"properties": {"entity_type": "ENUM"}},
        ]
        assert _graph_language(csharp_nodes) == "csharp"
        assert _graph_language([]) == "csharp"

    def test_type_vocabularies(self):
        from coderag_mcp.client import _member_types, _top_level_types

        zig_top = _top_level_types("zig")
        assert "UNION" in zig_top and "OPAQUE" in zig_top and "ERROR_SET" in zig_top
        assert "CLASS" not in zig_top
        zig_members = _member_types("zig")
        assert "FUNCTION" in zig_members and "CONST" in zig_members
        assert "METHOD" not in zig_members

    @pytest.mark.asyncio
    async def test_zig_dead_code(self):
        # A Zig function with no `[pub]` suffix is private by construction
        # and should be reported as dead when unreferenced + unexported.
        client = LightRAGClient()
        graph = GraphResponse(
            nodes=[
                {
                    "id": "AtomSim.internal",
                    "name": "AtomSim.internal",
                    "properties": {
                        "entity_type": "FUNCTION",
                        "entity_name": "AtomSim.internal",
                        "file_path": "src/atom_sim.zig",
                        "description": "fn internal",
                    },
                },
                {
                    "id": "AtomSim.pub",
                    "name": "AtomSim.pub",
                    "properties": {
                        "entity_type": "FUNCTION",
                        "entity_name": "AtomSim.pub",
                        "file_path": "src/atom_sim.zig",
                        "description": "fn pub [pub]",
                    },
                },
                {
                    "id": "atom_sim:EXPORTS",
                    "name": "atom_sim:EXPORTS",
                    "properties": {
                        "entity_type": "EXPORTS",
                        "file_path": "src/atom_sim.zig",
                        "description": "AtomSim.pub [MTIME=1700000000]",
                    },
                },
            ],
            edges=[],
        )
        client.get_knowledge_graph = AsyncMock(return_value=graph)
        resp = await client.find_dead_code()
        names = {e.name for e in resp.entities}
        # private (no [pub]) + not exported -> dead
        assert "AtomSim.internal" in names
        # pub + exported -> alive
        assert "AtomSim.pub" not in names
