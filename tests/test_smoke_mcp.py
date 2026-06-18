import pytest

pytest.importorskip("daniel_lightrag_mcp")


class TestMCPImports:
    def test_package_version(self):
        import daniel_lightrag_mcp
        assert daniel_lightrag_mcp.__version__ == "0.1.0"

    def test_client_class(self):
        from daniel_lightrag_mcp import LightRAGClient, LightRAGError
        assert issubclass(LightRAGClient, object)
        assert issubclass(LightRAGError, Exception)

    def test_server_instance(self):
        from daniel_lightrag_mcp.server import mcp_server
        assert mcp_server is not None
        assert hasattr(mcp_server, "name")
        assert hasattr(mcp_server, "list_tools")

    def test_all_models_importable(self):
        from daniel_lightrag_mcp import (
            InsertTextRequest, InsertTextsRequest, QueryRequest,
            DeleteDocRequest, DeleteEntityRequest, DeleteRelationRequest,
            DocumentsRequest, ClearCacheRequest, EntityUpdateRequest,
            RelationUpdateRequest, EntityExistsRequest, LoginRequest,
            InsertResponse, ScanResponse, UploadResponse, DocumentInfo,
            DocumentsResponse, PaginatedDocsResponse, DeleteDocByIdResponse,
            ClearDocumentsResponse, PipelineStatusResponse,
            TrackStatusResponse, StatusCountsResponse, ClearCacheResponse,
            DeletionResult, QueryResult, QueryResponse, EntityInfo,
            RelationInfo, GraphResponse, LabelsResponse,
            EntityExistsResponse, EntityUpdateResponse,
            RelationUpdateResponse, HealthResponse, AuthStatusResponse,
            LoginResponse,
        )
        assert InsertTextRequest is not None
        assert QueryRequest is not None

    def test_enums(self):
        from daniel_lightrag_mcp import DocStatus, QueryMode, PipelineStatus
        assert DocStatus.PENDING.value == "pending"
        assert DocStatus.PROCESSED.value == "processed"
        assert QueryMode.NAIVE.value == "naive"
        assert QueryMode.HYBRID.value == "hybrid"

    def test_ollama_models(self):
        from daniel_lightrag_mcp import (
            OllamaVersionResponse, OllamaTagsResponse,
            OllamaProcessResponse, OllamaGenerateRequest,
            OllamaChatMessage, OllamaChatRequest,
        )
        assert OllamaGenerateRequest is not None


class TestMCPModels:
    def test_insert_text_request(self):
        from daniel_lightrag_mcp.models import InsertTextRequest
        req = InsertTextRequest(text="hello world")
        assert req.text == "hello world"
        assert req.file_source == "text_input.txt"

    def test_query_request_defaults(self):
        from daniel_lightrag_mcp.models import QueryRequest, QueryMode
        q = QueryRequest(query="test")
        assert q.mode == QueryMode.HYBRID
        assert q.stream is False
        assert q.only_need_context is False

    def test_health_response(self):
        from daniel_lightrag_mcp.models import HealthResponse
        h = HealthResponse(status="ok", version="0.1.0")
        assert h.status == "ok"
        assert h.version == "0.1.0"

    def test_doc_status_enum_values(self):
        from daniel_lightrag_mcp.models import DocStatus
        assert DocStatus("pending") == DocStatus.PENDING
        assert DocStatus("processed") == DocStatus.PROCESSED
        assert DocStatus("failed") == DocStatus.FAILED
