"""Tests for in-memory storage backends: JsonKVStorage, JsonDocStatusStorage, NetworkXStorage."""

import os
import shutil
import pytest
import numpy as np
from unittest.mock import AsyncMock

from lightrag.base import DocStatus, DocProcessingStatus
from lightrag.types import KnowledgeGraph
from lightrag.kg.shared_storage import initialize_share_data, finalize_share_data


# ---------------------------------------------------------------------------
# Session-scoped shared data init (idempotent)
# ---------------------------------------------------------------------------
@pytest.fixture(scope="session", autouse=True)
def _shared_data():
    initialize_share_data()
    yield
    finalize_share_data()


# ---------------------------------------------------------------------------
# Common helpers
# ---------------------------------------------------------------------------
async def _mock_embedding(texts):
    """Return random 10‑dim vectors."""
    return np.random.rand(len(texts), 10)


def _global_config(tmp_path):
    return {
        "embedding_batch_num": 10,
        "working_dir": str(tmp_path),
        "vector_db_storage_cls_kwargs": {
            "cosine_better_than_threshold": 0.5,
        },
    }


# ===================================================================
# TestJsonKVStorage
# ===================================================================
class TestJsonKVStorage:
    @pytest.fixture
    async def storage(self, tmp_path):
        from lightrag.kg.json_kv_impl import JsonKVStorage

        s = JsonKVStorage(
            namespace="test_kv",
            workspace="",
            global_config=_global_config(tmp_path),
            embedding_func=AsyncMock(),
        )
        await s.initialize()
        yield s
        await s.drop()

    @pytest.mark.asyncio
    async def test_upsert_and_get_by_id(self, storage):
        await storage.upsert({"key1": {"value": "hello", "tag": "test"}})
        result = await storage.get_by_id("key1")
        assert result is not None
        assert result["value"] == "hello"
        assert result["tag"] == "test"
        assert result["_id"] == "key1"
        assert "create_time" in result
        assert "update_time" in result

    @pytest.mark.asyncio
    async def test_get_by_id_not_found(self, storage):
        result = await storage.get_by_id("nonexistent")
        assert result is None

    @pytest.mark.asyncio
    async def test_filter_keys(self, storage):
        await storage.upsert({"exists": {"data": "yes"}})
        keys = {"exists", "missing_a", "missing_b"}
        missing = await storage.filter_keys(keys)
        assert missing == {"missing_a", "missing_b"}

    @pytest.mark.asyncio
    async def test_delete(self, storage):
        await storage.upsert({"key_del": {"x": 1}})
        assert await storage.get_by_id("key_del") is not None
        await storage.delete(["key_del"])
        assert await storage.get_by_id("key_del") is None

    @pytest.mark.asyncio
    async def test_drop(self, storage):
        await storage.upsert({"k1": {"a": 1}, "k2": {"b": 2}})
        result = await storage.drop()
        assert result["status"] == "success"
        assert await storage.is_empty()

    @pytest.mark.asyncio
    async def test_get_by_id_and_fields(self, storage):
        await storage.upsert({"key1": {"name": "Alice", "age": 30, "email": "a@b.com"}})
        result = await storage.get_by_id("key1")
        assert result["name"] == "Alice"
        assert result["age"] == 30
        assert result["email"] == "a@b.com"

    @pytest.mark.asyncio
    async def test_upsert_overwrite(self, storage):
        await storage.upsert({"key1": {"value": "first"}})
        first = await storage.get_by_id("key1")
        assert first["value"] == "first"

        await storage.upsert({"key1": {"value": "second"}})
        second = await storage.get_by_id("key1")
        assert second["value"] == "second"
        assert second["_id"] == "key1"


# ===================================================================
# TestJsonDocStatusStorage
# ===================================================================
class TestJsonDocStatusStorage:
    @pytest.fixture
    async def storage(self, tmp_path):
        from lightrag.kg.json_doc_status_impl import JsonDocStatusStorage

        s = JsonDocStatusStorage(
            namespace="test_doc_status",
            workspace="",
            global_config=_global_config(tmp_path),
            embedding_func=AsyncMock(),
        )
        await s.initialize()
        yield s
        await s.drop()

    @pytest.mark.asyncio
    async def test_get_docs_by_status(self, storage):
        now = "2025-01-01T00:00:00"
        await storage.upsert({
            "doc1": {
                "content_summary": "Doc 1",
                "content_length": 10,
                "file_path": "/d1.txt",
                "status": DocStatus.PENDING.value,
                "created_at": now,
                "updated_at": now,
            },
            "doc2": {
                "content_summary": "Doc 2",
                "content_length": 20,
                "file_path": "/d2.txt",
                "status": DocStatus.PROCESSED.value,
                "created_at": now,
                "updated_at": now,
            },
            "doc3": {
                "content_summary": "Doc 3",
                "content_length": 30,
                "file_path": "/d3.txt",
                "status": DocStatus.PENDING.value,
                "created_at": now,
                "updated_at": now,
            },
        })

        pending_docs = await storage.get_docs_by_status(DocStatus.PENDING)
        assert len(pending_docs) == 2
        assert "doc1" in pending_docs
        assert "doc3" in pending_docs
        assert isinstance(pending_docs["doc1"], DocProcessingStatus)
        assert pending_docs["doc1"].status == DocStatus.PENDING

        processed_docs = await storage.get_docs_by_status(DocStatus.PROCESSED)
        assert len(processed_docs) == 1
        assert "doc2" in processed_docs

        failed_docs = await storage.get_docs_by_status(DocStatus.FAILED)
        assert len(failed_docs) == 0

    @pytest.mark.asyncio
    async def test_doc_status_init(self, storage):
        assert await storage.is_empty()

        now = "2025-01-01T00:00:00"
        await storage.upsert({
            "doc_a": {
                "content_summary": "Init doc",
                "content_length": 5,
                "file_path": "/init.txt",
                "status": DocStatus.PENDING.value,
                "created_at": now,
                "updated_at": now,
            },
        })

        raw = await storage.get_by_id("doc_a")
        assert raw is not None
        assert raw["status"] == DocStatus.PENDING.value
        assert raw["chunks_list"] == []

        pending = await storage.get_docs_by_status(DocStatus.PENDING)
        assert "doc_a" in pending


# ===================================================================
# TestNetworkXStorage
# ===================================================================
class TestNetworkXStorage:
    @pytest.fixture
    async def storage(self, tmp_path):
        from lightrag.kg.networkx_impl import NetworkXStorage

        s = NetworkXStorage(
            namespace="test_nx",
            workspace="",
            global_config=_global_config(tmp_path),
            embedding_func=_mock_embedding,
        )
        await s.initialize()
        yield s
        await s.drop()

    # -- node CRUD -----------------------------------------------------------
    @pytest.mark.asyncio
    async def test_upsert_node_and_get_node(self, storage):
        await storage.upsert_node("A", {"entity_type": "concept"})
        node = await storage.get_node("A")
        assert node is not None
        assert node["entity_type"] == "concept"

        assert await storage.get_node("NONEXIST") is None

    @pytest.mark.asyncio
    async def test_has_node(self, storage):
        await storage.upsert_node("X", {})
        assert await storage.has_node("X") is True
        assert await storage.has_node("Y") is False

    # -- edge CRUD + undirected property ------------------------------------
    @pytest.mark.asyncio
    async def test_upsert_edge_and_get_edge(self, storage):
        await storage.upsert_node("A", {})
        await storage.upsert_node("B", {})
        await storage.upsert_edge("A", "B", {"weight": 1.0, "relation": "knows"})

        edge_ab = await storage.get_edge("A", "B")
        assert edge_ab is not None
        assert edge_ab["weight"] == 1.0
        assert edge_ab["relation"] == "knows"

        edge_ba = await storage.get_edge("B", "A")
        assert edge_ba is not None
        assert edge_ba["weight"] == 1.0
        assert edge_ba["relation"] == "knows"

    @pytest.mark.asyncio
    async def test_has_edge(self, storage):
        await storage.upsert_node("A", {})
        await storage.upsert_node("B", {})
        await storage.upsert_edge("A", "B", {})

        assert await storage.has_edge("A", "B") is True
        assert await storage.has_edge("B", "A") is True
        assert await storage.has_edge("A", "C") is False

    @pytest.mark.asyncio
    async def test_node_degree(self, storage):
        await storage.upsert_node("A", {})
        await storage.upsert_node("B", {})
        await storage.upsert_node("C", {})
        await storage.upsert_edge("A", "B", {})
        await storage.upsert_edge("A", "C", {})

        assert await storage.node_degree("A") == 2
        assert await storage.node_degree("B") == 1
        assert await storage.node_degree("C") == 1

    @pytest.mark.asyncio
    async def test_delete_node(self, storage):
        await storage.upsert_node("DEL", {"x": 1})
        assert await storage.has_node("DEL") is True

        await storage.delete_node("DEL")
        assert await storage.has_node("DEL") is False
        assert await storage.get_node("DEL") is None

    @pytest.mark.asyncio
    async def test_get_all_labels(self, storage):
        await storage.upsert_node("banana", {})
        await storage.upsert_node("apple", {})
        await storage.upsert_node("cherry", {})

        labels = await storage.get_all_labels()
        assert labels == ["apple", "banana", "cherry"]

    @pytest.mark.asyncio
    async def test_get_knowledge_graph(self, storage):
        await storage.upsert_node("A", {"entity_type": "type_a"})
        await storage.upsert_node("B", {"entity_type": "type_b"})
        await storage.upsert_node("C", {"entity_type": "type_c"})
        await storage.upsert_edge("A", "B", {"relation": "r1"})
        await storage.upsert_edge("B", "C", {"relation": "r2"})

        kg = await storage.get_knowledge_graph("*", max_depth=5, max_nodes=10)
        assert isinstance(kg, KnowledgeGraph)

        node_ids = {n.id for n in kg.nodes}
        assert node_ids == {"A", "B", "C"}

        edge_ids = {e.id for e in kg.edges}
        assert "A-B" in edge_ids
        assert "B-C" in edge_ids

    @pytest.mark.asyncio
    async def test_get_nodes_batch(self, storage):
        await storage.upsert_node("A", {"desc": "first"})
        await storage.upsert_node("B", {"desc": "second"})
        await storage.upsert_node("C", {"desc": "third"})

        batch = await storage.get_nodes_batch(["A", "B"])
        assert set(batch.keys()) == {"A", "B"}
        assert batch["A"]["desc"] == "first"
        assert batch["B"]["desc"] == "second"

        batch_unknown = await storage.get_nodes_batch(["A", "NONEXIST"])
        assert "A" in batch_unknown
        assert "NONEXIST" not in batch_unknown
