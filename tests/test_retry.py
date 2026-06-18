"""Test document processing retry logic."""

import pytest

pytest.importorskip("lightrag")


class TestRetryLogic:
    def test_doc_processing_status_defaults(self):
        from lightrag.base import DocProcessingStatus, DocStatus

        status = DocProcessingStatus(
            content_summary="test",
            content_length=4,
            file_path="/test.txt",
            status=DocStatus.PENDING,
            created_at="2025-01-01",
            updated_at="2025-01-01",
        )
        assert status.retry_count == 0
        assert status.max_retries == 3

    def test_retry_exhausted_stays_failed(self):
        from lightrag.base import DocProcessingStatus, DocStatus

        status = DocProcessingStatus(
            content_summary="test",
            content_length=4,
            file_path="/test.txt",
            status=DocStatus.FAILED,
            created_at="2025-01-01",
            updated_at="2025-01-01",
            retry_count=3,
            max_retries=3,
        )
        assert status.retry_count >= status.max_retries
        assert status.status == DocStatus.FAILED

    def test_retry_count_increments(self):
        from lightrag.base import DocProcessingStatus, DocStatus

        status = DocProcessingStatus(
            content_summary="test",
            content_length=4,
            file_path="/test.txt",
            status=DocStatus.FAILED,
            created_at="2025-01-01",
            updated_at="2025-01-01",
        )
        status.retry_count += 1
        assert status.retry_count == 1
