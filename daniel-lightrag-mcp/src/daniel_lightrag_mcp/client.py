"""
LightRAG API client for MCP server integration.
"""

import json
import logging
import re
from typing import Any, Optional

import httpx

from .analyser_runner import run_analysis
from .models import (
    CodebaseStatsResponse,
    DocumentInfo,
    EntityCreateRequest,
    EntityCreateResponse,
    EntityDetailsResponse,
    EntityExistsResponse,
    EntityInfo,
    EntityNeighborsResponse,
    EntityUpdateRequest,
    EntityUpdateResponse,
    GetEntitiesInFileResponse,
    GraphResponse,
    InsertResponse,
    InsertTextRequest,
    LabelsResponse,
    ListEntitiesResponse,
    QueryDataResponse,
    QueryRequest,
    QueryResponse,
    RelatedFileInfo,
    RelatedFilesResponse,
    RelationCreateRequest,
    RelationCreateResponse,
    RelationInfo,
    RelationUpdateRequest,
    RelationUpdateResponse,
    SearchEntitiesResponse,
)


# Custom Exception Hierarchy
class LightRAGError(Exception):
    """Base exception for LightRAG client errors."""

    def __init__(
        self,
        message: str,
        status_code: Optional[int] = None,
        response_data: Optional[dict[str, Any]] = None,
    ):
        super().__init__(message)
        self.message = message
        self.status_code = status_code
        self.response_data = response_data or {}

    def to_dict(self) -> dict[str, Any]:
        return {
            "error_type": self.__class__.__name__,
            "message": self.message,
            "status_code": self.status_code,
            "response_data": self.response_data,
        }


class LightRAGConnectionError(LightRAGError):
    pass


class LightRAGAuthError(LightRAGError):
    pass


class LightRAGValidationError(LightRAGError):
    pass


class LightRAGAPIError(LightRAGError):
    pass


class LightRAGTimeoutError(LightRAGError):
    pass


class LightRAGServerError(LightRAGError):
    pass


class LightRAGClient:
    """Client for interacting with LightRAG API."""

    def __init__(
        self,
        base_url: str = "http://localhost:9621",
        api_key: Optional[str] = None,
        timeout: float = 30.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self.logger = logging.getLogger(__name__)

        headers = {}
        if api_key:
            headers["X-API-Key"] = api_key

        self.client = httpx.AsyncClient(timeout=timeout, headers=headers)
        self.logger.info(f"Initialized LightRAG client with base_url: {self.base_url}")

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.client.aclose()

    def _map_http_error(
        self,
        status_code: int,
        response_text: str,
        response_data: Optional[dict[str, Any]] = None,
    ) -> LightRAGError:
        error_message = f"HTTP {status_code}: {response_text}"
        parsed_data = response_data or {}
        if response_text:
            try:
                parsed_data = json.loads(response_text)
                if isinstance(parsed_data, dict) and "detail" in parsed_data:
                    error_message = f"HTTP {status_code}: {parsed_data['detail']}"
                elif isinstance(parsed_data, dict) and "message" in parsed_data:
                    error_message = f"HTTP {status_code}: {parsed_data['message']}"
            except json.JSONDecodeError:
                pass

        if status_code == 400:
            return LightRAGValidationError(
                f"Bad Request: {error_message}", status_code, parsed_data
            )
        elif status_code in (401, 403):
            return LightRAGAuthError(
                f"Unauthorized: {error_message}", status_code, parsed_data
            )
        elif status_code == 404:
            return LightRAGAPIError(
                f"Not Found: {error_message}", status_code, parsed_data
            )
        elif status_code == 408:
            return LightRAGTimeoutError(
                f"Request Timeout: {error_message}", status_code, parsed_data
            )
        elif status_code == 422:
            return LightRAGValidationError(
                f"Validation Error: {error_message}", status_code, parsed_data
            )
        elif status_code == 429:
            return LightRAGAPIError(
                f"Rate Limited: {error_message}", status_code, parsed_data
            )
        elif 500 <= status_code < 600:
            return LightRAGServerError(
                f"Server Error: {error_message}", status_code, parsed_data
            )
        else:
            return LightRAGAPIError(error_message, status_code, parsed_data)

    async def _make_request(
        self,
        method: str,
        endpoint: str,
        data: Optional[dict[str, Any]] = None,
        params: Optional[dict[str, Any]] = None,
        files: Optional[dict[str, Any]] = None,
    ) -> dict[str, Any]:
        url = f"{self.base_url}{endpoint}"
        self.logger.debug(f"Making {method} request to {url}")

        try:
            if method.upper() == "GET":
                response = await self.client.get(url, params=params)
            elif method.upper() == "POST":
                if files:
                    response = await self.client.post(url, data=data, files=files)
                else:
                    response = await self.client.post(url, json=data)
            elif method.upper() == "DELETE":
                response = await self.client.request("DELETE", url, json=data or {})
            else:
                raise LightRAGError(f"Unsupported HTTP method: {method}")

            response.raise_for_status()
            return response.json()

        except httpx.HTTPStatusError as e:
            raise self._map_http_error(e.response.status_code, e.response.text)
        except httpx.ConnectError as e:
            raise LightRAGConnectionError(f"Connection failed to {url}: {str(e)}")
        except httpx.TimeoutException as e:
            raise LightRAGTimeoutError(f"Request timeout for {method} {url}: {str(e)}")
        except httpx.RequestError as e:
            raise LightRAGConnectionError(
                f"Request failed for {method} {url}: {str(e)}"
            )

    # --- Query ---

    async def query_text(
        self, query: str, mode: str = "hybrid", only_need_context: bool = False
    ) -> QueryResponse:
        """Query LightRAG with text."""
        if not query or not query.strip():
            raise LightRAGValidationError("Query cannot be empty")
        valid_modes = ["naive", "local", "global", "hybrid"]
        if mode not in valid_modes:
            raise LightRAGValidationError(
                f"Invalid query mode '{mode}'. Must be one of: {valid_modes}"
            )

        request_data = QueryRequest(
            query=query, mode=mode, only_need_context=only_need_context
        )
        response_data = await self._make_request(
            "POST", "/query", request_data.model_dump()
        )
        return QueryResponse(**response_data)

    # --- Knowledge Graph ---

    async def get_knowledge_graph(self, label: str = "*") -> GraphResponse:
        """Retrieve the knowledge graph from LightRAG."""
        params = {"label": label}
        response_data = await self._make_request("GET", "/graphs", params=params)
        return GraphResponse(**response_data)

    async def get_graph_labels(self) -> LabelsResponse:
        """Get labels for entities and relations in the knowledge graph."""
        response_data = await self._make_request("GET", "/graph/label/list")
        if isinstance(response_data, list):
            response_data = {"labels": response_data}
        return LabelsResponse(**response_data)

    async def check_entity_exists(self, entity_name: str) -> EntityExistsResponse:
        """Check if an entity exists in the knowledge graph."""
        params = {"name": entity_name}
        response_data = await self._make_request(
            "GET", "/graph/entity/exists", params=params
        )
        return EntityExistsResponse(**response_data)

    async def create_entity(
        self, entity_name: str, entity_data: dict[str, Any]
    ) -> EntityCreateResponse:
        """Create a new entity in the knowledge graph."""
        request_data = EntityCreateRequest(
            entity_name=entity_name, entity_data=entity_data
        )
        response_data = await self._make_request(
            "POST", "/graph/entity/create", request_data.model_dump()
        )
        return EntityCreateResponse(**response_data)

    async def create_relation(
        self, source_entity: str, target_entity: str, relation_data: dict[str, Any]
    ) -> RelationCreateResponse:
        """Create a new relation in the knowledge graph."""
        request_data = RelationCreateRequest(
            source_entity=source_entity,
            target_entity=target_entity,
            relation_data=relation_data,
        )
        response_data = await self._make_request(
            "POST", "/graph/relation/create", request_data.model_dump()
        )
        return RelationCreateResponse(**response_data)

    async def update_entity(
        self,
        entity_id: str,
        properties: dict[str, Any],
        entity_name: Optional[str] = None,
    ) -> EntityUpdateResponse:
        """Update an entity in the knowledge graph."""
        if entity_name is None:
            entity_name = entity_id
        request_data = EntityUpdateRequest(
            entity_id=entity_id, entity_name=entity_name, updated_data=properties
        )
        response_data = await self._make_request(
            "POST", "/graph/entity/edit", request_data.model_dump()
        )
        return EntityUpdateResponse(**response_data)

    async def update_relation(
        self, source_id: str, target_id: str, updated_data: dict[str, Any]
    ) -> RelationUpdateResponse:
        """Update a relation in the knowledge graph."""
        request_data = RelationUpdateRequest(
            source_id=source_id, target_id=target_id, updated_data=updated_data
        )
        response_data = await self._make_request(
            "POST", "/graph/relation/edit", request_data.model_dump()
        )
        return RelationUpdateResponse(**response_data)

    # --- Related Files ---

    async def get_related_files(
        self, query: str, mode: str = "hybrid", limit: int = 10
    ) -> RelatedFilesResponse:
        """Get files related to a query or keywords.

        Uses /query/data endpoint which returns structured results with
        entities, relationships, chunks, and references (file paths).
        """
        request_data = QueryRequest(query=query, mode=mode, only_need_context=True)
        response_data = await self._make_request(
            "POST", "/query/data", request_data.model_dump()
        )
        result = QueryDataResponse(**response_data)

        files: list[RelatedFileInfo] = []
        seen_paths: set[str] = set()

        data = result.data
        if isinstance(data, dict):
            for entity in data.get("entities", []):
                fp = entity.get("properties", {}).get("file_path")
                if fp and fp not in seen_paths:
                    seen_paths.add(fp)
                    files.append(
                        RelatedFileInfo(
                            file_path=fp,
                            snippet=entity.get("properties", {}).get("description", ""),
                        )
                    )
                    if len(files) >= limit:
                        break

            if len(files) < limit:
                for ref in data.get("references", []):
                    fp = ref.get("file_path")
                    if fp and fp not in seen_paths:
                        seen_paths.add(fp)
                        files.append(
                            RelatedFileInfo(
                                file_path=fp,
                                chunk_id=ref.get("chunk_id"),
                            )
                        )
                        if len(files) >= limit:
                            break

        return RelatedFilesResponse(
            related_files=[
                DocumentInfo(
                    id=f.file_path,
                    title=f.file_path.split("/")[-1].split("\\")[-1],
                    snippet=f.snippet or "",
                )
                for f in files
            ],
            query=query,
            total_found=len(files),
        )

    # --- C# Analyser (keep for project analysis) ---

    async def insert_text(
        self, text: str, title: Optional[str] = None
    ) -> InsertResponse:
        """Insert text content into LightRAG."""
        file_source = f"{title}.txt" if title else "text_input.txt"
        request_data = InsertTextRequest(text=text, file_source=file_source)
        response_data = await self._make_request(
            "POST", "/documents/text", request_data.model_dump()
        )
        return InsertResponse(**response_data)

    async def update_relations(self, path: str) -> list[str]:
        """Run C# analyser on a project and ingest output into LightRAG."""
        self.logger.info(f"update_relations: analysing project at {path}")

        try:
            analysis_results = run_analysis(path)
        except Exception as e:
            self.logger.error(f"Analyser failed: {e}")
            return [f"ERROR: C# analyser failed: {e}"]

        if not analysis_results:
            return ["WARNING: C# analyser produced no output files to ingest"]

        self.logger.info(
            f"Ingesting {len(analysis_results)} analyser output files into LightRAG"
        )
        track_ids: list[str] = []
        errors: list[str] = []

        for filename, content in analysis_results:
            try:
                title = filename.replace(".txt", "")
                result = await self.insert_text(content, title=title)
                if hasattr(result, "track_id") and result.track_id:
                    track_ids.append(result.track_id)
                tid = getattr(result, "track_id", "N/A")
                self.logger.info(f"Ingested {filename} -> track_id={tid}")
            except Exception as e:
                msg = f"Failed to ingest {filename}: {e}"
                self.logger.error(msg)
                errors.append(msg)

        summary_parts: list[str] = [
            f"Analyser generated {len(analysis_results)} output files for {path}",
            f"Ingested {len(track_ids)} files into LightRAG",
        ]
        if errors:
            summary_parts.append(f"Errors: {'; '.join(errors)}")
        summary_parts.append(
            "LightRAG is now processing the analyser output "
            "to build the knowledge graph. "
            "This may take a few minutes depending on LLM speed.",
        )
        return summary_parts

    # --- Phase 22: Enhanced exploration tools ---

    async def query_data(
        self, query: str, mode: str = "hybrid", only_need_context: bool = True
    ) -> QueryDataResponse:
        """Structured query using /query/data endpoint.

        Returns entities, relationships, chunks, and references without LLM text.
        """
        request_data = QueryRequest(
            query=query, mode=mode, only_need_context=only_need_context
        )
        response_data = await self._make_request(
            "POST", "/query/data", request_data.model_dump()
        )
        return QueryDataResponse(**response_data)

    async def get_codebase_stats(self) -> CodebaseStatsResponse:
        """Get statistics about the codebase from the knowledge graph."""
        labels_resp = await self.get_graph_labels()
        entity_types = labels_resp.entity_labels or []
        relation_types = labels_resp.relation_labels or []

        entity_counts: dict[str, int] = {t: 0 for t in entity_types}
        relation_counts: dict[str, int] = {t: 0 for t in relation_types}
        all_files: set[str] = set()

        try:
            graph = await self.get_knowledge_graph(label="*")
            for node in graph.nodes:
                node_type = node.get("properties", {}).get("entity_type", "")
                if node_type in entity_counts:
                    entity_counts[node_type] += 1
                fp = node.get("file_path") or node.get("properties", {}).get(
                    "file_path", ""
                )
                if fp:
                    all_files.add(fp)

            for edge in graph.edges:
                keywords = edge.get("properties", {}).get("keywords", "")
                if keywords:
                    for kw in keywords.split(","):
                        kw = kw.strip()
                        if kw in relation_counts:
                            relation_counts[kw] += 1
        except Exception as e:
            self.logger.error(f"Error getting codebase stats: {e}")

        total_entities = sum(entity_counts.values())
        total_relations = sum(relation_counts.values())

        return CodebaseStatsResponse(
            total_entities=total_entities,
            total_relations=total_relations,
            entity_counts_by_type=entity_counts,
            relation_counts_by_type=relation_counts,
            unique_files=len(all_files),
            entity_types=entity_types,
            relation_types=relation_types,
        )

    async def get_entity_neighbors(self, entity_name: str) -> EntityNeighborsResponse:
        """Get all entities directly connected to an entity (1-hop neighbors)."""
        graph = await self.get_knowledge_graph(label="*")
        neighbors: list[EntityInfo] = []
        incoming: list[RelationInfo] = []
        outgoing: list[RelationInfo] = []

        for edge in graph.edges:
            src = edge.get("src_id") or edge.get("source_entity", "")
            tgt = edge.get("tgt_id") or edge.get("target_entity", "")

            if src == entity_name:
                outgoing.append(RelationInfo(**edge))
                for node in graph.nodes:
                    if node.get("id") == tgt or node.get("name") == tgt:
                        neighbors.append(EntityInfo(**node))
                        break
            elif tgt == entity_name:
                incoming.append(RelationInfo(**edge))
                for node in graph.nodes:
                    if node.get("id") == src or node.get("name") == src:
                        neighbors.append(EntityInfo(**node))
                        break

        unique_neighbors: dict[str, EntityInfo] = {}
        for n in neighbors:
            unique_neighbors[n.name] = n

        return EntityNeighborsResponse(
            entity_name=entity_name,
            neighbors=list(unique_neighbors.values()),
            incoming_relations=incoming,
            outgoing_relations=outgoing,
            total_neighbors=len(unique_neighbors),
            total_incoming=len(incoming),
            total_outgoing=len(outgoing),
        )

    async def get_entity_details(self, entity_name: str) -> EntityDetailsResponse:
        """Get full details of an entity including all its relations."""
        graph = await self.get_knowledge_graph(label="*")
        entity_info: Optional[EntityInfo] = None

        for node in graph.nodes:
            node_id = node.get("id") or node.get("name", "")
            if node_id == entity_name:
                entity_info = EntityInfo(**node)
                break

        if entity_info is None:
            raise LightRAGAPIError(f"Entity '{entity_name}' not found")

        incoming: list[RelationInfo] = []
        outgoing: list[RelationInfo] = []
        all_relations: list[RelationInfo] = []

        for edge in graph.edges:
            src = edge.get("src_id") or edge.get("source_entity", "")
            tgt = edge.get("tgt_id") or edge.get("target_entity", "")
            rel = RelationInfo(**edge)

            if src == entity_name or tgt == entity_name:
                all_relations.append(rel)
            if tgt == entity_name:
                incoming.append(rel)
            if src == entity_name:
                outgoing.append(rel)

        return EntityDetailsResponse(
            entity=entity_info,
            relations=all_relations,
            incoming_relations=incoming,
            outgoing_relations=outgoing,
        )

    async def list_entities_by_type(
        self, entity_type: str, limit: int = 100, offset: int = 0
    ) -> ListEntitiesResponse:
        """List entities of a specific type with pagination."""
        graph = await self.get_knowledge_graph(label="*")
        entities: list[EntityInfo] = []

        for node in graph.nodes:
            node_type = node.get("properties", {}).get("entity_type", "")
            if node_type == entity_type or node_type == entity_type.upper():
                entities.append(EntityInfo(**node))

        total = len(entities)
        paginated = entities[offset : offset + limit]

        return ListEntitiesResponse(
            entity_type=entity_type,
            entities=paginated,
            total=total,
            returned=len(paginated),
        )

    async def get_entities_in_file(self, file_path: str) -> GetEntitiesInFileResponse:
        """Get all entities from a specific file path."""
        entities: list[EntityInfo] = []
        fp_lower = file_path.lower().replace("\\", "/")

        try:
            graph = await self.get_knowledge_graph(label="*")
        except Exception as e:
            self.logger.error(f"Error fetching graph for file entities: {e}")
            return GetEntitiesInFileResponse(
                file_path=file_path,
                entities=[],
                total=0,
            )

        for node in graph.nodes:
            props = node.get("properties") or {}
            node_fp = node.get("file_path") or props.get("file_path", "")
            if node_fp:
                node_fp_clean = node_fp.lower().replace("\\", "/")
                if node_fp_clean == fp_lower:
                    entities.append(EntityInfo(**node))

        return GetEntitiesInFileResponse(
            file_path=file_path,
            entities=entities,
            total=len(entities),
        )

    async def search_entities(
        self, pattern: str, entity_type: Optional[str] = None, limit: int = 50
    ) -> SearchEntitiesResponse:
        """Search entities by regex pattern on names and descriptions."""
        try:
            compiled = re.compile(pattern, re.IGNORECASE)
        except re.error:
            raise LightRAGValidationError(f"Invalid regex pattern: {pattern}")

        try:
            graph = await self.get_knowledge_graph(label="*")
        except Exception as e:
            self.logger.error(f"Error fetching graph for search: {e}")
            return SearchEntitiesResponse(
                pattern=pattern,
                matches=[],
                total=0,
            )

        matches: list[EntityInfo] = []
        for node in graph.nodes:
            name = node.get("name", "")
            desc = node.get("properties", {}).get("description", "")

            if compiled.search(name) or compiled.search(desc):
                if entity_type:
                    node_type = node.get("properties", {}).get("entity_type", "")
                    if node_type.upper() != entity_type.upper():
                        continue
                matches.append(EntityInfo(**node))
                if len(matches) >= limit:
                    break

        return SearchEntitiesResponse(
            pattern=pattern,
            matches=matches,
            total=len(matches),
        )
