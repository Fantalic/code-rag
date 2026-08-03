"""Integration tests for graph entity/relation VDB embedding & file_path persistence.

Verifies that:
1.  Entity creation populates VDB with file_path and vector embedding
2.  Entity update preserves file_path in VDB (regression: aedit_entity dropped it)
3.  Relation creation populates VDB with file_path and vector embedding
4.  Relation update preserves file_path in VDB (regression: aedit_relation dropped it)
5.  VDB vectors exist after all CRUD operations
"""

import pytest

pytest.importorskip("lightrag")
pytestmark = [pytest.mark.integration, pytest.mark.asyncio]


def _unique(prefix: str) -> str:
    """Return a unique name per test (session-scoped server ⇒ no collisions)."""
    import uuid

    return f"{prefix}_{uuid.uuid4().hex[:8]}"


# ── Entity VDB tests ─────────────────────────────────────────────────────────


class TestEntityVDB:
    async def test_create_entity_embeds_with_file_path(self, client):
        name = _unique("Ent")
        r = await client.post(
            "/graph/entity/create",
            json={
                "entity_name": name,
                "entity_data": {
                    "description": "test entity for VDB",
                    "entity_type": "CLASS",
                    "source_id": "test.txt",
                    "file_path": "src/Entity.cs",
                },
            },
        )
        assert r.status_code == 200, r.text
        data = r.json()["data"]
        vd = data.get("vector_data")
        assert vd is not None, "Entity should have vector_data after creation"
        assert vd.get("file_path") == "src/Entity.cs", (
            f"Expected file_path='src/Entity.cs', got {vd.get('file_path')!r}"
        )
        assert vd.get("entity_name") == name
        assert "vector" in vd or "__vector__" in vd or "__id__" in vd, (
            "VDB entry should have vector or id field"
        )

    async def test_update_entity_preserves_file_path_in_vdb(self, client):
        """Regression test: aedit_entity was re-embedding without file_path."""
        name = _unique("Ent")
        # Create
        r = await client.post(
            "/graph/entity/create",
            json={
                "entity_name": name,
                "entity_data": {
                    "description": "original",
                    "entity_type": "CLASS",
                    "file_path": "src/MyClass.cs",
                },
            },
        )
        assert r.status_code == 200, r.text
        create_vd = r.json()["data"]["vector_data"]
        assert create_vd.get("file_path") == "src/MyClass.cs"

        # Update
        r = await client.post(
            "/graph/entity/edit",
            json={
                "entity_name": name,
                "updated_data": {
                    "description": "updated description",
                    "entity_type": "CLASS",
                    "file_path": "src/MyClass.cs",
                },
                "allow_rename": False,
            },
        )
        assert r.status_code == 200, r.text
        update_vd = r.json()["data"]["vector_data"]
        assert update_vd is not None, "Entity should have vector_data after update"
        assert update_vd.get("file_path") == "src/MyClass.cs", (
            f"file_path lost after update: got {update_vd.get('file_path')!r}"
        )
        assert update_vd.get("entity_name") == name

    async def test_create_entity_without_file_path_defaults(self, client):
        """When no file_path provided, it should default to 'manual_creation'."""
        name = _unique("Ent")
        r = await client.post(
            "/graph/entity/create",
            json={
                "entity_name": name,
                "entity_data": {
                    "description": "no file path",
                    "entity_type": "CLASS",
                },
            },
        )
        assert r.status_code == 200, r.text
        vd = r.json()["data"].get("vector_data")
        assert vd is not None
        assert vd.get("file_path") == "manual_creation", (
            f"Expected default 'manual_creation', got {vd.get('file_path')!r}"
        )

    async def test_update_entity_rename_preserves_vector(self, client):
        """Renaming an entity should re-index its vector under the new ID."""
        old_name = _unique("OldEnt")
        new_name = _unique("NewEnt")

        r = await client.post(
            "/graph/entity/create",
            json={
                "entity_name": old_name,
                "entity_data": {
                    "description": "will be renamed",
                    "entity_type": "CLASS",
                    "file_path": "src/Renamed.cs",
                },
            },
        )
        assert r.status_code == 200, r.text

        r = await client.post(
            "/graph/entity/edit",
            json={
                "entity_name": old_name,
                "updated_data": {
                    "entity_name": new_name,
                    "description": "renamed entity",
                    "file_path": "src/Renamed.cs",
                },
                "allow_rename": True,
            },
        )
        assert r.status_code == 200, r.text
        vd = r.json()["data"]["vector_data"]
        assert vd is not None
        assert vd.get("entity_name") == new_name
        assert vd.get("file_path") == "src/Renamed.cs"


# ── Relation VDB tests ───────────────────────────────────────────────────────


class TestRelationVDB:
    async def _make_entities(self, client) -> tuple[str, str]:
        """Helper to create two entities and return their names."""
        e1 = _unique("RelSrc")
        e2 = _unique("RelTgt")
        for name in (e1, e2):
            r = await client.post(
                "/graph/entity/create",
                json={
                    "entity_name": name,
                    "entity_data": {
                        "description": f"entity {name}",
                        "entity_type": "CLASS",
                        "file_path": f"src/{name}.cs",
                    },
                },
            )
            assert r.status_code == 200, f"Failed to create {name}: {r.text}"
        return e1, e2

    async def test_create_relation_embeds_with_file_path(self, client):
        src, tgt = await self._make_entities(client)
        r = await client.post(
            "/graph/relation/create",
            json={
                "source_entity": src,
                "target_entity": tgt,
                "relation_data": {
                    "description": f"{src} depends on {tgt}",
                    "keywords": "depends_on",
                    "weight": 1.0,
                    "source_id": "test.txt",
                    "file_path": "src/Relation.cs",
                },
            },
        )
        assert r.status_code == 200, r.text
        data = r.json()["data"]
        vd = data.get("vector_data")
        assert vd is not None, "Relation should have vector_data after creation"
        assert vd.get("file_path") == "src/Relation.cs", (
            f"Expected file_path='src/Relation.cs', got {vd.get('file_path')!r}"
        )
        assert vd.get("src_id") == src
        assert vd.get("tgt_id") == tgt

    async def test_update_relation_preserves_file_path_in_vdb(self, client):
        """Regression: aedit_relation was re-embedding without file_path."""
        src, tgt = await self._make_entities(client)

        # Create relation
        r = await client.post(
            "/graph/relation/create",
            json={
                "source_entity": src,
                "target_entity": tgt,
                "relation_data": {
                    "description": "original relation",
                    "keywords": "original",
                    "weight": 0.5,
                    "file_path": "src/MyRelation.cs",
                },
            },
        )
        assert r.status_code == 200, r.text
        create_vd = r.json()["data"]["vector_data"]
        assert create_vd.get("file_path") == "src/MyRelation.cs", (
            f"Create: expected src/MyRelation.cs, got {create_vd.get('file_path')!r}"
        )

        # Update relation
        r = await client.post(
            "/graph/relation/edit",
            json={
                "source_id": src,
                "target_id": tgt,
                "updated_data": {
                    "description": "updated relation description",
                    "keywords": "updated",
                    "weight": 1.0,
                    "file_path": "src/MyRelation.cs",
                },
            },
        )
        assert r.status_code == 200, r.text
        update_vd = r.json()["data"]["vector_data"]
        assert update_vd is not None, "Relation should have vector_data after update"
        assert update_vd.get("file_path") == "src/MyRelation.cs", (
            f"Update: file_path lost after edit: got {update_vd.get('file_path')!r}"
        )
        assert update_vd.get("src_id") == src
        assert update_vd.get("tgt_id") == tgt

    async def test_create_relation_without_file_path_defaults(self, client):
        """When no file_path provided, it should default to 'manual_creation'."""
        src, tgt = await self._make_entities(client)
        r = await client.post(
            "/graph/relation/create",
            json={
                "source_entity": src,
                "target_entity": tgt,
                "relation_data": {
                    "description": "no file path",
                    "keywords": "test",
                    "weight": 1.0,
                },
            },
        )
        assert r.status_code == 200, r.text
        vd = r.json()["data"].get("vector_data")
        assert vd is not None
        assert vd.get("file_path") == "manual_creation", (
            f"Expected default 'manual_creation', got {vd.get('file_path')!r}"
        )


# ── Full-workflow embedding integrity ────────────────────────────────────────


class TestGraphEmbeddingIntegrity:
    async def test_create_and_update_entity_has_valid_vector(self, client):
        """Verify vector_data is present and has expected fields after both
        create and update operations."""

        name = _unique("Integrity")

        r = await client.post(
            "/graph/entity/create",
            json={
                "entity_name": name,
                "entity_data": {
                    "description": "integrity check entity",
                    "entity_type": "CLASS",
                    "file_path": "src/Integrity.cs",
                },
            },
        )
        assert r.status_code == 200
        create = r.json()["data"]
        assert "vector_data" in create, "Create response should include vector_data"
        assert create["vector_data"]["file_path"] == "src/Integrity.cs"

        r = await client.post(
            "/graph/entity/edit",
            json={
                "entity_name": name,
                "updated_data": {
                    "description": "updated integrity",
                    "file_path": "src/Integrity.cs",
                },
                "allow_rename": False,
            },
        )
        assert r.status_code == 200
        update = r.json()["data"]
        assert "vector_data" in update, "Update response should include vector_data"
        assert update["vector_data"]["file_path"] == "src/Integrity.cs"

    async def test_edit_entity_without_file_path_keeps_original(
        self, client
    ):
        """When updated_data omits file_path, the VDB should retain the
        file_path from the graph node data (new_node_data.get('file_path'))."""
        name = _unique("NoFpEdit")

        r = await client.post(
            "/graph/entity/create",
            json={
                "entity_name": name,
                "entity_data": {
                    "description": "entity with file_path",
                    "entity_type": "CLASS",
                    "file_path": "src/Original.cs",
                },
            },
        )
        assert r.status_code == 200
        create_vd = r.json()["data"]["vector_data"]
        assert create_vd.get("file_path") == "src/Original.cs"

        # Update without file_path in updated_data
        r = await client.post(
            "/graph/entity/edit",
            json={
                "entity_name": name,
                "updated_data": {
                    "description": "updated without file_path",
                },
                "allow_rename": False,
            },
        )
        assert r.status_code == 200
        update_vd = r.json()["data"]["vector_data"]
        assert update_vd is not None
        # Should have fallen back to the graph node's stored file_path
        assert update_vd.get("file_path") == "src/Original.cs", (
            f"Expected 'src/Original.cs' (from graph node), got {update_vd.get('file_path')!r}"
        )

    async def test_edit_relation_without_file_path_keeps_original(
        self, client
    ):
        """Same as above for relations."""
        src = _unique("RelSrc2")
        tgt = _unique("RelTgt2")
        for name in (src, tgt):
            r = await client.post(
                "/graph/entity/create",
                json={
                    "entity_name": name,
                    "entity_data": {
                        "description": f"entity {name}",
                        "entity_type": "CLASS",
                        "file_path": f"src/{name}.cs",
                    },
                },
            )
            assert r.status_code == 200

        r = await client.post(
            "/graph/relation/create",
            json={
                "source_entity": src,
                "target_entity": tgt,
                "relation_data": {
                    "description": "relation with file_path",
                    "keywords": "test",
                    "weight": 1.0,
                    "file_path": "src/OriginalRel.cs",
                },
            },
        )
        assert r.status_code == 200
        create_vd = r.json()["data"]["vector_data"]
        assert create_vd.get("file_path") == "src/OriginalRel.cs"

        # Update without file_path
        r = await client.post(
            "/graph/relation/edit",
            json={
                "source_id": src,
                "target_id": tgt,
                "updated_data": {
                    "description": "updated without file_path",
                    "keywords": "updated",
                    "weight": 1.0,
                },
            },
        )
        assert r.status_code == 200
        update_vd = r.json()["data"]["vector_data"]
        assert update_vd is not None
        assert update_vd.get("file_path") == "src/OriginalRel.cs", (
            f"Expected 'src/OriginalRel.cs' (from graph edge), got {update_vd.get('file_path')!r}"
        )

    async def test_full_workflow_graph_has_vector_embeddings(self, client):
        """Create entities + relations, verify every VDB entry has a vector."""
        e1 = _unique("WfA")
        e2 = _unique("WfB")
        e3 = _unique("WfC")

        # Create three entities
        for name in (e1, e2, e3):
            r = await client.post(
                "/graph/entity/create",
                json={
                    "entity_name": name,
                    "entity_data": {
                        "description": f"workflow entity {name}",
                        "entity_type": "CLASS",
                        "file_path": f"src/{name}.cs",
                    },
                },
            )
            assert r.status_code == 200
            vd = r.json()["data"]["vector_data"]
            assert vd is not None, f"{name}: no vector_data on create"
            assert vd.get("file_path") == f"src/{name}.cs"

        # Create relations between them
        pairs = [(e1, e2), (e2, e3), (e1, e3)]
        for src, tgt in pairs:
            r = await client.post(
                "/graph/relation/create",
                json={
                    "source_entity": src,
                    "target_entity": tgt,
                    "relation_data": {
                        "description": f"{src} -> {tgt}",
                        "keywords": "workflow",
                        "weight": 1.0,
                        "file_path": "src/Workflow.cs",
                    },
                },
            )
            assert r.status_code == 200
            vd = r.json()["data"]["vector_data"]
            assert vd is not None, f"Relation {src}->{tgt}: no vector_data on create"
            assert vd.get("file_path") == "src/Workflow.cs"

        # Update all entities
        for name in (e1, e2, e3):
            r = await client.post(
                "/graph/entity/edit",
                json={
                    "entity_name": name,
                    "updated_data": {
                        "description": f"updated {name}",
                        "file_path": f"src/{name}.cs",
                    },
                    "allow_rename": False,
                },
            )
            assert r.status_code == 200
            vd = r.json()["data"]["vector_data"]
            assert vd is not None, f"{name}: no vector_data after update"
            assert vd.get("file_path") == f"src/{name}.cs"

        # Update all relations
        for src, tgt in pairs:
            r = await client.post(
                "/graph/relation/edit",
                json={
                    "source_id": src,
                    "target_id": tgt,
                    "updated_data": {
                        "description": f"updated {src} -> {tgt}",
                        "keywords": "workflow,updated",
                        "weight": 0.8,
                        "file_path": "src/Workflow.cs",
                    },
                },
            )
            assert r.status_code == 200
            vd = r.json()["data"]["vector_data"]
            assert vd is not None, f"Relation {src}->{tgt}: no vector_data after update"
            assert vd.get("file_path") == "src/Workflow.cs"
