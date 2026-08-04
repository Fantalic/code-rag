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
