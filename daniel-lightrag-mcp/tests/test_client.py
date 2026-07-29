"""
Unit tests for LightRAG client functionality.
"""

import pytest
import json
from unittest.mock import AsyncMock, MagicMock, patch
import httpx

from daniel_lightrag_mcp.client import (
    LightRAGClient,
    LightRAGError,
    LightRAGConnectionError,
    LightRAGAuthError,
    LightRAGValidationError,
    LightRAGAPIError,
    LightRAGTimeoutError,
    LightRAGServerError
)
from daniel_lightrag_mcp.models import (
    TextDocument,
    InsertResponse,
    QueryResponse,
    DocumentsResponse,
    HealthResponse
)


class TestLightRAGClientInitialization:
    """Test LightRAG client initialization."""
    
    def test_client_initialization_default(self):
        """Test client initialization with default parameters."""
        client = LightRAGClient()
        
        assert client.base_url == "http://localhost:9621"
        assert client.api_key is None
        assert client.timeout == 30.0
        assert client.client is not None
    
    def test_client_initialization_custom(self):
        """Test client initialization with custom parameters."""
        client = LightRAGClient(
            base_url="http://custom:8080",
            api_key="test_key",
            timeout=60.0
        )
        
        assert client.base_url == "http://custom:8080"
        assert client.api_key == "test_key"
        assert client.timeout == 60.0
    
    def test_client_initialization_with_api_key(self):
        """Test client initialization with API key sets headers."""
        client = LightRAGClient(api_key="test_key")
        
        # Check that the API key is set in headers
        assert "X-API-Key" in client.client.headers
        assert client.client.headers["X-API-Key"] == "test_key"


class TestErrorMapping:
    """Test HTTP error mapping to custom exceptions."""
    
    def test_map_http_error_400(self):
        """Test mapping of 400 Bad Request."""
        client = LightRAGClient()
        error = client._map_http_error(400, "Bad request", {"detail": "Invalid input"})
        
        assert isinstance(error, LightRAGValidationError)
        assert error.status_code == 400
        assert "Bad Request" in str(error)
    
    def test_map_http_error_401(self):
        """Test mapping of 401 Unauthorized."""
        client = LightRAGClient()
        error = client._map_http_error(401, "Unauthorized")
        
        assert isinstance(error, LightRAGAuthError)
        assert error.status_code == 401
        assert "Unauthorized" in str(error)
    
    def test_map_http_error_404(self):
        """Test mapping of 404 Not Found."""
        client = LightRAGClient()
        error = client._map_http_error(404, "Not found")
        
        assert isinstance(error, LightRAGAPIError)
        assert error.status_code == 404
        assert "Not Found" in str(error)
    
    def test_map_http_error_500(self):
        """Test mapping of 500 Internal Server Error."""
        client = LightRAGClient()
        error = client._map_http_error(500, "Internal server error")
        
        assert isinstance(error, LightRAGServerError)
        assert error.status_code == 500
        assert "Server Error" in str(error)
    
    def test_map_http_error_with_json_detail(self):
        """Test error mapping with JSON detail in response."""
        client = LightRAGClient()
        response_data = {"detail": "Validation failed for field 'text'"}
        error = client._map_http_error(422, json.dumps(response_data), response_data)
        
        assert isinstance(error, LightRAGValidationError)
        assert "Validation failed for field 'text'" in str(error)


@pytest.mark.asyncio
class TestDocumentManagementMethods:
    """Test document management client methods."""
    
    async def test_insert_text_success(self, lightrag_client, mock_response, sample_insert_response):
        """Test successful text insertion."""
        # Setup mock
        response = mock_response(200, sample_insert_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.insert_text("test content", title="Test Title")
        
        # Verify
        assert isinstance(result, InsertResponse)
        assert result.id == "doc_123"
        assert result.status == "success"
        
        # Verify API call
        lightrag_client.client.post.assert_called_once()
        call_args = lightrag_client.client.post.call_args
        assert call_args[0][0] == "http://localhost:9621/documents/text"
        
        # Verify request data
        request_data = call_args[1]["json"]
        assert request_data["text"] == "test content"
        assert request_data["file_source"] == "Test Title.txt"
    
    async def test_insert_text_empty_content(self, lightrag_client, mock_response, sample_insert_response):
        """Test text insertion with empty content (should be allowed)."""
        # Setup mock
        response = mock_response(200, sample_insert_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute - empty content should be allowed
        result = await lightrag_client.insert_text("")
        
        # Verify
        assert isinstance(result, InsertResponse)
        lightrag_client.client.post.assert_called_once()
    
    async def test_insert_texts_success(self, lightrag_client, mock_response, sample_insert_response):
        """Test successful multiple text insertion."""
        # Setup mock
        response = mock_response(200, sample_insert_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Create test documents
        texts = [
            TextDocument(content="Text 1", title="Title 1"),
            TextDocument(content="Text 2", title="Title 2")
        ]
        
        # Execute
        result = await lightrag_client.insert_texts(texts)
        
        # Verify
        assert isinstance(result, InsertResponse)
        lightrag_client.client.post.assert_called_once()
    
    async def test_upload_document_success(self, lightrag_client, mock_response):
        """Test successful document upload."""
        # Setup mock
        upload_response = {"filename": "test.txt", "status": "uploaded"}
        response = mock_response(200, upload_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Mock file operations
        with patch('os.path.exists', return_value=True), \
             patch('os.access', return_value=True), \
             patch('os.path.getsize', return_value=1024), \
             patch('builtins.open', create=True) as mock_open:
            
            mock_file = MagicMock()
            mock_open.return_value.__enter__.return_value = mock_file
            
            # Execute
            result = await lightrag_client.upload_document("/path/to/test.txt")
            
            # Verify
            assert result.filename == "test.txt"
            assert result.status == "uploaded"
            lightrag_client.client.post.assert_called_once()
    
    async def test_upload_document_file_not_found(self, lightrag_client):
        """Test document upload with file not found."""
        with patch('os.path.exists', return_value=False):
            with pytest.raises(LightRAGValidationError, match="File not found"):
                await lightrag_client.upload_document("/nonexistent/file.txt")
    
    async def test_scan_documents_success(self, lightrag_client, mock_response):
        """Test successful document scanning."""
        # Setup mock
        scan_response = {"scanned": 5, "new_documents": ["doc1.txt", "doc2.txt"]}
        response = mock_response(200, scan_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.scan_documents()
        
        # Verify
        assert result.scanned == 5
        assert len(result.new_documents) == 2
        lightrag_client.client.post.assert_called_once_with(
            "http://localhost:9621/documents/scan", json=None
        )
    
    async def test_get_documents_success(self, lightrag_client, mock_response, sample_documents_response):
        """Test successful document retrieval."""
        # Setup mock
        response = mock_response(200, sample_documents_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_documents()
        
        # Verify
        assert isinstance(result, DocumentsResponse)
        assert len(result.documents) == 1
        assert result.documents[0].id == "doc_123"
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/documents", params=None
        )
    
    async def test_get_documents_paginated_success(self, lightrag_client, mock_response):
        """Test successful paginated document retrieval."""
        # Setup mock
        paginated_response = {
            "documents": [{"id": "doc_123", "title": "Test", "status": "processed"}],
            "pagination": {"page": 1, "page_size": 10, "total_pages": 1, "total_items": 1}
        }
        response = mock_response(200, paginated_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_documents_paginated(page=1, page_size=10)
        
        # Verify
        assert len(result.documents) == 1
        assert result.pagination.page == 1
        assert result.pagination.page_size == 10
        lightrag_client.client.post.assert_called_once()
    
    async def test_delete_document_success(self, lightrag_client, mock_response):
        """Test successful document deletion."""
        # Setup mock
        delete_response = {"deleted": True, "document_id": "doc_123"}
        response = mock_response(200, delete_response)
        lightrag_client.client.request = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.delete_document("doc_123")
        
        # Verify
        assert result.deleted is True
        assert result.document_id == "doc_123"
        lightrag_client.client.request.assert_called_once()
    
    async def test_clear_documents_success(self, lightrag_client, mock_response):
        """Test successful document clearing."""
        # Setup mock
        clear_response = {"status": "success", "message": "All documents cleared"}
        response = mock_response(200, clear_response)
        lightrag_client.client.delete = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.clear_documents()
        
        # Verify
        assert result.status == "success"
        assert result.message == "All documents cleared"
        lightrag_client.client.delete.assert_called_once_with(
            "http://localhost:9621/documents"
        )


@pytest.mark.asyncio
class TestQueryMethods:
    """Test query client methods."""
    
    async def test_query_text_success(self, lightrag_client, mock_response, sample_query_response):
        """Test successful text query."""
        # Setup mock
        response = mock_response(200, sample_query_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.query_text("test query", mode="hybrid", only_need_context=False)
        
        # Verify
        assert isinstance(result, QueryResponse)
        assert result.query == "test query"
        assert len(result.results) == 1
        assert result.results[0].document_id == "doc_123"
        
        # Verify API call
        lightrag_client.client.post.assert_called_once()
        call_args = lightrag_client.client.post.call_args
        assert call_args[0][0] == "http://localhost:9621/query"
        
        # Verify request data
        request_data = call_args[1]["json"]
        assert request_data["query"] == "test query"
        assert request_data["mode"] == "hybrid"
        assert request_data["only_need_context"] is False
    
    async def test_query_text_validation_error_empty_query(self, lightrag_client):
        """Test query with empty query string."""
        with pytest.raises(LightRAGValidationError, match="Query cannot be empty"):
            await lightrag_client.query_text("")
    
    async def test_query_text_validation_error_invalid_mode(self, lightrag_client):
        """Test query with invalid mode."""
        with pytest.raises(LightRAGValidationError, match="Invalid query mode"):
            await lightrag_client.query_text("test query", mode="invalid_mode")
    
    async def test_query_text_stream_success(self, lightrag_client, mock_streaming_response):
        """Test successful streaming text query."""
        # Setup mock
        chunks = ["chunk 1", "chunk 2", "chunk 3"]
        streaming_response = mock_streaming_response(chunks)
        
        # Mock the stream context manager
        mock_stream_context = AsyncMock()
        mock_stream_context.__aenter__ = AsyncMock(return_value=streaming_response)
        mock_stream_context.__aexit__ = AsyncMock(return_value=None)
        lightrag_client.client.stream = MagicMock(return_value=mock_stream_context)
        
        # Execute and collect results
        results = []
        async for chunk in lightrag_client.query_text_stream("test query", mode="hybrid"):
            results.append(chunk)
        
        # Verify
        assert results == chunks
        lightrag_client.client.stream.assert_called_once_with(
            "POST", "http://localhost:9621/query/stream", json={
                "query": "test query",
                "mode": "hybrid",
                "only_need_context": False,
                "stream": True
            }
        )
    
    async def test_query_text_stream_validation_error(self, lightrag_client):
        """Test streaming query with validation error."""
        with pytest.raises(LightRAGValidationError, match="Query cannot be empty"):
            async for _ in lightrag_client.query_text_stream(""):
                pass


@pytest.mark.asyncio
class TestKnowledgeGraphMethods:
    """Test knowledge graph client methods."""
    
    async def test_get_knowledge_graph_success(self, lightrag_client, mock_response, sample_graph_response):
        """Test successful knowledge graph retrieval."""
        # Setup mock
        response = mock_response(200, sample_graph_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_knowledge_graph()
        
        # Verify
        assert len(result.entities) == 1
        assert len(result.relations) == 1
        assert result.entities[0].id == "entity_123"
        assert result.relations[0].id == "rel_123"
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/graphs", params={"label": "*"}
        )
    
    async def test_get_graph_labels_success(self, lightrag_client, mock_response):
        """Test successful graph labels retrieval."""
        # Setup mock
        labels_response = {
            "entity_labels": ["Person", "Organization"],
            "relation_labels": ["works_for", "located_in"]
        }
        response = mock_response(200, labels_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_graph_labels()
        
        # Verify
        assert len(result.entity_labels) == 2
        assert len(result.relation_labels) == 2
        assert "Person" in result.entity_labels
        assert "works_for" in result.relation_labels
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/graph/label/list", params=None
        )
    
    async def test_check_entity_exists_success(self, lightrag_client, mock_response):
        """Test successful entity existence check."""
        # Setup mock
        exists_response = {"exists": True, "entity_name": "Test Entity", "entity_id": "ent_123"}
        response = mock_response(200, exists_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.check_entity_exists("Test Entity")
        
        # Verify
        assert result.exists is True
        assert result.entity_name == "Test Entity"
        assert result.entity_id == "ent_123"
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/graph/entity/exists", 
            params={"name": "Test Entity"}
        )
    
    async def test_update_entity_success(self, lightrag_client, mock_response):
        """Test successful entity update."""
        # Setup mock
        update_response = {"updated": True, "entity_id": "ent_123"}
        response = mock_response(200, update_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        properties = {"name": "Updated Entity", "type": "concept"}
        result = await lightrag_client.update_entity("ent_123", properties)
        
        # Verify
        assert result.updated is True
        assert result.entity_id == "ent_123"
        lightrag_client.client.post.assert_called_once()
        
        # Verify request data
        call_args = lightrag_client.client.post.call_args
        request_data = call_args[1]["json"]
        assert request_data["entity_id"] == "ent_123"
        assert request_data["updated_data"] == properties

    async def test_update_relation_success(self, lightrag_client, mock_response):
        """Test successful relation update."""
        # Setup mock
        update_response = {"updated": True, "relation_id": "rel_123"}
        response = mock_response(200, update_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        properties = {"type": "strongly_related", "weight": 0.9}
        result = await lightrag_client.update_relation("ent_123", "ent_456", properties)
        
        # Verify
        assert result.updated is True
        assert result.relation_id == "rel_123"
        lightrag_client.client.post.assert_called_once()
    
    async def test_delete_entity_success(self, lightrag_client, mock_response):
        """Test successful entity deletion."""
        # Setup mock
        delete_response = {"deleted": True, "id": "ent_123", "type": "entity"}
        response = mock_response(200, delete_response)
        lightrag_client.client.request = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.delete_entity("ent_123")
        
        # Verify
        assert result.deleted is True
        assert result.id == "ent_123"
        assert result.type == "entity"
        lightrag_client.client.request.assert_called_once()
    
    async def test_delete_relation_success(self, lightrag_client, mock_response):
        """Test successful relation deletion."""
        # Setup mock
        delete_response = {"deleted": True, "id": "rel_123", "type": "relation"}
        response = mock_response(200, delete_response)
        lightrag_client.client.request = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.delete_relation("rel_123")
        
        # Verify
        assert result.deleted is True
        assert result.id == "rel_123"
        assert result.type == "relation"
        lightrag_client.client.request.assert_called_once()


@pytest.mark.asyncio
class TestSystemManagementMethods:
    """Test system management client methods."""
    
    async def test_get_pipeline_status_success(self, lightrag_client, mock_response, sample_pipeline_status_response):
        """Test successful pipeline status retrieval."""
        # Setup mock
        response = mock_response(200, sample_pipeline_status_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_pipeline_status()
        
        # Verify
        assert result.status == "running"
        assert result.progress == 75.5
        assert result.current_task == "processing documents"
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/documents/pipeline_status", params=None
        )
    
    async def test_get_track_status_success(self, lightrag_client, mock_response):
        """Test successful track status retrieval."""
        # Setup mock
        track_response = {"track_id": "track_123", "status": "completed", "progress": 100.0}
        response = mock_response(200, track_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_track_status("track_123")
        
        # Verify
        assert result.track_id == "track_123"
        assert result.status == "completed"
        assert result.progress == 100.0
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/documents/track_status/track_123", params=None
        )
    
    async def test_get_document_status_counts_success(self, lightrag_client, mock_response, sample_status_counts_response):
        """Test successful document status counts retrieval."""
        # Setup mock
        response = mock_response(200, sample_status_counts_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_document_status_counts()
        
        # Verify
        assert result.pending == 5
        assert result.processing == 2
        assert result.processed == 100
        assert result.failed == 1
        assert result.total == 108
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/documents/status_counts", params=None
        )
    
    async def test_clear_cache_success(self, lightrag_client, mock_response):
        """Test successful cache clearing."""
        # Setup mock
        cache_response = {"status": "success", "message": "Cache cleared", "cache_type": "all"}
        response = mock_response(200, cache_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.clear_cache()
        
        # Verify
        assert result.status == "success"
        assert result.message == "Cache cleared"
        lightrag_client.client.post.assert_called_once()
    
    async def test_clear_cache_with_type_success(self, lightrag_client, mock_response):
        """Test successful cache clearing with specific type."""
        # Setup mock
        cache_response = {"status": "success", "message": "Query cache cleared", "cache_type": "query"}
        response = mock_response(200, cache_response)
        lightrag_client.client.post = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.clear_cache(cache_type="query")
        
        # Verify
        assert result.status == "success"
        assert result.cache_type == "query"
        lightrag_client.client.post.assert_called_once()
        
        # Verify request data
        call_args = lightrag_client.client.post.call_args
        request_data = call_args[1]["json"]
        assert request_data["cache_type"] == "query"
    
    async def test_get_health_success(self, lightrag_client, mock_response, sample_health_response):
        """Test successful health check."""
        # Setup mock
        response = mock_response(200, sample_health_response)
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute
        result = await lightrag_client.get_health()
        
        # Verify
        assert isinstance(result, HealthResponse)
        assert result.status == "healthy"
        assert result.version == "1.0.0"
        assert result.uptime == 3600.0
        assert result.database_status == "connected"
        lightrag_client.client.get.assert_called_once_with(
            "http://localhost:9621/health", params=None
        )


@pytest.mark.asyncio
class TestErrorHandling:
    """Test error handling in client methods."""
    
    async def test_http_status_error_handling(self, lightrag_client, mock_response):
        """Test handling of HTTP status errors."""
        # Setup mock to raise HTTP error
        response = mock_response(400, {"detail": "Bad request"}, "Bad request")
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute and verify error
        with pytest.raises(LightRAGValidationError, match="Bad Request"):
            await lightrag_client.get_health()
    
    async def test_connection_error_handling(self, lightrag_client):
        """Test handling of connection errors."""
        # Setup mock to raise connection error
        lightrag_client.client.get = AsyncMock(
            side_effect=httpx.ConnectError("Connection failed")
        )
        
        # Execute and verify error
        with pytest.raises(LightRAGConnectionError, match="Connection failed"):
            await lightrag_client.get_health()
    
    async def test_timeout_error_handling(self, lightrag_client):
        """Test handling of timeout errors."""
        # Setup mock to raise timeout error
        lightrag_client.client.get = AsyncMock(
            side_effect=httpx.TimeoutException("Request timeout")
        )
        
        # Execute and verify error
        with pytest.raises(LightRAGTimeoutError, match="Request timeout"):
            await lightrag_client.get_health()
    
    async def test_json_decode_error_handling(self, lightrag_client):
        """Test handling of JSON decode errors."""
        # Setup mock with invalid JSON response
        response = MagicMock()
        response.status_code = 200
        response.raise_for_status = MagicMock()
        response.json.side_effect = json.JSONDecodeError("Invalid JSON", "", 0)
        response.text = "Invalid JSON response"
        response.headers = {}  # Add proper headers mock
        
        lightrag_client.client.get = AsyncMock(return_value=response)
        
        # Execute and verify error (it will be wrapped in LightRAGError due to exception handling)
        with pytest.raises(LightRAGError, match="Invalid JSON response"):
            await lightrag_client.get_health()
    
    async def test_streaming_error_handling(self, lightrag_client):
        """Test error handling in streaming requests."""
        # Setup mock to raise HTTP error in streaming
        mock_stream_context = AsyncMock()
        mock_stream_context.__aenter__ = AsyncMock(
            side_effect=httpx.HTTPStatusError(
                message="HTTP 500",
                request=MagicMock(),
                response=MagicMock(status_code=500, text="Server error")
            )
        )
        lightrag_client.client.stream = MagicMock(return_value=mock_stream_context)
        
        # Execute and verify error
        with pytest.raises(LightRAGServerError, match="Server Error"):
            async for _ in lightrag_client.query_text_stream("test query"):
                pass


@pytest.mark.asyncio
class TestContextManager:
    """Test client context manager functionality."""
    
    async def test_context_manager_usage(self):
        """Test client can be used as async context manager."""
        async with LightRAGClient() as client:
            assert client.client is not None
        
        # Client should be closed after context exit
        # Note: We can't easily test this without mocking the httpx client
    
    async def test_manual_close(self):
        """Test manual client closing."""
        client = LightRAGClient()
        
        # Mock the httpx client close method
        client.client.aclose = AsyncMock()
        
        # Close the client
        await client.__aexit__(None, None, None)
        
        # Verify close was called
        client.client.aclose.assert_called_once()


@pytest.mark.asyncio
class TestUpdateRelations:
    """Test the update_relations method (C# analyser pipeline)."""

    async def test_update_relations_success(self, lightrag_client):
        """Test successful analyser run and ingestion."""
        mock_results = [
            ("Calc.txt", "class Calc { int Add() => 0; }"),
            ("Prog.txt", "class Program { void Main() {} }"),
        ]
        with patch("daniel_lightrag_mcp.client.run_analysis", return_value=mock_results):
            lightrag_client.insert_text = AsyncMock(
                side_effect=[
                    MagicMock(track_id="track_001"),
                    MagicMock(track_id="track_002"),
                ]
            )
            result = await lightrag_client.update_relations("/path/to/project.sln")

        assert len(result) == 3
        assert "Analyser generated 2 output files" in result[0]
        assert "Ingested 2 files into LightRAG" in result[1]
        assert "LightRAG is now processing" in result[2]
        assert lightrag_client.insert_text.call_count == 2

    async def test_update_relations_analyser_fails(self, lightrag_client):
        """Test when the C# analyser itself fails."""
        with patch("daniel_lightrag_mcp.client.run_analysis", side_effect=RuntimeError("analyser crashed")):
            result = await lightrag_client.update_relations("/path/to/project.sln")

        assert len(result) == 1
        assert "ERROR" in result[0]
        assert "analyser crashed" in result[0]

    async def test_update_relations_no_output(self, lightrag_client):
        """Test when analyser produces no output files."""
        with patch("daniel_lightrag_mcp.client.run_analysis", return_value=[]):
            result = await lightrag_client.update_relations("/path/to/project.sln")

        assert len(result) == 1
        assert "WARNING" in result[0]
        assert "no output files" in result[0]

    async def test_update_relations_partial_ingestion_failure(self, lightrag_client):
        """Test when some files fail to ingest."""
        mock_results = [
            ("Calc.txt", "content"),
            ("Prog.txt", "content"),
            ("Data.txt", "content"),
        ]
        with patch("daniel_lightrag_mcp.client.run_analysis", return_value=mock_results):
            lightrag_client.insert_text = AsyncMock(
                side_effect=[
                    MagicMock(track_id="track_001"),
                    Exception("ingestion failed for Prog.txt"),
                    MagicMock(track_id="track_003"),
                ]
            )
            result = await lightrag_client.update_relations("/path/to/project.sln")

        assert len(result) == 4
        assert "Analyser generated 3 output files" in result[0]
        assert "Ingested 2 files into LightRAG" in result[1]
        assert "Errors" in result[2]
        assert "ingestion failed for Prog.txt" in result[2]
        assert lightrag_client.insert_text.call_count == 3

    async def test_update_relations_uses_filename_as_title(self, lightrag_client):
        """Test that filename (without .txt) is passed as title to insert_text."""
        mock_results = [("MyFile.txt", "content")]
        with patch("daniel_lightrag_mcp.client.run_analysis", return_value=mock_results):
            lightrag_client.insert_text = AsyncMock(return_value=MagicMock(track_id="track_001"))
            await lightrag_client.update_relations("/path/to/project.sln")

        lightrag_client.insert_text.assert_called_once_with("content", title="MyFile")


class TestRelationDetailMethod:
    """Test get_relation_detail client method."""

    async def test_get_relation_detail_success(self, lightrag_client, mock_response):
        """Test successful relation detail retrieval."""
        graph_data = {
            "nodes": [
                {
                    "id": "TestLib.Calculator",
                    "properties": {"entity_name": "TestLib.Calculator", "entity_type": "CLASS"},
                }
            ],
            "edges": [
                {
                    "id": "rel_1",
                    "source": "TestLib.Calculator",
                    "target": "TestLib.Calculator.Add",
                    "properties": {"keywords": "defines,method", "weight": 1.0, "source_id": "TestLib__Calculator.cs.txt"},
                    "weight": 1.0,
                },
                {
                    "id": "rel_2",
                    "source": "TestLib.Calculator",
                    "target": "TestLib.DataStore.GetAll",
                    "properties": {"keywords": "calls", "weight": 0.5, "source_id": "TestLib__Calculator.cs.txt"},
                    "weight": 0.5,
                },
            ],
        }
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.get_relation_detail("TestLib.Calculator", "defines")

        assert result.entity_name == "TestLib.Calculator"
        assert result.relation_type == "defines"
        assert result.total == 1
        assert result.relations[0].source_entity == "TestLib.Calculator"
        assert result.relations[0].target_entity == "TestLib.Calculator.Add"

    async def test_get_relation_detail_no_matches(self, lightrag_client, mock_response):
        """Test when no relations match the type filter."""
        graph_data = {
            "nodes": [],
            "edges": [
                {
                    "id": "rel_1",
                    "source": "A",
                    "target": "B",
                    "properties": {"keywords": "calls"},
                    "weight": 0.5,
                }
            ],
        }
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.get_relation_detail("A", "defines")

        assert result.total == 0
        assert result.relations == []

    async def test_get_relation_detail_graph_error(self, lightrag_client):
        """Test error handling when graph fetch fails."""
        lightrag_client.get_knowledge_graph = AsyncMock(side_effect=Exception("Connection failed"))

        result = await lightrag_client.get_relation_detail("TestLib.Calculator", "defines")

        assert result.total == 0
        assert result.relations == []


class TestMethodSignatureMethod:
    """Test get_method_signature client method."""

    async def test_get_method_signature_success(self, lightrag_client, mock_response):
        """Test successful method signature retrieval."""
        graph_data = {
            "nodes": [
                {
                    "id": "TestLib.Calculator.Add",
                    "properties": {
                        "entity_name": "TestLib.Calculator.Add",
                        "entity_type": "METHOD",
                        "description": "Add(int a, int b) -> int",
                        "file_path": "/path/Calculator.cs",
                    },
                }
            ],
            "edges": [],
        }
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.get_method_signature("TestLib.Calculator.Add")

        assert result.entity_name == "TestLib.Calculator.Add"
        assert result.return_type == "int"
        assert len(result.parameters) == 2
        assert result.parameters[0]["name"] == "int"
        assert result.parameters[0]["type"] == "a"
        assert result.parameters[1]["name"] == "int"
        assert result.parameters[1]["type"] == "b"
        assert result.file_path == "/path/Calculator.cs"

    async def test_get_method_signature_not_found(self, lightrag_client, mock_response):
        """Test when entity is not found in graph."""
        graph_data = {"nodes": [], "edges": []}
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.get_method_signature("NonExistent.Method")

        assert result.entity_name == "NonExistent.Method"
        assert result.return_type == ""
        assert result.parameters == []


class TestFindRelatedFilesMethod:
    """Test find_related_files client method."""

    async def test_find_related_files_by_entity(self, lightrag_client, mock_response):
        """Test finding files related to an entity name."""
        graph_data = {
            "nodes": [
                {
                    "id": "TestLib.Calculator",
                    "properties": {
                        "entity_name": "TestLib.Calculator",
                        "entity_type": "CLASS",
                        "file_path": "E:/dev/Calculator.cs",
                    },
                },
                {
                    "id": "TestLib.Calculator.Add",
                    "properties": {
                        "entity_name": "TestLib.Calculator.Add",
                        "entity_type": "METHOD",
                        "file_path": "E:/dev/Calculator.cs",
                    },
                },
            ],
            "edges": [],
        }
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.find_related_files("TestLib.Calculator")

        assert result.query == "TestLib.Calculator"
        assert result.total_files >= 1
        assert any("Calculator.cs" in f for f in result.files)

    async def test_find_related_files_no_matches(self, lightrag_client, mock_response):
        """Test when no related files are found."""
        graph_data = {
            "nodes": [
                {
                    "id": "TestLib.Other",
                    "properties": {
                        "entity_name": "TestLib.Other",
                        "entity_type": "CLASS",
                        "file_path": "E:/dev/Other.cs",
                    },
                }
            ],
            "edges": [],
        }
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.find_related_files("NonExistent")

        assert result.total_files == 0
        assert result.files == []


class TestInterfaceMethodsMethod:
    """Test get_interface_methods client method."""

    async def test_get_interface_methods_success(self, lightrag_client, mock_response):
        """Test successful interface methods retrieval."""
        graph_data = {
            "nodes": [
                {
                    "id": "TestLib.IOperation",
                    "properties": {
                        "entity_name": "TestLib.IOperation",
                        "entity_type": "INTERFACE",
                    },
                },
                {
                    "id": "TestLib.Calculator",
                    "properties": {
                        "entity_name": "TestLib.Calculator",
                        "entity_type": "CLASS",
                    },
                },
            ],
            "edges": [
                {
                    "id": "rel_1",
                    "source": "TestLib.IOperation",
                    "target": "TestLib.IOperation.Execute",
                    "properties": {"keywords": "defines,method", "description": "Execute() -> int"},
                },
                {
                    "id": "rel_2",
                    "source": "TestLib.Calculator",
                    "target": "TestLib.IOperation",
                    "properties": {"keywords": "implements"},
                },
            ],
        }
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.get_interface_methods("TestLib.IOperation")

        assert result.interface_name == "TestLib.IOperation"
        assert len(result.methods) == 1
        assert result.methods[0].method_name == "TestLib.IOperation.Execute"
        assert "TestLib.Calculator" in result.implementing_classes

    async def test_get_interface_methods_not_found(self, lightrag_client, mock_response):
        """Test when interface is not found in graph."""
        graph_data = {"nodes": [], "edges": []}
        response = mock_response(200, graph_data)
        lightrag_client.client.get = AsyncMock(return_value=response)

        result = await lightrag_client.get_interface_methods("NonExistent.Interface")

        assert result.interface_name == "NonExistent.Interface"
        assert result.methods == []
        assert result.implementing_classes == []