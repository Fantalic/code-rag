"""
Unit tests for MCP server functionality.
"""

import pytest
import json
from unittest.mock import AsyncMock, patch, MagicMock

from daniel_lightrag_mcp.server import (
    handle_list_tools, 
    handle_call_tool,
    _validate_tool_arguments,
    _create_success_response,
    _create_error_response
)
from daniel_lightrag_mcp.client import (
    LightRAGError, 
    LightRAGConnectionError, 
    LightRAGValidationError,
    LightRAGAPIError
)


class TestServerToolListing:
    """Test MCP server tool listing functionality."""
    
    @pytest.mark.asyncio
    async def test_handle_list_tools(self):
        """Test that all 22 tools are listed correctly."""
        result = await handle_list_tools()
        
        assert len(result) == 22
        
        # Check that all expected tools are present
        tool_names = [tool.name for tool in result]
        
        # Document Management Tools (7 tools)
        document_tools = [
            "insert_text", "insert_texts", "upload_document", "scan_documents",
            "get_documents", "get_documents_paginated", "delete_document"
        ]
        
        # Query Tools (3 tools)
        query_tools = ["query_text", "query_text_stream", "get_related_files"]
        
        # Knowledge Graph Tools (8 tools)
        graph_tools = [
            "get_knowledge_graph", "get_graph_labels", "check_entity_exists",
            "update_entity", "update_relation", "update_relations",
            "delete_entity", "delete_relation"
        ]
        
        # System Management Tools (4 tools)
        system_tools = [
            "get_pipeline_status", "get_track_status", "get_document_status_counts",
            "get_health"
        ]
        
        all_expected_tools = document_tools + query_tools + graph_tools + system_tools
        
        for tool_name in all_expected_tools:
            assert tool_name in tool_names, f"Tool {tool_name} not found in listed tools"
        
        # Verify tool schemas have required properties
        for tool in result:
            assert hasattr(tool, 'name')
            assert hasattr(tool, 'description')
            assert hasattr(tool, 'inputSchema')
            assert tool.description is not None
            assert tool.inputSchema is not None


class TestToolArgumentValidation:
    """Test tool argument validation functionality."""
    
    def test_validate_required_arguments_success(self):
        """Test successful validation of required arguments."""
        _validate_tool_arguments("insert_text", {"text": "test content"})
        _validate_tool_arguments("query_text", {"query": "test query"})
        _validate_tool_arguments("delete_document", {"document_id": "doc_123"})
        _validate_tool_arguments("update_entity", {"entity_id": "ent_123", "properties": {}})
    
    def test_validate_missing_required_arguments(self):
        """Test validation failure for missing required arguments."""
        with pytest.raises(LightRAGValidationError, match="Missing required arguments"):
            _validate_tool_arguments("insert_text", {})
        
        with pytest.raises(LightRAGValidationError, match="Missing required arguments"):
            _validate_tool_arguments("query_text", {})
        
        with pytest.raises(LightRAGValidationError, match="Missing required arguments"):
            _validate_tool_arguments("update_entity", {"entity_id": "ent_123"})
    
    def test_validate_pagination_arguments(self):
        """Test validation of pagination arguments."""
        _validate_tool_arguments("get_documents_paginated", {"page": 1, "page_size": 10})
        
        with pytest.raises(LightRAGValidationError, match="Page must be a positive integer"):
            _validate_tool_arguments("get_documents_paginated", {"page": 0, "page_size": 10})
        
        with pytest.raises(LightRAGValidationError, match="Page size must be an integer"):
            _validate_tool_arguments("get_documents_paginated", {"page": 1, "page_size": 101})
    
    def test_validate_query_mode(self):
        """Test validation of query mode arguments."""
        for mode in ["naive", "local", "global", "hybrid"]:
            _validate_tool_arguments("query_text", {"query": "test", "mode": mode})
        
        with pytest.raises(LightRAGValidationError, match="Invalid query mode"):
            _validate_tool_arguments("query_text", {"query": "test", "mode": "invalid"})


class TestResponseCreation:
    """Test response creation utilities."""

    def _assert_success_response(self, response: dict, expected_data: dict):
        """Assert a success response dict has the right shape."""
        assert isinstance(response, dict)
        assert "content" in response
        assert len(response["content"]) == 1
        assert response["content"][0]["type"] == "text"
        parsed = json.loads(response["content"][0]["text"])
        assert parsed == expected_data

    def _assert_error_response(self, response: dict, tool_name: str, error_type: str, message: str):
        """Assert an error response dict has the right shape."""
        assert isinstance(response, dict)
        assert response.get("isError") is True
        assert "content" in response
        assert len(response["content"]) == 1
        parsed = json.loads(response["content"][0]["text"])
        assert parsed["tool"] == tool_name
        assert parsed["error_type"] == error_type
        assert parsed["message"] == message

    def test_create_success_response(self):
        """Test creation of success responses."""
        test_result = {"status": "success", "data": "test"}
        response = _create_success_response(test_result, "test_tool")
        self._assert_success_response(response, test_result)
    
    def test_create_error_response_lightrag_error(self):
        """Test creation of error responses for LightRAG errors."""
        error = LightRAGValidationError("Test validation error", status_code=400)
        response = _create_error_response(error, "test_tool")
        self._assert_error_response(
            response, "test_tool", "LightRAGValidationError", "Test validation error"
        )
    
    def test_create_error_response_generic_error(self):
        """Test creation of error responses for generic errors."""
        error = ValueError("Generic error")
        response = _create_error_response(error, "test_tool")
        self._assert_error_response(
            response, "test_tool", "ValueError", "Generic error"
        )


@pytest.mark.asyncio
class TestDocumentManagementTools:
    """Test document management MCP tools."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_insert_text_success(self, mock_client):
        """Test successful text insertion."""
        mock_result = {"id": "doc_123", "status": "success"}
        mock_client.insert_text = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("insert_text", {"text": "test content"})
        
        assert "isError" not in result or not result["isError"]
        mock_client.insert_text.assert_called_once_with("test content")
        
        content = json.loads(result["content"][0]["text"])
        assert content == mock_result
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_insert_texts_success(self, mock_client):
        """Test successful multiple text insertion."""
        mock_result = {"id": "batch_123", "status": "success"}
        mock_client.insert_texts = AsyncMock(return_value=mock_result)
        
        texts = [{"content": "text 1"}, {"content": "text 2"}]
        result = await handle_call_tool("insert_texts", {"texts": texts})
        
        assert "isError" not in result or not result["isError"]
        mock_client.insert_texts.assert_called_once_with(texts)
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_upload_document_success(self, mock_client):
        """Test successful document upload."""
        mock_result = {"filename": "test.txt", "status": "uploaded"}
        mock_client.upload_document = AsyncMock(return_value=mock_result)
        
        with patch('os.path.exists', return_value=True), \
             patch('os.access', return_value=True), \
             patch('os.path.getsize', return_value=1024):
            result = await handle_call_tool("upload_document", {"file_path": "/path/to/file.txt"})
        
        assert "isError" not in result or not result["isError"]
        mock_client.upload_document.assert_called_once_with("/path/to/file.txt")
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_scan_documents_success(self, mock_client):
        """Test successful document scanning."""
        mock_result = {"scanned": 5, "new_documents": ["doc1.txt", "doc2.txt"]}
        mock_client.scan_documents = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("scan_documents", {})
        
        assert "isError" not in result or not result["isError"]
        mock_client.scan_documents.assert_called_once()
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_documents_success(self, mock_client):
        """Test successful document retrieval."""
        mock_result = {"documents": [{"id": "doc_123", "title": "Test"}], "total": 1}
        mock_client.get_documents = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_documents", {})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_documents.assert_called_once()
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_documents_paginated_success(self, mock_client):
        """Test successful paginated document retrieval."""
        mock_result = {
            "documents": [{"id": "doc_123", "title": "Test"}],
            "pagination": {"page": 1, "page_size": 10, "total_pages": 1}
        }
        mock_client.get_documents_paginated = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_documents_paginated", {"page": 1, "page_size": 10})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_documents_paginated.assert_called_once_with(1, 10)
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_delete_document_success(self, mock_client):
        """Test successful document deletion."""
        mock_result = {"deleted": True, "document_id": "doc_123"}
        mock_client.delete_document = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("delete_document", {"document_id": "doc_123"})
        
        assert "isError" not in result or not result["isError"]
        mock_client.delete_document.assert_called_once_with("doc_123")


@pytest.mark.asyncio
class TestQueryTools:
    """Test query MCP tools."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_query_text_success(self, mock_client):
        """Test successful text query."""
        mock_result = {
            "query": "test query",
            "results": [{"document_id": "doc_123", "snippet": "test snippet", "score": 0.95}],
            "total_results": 1
        }
        mock_client.query_text = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool(
            "query_text",
            {"query": "test query", "mode": "hybrid", "only_need_context": False}
        )
        
        assert "isError" not in result or not result["isError"]
        mock_client.query_text.assert_called_once_with(
            "test query", mode="hybrid", only_need_context=False
        )
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_query_text_stream_success(self, mock_client):
        """Test successful streaming text query."""
        async def mock_stream(query, mode="hybrid", only_need_context=False):
            yield "chunk 1"
            yield "chunk 2"
            yield "chunk 3"
        
        mock_client.query_text_stream = mock_stream
        
        result = await handle_call_tool(
            "query_text_stream",
            {"query": "test query", "mode": "hybrid"}
        )
        
        assert "isError" not in result or not result["isError"]
        
        content = json.loads(result["content"][0]["text"])
        assert "streaming_response" in content
        assert content["streaming_response"] == "chunk 1chunk 2chunk 3"


@pytest.mark.asyncio
class TestKnowledgeGraphTools:
    """Test knowledge graph MCP tools."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_knowledge_graph_success(self, mock_client):
        """Test successful knowledge graph retrieval."""
        mock_result = {
            "entities": [{"id": "ent_123", "name": "Test Entity"}],
            "relations": [{"id": "rel_123", "source_entity": "ent_123", "target_entity": "ent_456"}],
            "total_entities": 1,
            "total_relations": 1
        }
        mock_client.get_knowledge_graph = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_knowledge_graph", {})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_knowledge_graph.assert_called_once()
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_graph_labels_success(self, mock_client):
        """Test successful graph labels retrieval."""
        mock_result = {
            "entity_labels": ["Person", "Organization", "Location"],
            "relation_labels": ["works_for", "located_in", "related_to"]
        }
        mock_client.get_graph_labels = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_graph_labels", {})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_graph_labels.assert_called_once()
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_check_entity_exists_success(self, mock_client):
        """Test successful entity existence check."""
        mock_result = {"exists": True, "entity_name": "Test Entity", "entity_id": "ent_123"}
        mock_client.check_entity_exists = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("check_entity_exists", {"entity_name": "Test Entity"})
        
        assert "isError" not in result or not result["isError"]
        mock_client.check_entity_exists.assert_called_once_with("Test Entity")
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_update_entity_success(self, mock_client):
        """Test successful entity update."""
        mock_result = {"updated": True, "entity_id": "ent_123"}
        mock_client.update_entity = AsyncMock(return_value=mock_result)
        
        properties = {"name": "Updated Entity", "type": "concept"}
        result = await handle_call_tool("update_entity", {"entity_id": "ent_123", "properties": properties})
        
        assert "isError" not in result or not result["isError"]
        mock_client.update_entity.assert_called_once_with("ent_123", properties)
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_update_relation_success(self, mock_client):
        """Test successful relation update."""
        mock_result = {"updated": True, "relation_id": "rel_123"}
        mock_client.update_relation = AsyncMock(return_value=mock_result)
        
        properties = {"type": "strongly_related", "weight": 0.9}
        result = await handle_call_tool(
            "update_relation",
            {"relation_id": "rel_123", "properties": properties}
        )
        
        assert "isError" not in result or not result["isError"]
        mock_client.update_relation.assert_called_once_with("rel_123", properties)
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_delete_entity_success(self, mock_client):
        """Test successful entity deletion."""
        mock_result = {"deleted": True, "id": "ent_123", "type": "entity"}
        mock_client.delete_entity = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("delete_entity", {"entity_id": "ent_123"})
        
        assert "isError" not in result or not result["isError"]
        mock_client.delete_entity.assert_called_once_with("ent_123")
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_delete_relation_success(self, mock_client):
        """Test successful relation deletion."""
        mock_result = {"deleted": True, "id": "rel_123", "type": "relation"}
        mock_client.delete_relation = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("delete_relation", {"relation_id": "rel_123"})
        
        assert "isError" not in result or not result["isError"]
        mock_client.delete_relation.assert_called_once_with("rel_123")

    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_update_relations_success(self, mock_client):
        """Test successful update_relations tool call."""
        mock_client.update_relations = AsyncMock(
            return_value=[
                "Analyser generated 4 output files",
                "Ingested 4 files into LightRAG",
                "LightRAG is now processing the analyser output"
            ]
        )
        result = await handle_call_tool(
            "update_relations",
            {"absolute_project_root_path": "/path/to/project.sln"}
        )
        assert "isError" not in result or not result["isError"]
        mock_client.update_relations.assert_called_once_with("/path/to/project.sln")
        assert "Analyser generated" in result["content"][0]["text"]

    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_update_relations_missing_path(self, mock_client):
        """Test update_relations with missing path argument."""
        result = await handle_call_tool("update_relations", {})
        assert result.get("isError") is True
        text = result["content"][0]["text"]
        assert "required" in text.lower() or "path" in text.lower()


@pytest.mark.asyncio
class TestSystemManagementTools:
    """Test system management MCP tools."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_pipeline_status_success(self, mock_client):
        """Test successful pipeline status retrieval."""
        mock_result = {
            "status": "running",
            "progress": 75.5,
            "current_task": "processing documents"
        }
        mock_client.get_pipeline_status = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_pipeline_status", {})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_pipeline_status.assert_called_once()
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_track_status_success(self, mock_client):
        """Test successful track status retrieval."""
        mock_result = {
            "track_id": "track_123",
            "status": "completed",
            "progress": 100.0
        }
        mock_client.get_track_status = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_track_status", {"track_id": "track_123"})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_track_status.assert_called_once_with("track_123")
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_document_status_counts_success(self, mock_client):
        """Test successful document status counts retrieval."""
        mock_result = {
            "pending": 5,
            "processing": 2,
            "processed": 100,
            "failed": 1,
            "total": 108
        }
        mock_client.get_document_status_counts = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_document_status_counts", {})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_document_status_counts.assert_called_once()
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_get_health_success(self, mock_client):
        """Test successful health check."""
        mock_result = {
            "status": "healthy",
            "version": "1.0.0",
            "uptime": 3600.0,
            "database_status": "connected"
        }
        mock_client.get_health = AsyncMock(return_value=mock_result)
        
        result = await handle_call_tool("get_health", {})
        
        assert "isError" not in result or not result["isError"]
        mock_client.get_health.assert_called_once()


@pytest.mark.asyncio
class TestErrorHandling:
    """Test error handling in MCP tools."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_tool_validation_error(self, mock_client):
        """Test handling of validation errors in tool call."""
        result = await handle_call_tool("insert_text", {})
        assert result.get("isError") is True
        content = json.loads(result["content"][0]["text"])
        assert content["error_type"] == "LightRAGValidationError"
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_client_connection_error(self, mock_client):
        """Test handling of connection errors."""
        mock_client.insert_text = AsyncMock(
            side_effect=LightRAGConnectionError("Connection failed")
        )
        
        result = await handle_call_tool("insert_text", {"text": "test"})
        assert result.get("isError") is True
        content = json.loads(result["content"][0]["text"])
        assert content["error_type"] == "LightRAGConnectionError"
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_unknown_tool_error(self, mock_client):
        """Test handling of unknown tool calls."""
        result = await handle_call_tool("unknown_tool", {})
        assert result.get("isError") is True
        assert "Unknown tool: unknown_tool" in result["content"][0]["text"]
    
    @patch('daniel_lightrag_mcp.server.lightrag_client', None)
    async def test_client_initialization_error(self):
        """Test handling of client initialization errors."""
        with patch('daniel_lightrag_mcp.server.LightRAGClient', side_effect=Exception("Init failed")):
            result = await handle_call_tool("get_health", {})
            
            assert result.get("isError") is True
            content = json.loads(result["content"][0]["text"])
            assert content["error_type"] == "LightRAGConnectionError"
            assert "Failed to initialize LightRAG client" in content["message"]
