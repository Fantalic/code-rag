"""
This module contains all graph-related routes for the LightRAG API.
"""

from typing import Optional, Any
import traceback
from fastapi import APIRouter, Depends, Query, HTTPException
from pydantic import BaseModel, Field

from lightrag.utils import logger
from ..utils_api import get_combined_auth_dependency

router = APIRouter(tags=["graph"])


class EntityUpdateRequest(BaseModel):
    entity_name: str
    updated_data: dict[str, Any]
    allow_rename: bool = False


class RelationUpdateRequest(BaseModel):
    source_id: str
    target_id: str
    updated_data: dict[str, Any]


class EntityMergeRequest(BaseModel):
    entities_to_change: list[str] = Field(
        ...,
        description="List of entity names to be merged and deleted. These are typically duplicate or misspelled entities.",
        min_length=1,
        examples=[["Elon Msk", "Ellon Musk"]],
    )
    entity_to_change_into: str = Field(
        ...,
        description="Target entity name that will receive all relationships from the source entities. This entity will be preserved.",
        min_length=1,
        examples=["Elon Musk"],
    )


class EntityCreateRequest(BaseModel):
    entity_name: str = Field(
        ...,
        description="Unique name for the new entity",
        min_length=1,
        examples=["Tesla"],
    )
    entity_data: dict[str, Any] = Field(
        ...,
        description="Dictionary containing entity properties. Common fields include 'description' and 'entity_type'.",
        examples=[
            {
                "description": "Electric vehicle manufacturer",
                "entity_type": "ORGANIZATION",
            }
        ],
    )


class RelationCreateRequest(BaseModel):
    source_entity: str = Field(
        ...,
        description="Name of the source entity. This entity must already exist in the knowledge graph.",
        min_length=1,
        examples=["Elon Musk"],
    )
    target_entity: str = Field(
        ...,
        description="Name of the target entity. This entity must already exist in the knowledge graph.",
        min_length=1,
        examples=["Tesla"],
    )
    relation_data: dict[str, Any] = Field(
        ...,
        description="Dictionary containing relationship properties. Common fields include 'description', 'keywords', and 'weight'.",
        examples=[
            {
                "description": "Elon Musk is the CEO of Tesla",
                "keywords": "CEO, founder",
                "weight": 1.0,
            }
        ],
    )


class BatchEntityItem(BaseModel):
    entity_name: str = Field(
        ...,
        description="Unique name for the new entity",
        min_length=1,
    )
    entity_data: dict[str, Any] = Field(
        ...,
        description="Dictionary containing entity properties.",
    )


class BatchEntityCreateRequest(BaseModel):
    entities: list[BatchEntityItem] = Field(
        ...,
        description="List of entities to create",
        min_length=1,
    )


class BatchRelationItem(BaseModel):
    source_entity: str = Field(
        ...,
        description="Name of the source entity",
        min_length=1,
    )
    target_entity: str = Field(
        ...,
        description="Name of the target entity",
        min_length=1,
    )
    relation_data: dict[str, Any] = Field(
        ...,
        description="Dictionary containing relationship properties.",
    )


class BatchRelationCreateRequest(BaseModel):
    relations: list[BatchRelationItem] = Field(
        ...,
        description="List of relations to create",
        min_length=1,
    )


class BatchImportEntityItem(BaseModel):
    entity_name: str = Field(..., min_length=1)
    entity_data: dict[str, Any] = Field(...)


class BatchImportRelationItem(BaseModel):
    source_entity: str = Field(..., min_length=1)
    target_entity: str = Field(..., min_length=1)
    relation_data: dict[str, Any] = Field(...)


class BatchImportRequest(BaseModel):
    entities: list[BatchImportEntityItem] = Field(
        default_factory=list, description="List of entities to create"
    )
    relations: list[BatchImportRelationItem] = Field(
        default_factory=list, description="List of relations to create"
    )


def create_graph_routes(rag, api_key: Optional[str] = None):
    combined_auth = get_combined_auth_dependency(api_key)

    @router.get("/graph/label/list", dependencies=[Depends(combined_auth)])
    async def get_graph_labels():
        """
        Get all graph labels

        Returns:
            List[str]: List of graph labels
        """
        try:
            return await rag.get_graph_labels()
        except Exception as e:
            logger.error(f"Error getting graph labels: {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error getting graph labels: {str(e)}"
            )

    @router.get("/graph/label/popular", dependencies=[Depends(combined_auth)])
    async def get_popular_labels(
        limit: int = Query(
            300, description="Maximum number of popular labels to return", ge=1, le=1000
        ),
    ):
        """
        Get popular labels by node degree (most connected entities)

        Args:
            limit (int): Maximum number of labels to return (default: 300, max: 1000)

        Returns:
            List[str]: List of popular labels sorted by degree (highest first)
        """
        try:
            return await rag.chunk_entity_relation_graph.get_popular_labels(limit)
        except Exception as e:
            logger.error(f"Error getting popular labels: {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error getting popular labels: {str(e)}"
            )

    @router.get("/graph/label/search", dependencies=[Depends(combined_auth)])
    async def search_labels(
        q: str = Query(..., description="Search query string"),
        limit: int = Query(
            50, description="Maximum number of search results to return", ge=1, le=100
        ),
    ):
        """
        Search labels with fuzzy matching

        Args:
            q (str): Search query string
            limit (int): Maximum number of results to return (default: 50, max: 100)

        Returns:
            List[str]: List of matching labels sorted by relevance
        """
        try:
            return await rag.chunk_entity_relation_graph.search_labels(q, limit)
        except Exception as e:
            logger.error(f"Error searching labels with query '{q}': {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error searching labels: {str(e)}"
            )

    @router.get("/graphs", dependencies=[Depends(combined_auth)])
    async def get_knowledge_graph(
        label: str = Query(..., description="Label to get knowledge graph for"),
        max_depth: int = Query(3, description="Maximum depth of graph", ge=1),
        max_nodes: int = Query(1000, description="Maximum nodes to return", ge=1),
    ):
        """
        Retrieve a connected subgraph of nodes where the label includes the specified label.
        When reducing the number of nodes, the prioritization criteria are as follows:
            1. Hops(path) to the staring node take precedence
            2. Followed by the degree of the nodes

        Args:
            label (str): Label of the starting node
            max_depth (int, optional): Maximum depth of the subgraph,Defaults to 3
            max_nodes: Maxiumu nodes to return

        Returns:
            Dict[str, List[str]]: Knowledge graph for label
        """
        try:
            # Log the label parameter to check for leading spaces
            logger.debug(
                f"get_knowledge_graph called with label: '{label}' (length: {len(label)}, repr: {repr(label)})"
            )

            return await rag.get_knowledge_graph(
                node_label=label,
                max_depth=max_depth,
                max_nodes=max_nodes,
            )
        except Exception as e:
            logger.error(f"Error getting knowledge graph for label '{label}': {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error getting knowledge graph: {str(e)}"
            )

    @router.get("/graph/entity/exists", dependencies=[Depends(combined_auth)])
    async def check_entity_exists(
        name: str = Query(..., description="Entity name to check"),
    ):
        """
        Check if an entity with the given name exists in the knowledge graph

        Args:
            name (str): Name of the entity to check

        Returns:
            Dict[str, bool]: Dictionary with 'exists' key indicating if entity exists
        """
        try:
            exists = await rag.chunk_entity_relation_graph.has_node(name)
            return {"exists": exists}
        except Exception as e:
            logger.error(f"Error checking entity existence for '{name}': {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error checking entity existence: {str(e)}"
            )

    @router.post("/graph/entity/edit", dependencies=[Depends(combined_auth)])
    async def update_entity(request: EntityUpdateRequest):
        """
        Update an entity's properties in the knowledge graph

        Args:
            request (EntityUpdateRequest): Request containing entity name, updated data, and rename flag

        Returns:
            Dict: Updated entity information
        """
        try:
            result = await rag.aedit_entity(
                entity_name=request.entity_name,
                updated_data=request.updated_data,
                allow_rename=request.allow_rename,
            )
            return {
                "status": "success",
                "message": "Entity updated successfully",
                "data": result,
            }
        except ValueError as ve:
            logger.error(
                f"Validation error updating entity '{request.entity_name}': {str(ve)}"
            )
            raise HTTPException(status_code=400, detail=str(ve))
        except Exception as e:
            logger.error(f"Error updating entity '{request.entity_name}': {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error updating entity: {str(e)}"
            )

    @router.post("/graph/relation/edit", dependencies=[Depends(combined_auth)])
    async def update_relation(request: RelationUpdateRequest):
        """Update a relation's properties in the knowledge graph

        Args:
            request (RelationUpdateRequest): Request containing source ID, target ID and updated data

        Returns:
            Dict: Updated relation information
        """
        try:
            result = await rag.aedit_relation(
                source_entity=request.source_id,
                target_entity=request.target_id,
                updated_data=request.updated_data,
            )
            return {
                "status": "success",
                "message": "Relation updated successfully",
                "data": result,
            }
        except ValueError as ve:
            logger.error(
                f"Validation error updating relation between '{request.source_id}' and '{request.target_id}': {str(ve)}"
            )
            raise HTTPException(status_code=400, detail=str(ve))
        except Exception as e:
            logger.error(
                f"Error updating relation between '{request.source_id}' and '{request.target_id}': {str(e)}"
            )
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error updating relation: {str(e)}"
            )

    @router.post("/graph/entity/create", dependencies=[Depends(combined_auth)])
    async def create_entity(request: EntityCreateRequest):
        """
        Create a new entity in the knowledge graph

        This endpoint creates a new entity node in the knowledge graph with the specified
        properties. The system automatically generates vector embeddings for the entity
        to enable semantic search and retrieval.

        Request Body:
            entity_name (str): Unique name identifier for the entity
            entity_data (dict): Entity properties including:
                - description (str): Textual description of the entity
                - entity_type (str): Category/type of the entity (e.g., PERSON, ORGANIZATION, LOCATION)
                - Additional custom properties as needed

        Response Schema:
            {
                "status": "success",
                "message": "Entity 'Tesla' created successfully",
                "data": {
                    "entity_name": "Tesla",
                    "description": "Electric vehicle manufacturer",
                    "entity_type": "ORGANIZATION",
                    ... (other entity properties)
                }
            }

        HTTP Status Codes:
            200: Entity created successfully
            400: Invalid request (e.g., missing required fields, duplicate entity)
            500: Internal server error

        Example Request:
            POST /graph/entity/create
            {
                "entity_name": "Tesla",
                "entity_data": {
                    "description": "Electric vehicle manufacturer",
                    "entity_type": "ORGANIZATION"
                }
            }
        """
        try:
            # Use the proper acreate_entity method which handles:
            # - Graph lock for concurrency
            # - Vector embedding creation in entities_vdb
            # - Metadata population and defaults
            # - Index consistency via _edit_entity_done
            result = await rag.acreate_entity(
                entity_name=request.entity_name,
                entity_data=request.entity_data,
            )

            return {
                "status": "success",
                "message": f"Entity '{request.entity_name}' created successfully",
                "data": result,
            }
        except ValueError as ve:
            logger.error(
                f"Validation error creating entity '{request.entity_name}': {str(ve)}"
            )
            raise HTTPException(status_code=400, detail=str(ve))
        except Exception as e:
            logger.error(f"Error creating entity '{request.entity_name}': {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error creating entity: {str(e)}"
            )

    @router.post("/graph/relation/create", dependencies=[Depends(combined_auth)])
    async def create_relation(request: RelationCreateRequest):
        """
        Create a new relationship between two entities in the knowledge graph

        This endpoint establishes a directed relationship between two existing entities.
        Both the source and target entities must already exist in the knowledge graph.
        The system automatically generates vector embeddings for the relationship to
        enable semantic search and graph traversal.

        Prerequisites:
            - Both source_entity and target_entity must exist in the knowledge graph
            - Use /graph/entity/create to create entities first if they don't exist

        Request Body:
            source_entity (str): Name of the source entity (relationship origin)
            target_entity (str): Name of the target entity (relationship destination)
            relation_data (dict): Relationship properties including:
                - description (str): Textual description of the relationship
                - keywords (str): Comma-separated keywords describing the relationship type
                - weight (float): Relationship strength/importance (default: 1.0)
                - Additional custom properties as needed

        Response Schema:
            {
                "status": "success",
                "message": "Relation created successfully between 'Elon Musk' and 'Tesla'",
                "data": {
                    "src_id": "Elon Musk",
                    "tgt_id": "Tesla",
                    "description": "Elon Musk is the CEO of Tesla",
                    "keywords": "CEO, founder",
                    "weight": 1.0,
                    ... (other relationship properties)
                }
            }

        HTTP Status Codes:
            200: Relationship created successfully
            400: Invalid request (e.g., missing entities, invalid data, duplicate relationship)
            500: Internal server error

        Example Request:
            POST /graph/relation/create
            {
                "source_entity": "Elon Musk",
                "target_entity": "Tesla",
                "relation_data": {
                    "description": "Elon Musk is the CEO of Tesla",
                    "keywords": "CEO, founder",
                    "weight": 1.0
                }
            }
        """
        try:
            # Use the proper acreate_relation method which handles:
            # - Graph lock for concurrency
            # - Entity existence validation
            # - Duplicate relation checks
            # - Vector embedding creation in relationships_vdb
            # - Index consistency via _edit_relation_done
            result = await rag.acreate_relation(
                source_entity=request.source_entity,
                target_entity=request.target_entity,
                relation_data=request.relation_data,
            )

            return {
                "status": "success",
                "message": f"Relation created successfully between '{request.source_entity}' and '{request.target_entity}'",
                "data": result,
            }
        except ValueError as ve:
            logger.error(
                f"Validation error creating relation between '{request.source_entity}' and '{request.target_entity}': {str(ve)}"
            )
            raise HTTPException(status_code=400, detail=str(ve))
        except Exception as e:
            logger.error(
                f"Error creating relation between '{request.source_entity}' and '{request.target_entity}': {str(e)}"
            )
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error creating relation: {str(e)}"
            )

    @router.post("/graph/entities/batch-create", dependencies=[Depends(combined_auth)])
    async def batch_create_entities(request: BatchEntityCreateRequest):
        """
        Batch create multiple entities in the knowledge graph

        Creates all entities without intermediate graph saves, then saves once at the end.
        Significantly faster than creating entities one by one for large batches.

        Request Body:
            entities (list): List of objects with:
                - entity_name (str): Unique name for the entity
                - entity_data (dict): Entity properties (description, entity_type, etc.)

        Returns:
            {"status": "success", "created": int, "total": int, "errors": [...]}
        """
        created = 0
        errors = []
        for item in request.entities:
            try:
                await rag.acreate_entity(
                    item.entity_name, item.entity_data, no_save=True
                )
                created += 1
            except ValueError as ve:
                errors.append({"entity": item.entity_name, "error": str(ve)})
            except Exception as e:
                errors.append({"entity": item.entity_name, "error": str(e)})
        await rag.asave_graph()
        logger.info(f"Batch created {created}/{len(request.entities)} entities")
        return {
            "status": "success",
            "message": f"Batch created {created} entities",
            "data": {
                "created": created,
                "total": len(request.entities),
                "errors": errors,
            },
        }

    @router.post("/graph/relations/batch-create", dependencies=[Depends(combined_auth)])
    async def batch_create_relations(request: BatchRelationCreateRequest):
        """
        Batch create multiple relations in the knowledge graph

        Creates all relations without intermediate graph saves or entity existence
        validation, then saves once at the end. Caller must ensure all source/target
        entities already exist (e.g., by calling batch entity create first).

        Request Body:
            relations (list): List of objects with:
                - source_entity (str): Name of the source entity
                - target_entity (str): Name of the target entity
                - relation_data (dict): Relation properties (description, keywords, weight, etc.)

        Returns:
            {"status": "success", "created": int, "total": int, "errors": [...]}
        """
        created = 0
        errors = []
        for item in request.relations:
            try:
                await rag.acreate_relation(
                    item.source_entity,
                    item.target_entity,
                    item.relation_data,
                    no_save=True,
                    skip_validation=True,
                )
                created += 1
            except Exception as e:
                errors.append(
                    {
                        "source": item.source_entity,
                        "target": item.target_entity,
                        "error": str(e),
                    }
                )
        await rag.asave_graph()
        logger.info(f"Batch created {created}/{len(request.relations)} relations")
        return {
            "status": "success",
            "message": f"Batch created {created} relations",
            "data": {
                "created": created,
                "total": len(request.relations),
                "errors": errors,
            },
        }

    @router.post("/graph/batch-import", dependencies=[Depends(combined_auth)])
    async def batch_import(request: BatchImportRequest):
        """
        Batch import entities and relations with maximum performance.

        Uses a single graph DB lock and batch VDB upserts (which internally
        parallelize embedding computation via asyncio.gather). Saves once
        at the end. Significantly faster than separate batch endpoints for
        large imports (1000+ items).

        Request Body:
            entities (list, optional): List of {entity_name, entity_data}
            relations (list, optional): List of {source_entity, target_entity, relation_data}

        Returns:
            {"status": "success", "entities_created": int, "relations_created": int}
        """
        try:
            result = await rag.abatch_import(
                [e.model_dump() for e in request.entities],
                [r.model_dump() for r in request.relations],
            )
            return {
                "status": "success",
                "message": f"Batch imported {result['entities_created']} entities and {result['relations_created']} relations",
                "data": result,
            }
        except Exception as e:
            logger.error(f"Error in batch import: {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error in batch import: {str(e)}"
            )

    @router.post("/graph/entities/merge", dependencies=[Depends(combined_auth)])
    async def merge_entities(request: EntityMergeRequest):
        """
        Merge multiple entities into a single entity, preserving all relationships

        This endpoint consolidates duplicate or misspelled entities while preserving the entire
        graph structure. It's particularly useful for cleaning up knowledge graphs after document
        processing or correcting entity name variations.

        What the Merge Operation Does:
            1. Deletes the specified source entities from the knowledge graph
            2. Transfers all relationships from source entities to the target entity
            3. Intelligently merges duplicate relationships (if multiple sources have the same relationship)
            4. Updates vector embeddings for accurate retrieval and search
            5. Preserves the complete graph structure and connectivity
            6. Maintains relationship properties and metadata

        Use Cases:
            - Fixing spelling errors in entity names (e.g., "Elon Msk" -> "Elon Musk")
            - Consolidating duplicate entities discovered after document processing
            - Merging name variations (e.g., "NY", "New York", "New York City")
            - Cleaning up the knowledge graph for better query performance
            - Standardizing entity names across the knowledge base

        Request Body:
            entities_to_change (list[str]): List of entity names to be merged and deleted
            entity_to_change_into (str): Target entity that will receive all relationships

        Response Schema:
            {
                "status": "success",
                "message": "Successfully merged 2 entities into 'Elon Musk'",
                "data": {
                    "merged_entity": "Elon Musk",
                    "deleted_entities": ["Elon Msk", "Ellon Musk"],
                    "relationships_transferred": 15,
                    ... (merge operation details)
                }
            }

        HTTP Status Codes:
            200: Entities merged successfully
            400: Invalid request (e.g., empty entity list, target entity doesn't exist)
            500: Internal server error

        Example Request:
            POST /graph/entities/merge
            {
                "entities_to_change": ["Elon Msk", "Ellon Musk"],
                "entity_to_change_into": "Elon Musk"
            }

        Note:
            - The target entity (entity_to_change_into) must exist in the knowledge graph
            - Source entities will be permanently deleted after the merge
            - This operation cannot be undone, so verify entity names before merging
        """
        try:
            result = await rag.amerge_entities(
                source_entities=request.entities_to_change,
                target_entity=request.entity_to_change_into,
            )
            return {
                "status": "success",
                "message": f"Successfully merged {len(request.entities_to_change)} entities into '{request.entity_to_change_into}'",
                "data": result,
            }
        except ValueError as ve:
            logger.error(
                f"Validation error merging entities {request.entities_to_change} into '{request.entity_to_change_into}': {str(ve)}"
            )
            raise HTTPException(status_code=400, detail=str(ve))
        except Exception as e:
            logger.error(
                f"Error merging entities {request.entities_to_change} into '{request.entity_to_change_into}': {str(e)}"
            )
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error merging entities: {str(e)}"
            )

    @router.delete("/graph", dependencies=[Depends(combined_auth)])
    async def clear_graph():
        """
        Delete all graph data (entities, relations, vector indexes).

        This endpoint clears the entire knowledge graph including:
        - All entity nodes and their properties
        - All relationship edges and their properties
        - All vector embeddings for entities and relationships

        Use this when you need to reset the graph completely (e.g., before
        re-scanning a project from scratch).

        Returns:
            dict: Status of the operation with details about each cleared storage
        """
        try:
            result = await rag.aclear_graph()
            return {
                "status": "success",
                "message": "Graph data cleared successfully",
                "data": result,
            }
        except Exception as e:
            logger.error(f"Error clearing graph: {str(e)}")
            logger.error(traceback.format_exc())
            raise HTTPException(
                status_code=500, detail=f"Error clearing graph: {str(e)}"
            )

    return router
