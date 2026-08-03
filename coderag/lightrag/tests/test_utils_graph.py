import pytest
from unittest.mock import AsyncMock, patch, ANY

from lightrag.utils_graph import (
    adelete_by_entity,
    adelete_by_relation,
    acreate_entity,
    acreate_relation,
    aedit_entity,
    aedit_relation,
    amerge_entities,
    _merge_entity_attributes,
    _merge_relation_attributes,
    get_entity_info,
    get_relation_info,
)
from lightrag.base import DeletionResult
from lightrag.constants import GRAPH_FIELD_SEP


class _AsyncLock:
    async def __aenter__(self):
        return self
    async def __aexit__(self, *args):
        pass


def _mock_mdhash(s, prefix=""):
    return f"{prefix}{s}"


@pytest.fixture(autouse=True)
def _mock_deps():
    with patch("lightrag.utils_graph.get_graph_db_lock") as mock_lock, \
         patch("lightrag.utils_graph.compute_mdhash_id") as mock_mdhash, \
         patch("lightrag.utils_graph.logger"):
        mock_lock.return_value = _AsyncLock()
        mock_mdhash.side_effect = _mock_mdhash
        yield


@pytest.fixture
def graph():
    return AsyncMock()


@pytest.fixture
def entities_vdb():
    return AsyncMock()


@pytest.fixture
def relationships_vdb():
    return AsyncMock()


@pytest.mark.asyncio
class TestAdeleteByEntity:
    async def test_entity_exists(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = True
        graph.get_node_edges.return_value = [("entity_A", "other")]

        result = await adelete_by_entity(
            graph, entities_vdb, relationships_vdb, "entity_A"
        )

        assert result.status == "success"
        assert result.doc_id == "entity_A"
        assert result.status_code == 200
        entities_vdb.delete_entity.assert_awaited_once_with("entity_A")
        relationships_vdb.delete_entity_relation.assert_awaited_once_with("entity_A")
        graph.delete_node.assert_awaited_once_with("entity_A")
        graph.index_done_callback.assert_awaited_once()
        entities_vdb.index_done_callback.assert_awaited_once()
        relationships_vdb.index_done_callback.assert_awaited_once()

    async def test_entity_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = False

        result = await adelete_by_entity(
            graph, entities_vdb, relationships_vdb, "missing"
        )

        assert isinstance(result, DeletionResult)
        assert result.status == "not_found"
        assert result.status_code == 404
        graph.delete_node.assert_not_awaited()
        entities_vdb.delete_entity.assert_not_awaited()

    async def test_vdb_error(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = True
        graph.get_node_edges.return_value = []
        entities_vdb.delete_entity.side_effect = Exception("VDB error")

        result = await adelete_by_entity(
            graph, entities_vdb, relationships_vdb, "entity_A"
        )

        assert result.status == "fail"
        assert result.status_code == 500

    async def test_graph_error(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = True
        graph.get_node_edges.return_value = []
        graph.delete_node.side_effect = Exception("Graph error")

        result = await adelete_by_entity(
            graph, entities_vdb, relationships_vdb, "entity_A"
        )

        assert result.status == "fail"
        assert result.status_code == 500


@pytest.mark.asyncio
class TestAdeleteByRelation:
    async def test_relation_exists(self, graph, entities_vdb, relationships_vdb):
        graph.has_edge.return_value = True

        result = await adelete_by_relation(
            graph, relationships_vdb, "src", "tgt"
        )

        assert result.status == "success"
        assert result.status_code == 200
        relationships_vdb.delete.assert_awaited_once()
        graph.remove_edges.assert_awaited_once_with([("src", "tgt")])
        relationships_vdb.index_done_callback.assert_awaited_once()
        graph.index_done_callback.assert_awaited_once()

    async def test_relation_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.has_edge.return_value = False

        result = await adelete_by_relation(
            graph, relationships_vdb, "src", "tgt"
        )

        assert result.status == "not_found"
        assert result.status_code == 404
        graph.remove_edges.assert_not_awaited()
        relationships_vdb.delete.assert_not_awaited()

    async def test_vdb_error(self, graph, entities_vdb, relationships_vdb):
        graph.has_edge.return_value = True
        relationships_vdb.delete.side_effect = Exception("VDB error")

        result = await adelete_by_relation(
            graph, relationships_vdb, "src", "tgt"
        )

        assert result.status == "fail"
        assert result.status_code == 500


@pytest.mark.asyncio
class TestAcreateEntity:
    async def test_creates_entity(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = False
        graph.get_node.return_value = {
            "description": "test entity",
            "entity_type": "TEST",
            "source_id": "manual",
        }

        result = await acreate_entity(
            graph, entities_vdb, relationships_vdb, "new_entity",
            {"description": "test entity", "entity_type": "TEST"},
        )

        assert result["entity_name"] == "new_entity"
        graph.upsert_node.assert_awaited_once()
        entities_vdb.upsert.assert_awaited_once()
        graph.index_done_callback.assert_awaited_once()
        entities_vdb.index_done_callback.assert_awaited_once()
        relationships_vdb.index_done_callback.assert_awaited_once()

    async def test_already_exists(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = True

        with pytest.raises(ValueError, match="already exists"):
            await acreate_entity(
                graph, entities_vdb, relationships_vdb, "existing",
                {"description": "test"},
            )

        graph.upsert_node.assert_not_awaited()
        entities_vdb.upsert.assert_not_awaited()

    async def test_vdb_error(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = False
        entities_vdb.upsert.side_effect = Exception("VDB error")

        with pytest.raises(Exception, match="VDB error"):
            await acreate_entity(
                graph, entities_vdb, relationships_vdb, "new_entity",
                {"description": "test"},
            )


@pytest.mark.asyncio
class TestAcreateRelation:
    async def test_creates_relation(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = True
        graph.has_edge.return_value = False
        graph.get_edge.return_value = {
            "description": "test rel",
            "keywords": "kw1,kw2",
            "source_id": "manual",
            "weight": "1.0",
        }

        result = await acreate_relation(
            graph, entities_vdb, relationships_vdb, "src", "tgt",
            {"description": "test rel", "keywords": "kw1,kw2"},
        )

        assert result["src_entity"] == "src"
        assert result["tgt_entity"] == "tgt"
        graph.upsert_edge.assert_awaited_once()
        relationships_vdb.upsert.assert_awaited_once()
        relationships_vdb.index_done_callback.assert_awaited_once()
        graph.index_done_callback.assert_awaited_once()

    async def test_source_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.side_effect = lambda name: name == "tgt"

        with pytest.raises(ValueError, match="Source entity.*does not exist"):
            await acreate_relation(
                graph, entities_vdb, relationships_vdb, "src", "tgt",
                {"description": "test"},
            )

        graph.upsert_edge.assert_not_awaited()

    async def test_target_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.side_effect = lambda name: name == "src"

        with pytest.raises(ValueError, match="Target entity.*does not exist"):
            await acreate_relation(
                graph, entities_vdb, relationships_vdb, "src", "tgt",
                {"description": "test"},
            )

        graph.upsert_edge.assert_not_awaited()

    async def test_merges_keywords(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = True
        graph.has_edge.return_value = True
        graph.get_edge.return_value = {"keywords": "existing_kw"}

        result = await acreate_relation(
            graph, entities_vdb, relationships_vdb, "src", "tgt",
            {"description": "test", "keywords": "new_kw"},
        )

        stored_data = graph.upsert_edge.await_args.args[2]
        keywords_set = set(k.strip() for k in stored_data["keywords"].split(","))
        assert keywords_set == {"existing_kw", "new_kw"}


@pytest.mark.asyncio
class TestAeditEntity:
    async def test_update_description(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = True
        graph.get_node.return_value = {
            "description": "old desc",
            "entity_type": "TEST",
            "source_id": "src",
        }

        result = await aedit_entity(
            graph, entities_vdb, relationships_vdb, "my_entity",
            {"description": "new desc"},
        )

        assert "entity_name" in result
        graph.upsert_node.assert_awaited_once_with("my_entity", ANY)
        entities_vdb.upsert.assert_awaited_once()
        graph.index_done_callback.assert_awaited_once()
        entities_vdb.index_done_callback.assert_awaited_once()
        relationships_vdb.index_done_callback.assert_awaited_once()

    async def test_rename_entity(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.side_effect = lambda name: name == "old_name"
        graph.get_node.return_value = {
            "description": "desc",
            "entity_type": "TEST",
            "source_id": "src",
        }
        graph.get_node_edges.return_value = [("old_name", "X")]
        graph.get_edge.return_value = {
            "description": "rel desc",
            "keywords": "kw",
            "source_id": "src2",
            "weight": "1.0",
        }

        result = await aedit_entity(
            graph, entities_vdb, relationships_vdb, "old_name",
            {"entity_name": "new_name", "description": "new desc"},
        )

        graph.upsert_node.assert_awaited_with("new_name", ANY)
        graph.upsert_edge.assert_awaited()
        graph.delete_node.assert_awaited_once_with("old_name")
        entities_vdb.delete.assert_awaited_once()
        relationships_vdb.delete.assert_awaited_once()
        entities_vdb.upsert.assert_awaited_once()
        relationships_vdb.upsert.assert_awaited()
        graph.index_done_callback.assert_awaited()

    async def test_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.return_value = False

        with pytest.raises(ValueError, match="does not exist"):
            await aedit_entity(
                graph, entities_vdb, relationships_vdb, "missing",
                {"description": "new"},
            )

        graph.upsert_node.assert_not_awaited()

    async def test_rename_existing_target(self, graph, entities_vdb, relationships_vdb):
        graph.has_node.side_effect = lambda name: True
        graph.get_node.return_value = {
            "description": "desc",
            "entity_type": "TEST",
            "source_id": "src",
        }

        with pytest.raises(ValueError, match="already exists"):
            await aedit_entity(
                graph, entities_vdb, relationships_vdb, "old_name",
                {"entity_name": "existing_target"},
            )


@pytest.mark.asyncio
class TestAeditRelation:
    async def test_update(self, graph, entities_vdb, relationships_vdb):
        graph.has_edge.return_value = True
        graph.get_edge.return_value = {
            "description": "old desc",
            "keywords": "kw",
            "source_id": "src",
            "weight": "1.0",
        }

        result = await aedit_relation(
            graph, entities_vdb, relationships_vdb, "src", "tgt",
            {"description": "new desc"},
        )

        assert result["src_entity"] == "src"
        assert result["tgt_entity"] == "tgt"
        relationships_vdb.delete.assert_awaited_once()
        graph.upsert_edge.assert_awaited_once_with("src", "tgt", ANY)
        relationships_vdb.upsert.assert_awaited_once()
        relationships_vdb.index_done_callback.assert_awaited_once()
        graph.index_done_callback.assert_awaited_once()

    async def test_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.has_edge.return_value = False

        with pytest.raises(ValueError, match="does not exist"):
            await aedit_relation(
                graph, entities_vdb, relationships_vdb, "src", "tgt",
                {"description": "new"},
            )

        graph.upsert_edge.assert_not_awaited()
        relationships_vdb.upsert.assert_not_awaited()


@pytest.mark.asyncio
class TestAmergeEntities:
    @pytest.fixture(autouse=True)
    def _entity_data(self):
        return {
            "A": {"description": "desc A", "entity_type": "TYPE", "source_id": "src1"},
            "B": {"description": "desc B", "entity_type": "TYPE", "source_id": "src2"},
        }

    async def test_merge_two_into_one(self, graph, entities_vdb, relationships_vdb, _entity_data):
        entities = dict(_entity_data)
        graph.has_node.side_effect = lambda name: name in entities
        graph.get_node.side_effect = lambda name: entities.get(name, {})
        graph.get_node_edges.side_effect = lambda name: {
            "A": [("A", "X")],
            "B": [("B", "Y")],
        }.get(name, [])
        graph.get_edge.side_effect = lambda src, tgt: {
            ("A", "X"): {"description": "rel A-X", "keywords": "k", "source_id": "s", "weight": "1.0"},
            ("B", "Y"): {"description": "rel B-Y", "keywords": "k", "source_id": "s", "weight": "1.0"},
        }.get((src, tgt), {})

        result = await amerge_entities(
            graph, entities_vdb, relationships_vdb,
            ["A", "B"], "T",
        )

        assert result["entity_name"] == "T"
        graph.upsert_node.assert_awaited_with("T", ANY)
        graph.delete_node.assert_any_await("A")
        graph.delete_node.assert_any_await("B")
        entities_vdb.upsert.assert_awaited()
        entities_vdb.delete.assert_awaited()
        graph.upsert_edge.assert_awaited()
        relationships_vdb.upsert.assert_awaited()
        relationships_vdb.delete.assert_awaited()
        graph.index_done_callback.assert_awaited()
        entities_vdb.index_done_callback.assert_awaited()
        relationships_vdb.index_done_callback.assert_awaited()

    async def test_target_exists(self, graph, entities_vdb, relationships_vdb, _entity_data):
        entities = dict(_entity_data)
        entities["T"] = {"description": "desc T", "entity_type": "TYPE_T", "source_id": "src_T"}
        graph.has_node.side_effect = lambda name: name in entities
        graph.get_node.side_effect = lambda name: entities.get(name, {})
        graph.get_node_edges.return_value = []
        graph.get_edge.return_value = {}

        result = await amerge_entities(
            graph, entities_vdb, relationships_vdb,
            ["A", "B"], "T",
        )

        assert result["entity_name"] == "T"
        graph.upsert_node.assert_awaited_with("T", ANY)

    async def test_self_loop_skip(self, graph, entities_vdb, relationships_vdb, _entity_data):
        entities = dict(_entity_data)
        graph.has_node.side_effect = lambda name: name in entities
        graph.get_node.side_effect = lambda name: entities.get(name, {})
        graph.get_node_edges.side_effect = lambda name: {
            "A": [("A", "B")],
            "B": [("B", "X")],
        }.get(name, [])
        graph.get_edge.side_effect = lambda src, tgt: {
            ("A", "B"): {"description": "rel A-B", "keywords": "k", "source_id": "s", "weight": "1.0"},
            ("B", "X"): {"description": "rel B-X", "keywords": "k", "source_id": "s", "weight": "1.0"},
        }.get((src, tgt), {})

        result = await amerge_entities(
            graph, entities_vdb, relationships_vdb,
            ["A", "B"], "T",
        )

        assert result["entity_name"] == "T"
        self_loop_call = False
        upsert_edge_awaits = graph.upsert_edge.await_args_list
        for call in upsert_edge_awaits:
            args, kwargs = call
            if len(args) >= 1 and args[0] == "T" and len(args) >= 2 and args[1] == "T":
                self_loop_call = True
        assert not self_loop_call, "self-loop edge should have been skipped"
        graph.upsert_edge.assert_any_await("T", "X", ANY)

    async def test_merge_strategy(self, graph, entities_vdb, relationships_vdb):
        entities = {
            "A": {"description": "AAA", "entity_type": "TYPE1", "source_id": "s1"},
            "B": {"description": "BBB", "entity_type": "TYPE2", "source_id": "s2<SEP>s3"},
        }
        graph.has_node.side_effect = lambda name: name in entities
        graph.get_node.side_effect = lambda name: entities.get(name, {})
        graph.get_node_edges.return_value = []
        graph.get_edge.return_value = {}

        strategy = {
            "description": "concatenate",
            "entity_type": "keep_first",
            "source_id": "join_unique",
        }

        result = await amerge_entities(
            graph, entities_vdb, relationships_vdb,
            ["A", "B"], "T",
            merge_strategy=strategy,
        )

        upsert_data = graph.upsert_node.await_args.args[1]
        assert "AAA" in upsert_data.get("description", "")
        assert "BBB" in upsert_data.get("description", "")
        assert upsert_data.get("entity_type") == "TYPE1"
        source_parts = set(upsert_data.get("source_id", "").split("<SEP>"))
        assert source_parts == {"s1", "s2", "s3"}


class TestMergeEntityAttributes:
    def test_concatenate(self):
        data_list = [
            {"description": "first", "entity_type": "A"},
            {"description": "second", "entity_type": "B"},
        ]
        strategy = {"description": "concatenate", "entity_type": "keep_first"}

        result = _merge_entity_attributes(data_list, strategy)

        assert result["description"] == "first\n\nsecond"
        assert result["entity_type"] == "A"

    def test_keep_first(self):
        data_list = [
            {"description": "first desc", "entity_type": "TYPE_A"},
            {"description": "second desc", "entity_type": "TYPE_B"},
        ]
        strategy = {"description": "keep_first", "entity_type": "keep_first"}

        result = _merge_entity_attributes(data_list, strategy)

        assert result["description"] == "first desc"
        assert result["entity_type"] == "TYPE_A"

    def test_join_unique(self):
        data_list = [
            {"source_id": "src1"},
            {"source_id": f"src2{GRAPH_FIELD_SEP}src3"},
        ]
        strategy = {"source_id": "join_unique"}

        result = _merge_entity_attributes(data_list, strategy)

        parts = set(result["source_id"].split(GRAPH_FIELD_SEP))
        assert parts == {"src1", "src2", "src3"}

    def test_default_strategy(self):
        data_list = [{"custom_field": "value1"}, {"custom_field": "value2"}]
        strategy = {}

        result = _merge_entity_attributes(data_list, strategy)

        assert result["custom_field"] == "value1"


class TestMergeRelationAttributes:
    def test_concatenate(self):
        data_list = [
            {"description": "rel A", "weight": 1.0},
            {"description": "rel B", "weight": 2.0},
        ]
        strategy = {"description": "concatenate", "weight": "max"}

        result = _merge_relation_attributes(data_list, strategy)

        assert result["description"] == "rel A\n\nrel B"
        assert result["weight"] == 2.0

    def test_max_weight(self):
        data_list = [
            {"weight": "1.5"},
            {"weight": "3.0"},
            {"weight": "2.0"},
        ]
        strategy = {"weight": "max"}

        result = _merge_relation_attributes(data_list, strategy)

        assert result["weight"] == 3.0

    def test_keep_first(self):
        data_list = [
            {"description": "first rel", "keywords": "kw1"},
            {"description": "second rel", "keywords": "kw2"},
        ]
        strategy = {"description": "keep_first", "keywords": "keep_first"}

        result = _merge_relation_attributes(data_list, strategy)

        assert result["description"] == "first rel"
        assert result["keywords"] == "kw1"


@pytest.mark.asyncio
class TestGetEntityInfo:
    async def test_without_vector_data(self, graph, entities_vdb, relationships_vdb):
        graph.get_node.return_value = {
            "description": "test entity",
            "source_id": "src123",
        }

        result = await get_entity_info(
            graph, entities_vdb, "my_entity", include_vector_data=False
        )

        assert result["entity_name"] == "my_entity"
        assert result["source_id"] == "src123"
        assert result["graph_data"] == graph.get_node.return_value
        assert "vector_data" not in result
        entities_vdb.get_by_id.assert_not_awaited()

    async def test_with_vector_data(self, graph, entities_vdb, relationships_vdb):
        graph.get_node.return_value = {
            "description": "test entity",
            "source_id": "src123",
        }
        entities_vdb.get_by_id.return_value = {"embedding": [0.1, 0.2]}

        result = await get_entity_info(
            graph, entities_vdb, "my_entity", include_vector_data=True
        )

        assert result["entity_name"] == "my_entity"
        assert result["vector_data"] == {"embedding": [0.1, 0.2]}
        entities_vdb.get_by_id.assert_awaited_once()

    async def test_entity_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.get_node.return_value = None

        result = await get_entity_info(
            graph, entities_vdb, "missing", include_vector_data=False
        )

        assert result["entity_name"] == "missing"
        assert result["source_id"] is None
        assert result["graph_data"] is None


@pytest.mark.asyncio
class TestGetRelationInfo:
    async def test_without_vector_data(self, graph, entities_vdb, relationships_vdb):
        graph.get_edge.return_value = {
            "description": "test rel",
            "source_id": "src123",
        }

        result = await get_relation_info(
            graph, relationships_vdb, "src", "tgt", include_vector_data=False
        )

        assert result["src_entity"] == "src"
        assert result["tgt_entity"] == "tgt"
        assert result["source_id"] == "src123"
        assert "vector_data" not in result
        relationships_vdb.get_by_id.assert_not_awaited()

    async def test_with_vector_data(self, graph, entities_vdb, relationships_vdb):
        graph.get_edge.return_value = {
            "description": "test rel",
            "source_id": "src123",
        }
        relationships_vdb.get_by_id.return_value = {"embedding": [0.3, 0.4]}

        result = await get_relation_info(
            graph, relationships_vdb, "src", "tgt", include_vector_data=True
        )

        assert result["vector_data"] == {"embedding": [0.3, 0.4]}
        relationships_vdb.get_by_id.assert_awaited_once()

    async def test_relation_not_found(self, graph, entities_vdb, relationships_vdb):
        graph.get_edge.return_value = None

        result = await get_relation_info(
            graph, relationships_vdb, "src", "tgt", include_vector_data=False
        )

        assert result["source_id"] is None
        assert result["graph_data"] is None
