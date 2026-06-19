"""
Integration tests for the MCP server with mock LightRAG server responses.
"""

import pytest
import json
import asyncio
from unittest.mock import AsyncMock, MagicMock, patch
from typing import Dict, Any, List

from mcp.types import CallToolRequest, CallToolResult
from daniel_lightrag_mcp.server import handle_call_tool, handle_list_tools
from daniel_lightrag_mcp.client import (
    LightRAGClient, LightRAGError, LightRAGValidationError,
    LightRAGConnectionError, LightRAGAPIError
)


class MockLightRAGServer:
    """Mock LightRAG server for integration testing."""
    
    def __init__(self):
        self.documents: dict[str, dict[str, Any]] = {}
        self.entities: dict[str, dict[str, Any]] = {}
        self.relations: dict[str, dict[str, Any]] = {}
        self.next_doc_id = 1
        self.next_entity_id = 1
        self.next_relation_id = 1
        self.pipeline_status = "idle"
        self.cache_cleared = False
    
    def generate_doc_id(self) -> str:
        """Generate a new document ID."""
        doc_id = f"doc_{self.next_doc_id:03d}"
        self.next_doc_id += 1
        return doc_id
    
    def generate_entity_id(self) -> str:
        """Generate a new entity ID."""
        entity_id = f"ent_{self.next_entity_id:03d}"
        self.next_entity_id += 1
        return entity_id
    
    def generate_relation_id(self) -> str:
        """Generate a new relation ID."""
        relation_id = f"rel_{self.next_relation_id:03d}"
        self.next_relation_id += 1
        return relation_id
    
    def insert_text(self, content: str, title: str = None) -> dict[str, Any]:
        """Mock text insertion."""
        doc_id = self.generate_doc_id()
        self.documents[doc_id] = {
            "id": doc_id,
            "title": title,
            "content": content,
            "status": "processed",
            "created_at": "2024-01-01T00:00:00Z"
        }
        return {"id": doc_id, "status": "success", "message": "Document inserted successfully"}
    
    def insert_texts(self, texts: list[dict[str, Any]]) -> dict[str, Any]:
        """Mock multiple text insertion."""
        doc_ids = []
        for text_data in texts:
            doc_id = self.generate_doc_id()
            self.documents[doc_id] = {
                "id": doc_id,
                "title": text_data.get("title"),
                "content": text_data["content"],
                "status": "processed",
                "created_at": "2024-01-01T00:00:00Z"
            }
            doc_ids.append(doc_id)
        
        return {
            "id": f"batch_{len(doc_ids)}",
            "status": "success",
            "message": f"Inserted {len(doc_ids)} documents",
            "document_ids": doc_ids
        }
    
    def get_documents(self) -> dict[str, Any]:
        """Mock document retrieval."""
        documents = [
            {
                "id": doc_id,
                "title": doc_data["title"],
                "status": doc_data["status"],
                "created_at": doc_data["created_at"]
            }
            for doc_id, doc_data in self.documents.items()
        ]
        return {"documents": documents, "total": len(documents)}
    
    def delete_document(self, document_id: str) -> dict[str, Any]:
        """Mock document deletion."""
        if document_id in self.documents:
            del self.documents[document_id]
            return {"deleted": True, "document_id": document_id, "message": "Document deleted"}
        else:
            return {"deleted": False, "document_id": document_id, "message": "Document not found"}
    
    def query_text(self, query: str, mode: str = "hybrid", only_need_context: bool = False) -> dict[str, Any]:
        """Mock text query."""
        # Simple mock: return documents that contain query terms
        results = []
        for doc_id, doc_data in self.documents.items():
            if query.lower() in doc_data["content"].lower():
                results.append({
                    "document_id": doc_id,
                    "snippet": doc_data["content"][:100] + "...",
                    "score": 0.95,
                    "metadata": {"title": doc_data["title"]}
                })
        
        return {
            "query": query,
            "results": results,
            "total_results": len(results),
            "processing_time": 0.123,
            "context": f"Found {len(results)} relevant documents"
        }
    
    def get_knowledge_graph(self) -> dict[str, Any]:
        """Mock knowledge graph retrieval."""
        entities = [
            {
                "id": entity_id,
                "name": entity_data["name"],
                "type": entity_data.get("type", "concept"),
                "properties": entity_data.get("properties", {}),
                "created_at": entity_data.get("created_at", "2024-01-01T00:00:00Z")
            }
            for entity_id, entity_data in self.entities.items()
        ]
        
        relations = [
            {
                "id": relation_id,
                "source_entity": relation_data["source_entity"],
                "target_entity": relation_data["target_entity"],
                "type": relation_data["type"],
                "properties": relation_data.get("properties", {}),
                "weight": relation_data.get("weight", 0.5)
            }
            for relation_id, relation_data in self.relations.items()
        ]
        
        return {
            "entities": entities,
            "relations": relations,
            "total_entities": len(entities),
            "total_relations": len(relations)
        }
    
    def get_health(self) -> dict[str, Any]:
        """Mock health check."""
        return {
            "status": "healthy",
            "version": "1.0.0",
            "uptime": 3600.0,
            "database_status": "connected",
            "cache_status": "active" if not self.cache_cleared else "cleared",
            "message": "All systems operational"
        }
    
    def clear_cache(self) -> dict[str, Any]:
        """Mock cache clearing."""
        self.cache_cleared = True
        return {"cleared": True, "cache_type": "all", "message": "Cache cleared successfully"}


@pytest.fixture
def mock_lightrag_server():
    """Create a mock LightRAG server."""
    return MockLightRAGServer()


@pytest.fixture
async def mock_client_with_server(mock_lightrag_server):
    """Create a mock client that uses the mock server."""
    client = LightRAGClient()
    
    # Mock all client methods to use the mock server
    client.insert_text = AsyncMock(side_effect=lambda content, title=None: mock_lightrag_server.insert_text(content, title))
    client.insert_texts = AsyncMock(side_effect=mock_lightrag_server.insert_texts)
    client.get_documents = AsyncMock(side_effect=mock_lightrag_server.get_documents)
    client.delete_document = AsyncMock(side_effect=mock_lightrag_server.delete_document)
    client.query_text = AsyncMock(side_effect=mock_lightrag_server.query_text)
    client.get_knowledge_graph = AsyncMock(side_effect=mock_lightrag_server.get_knowledge_graph)
    client.get_health = AsyncMock(side_effect=mock_lightrag_server.get_health)
    client.clear_cache = AsyncMock(side_effect=mock_lightrag_server.clear_cache)
    
    return client


class TestFullWorkflowIntegration:
    """Test complete workflows using the MCP server with mock responses."""
    
    @pytest.mark.asyncio
    async def test_document_management_workflow(self, mock_client_with_server, mock_lightrag_server):
        """Test complete document management workflow."""
        with patch('daniel_lightrag_mcp.server.lightrag_client') as mock_client:
            # Setup mock client
            mock_client.return_value = mock_client_with_server
            mock_client.insert_text = mock_client_with_server.insert_text
            mock_client.get_documents = mock_client_with_server.get_documents
            mock_client.delete_document = mock_client_with_server.delete_document
            
            # Step 1: Insert a document
            insert_result = await handle_call_tool(
                "insert_text",
                {"text": "This is a test document about artificial intelligence."}
            )
            assert insert_result["status"] == "success"
            doc_id = insert_result["id"]
            
            # Step 2: Verify document was inserted by retrieving all documents
            get_docs_result = await handle_call_tool("get_documents", {})
            assert get_docs_result["total"] == 1
            assert get_docs_result["documents"][0]["id"] == doc_id
            
            # Step 3: Delete the document
            delete_result = await handle_call_tool("delete_document", {"document_id": doc_id})
            assert delete_result["deleted"] is True
            assert delete_result["document_id"] == doc_id
            
            # Step 4: Verify document was deleted
            get_docs_result2 = await handle_call_tool("get_documents", {})
            assert get_docs_result2["total"] == 0
    
    @pytest.mark.asyncio
    async def test_query_workflow(self, mock_client_with_server, mock_lightrag_server):
        """Test query workflow with document insertion and querying."""
        with patch('daniel_lightrag_mcp.server.lightrag_client') as mock_client:
            # Setup mock client
            mock_client.return_value = mock_client_with_server
            mock_client.insert_text = mock_client_with_server.insert_text
            mock_client.query_text = mock_client_with_server.query_text
            
            # Step 1: Insert test documents
            documents = [
                "Artificial intelligence is transforming the world.",
                "Machine learning algorithms are powerful tools.",
                "Natural language processing enables human-computer interaction."
            ]
            
            for doc_content in documents:
                insert_result = await handle_call_tool("insert_text", {"text": doc_content})
                assert insert_result["status"] == "success"
            
            # Step 2: Query for documents about "artificial intelligence"
            query_result = await handle_call_tool(
                "query_text",
                {
                    "query": "artificial intelligence",
                    "mode": "hybrid",
                    "only_need_context": False
                }
            )
            assert query_result["query"] == "artificial intelligence"
            assert query_result["total_results"] == 1
            assert len(query_result["results"]) == 1
            assert "artificial intelligence" in query_result["results"][0]["snippet"].lower()
            
            # Step 3: Query for documents about "machine learning"
            query_result2 = await handle_call_tool(
                "query_text",
                {
                    "query": "machine learning",
                    "mode": "local"
                }
            )
            assert query_result2["total_results"] == 1
            assert "machine learning" in query_result2["results"][0]["snippet"].lower()
    
    @pytest.mark.asyncio
    async def test_system_management_workflow(self, mock_client_with_server):
        """Test system management workflow."""
        with patch('daniel_lightrag_mcp.server.lightrag_client') as mock_client:
            # Setup mock client
            mock_client.return_value = mock_client_with_server
            mock_client.get_health = mock_client_with_server.get_health
            mock_client.clear_cache = mock_client_with_server.clear_cache
            
            # Step 1: Check system health
            health_result = await handle_call_tool("get_health", {})
            assert health_result["status"] == "healthy"
            assert health_result["database_status"] == "connected"
            assert health_result["cache_status"] == "active"
            
            # Step 2: Clear cache
            clear_result = await handle_call_tool("clear_cache", {})
            assert clear_result["cleared"] is True
            assert clear_result["cache_type"] == "all"
            
            # Step 3: Check health again to verify cache status changed
            health_result2 = await handle_call_tool("get_health", {})
            assert health_result2["cache_status"] == "cleared"


@pytest.mark.asyncio
class TestErrorScenarios:
    """Test error handling scenarios in integration context."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_connection_error_handling(self, mock_client):
        """Test handling of connection errors during tool execution."""
        # Setup mock client to raise connection error
        mock_client.get_health = AsyncMock(
            side_effect=LightRAGConnectionError("Connection failed to LightRAG server")
        )
        
        # Execute health check - connection errors propagate as raises
        with pytest.raises(LightRAGConnectionError, match="Connection failed"):
            await handle_call_tool("get_health", {})
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_validation_error_handling(self, mock_client):
        """Test handling of validation errors during tool execution."""
        # Test with invalid pagination parameters - validation errors propagate as raises
        with pytest.raises(LightRAGValidationError, match="Page must be a positive integer"):
            await handle_call_tool("get_documents_paginated", {"page": 0, "page_size": 10})
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_api_error_handling(self, mock_client):
        """Test handling of API errors during tool execution."""
        # Setup mock client to raise API error
        mock_client.delete_document = AsyncMock(
            side_effect=LightRAGAPIError("Document not found", status_code=404)
        )
        
        # Execute document deletion - API errors propagate as raises
        with pytest.raises(LightRAGAPIError, match="Document not found"):
            await handle_call_tool("delete_document", {"document_id": "nonexistent_doc"})


@pytest.mark.asyncio
class TestStreamingIntegration:
    """Test streaming functionality in integration context."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_streaming_query_integration(self, mock_client):
        """Test streaming query functionality."""
        # Setup mock streaming response
        async def mock_stream():
            chunks = [
                "This is the first chunk of the response. ",
                "Here comes the second chunk with more information. ",
                "And finally, the third chunk completes the answer."
            ]
            for chunk in chunks:
                yield chunk
        
        # Setup mock client with proper async generator
        async def mock_stream_func(query, mode="hybrid", only_need_context=False):
            chunks = [
                "This is the first chunk of the response. ",
                "Here comes the second chunk with more information. ",
                "And finally, the third chunk completes the answer."
            ]
            for chunk in chunks:
                yield chunk
        
        mock_client.query_text_stream = mock_stream_func
        
        # Execute streaming query
        result = await handle_call_tool(
            "query_text_stream",
            {
                "query": "Tell me about artificial intelligence",
                "mode": "hybrid"
            }
        )
        
        # Verify successful streaming response
        response_data = json.loads(result.content[0].text)
        assert "streaming_response" in response_data
        
        # Verify all chunks were collected
        full_response = response_data["streaming_response"]
        assert "first chunk" in full_response
        assert "second chunk" in full_response
        assert "third chunk" in full_response
        
        # Verify streaming worked correctly (chunks were collected)
        assert full_response == "This is the first chunk of the response. Here comes the second chunk with more information. And finally, the third chunk completes the answer."


@pytest.mark.asyncio
class TestConcurrentOperations:
    """Test concurrent operations and race conditions."""
    
    @patch('daniel_lightrag_mcp.server.lightrag_client')
    async def test_concurrent_document_operations(self, mock_client, mock_lightrag_server):
        """Test concurrent document operations."""
        # Setup mock client
        async def mock_insert_text(content, title=None):
            await asyncio.sleep(0.01)  # Simulate network delay
            return mock_lightrag_server.insert_text(content, title)
        
        mock_client.insert_text = mock_insert_text
        
        # Execute multiple concurrent insertions
        tasks = []
        for i in range(5):
            tasks.append(
                handle_call_tool("insert_text", {"text": f"Concurrent document {i}"})
            )
        
        # Wait for all operations to complete
        results = await asyncio.gather(*tasks)
        
        # Verify all operations succeeded
        for result in results:
            assert result["status"] == "success"
        
        # Verify all documents were inserted
        assert len(mock_lightrag_server.documents) == 5


@pytest.mark.asyncio
class TestToolListingIntegration:
    """Test tool listing functionality in integration context."""
    
    async def test_tool_listing_completeness(self):
        """Test that all required tools are listed with correct schemas."""
        tools = await handle_list_tools()
        
        # handle_list_tools returns a list of Tool objects directly
        assert len(tools) == 22
        
        # Group tools by category and verify counts
        tool_names = [tool.name for tool in tools]
        
        document_tools = [name for name in tool_names if name in [
            "insert_text", "insert_texts", "upload_document", "scan_documents",
            "get_documents", "get_documents_paginated", "delete_document"
        ]]
        assert len(document_tools) == 7
        
        query_tools = [name for name in tool_names if name in [
            "query_text", "query_text_stream", "get_related_files"
        ]]
        assert len(query_tools) == 3
        
        graph_tools = [name for name in tool_names if name in [
            "get_knowledge_graph", "get_graph_labels", "check_entity_exists",
            "update_entity", "update_relation", "update_relations",
            "delete_entity", "delete_relation"
        ]]
        assert len(graph_tools) == 8
        
        system_tools = [name for name in tool_names if name in [
            "get_pipeline_status", "get_track_status", "get_document_status_counts",
            "get_health"
        ]]
        assert len(system_tools) == 4
        
        # Verify each tool has proper schema structure
        for tool in tools:
            assert tool.name is not None
            assert tool.description is not None
            assert tool.inputSchema is not None
            assert isinstance(tool.inputSchema, dict)
            assert "type" in tool.inputSchema
            assert tool.inputSchema["type"] == "object"
            assert "properties" in tool.inputSchema
            assert "required" in tool.inputSchema
    
    async def test_tool_schema_validation(self):
        """Test that tool schemas are valid and complete."""
        tools = await handle_list_tools()
        
        # Find specific tools and verify their schemas
        tools_by_name = {tool.name: tool for tool in tools}
        
        # Test insert_text schema
        insert_text_tool = tools_by_name["insert_text"]
        schema = insert_text_tool.inputSchema
        assert "text" in schema["properties"]
        assert schema["properties"]["text"]["type"] == "string"
        assert "text" in schema["required"]
        
        # Test query_text schema
        query_text_tool = tools_by_name["query_text"]
        schema = query_text_tool.inputSchema
        assert "query" in schema["properties"]
        assert "mode" in schema["properties"]
        assert schema["properties"]["mode"]["enum"] == ["naive", "local", "global", "hybrid"]
        assert "query" in schema["required"]
        
        # Test get_documents_paginated schema
        paginated_tool = tools_by_name["get_documents_paginated"]
        schema = paginated_tool.inputSchema
        assert "page" in schema["properties"]
        assert "page_size" in schema["properties"]
        assert schema["properties"]["page"]["minimum"] == 1
        assert schema["properties"]["page_size"]["maximum"] == 100