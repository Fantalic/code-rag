import os
import sys

sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import pytest
from unittest.mock import patch, MagicMock

from lightrag.lightrag import LightRAG
from lightrag.base import QueryParam
from lightrag.kg import STORAGE_IMPLEMENTATIONS, verify_storage_implementation


@patch("lightrag.kg.json_doc_status_impl.JsonDocStatusStorage")
@patch("lightrag.kg.networkx_impl.NetworkXStorage")
@patch("lightrag.kg.nano_vector_db_impl.NanoVectorDBStorage")
@patch("lightrag.kg.json_kv_impl.JsonKVStorage")
@patch("lightrag.kg.shared_storage.initialize_share_data")
@patch("lightrag.lightrag.os.makedirs")
@patch("lightrag.lightrag.os.path.exists", return_value=True)
@patch("lightrag.lightrag.TiktokenTokenizer")
@patch("lightrag.lightrag.priority_limit_async_func_call")
@patch("lightrag.lightrag.check_storage_env_vars")
@patch("lightrag.lightrag.verify_storage_implementation")
class TestLightRAGInit:
    """Test LightRAG initialization and default parameter handling"""

    def test_default_init(
        self,
        mock_verify,
        mock_check_env,
        mock_priority,
        mock_tokenizer,
        mock_exists,
        mock_makedirs,
        mock_share_data,
        mock_json_kv,
        mock_nano,
        mock_networkx,
        mock_doc_status,
    ):
        mock_priority.return_value = lambda f: f
        mock_tokenizer.return_value = MagicMock()

        rag = LightRAG(working_dir="./test_rag", llm_model_func=MagicMock())

        assert rag.working_dir == "./test_rag"
        assert rag.kv_storage == "JsonKVStorage"
        assert rag.vector_storage == "NanoVectorDBStorage"
        assert rag.graph_storage == "NetworkXStorage"
        assert rag.doc_status_storage == "JsonDocStatusStorage"
        assert rag.llm_model_name == "gpt-4o-mini"
        assert rag.tiktoken_model_name == "gpt-4o-mini"
        assert rag.chunk_token_size == 1200
        assert rag.chunk_overlap_token_size == 100
        assert rag.enable_llm_cache is True
        assert rag.max_parallel_insert == 2
        assert rag.embedding_func_max_async == 8
        assert rag.embedding_batch_num == 10
        assert rag.auto_manage_storages_states is False
        assert rag.max_graph_nodes == 0
        assert rag.cosine_better_than_threshold == 0.2

    def test_custom_params(
        self,
        mock_verify,
        mock_check_env,
        mock_priority,
        mock_tokenizer,
        mock_exists,
        mock_makedirs,
        mock_share_data,
        mock_json_kv,
        mock_nano,
        mock_networkx,
        mock_doc_status,
    ):
        mock_priority.return_value = lambda f: f
        mock_tokenizer.return_value = MagicMock()

        rag = LightRAG(
            working_dir="./custom_rag",
            llm_model_func=MagicMock(),
            llm_model_name="gpt-4o",
            llm_model_kwargs={"temperature": 0.7, "max_tokens": 2048},
            chunk_token_size=512,
            chunk_overlap_token_size=50,
            top_k=20,
            max_parallel_insert=5,
            enable_llm_cache=False,
            embedding_batch_num=32,
            embedding_func_max_async=16,
            max_graph_nodes=100,
            cosine_better_than_threshold=0.5,
        )

        assert rag.working_dir == "./custom_rag"
        assert rag.llm_model_name == "gpt-4o"
        assert rag.llm_model_kwargs == {"temperature": 0.7, "max_tokens": 2048}
        assert rag.chunk_token_size == 512
        assert rag.chunk_overlap_token_size == 50
        assert rag.top_k == 20
        assert rag.max_parallel_insert == 5
        assert rag.enable_llm_cache is False
        assert rag.embedding_batch_num == 32
        assert rag.embedding_func_max_async == 16
        assert rag.max_graph_nodes == 100
        assert rag.cosine_better_than_threshold == 0.5

    def test_init_without_working_dir(
        self,
        mock_verify,
        mock_check_env,
        mock_priority,
        mock_tokenizer,
        mock_exists,
        mock_makedirs,
        mock_share_data,
        mock_json_kv,
        mock_nano,
        mock_networkx,
        mock_doc_status,
    ):
        mock_priority.return_value = lambda f: f
        mock_tokenizer.return_value = MagicMock()

        rag = LightRAG(llm_model_func=MagicMock())

        assert rag.working_dir == "./rag_storage"
        assert rag.kv_storage == "JsonKVStorage"
        assert rag.vector_storage == "NanoVectorDBStorage"
        assert rag.graph_storage == "NetworkXStorage"
        assert rag.doc_status_storage == "JsonDocStatusStorage"


@patch("lightrag.kg.json_doc_status_impl.JsonDocStatusStorage")
@patch("lightrag.kg.networkx_impl.NetworkXStorage")
@patch("lightrag.kg.nano_vector_db_impl.NanoVectorDBStorage")
@patch("lightrag.kg.json_kv_impl.JsonKVStorage")
@patch("lightrag.kg.shared_storage.initialize_share_data")
@patch("lightrag.lightrag.os.makedirs")
@patch("lightrag.lightrag.os.path.exists", return_value=True)
@patch("lightrag.lightrag.TiktokenTokenizer")
@patch("lightrag.lightrag.priority_limit_async_func_call")
@patch("lightrag.lightrag.check_storage_env_vars")
@patch("lightrag.lightrag.verify_storage_implementation")
class TestLightRAGConfig:
    """Test LightRAG configuration parameters relevant to insert and query"""

    def test_insert_config(
        self,
        mock_verify,
        mock_check_env,
        mock_priority,
        mock_tokenizer,
        mock_exists,
        mock_makedirs,
        mock_share_data,
        mock_json_kv,
        mock_nano,
        mock_networkx,
        mock_doc_status,
    ):
        mock_priority.return_value = lambda f: f
        mock_tokenizer.return_value = MagicMock()

        rag = LightRAG(
            working_dir="./test_rag",
            llm_model_func=MagicMock(),
            chunk_token_size=1024,
            chunk_overlap_token_size=200,
            max_parallel_insert=4,
            embedding_batch_num=64,
            embedding_func_max_async=12,
        )

        assert rag.chunk_token_size == 1024
        assert rag.chunk_overlap_token_size == 200
        assert rag.max_parallel_insert == 4
        assert rag.embedding_batch_num == 64
        assert rag.embedding_func_max_async == 12

    def test_query_config(
        self,
        mock_verify,
        mock_check_env,
        mock_priority,
        mock_tokenizer,
        mock_exists,
        mock_makedirs,
        mock_share_data,
        mock_json_kv,
        mock_nano,
        mock_networkx,
        mock_doc_status,
    ):
        mock_priority.return_value = lambda f: f
        mock_tokenizer.return_value = MagicMock()

        rag = LightRAG(
            working_dir="./test_rag",
            llm_model_func=MagicMock(),
            top_k=30,
            chunk_top_k=15,
            max_entity_tokens=4000,
            max_relation_tokens=6000,
            max_total_tokens=20000,
            cosine_threshold=0.3,
        )

        assert rag.top_k == 30
        assert rag.chunk_top_k == 15
        assert rag.max_entity_tokens == 4000
        assert rag.max_relation_tokens == 6000
        assert rag.max_total_tokens == 20000
        assert rag.cosine_threshold == 0.3

        param = QueryParam()
        assert hasattr(param, "mode")
        assert hasattr(param, "top_k")
        assert hasattr(param, "only_need_context")
        assert hasattr(param, "stream")


class TestStorageVerification:
    """Test storage implementation enumeration and verification"""

    def test_storage_implementations(self):
        assert "KV_STORAGE" in STORAGE_IMPLEMENTATIONS
        assert "GRAPH_STORAGE" in STORAGE_IMPLEMENTATIONS
        assert "VECTOR_STORAGE" in STORAGE_IMPLEMENTATIONS
        assert "DOC_STATUS_STORAGE" in STORAGE_IMPLEMENTATIONS

        kv = STORAGE_IMPLEMENTATIONS["KV_STORAGE"]["implementations"]
        assert "JsonKVStorage" in kv

        graph = STORAGE_IMPLEMENTATIONS["GRAPH_STORAGE"]["implementations"]
        assert "NetworkXStorage" in graph

        vector = STORAGE_IMPLEMENTATIONS["VECTOR_STORAGE"]["implementations"]
        assert "NanoVectorDBStorage" in vector

        doc = STORAGE_IMPLEMENTATIONS["DOC_STATUS_STORAGE"]["implementations"]
        assert "JsonDocStatusStorage" in doc

    def test_verify_storage_valid(self):
        verify_storage_implementation("KV_STORAGE", "JsonKVStorage")
        verify_storage_implementation("GRAPH_STORAGE", "NetworkXStorage")
        verify_storage_implementation("VECTOR_STORAGE", "NanoVectorDBStorage")
        verify_storage_implementation("DOC_STATUS_STORAGE", "JsonDocStatusStorage")

    def test_verify_storage_invalid_type(self):
        with pytest.raises(ValueError, match="Unknown storage type"):
            verify_storage_implementation("UNKNOWN_STORAGE", "Whatever")

    def test_verify_storage_incompatible_impl(self):
        with pytest.raises(ValueError, match="is not compatible"):
            verify_storage_implementation("KV_STORAGE", "Neo4JStorage")
