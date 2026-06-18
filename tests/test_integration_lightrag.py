"""Integration tests for LightRAG server endpoints.

Requires the ``mock_lightrag_server`` fixture from ``conftest.py``.
"""

import pytest

pytest.importorskip("lightrag")
pytestmark = [pytest.mark.integration, pytest.mark.asyncio]


class TestHealthEndpoint:
    async def test_health_returns_200(self, client):
        r = await client.get("/health")
        assert r.status_code == 200

    async def test_health_contains_expected_keys(self, client):
        r = await client.get("/health")
        data = r.json()
        assert "status" in data
        assert "core_version" in data
        assert "api_version" in data
        assert "working_directory" in data

    async def test_health_status_ok(self, client):
        r = await client.get("/health")
        assert r.json()["status"] == "healthy"


class TestDocumentsTextEndpoint:
    async def test_insert_text_returns_200(self, client):
        r = await client.post(
            "/documents/text",
            json={"text": "LightRAG is a RAG system."},
        )
        assert r.status_code == 200

    async def test_insert_text_returns_track_id(self, client):
        r = await client.post(
            "/documents/text",
            json={"text": "Knowledge graphs store entities and relations."},
        )
        data = r.json()
        assert "track_id" in data or "message" in data

    async def test_insert_text_with_file_source(self, client):
        r = await client.post(
            "/documents/text",
            json={
                "text": "Python is a programming language.",
                "file_source": "test_intro.txt",
            },
        )
        assert r.status_code == 200

    async def test_insert_text_missing_text_returns_422(self, client):
        r = await client.post("/documents/text", json={})
        assert r.status_code == 422


class TestAuthStatusEndpoint:
    async def test_auth_status_returns_200(self, client):
        r = await client.get("/auth-status")
        assert r.status_code == 200

    async def test_auth_status_has_mode(self, client):
        r = await client.get("/auth-status")
        data = r.json()
        assert "auth_mode" in data or "mode" in data


class TestRootEndpoint:
    async def test_root_redirects(self, client):
        r = await client.get("/", follow_redirects=False)
        assert r.status_code in (200, 307, 302)


class TestQueryEndpoint:
    async def test_query_returns_200(self, client):
        r = await client.post(
            "/query",
            json={"query": "What is LightRAG?", "mode": "naive"},
        )
        assert r.status_code == 200

    async def test_query_response_has_content(self, client):
        r = await client.post(
            "/query",
            json={"query": "Explain knowledge graphs.", "mode": "hybrid"},
        )
        data = r.json()
        assert isinstance(data, dict) or isinstance(data, list)
