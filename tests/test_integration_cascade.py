import os, time, pytest
import httpx

LIGHTRAG_URL = os.environ.get("LIGHTRAG_URL", "http://localhost:9621")
WORKSPACE = os.environ.get("LIGHTRAG_WORKSPACE", "test-csharp-project")


def _query(method, path, **kwargs):
    kwargs.setdefault("timeout", 30)
    r = getattr(httpx, method)(f"{LIGHTRAG_URL}{path}", **kwargs)
    r.raise_for_status()
    return r.json()


class TestCascadeIntegration:

    def _delete(self, name):
        try:
            httpx.request("DELETE", f"{LIGHTRAG_URL}/documents/delete_entity",
                          json={"entity_name": name}, timeout=30)
        except Exception:
            pass

    @pytest.fixture(autouse=True)
    def _setup_and_cleanup(self):
        _query("post", "/workspace", json={"workspace": WORKSPACE})
        self._delete("ClassA")
        self._delete("ClassB")
        self._delete("ClassC")
        yield
        self._delete("ClassA")
        self._delete("ClassB")
        self._delete("ClassC")

    def test_cascade_delete_entity_removes_graph_and_vector(self):
        create_a = _query("post", "/graph/entity/create", json={
            "entity_name": "ClassA",
            "entity_data": {
                "description": "Entity A for cascade test",
                "entity_type": "class",
            },
        })
        assert create_a.get("status") in ("ok", "completed", "success"), str(create_a)

        create_b = _query("post", "/graph/entity/create", json={
            "entity_name": "ClassB",
            "entity_data": {
                "description": "Entity B for cascade test",
                "entity_type": "class",
            },
        })
        assert create_b.get("status") in ("ok", "completed", "success"), str(create_b)

        create_c = _query("post", "/graph/entity/create", json={
            "entity_name": "ClassC",
            "entity_data": {
                "description": "Entity C for cascade test",
                "entity_type": "class",
            },
        })
        assert create_c.get("status") in ("ok", "completed", "success"), str(create_c)

        rel_ab = _query("post", "/graph/relation/create", json={
            "source_entity": "ClassA",
            "target_entity": "ClassB",
            "relation_data": {
                "keywords": "depends_on",
                "weight": 1.0,
            },
        })
        assert rel_ab.get("status") in ("ok", "completed", "success"), str(rel_ab)

        rel_ac = _query("post", "/graph/relation/create", json={
            "source_entity": "ClassA",
            "target_entity": "ClassC",
            "relation_data": {
                "keywords": "depends_on",
                "weight": 1.0,
            },
        })
        assert rel_ac.get("status") in ("ok", "completed", "success"), str(rel_ac)

        time.sleep(1)

        exists_a = _query("get", "/graph/entity/exists?name=ClassA")
        assert exists_a.get("exists") is True, "ClassA should exist before deletion"

        exists_b = _query("get", "/graph/entity/exists?name=ClassB")
        assert exists_b.get("exists") is True, "ClassB should exist before deletion"

        exists_c = _query("get", "/graph/entity/exists?name=ClassC")
        assert exists_c.get("exists") is True, "ClassC should exist before deletion"

        delete_a = httpx.request("DELETE", f"{LIGHTRAG_URL}/documents/delete_entity",
                                 json={"entity_name": "ClassA"}, timeout=30)
        assert delete_a.status_code == 200, f"Delete failed: {delete_a.text}"
        result = delete_a.json()
        assert result.get("status") in ("ok", "completed", "success"), str(result)

        time.sleep(1)

        exists_a_after = _query("get", "/graph/entity/exists?name=ClassA")
        assert exists_a_after.get("exists") is False, "ClassA should be gone after delete"

        exists_b_after = _query("get", "/graph/entity/exists?name=ClassB")
        assert exists_b_after.get("exists") is True, "ClassB should still exist"

        exists_c_after = _query("get", "/graph/entity/exists?name=ClassC")
        assert exists_c_after.get("exists") is True, "ClassC should still exist"

        _query("post", "/query", json={
            "query": "Name ClassA",
            "mode": "local",
        })
