"""Test Pydantic V2 migration — ConfigDict, Optional fields."""

import sys

# Prevent argparse from choking on pytest arguments during module-level imports
sys.argv = ["lightrag-server"]

import pytest

pytest.importorskip("lightrag")


class TestPydanticV2Migration:
    def test_doc_status_response_file_path_optional(self):
        from lightrag.api.routers.document_routes import DocStatusResponse

        with_null = DocStatusResponse(
            id="doc1",
            content_summary="test",
            content_length=4,
            status="pending",
            created_at="2025-01-01",
            updated_at="2025-01-01",
            file_path=None,
        )
        assert with_null.file_path is None

        with_value = DocStatusResponse(
            id="doc2",
            content_summary="test",
            content_length=4,
            status="pending",
            created_at="2025-01-01",
            updated_at="2025-01-01",
            file_path="/some/path.txt",
        )
        assert with_value.file_path == "/some/path.txt"

    def test_scan_response_has_model_config(self):
        from lightrag.api.routers.document_routes import ScanResponse
        assert hasattr(ScanResponse, "model_config")

    def test_insert_text_request_validates(self):
        from lightrag.api.routers.document_routes import InsertTextRequest
        import pydantic

        with pytest.raises(pydantic.ValidationError):
            InsertTextRequest(text="", file_source="test.txt")

        ok = InsertTextRequest(text="hello", file_source="test.txt")
        assert ok.text == "hello"

    def test_track_status_response_deserialization(self):
        from lightrag.api.routers.document_routes import TrackStatusResponse

        data = {
            "track_id": "track_123",
            "documents": [
                {
                    "id": "doc1",
                    "content_summary": "test",
                    "content_length": 4,
                    "status": "processed",
                    "created_at": "2025-01-01",
                    "updated_at": "2025-01-01",
                    "file_path": None,
                }
            ],
            "total_count": 1,
            "status_summary": {"processed": 1},
        }
        resp = TrackStatusResponse(**data)
        assert resp.track_id == "track_123"
        assert resp.total_count == 1
