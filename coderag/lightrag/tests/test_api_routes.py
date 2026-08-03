"""
Tests for LightRAG API routes.

Tests the FastAPI route handlers for graph and document management endpoints,
using mocked RAG instances to avoid requiring actual storage backends.
"""

import os
import sys
from unittest.mock import AsyncMock, patch

import pytest
from fastapi import FastAPI
from fastapi.routing import APIRouter
from fastapi.testclient import TestClient
from pathlib import Path

# ----  Block argparse from consuming pytest arguments  ----
# lightrag/api/auth.py calls get_global_args() → parse_args() at import time.
# We must isolate the real argv and set env defaults so argparse does not fail.
_orig_argv = sys.argv
sys.argv = ["lightrag_server.py"]

# Required env defaults so parse_args() does not try to read missing files
os.environ.setdefault("LLM_BINDING", "ollama")
os.environ.setdefault("EMBEDDING_BINDING", "ollama")

from lightrag.api.routers.graph_routes import create_graph_routes
from lightrag.api.routers.document_routes import create_document_routes, DocumentManager
from lightrag.base import DeletionResult

# Restore original argv so pytest can continue
sys.argv = _orig_argv


async def _noop_auth():
    pass


def _make_rag_mock():
    rag = AsyncMock()
    rag.chunk_entity_relation_graph = AsyncMock()
    rag.acreate_entity = AsyncMock()
    rag.aedit_entity = AsyncMock()
    rag.acreate_relation = AsyncMock()
    rag.aedit_relation = AsyncMock()
    rag.adelete_by_entity = AsyncMock()
    rag.adelete_by_relation = AsyncMock()
    rag.get_knowledge_graph = AsyncMock()
    return rag


@pytest.fixture
def mock_rag():
    return _make_rag_mock()


@pytest.fixture
def client(mock_rag, tmp_path):
    input_dir = tmp_path / "inputs"
    input_dir.mkdir()
    doc_manager = DocumentManager(str(input_dir))

    # Fresh router per test to prevent module-level state leak
    fresh_graph_router = APIRouter(tags=["graph"])
    fresh_doc_router = APIRouter(prefix="/documents", tags=["documents"])

    with (
        patch(
            "lightrag.api.routers.graph_routes.router",
            fresh_graph_router,
        ),
        patch(
            "lightrag.api.routers.graph_routes.get_combined_auth_dependency",
            return_value=_noop_auth,
        ),
        patch(
            "lightrag.api.routers.document_routes.router",
            fresh_doc_router,
        ),
        patch(
            "lightrag.api.routers.document_routes.get_combined_auth_dependency",
            return_value=_noop_auth,
        ),
    ):
        app = FastAPI()

        @app.get("/health")
        async def health():
            return {"status": "healthy"}

        @app.get("/api/health")
        async def api_health():
            return {"status": "healthy"}

        app.include_router(create_graph_routes(mock_rag))
        app.include_router(create_document_routes(mock_rag, doc_manager))

        return TestClient(app)


class TestHealth:
    def test_health_endpoint(self, client):
        response = client.get("/health")
        assert response.status_code == 200
        assert response.json()["status"] == "healthy"

    def test_api_health_endpoint(self, client):
        response = client.get("/api/health")
        assert response.status_code == 200
        assert response.json()["status"] == "healthy"


class TestEntityExists:
    def test_entity_exists_true(self, client, mock_rag):
        mock_rag.chunk_entity_relation_graph.has_node = AsyncMock(return_value=True)
        response = client.get("/graph/entity/exists", params={"name": "TestEntity"})
        assert response.status_code == 200
        assert response.json() == {"exists": True}

    def test_entity_exists_false(self, client, mock_rag):
        mock_rag.chunk_entity_relation_graph.has_node = AsyncMock(return_value=False)
        response = client.get(
            "/graph/entity/exists", params={"name": "NonExistent"}
        )
        assert response.status_code == 200
        assert response.json() == {"exists": False}


class TestCreateEntity:
    def test_create_entity_success(self, client, mock_rag):
        mock_rag.acreate_entity.return_value = {
            "entity_name": "NewEntity",
            "description": "A test entity",
            "entity_type": "PERSON",
        }
        response = client.post(
            "/graph/entity/create",
            json={
                "entity_name": "NewEntity",
                "entity_data": {
                    "description": "A test entity",
                    "entity_type": "PERSON",
                },
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "success"
        assert "NewEntity" in data["message"]
        assert data["data"]["entity_name"] == "NewEntity"

    def test_create_entity_duplicate(self, client, mock_rag):
        mock_rag.acreate_entity.side_effect = ValueError("Entity 'Duplicate' already exists")
        response = client.post(
            "/graph/entity/create",
            json={
                "entity_name": "Duplicate",
                "entity_data": {"description": "Already present"},
            },
        )
        assert response.status_code == 400
        assert "already exists" in response.text.lower()


class TestEditEntity:
    def test_edit_entity_success(self, client, mock_rag):
        mock_rag.aedit_entity.return_value = {
            "entity_name": "EditedEntity",
            "description": "Updated description",
        }
        response = client.post(
            "/graph/entity/edit",
            json={
                "entity_name": "EditedEntity",
                "updated_data": {"description": "Updated description"},
                "allow_rename": False,
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "success"
        assert data["data"]["description"] == "Updated description"

    def test_edit_entity_not_found(self, client, mock_rag):
        mock_rag.aedit_entity.side_effect = ValueError("Entity 'Ghost' not found")
        response = client.post(
            "/graph/entity/edit",
            json={
                "entity_name": "Ghost",
                "updated_data": {"description": "Nope"},
            },
        )
        assert response.status_code == 400


class TestCreateRelation:
    def test_create_relation_success(self, client, mock_rag):
        mock_rag.acreate_relation.return_value = {
            "src_id": "Alice",
            "tgt_id": "Bob",
            "description": "knows",
        }
        response = client.post(
            "/graph/relation/create",
            json={
                "source_entity": "Alice",
                "target_entity": "Bob",
                "relation_data": {
                    "description": "knows",
                    "keywords": "friend",
                    "weight": 1.0,
                },
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "success"
        assert "Alice" in data["message"]
        assert "Bob" in data["message"]

    def test_create_relation_missing_entity(self, client, mock_rag):
        mock_rag.acreate_relation.side_effect = ValueError(
            "Source entity 'Alice' not found"
        )
        response = client.post(
            "/graph/relation/create",
            json={
                "source_entity": "Alice",
                "target_entity": "Bob",
                "relation_data": {"description": "knows"},
            },
        )
        assert response.status_code == 400


class TestEditRelation:
    def test_edit_relation_success(self, client, mock_rag):
        mock_rag.aedit_relation.return_value = {
            "src_id": "Alice",
            "tgt_id": "Bob",
            "description": "updated relation",
        }
        response = client.post(
            "/graph/relation/edit",
            json={
                "source_id": "Alice",
                "target_id": "Bob",
                "updated_data": {"description": "updated relation"},
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "success"
        assert data["data"]["description"] == "updated relation"

    def test_edit_relation_not_found(self, client, mock_rag):
        mock_rag.aedit_relation.side_effect = ValueError("Relation not found")
        response = client.post(
            "/graph/relation/edit",
            json={
                "source_id": "Ghost",
                "target_id": "Nowhere",
                "updated_data": {"description": "Nope"},
            },
        )
        assert response.status_code == 400


def _delete_json(client, url, data):
    """Helper: DELETE with JSON body (TestClient.delete may not support json=)."""
    return client.request("DELETE", url, json=data)


class TestDeleteEntity:
    def test_delete_entity_success(self, client, mock_rag):
        mock_rag.adelete_by_entity.return_value = DeletionResult(
            status="success",
            doc_id="",
            message="Entity 'Target' deleted successfully",
        )
        response = _delete_json(
            client, "/documents/delete_entity", {"entity_name": "Target"}
        )
        assert response.status_code == 200
        assert response.json()["status"] == "success"

    def test_delete_entity_not_found(self, client, mock_rag):
        mock_rag.adelete_by_entity.return_value = DeletionResult(
            status="not_found",
            doc_id="",
            message="Entity 'Ghost' not found",
        )
        response = _delete_json(
            client, "/documents/delete_entity", {"entity_name": "Ghost"}
        )
        assert response.status_code == 404


class TestDeleteRelation:
    def test_delete_relation_success(self, client, mock_rag):
        mock_rag.adelete_by_relation.return_value = DeletionResult(
            status="success",
            doc_id="",
            message="Relation deleted successfully",
        )
        response = _delete_json(
            client,
            "/documents/delete_relation",
            {"source_entity": "Alice", "target_entity": "Bob"},
        )
        assert response.status_code == 200
        assert response.json()["status"] == "success"

    def test_delete_relation_not_found(self, client, mock_rag):
        mock_rag.adelete_by_relation.return_value = DeletionResult(
            status="not_found",
            doc_id="",
            message="Relation not found",
        )
        response = _delete_json(
            client,
            "/documents/delete_relation",
            {"source_entity": "Ghost", "target_entity": "Nowhere"},
        )
        assert response.status_code == 404


class TestGraphs:
    def test_get_graphs(self, client, mock_rag):
        expected = {
            "nodes": [{"id": "N", "label": "N"}],
            "edges": [{"source": "N", "target": "M"}],
        }
        mock_rag.get_knowledge_graph.return_value = expected

        response = client.get(
            "/graphs",
            params={"label": "N", "max_depth": 2, "max_nodes": 50},
        )
        assert response.status_code == 200
        assert response.json() == expected
        mock_rag.get_knowledge_graph.assert_called_once_with(
            node_label="N", max_depth=2, max_nodes=50
        )
