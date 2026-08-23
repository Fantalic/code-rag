"""
LightRAG API client for MCP server integration.
"""

import hashlib
import json
import logging
import re
import time
from typing import Any, Optional

import httpx

from .analyser_runner import run_analysis
from .filegraph import build_file_graph
from .models import (
    DeadCodeEntity,
    DeadCodeResponse,
    DocumentInfo,
    EntityCreateRequest,
    EntityCreateResponse,
    EntityDetailsResponse,
    EntityExistsResponse,
    EntityInfo,
    EntityNeighborsResponse,
    EntityUpdateRequest,
    EntityUpdateResponse,
    FileCyclesResponse,
    FileDependenciesResponse,
    FileStatInfo,
    FileStatsResponse,
    GetEntitiesInFileResponse,
    GetNodesResponse,
    GraphResponse,
    InsertResponse,
    InsertTextRequest,
    InterfaceMethodInfo,
    InterfaceMethodsResponse,
    LabelsResponse,
    ListEntitiesResponse,
    QueryDataResponse,
    QueryRequest,
    QueryResponse,
    RelatedFileInfo,
    RelatedFilesResponse,
    RelationCreateRequest,
    RelationCreateResponse,
    RelationDetailResponse,
    RelationInfo,
    RelationUpdateRequest,
    RelationUpdateResponse,
    SearchEntitiesResponse,
)


def _extract_entity_name(node: dict[str, Any], fallback: str) -> str:
    """Extract entity name from node with fallback chain."""
    name = (
        node.get("properties", {}).get("entity_name")
        or node.get("name")
        or node.get("id")
    )
    if not name:
        name = fallback
    return name


def _safe_entity_data(node: dict[str, Any], fallback_name: str) -> dict[str, Any]:
    """Build entity_data dict for EntityInfo construction with safe name extraction."""
    node_id = node.get("id") or node.get("name") or ""
    name = (
        node.get("properties", {}).get("entity_name")
        or node.get("name")
        or node.get("id")
    )
    if not name:
        name = fallback_name
    props = node.get("properties") or {}
    return {
        "id": node_id,
        "name": name,
        "type": props.get("entity_type"),
        "properties": props,
    }


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
        timeout: Optional[float] = None,
    ) -> dict[str, Any]:
        url = f"{self.base_url}{endpoint}"
        self.logger.debug(f"Making {method} request to {url}")

        effective_timeout = timeout if timeout is not None else self.timeout

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
            "POST", "/query", request_data.model_dump(), timeout=600.0
        )
        return QueryResponse(**response_data)

    # --- Knowledge Graph ---

    async def get_knowledge_graph(
        self, label: str = "*", max_depth: int = 3
    ) -> GraphResponse:
        """Retrieve the knowledge graph from LightRAG.

        Args:
            label: Entity label filter
            max_depth: Detail level — 1=types only, 2=+members, 3=all (default)
        """
        params = {"label": label}
        response_data = await self._make_request("GET", "/graphs", params=params)
        result = GraphResponse(**response_data)

        if max_depth >= 3:
            return result

        top_level_types = {"CLASS", "INTERFACE", "STRUCT", "ENUM", "NAMESPACE"}
        member_types = {"METHOD", "PROPERTY"}

        if max_depth == 1:
            keep_types = top_level_types
        else:
            keep_types = top_level_types | member_types

        filtered_nodes = []
        for node in result.nodes:
            props = node.get("properties") or {}
            ntype = props.get("entity_type", "")
            if ntype in keep_types:
                filtered_nodes.append(node)

        filtered_edges = []
        if max_depth >= 1:
            filtered_node_names = {
                n.get("id") or n.get("name", "") for n in filtered_nodes
            }
            for edge in result.edges:
                src = edge.get("source") or edge.get("src_id", "")
                tgt = edge.get("target") or edge.get("tgt_id", "")
                if src in filtered_node_names and tgt in filtered_node_names:
                    filtered_edges.append(edge)

        result.nodes = filtered_nodes
        result.edges = filtered_edges
        result.is_truncated = len(filtered_nodes) < len(result.nodes)
        return result

    async def get_graph_labels(
        self,
        node: Optional[str] = None,
        limit: int = 100,
        offset: int = 0,
    ) -> LabelsResponse:
        """Get entity + relation labels in the codebase.

        If `node` is given, restricts to types connected to that member
        (both incoming and outgoing). Otherwise returns all types in the
        codebase. Paginated via `offset`/`limit` for iteration over large
        lists — call again with higher offset to fetch the next page.
        """
        if node is None:
            response_data = await self._make_request("GET", "/graph/label/list")
            if isinstance(response_data, list):
                response_data = {"labels": response_data}
            base = LabelsResponse(**response_data)
        else:
            graph = await self.get_knowledge_graph(label="*")
            entity_types: set[str] = set()
            relation_keywords: set[str] = set()
            for edge in graph.edges:
                src = edge.get("source") or edge.get("src_id") or ""
                tgt = edge.get("target") or edge.get("tgt_id") or ""
                if node not in (src, tgt):
                    continue
                kw = edge.get("properties", {}).get("keywords", "")
                if kw:
                    for piece in (
                        kw.split(",") if isinstance(kw, str) else [str(p) for p in kw]
                    ):
                        piece = piece.strip()
                        if piece:
                            relation_keywords.add(piece)
                for endpoint_node in graph.nodes:
                    end_id = endpoint_node.get("id") or endpoint_node.get("name") or ""
                    end_props = endpoint_node.get("properties") or {}
                    end_name = end_props.get("entity_name") or end_id
                    if end_name in (src, tgt):
                        end_type = end_props.get("entity_type", "")
                        if end_type:
                            entity_types.add(end_type)
            base = LabelsResponse(
                entity_labels=sorted(entity_types),
                relation_labels=sorted(relation_keywords),
                node=node,
            )

        entity_total = len(base.entity_labels)
        relation_total = len(base.relation_labels)
        return LabelsResponse(
            entity_labels=base.entity_labels[offset : offset + limit],
            relation_labels=base.relation_labels[offset : offset + limit],
            node=base.node,
            entity_labels_total=entity_total,
            relation_labels_total=relation_total,
        )

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
                props = entity.get("properties") or {}
                fp = entity.get("file_path") or props.get("file_path")
                desc = entity.get("description") or props.get("description", "")
                if fp and fp not in seen_paths:
                    seen_paths.add(fp)
                    files.append(
                        RelatedFileInfo(
                            file_path=fp,
                            snippet=desc,
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

    # --- Derived file-level views ---

    async def get_file_dependencies(self, file_path: str) -> FileDependenciesResponse:
        """Return files this file references and files that depend on it.

        Derived on demand from the knowledge graph; no file nodes are stored.
        """
        graph = await self.get_knowledge_graph(label="*")
        fg = build_file_graph(graph.nodes, graph.edges)
        resolved = fg.resolve_file(file_path)
        if resolved is None:
            return FileDependenciesResponse(file_path=file_path, not_found=True)
        return FileDependenciesResponse(
            file_path=resolved,
            references=fg.references(resolved),
            dependents=fg.dependents(resolved),
            entities=fg.entities(resolved),
            not_found=False,
        )

    async def file_dependency_cycles(self) -> FileCyclesResponse:
        """Return dependency cycles between files (Tarjan SCC)."""
        graph = await self.get_knowledge_graph(label="*")
        fg = build_file_graph(graph.nodes, graph.edges)
        cycles = fg.cycles()
        return FileCyclesResponse(cycles=cycles, count=len(cycles))

    async def get_file_stats(self) -> FileStatsResponse:
        """Return per-file stats (nodes, fan-in, fan-out)."""
        graph = await self.get_knowledge_graph(label="*")
        fg = build_file_graph(graph.nodes, graph.edges)
        files = [
            FileStatInfo(
                file_path=f["file_path"],
                nodes=f["nodes"],
                fan_in=f["fan_in"],
                fan_out=f["fan_out"],
            )
            for f in fg.stats()
        ]
        return FileStatsResponse(files=files, total_files=len(files))

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
        """
        DEPRECATED: Analysis is now handled by the Go watchdog via `coderag scan`.
        This method is kept for potential future use but always returns a deprecation notice.
        Analysis in production should be triggered via `coderag scan <project-dir>` from the host CLI.
        The analyser binary (CSharpAnalyzerTool.exe) runs on the host, not in the MCP Docker container.
        """
        self.logger.warning(
            "update_relations is deprecated. "
            "Use 'coderag scan <project-dir>' from the CLI instead. "
            "Analysis is now handled by the Go watchdog when 'coderag mcp' is running."
        )
        return [
            "DEPRECATED: update_relations is no longer supported.",
            "Use 'coderag scan <project-dir>' to run the C# analyser and populate the knowledge graph.",
            "The watchdog automatically analyzes files when 'coderag mcp <project-dir>' is running.",
        ]

    # --- Phase 22: Enhanced exploration tools ---

    async def query_data(
        self, query: str, mode: str = "hybrid", only_need_context: bool = False
    ) -> QueryDataResponse:
        """Vector search using /query/data endpoint (LLM-free).

        The raw query is embedded with the server's embedding model and matched
        against the entity/relationship/chunk vector stores. Returns entities,
        relationships, chunks, and references without any LLM keyword
        extraction or generation.
        """
        request_data = QueryRequest(
            query=query, mode=mode, only_need_context=only_need_context
        )
        response_data = await self._make_request(
            "POST", "/query/data", request_data.model_dump()
        )
        return QueryDataResponse(**response_data)

    async def get_entity_neighbors(self, entity_name: str) -> EntityNeighborsResponse:
        """Get all entities directly connected to an entity (1-hop neighbors)."""
        graph = await self.get_knowledge_graph(label="*")
        neighbors: list[EntityInfo] = []
        incoming: list[RelationInfo] = []
        outgoing: list[RelationInfo] = []

        for edge in graph.edges:
            src = edge.get("source") or edge.get("src_id") or ""
            tgt = edge.get("target") or edge.get("tgt_id") or ""

            rel_data = {
                "id": edge.get("id", f"{src}-{tgt}"),
                "source_entity": src,
                "target_entity": tgt,
                "type": edge.get("type", ""),
                "properties": edge.get("properties", {}),
                "weight": edge.get("weight"),
            }
            rel = RelationInfo(**rel_data)

            if src == entity_name:
                outgoing.append(rel)
                for node in graph.nodes:
                    node_id = node.get("id") or node.get("name") or ""
                    if node_id == tgt:
                        fallback_name = (
                            f"neighbor_{node_id}"
                            if node_id
                            else f"neighbor_{hashlib.md5(str(node).encode()).hexdigest()[:8]}"
                        )
                        entity_data = _safe_entity_data(node, fallback_name)
                        try:
                            neighbors.append(EntityInfo(**entity_data))
                        except Exception:
                            entity_data["name"] = fallback_name
                            neighbors.append(EntityInfo(**entity_data))
                        break
            elif tgt == entity_name:
                incoming.append(rel)
                for node in graph.nodes:
                    node_id = node.get("id") or node.get("name") or ""
                    if node_id == src:
                        fallback_name = (
                            f"neighbor_{node_id}"
                            if node_id
                            else f"neighbor_{hashlib.md5(str(node).encode()).hexdigest()[:8]}"
                        )
                        entity_data = _safe_entity_data(node, fallback_name)
                        try:
                            neighbors.append(EntityInfo(**entity_data))
                        except Exception:
                            entity_data["name"] = fallback_name
                            neighbors.append(EntityInfo(**entity_data))
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

    async def get_entity_details(
        self, entity_name: str, depth: int = 3
    ) -> EntityDetailsResponse:
        """Get full details of an entity including its relations.

        Args:
            entity_name: Name of the entity
            depth: Detail level — 1=entity only, 2=structural, 3=full (default)
        """
        graph = await self.get_knowledge_graph(label="*")
        entity_info: Optional[EntityInfo] = None

        for node in graph.nodes:
            node_id = node.get("id") or node.get("name") or ""
            if node_id == entity_name:
                entity_data = _safe_entity_data(node, entity_name)
                try:
                    entity_info = EntityInfo(**entity_data)
                except Exception:
                    entity_data["name"] = entity_name
                    entity_info = EntityInfo(**entity_data)
                break

        if entity_info is None:
            raise LightRAGAPIError(f"Entity '{entity_name}' not found")

        incoming: list[RelationInfo] = []
        outgoing: list[RelationInfo] = []
        all_relations: list[RelationInfo] = []

        for edge in graph.edges:
            src = edge.get("source") or edge.get("src_id") or ""
            tgt = edge.get("target") or edge.get("tgt_id") or ""

            rel_data = {
                "id": edge.get("id", f"{src}-{tgt}"),
                "source_entity": src,
                "target_entity": tgt,
                "type": edge.get("type", ""),
                "properties": edge.get("properties", {}),
                "weight": edge.get("weight"),
            }
            rel = RelationInfo(**rel_data)

            if src == entity_name or tgt == entity_name:
                all_relations.append(rel)
            if tgt == entity_name:
                incoming.append(rel)
            if src == entity_name:
                outgoing.append(rel)

        # Filter by depth
        if depth < 3:
            structural_kw = {
                "contains",
                "defines",
                "has",
                "nested",
                "method",
                "property",
            }
            all_relations = [
                r
                for r in all_relations
                if structural_kw
                & set(
                    kw.strip()
                    for kw in r.properties.get("keywords", "").split(",")
                    if kw.strip()
                )
            ]
            incoming = [r for r in all_relations if r.target_entity == entity_name]
            outgoing = [r for r in all_relations if r.source_entity == entity_name]

        if depth == 1:
            all_relations = []
            incoming = []
            outgoing = []

        return EntityDetailsResponse(
            entity=entity_info,
            relations=all_relations,
            incoming_relations=incoming,
            outgoing_relations=outgoing,
        )

    async def list_entities_by_type(
        self,
        entity_type: str,
        node: Optional[str] = None,
        limit: int = 100,
        offset: int = 0,
    ) -> ListEntitiesResponse:
        """List entities of a specific type with pagination and optional node filter."""
        graph = await self.get_knowledge_graph(label="*")

        connected_names: Optional[set[str]] = None
        if node:
            connected_names = set()
            for edge in graph.edges:
                src = edge.get("source") or edge.get("src_id") or ""
                tgt = edge.get("target") or edge.get("tgt_id") or ""
                if src == node:
                    connected_names.add(tgt)
                elif tgt == node:
                    connected_names.add(src)

        entities: list[EntityInfo] = []
        for n in graph.nodes:
            node_type = n.get("properties", {}).get("entity_type", "")
            if node_type.lower() != entity_type.lower():
                continue
            if connected_names is not None:
                node_id = n.get("id") or n.get("name") or ""
                props = n.get("properties") or {}
                candidate_names = {
                    node_id,
                    props.get("entity_name", ""),
                    props.get("name", ""),
                }
                if not (candidate_names & connected_names):
                    continue
            node_id = n.get("id") or n.get("name") or ""
            fallback_name = (
                f"{entity_type.lower()}_{node_id}"
                if node_id
                else f"entity_{hashlib.md5(str(n).encode()).hexdigest()[:8]}"
            )
            entity_data = _safe_entity_data(n, fallback_name)
            try:
                entities.append(EntityInfo(**entity_data))
            except Exception:
                entity_data["name"] = fallback_name
                entities.append(EntityInfo(**entity_data))

        total = len(entities)
        paginated = entities[offset : offset + limit]

        return ListEntitiesResponse(
            entity_type=entity_type,
            node=node,
            entities=paginated,
            total=total,
            returned=len(paginated),
        )

    async def get_nodes(
        self,
        entity_type: str = "",
        name: str = "",
        file_path: str = "",
        connected_to: str = "",
        limit: int = 100,
        offset: int = 0,
    ) -> GetNodesResponse:
        """Filter entities by any combination of: entity_type, name (substring),
        file_path (substring), connected_to (graph connection). Empty/omitted
        filters are no-ops. All filters AND-combine. Paginated via offset/limit.
        """
        try:
            graph = await self.get_knowledge_graph(label="*")
        except Exception as e:
            self.logger.error(f"Error fetching graph for get_nodes: {e}")
            return GetNodesResponse(
                entities=[],
                total=0,
                returned=0,
                filters={
                    k: v
                    for k, v in {
                        "entity_type": entity_type,
                        "name": name,
                        "file_path": file_path,
                        "connected_to": connected_to,
                    }.items()
                    if v
                },
            )

        etype_lower = entity_type.strip().lower()
        name_lower = name.strip().lower()
        fpath_lower = file_path.strip().lower().replace("\\", "/")
        conn = connected_to.strip()

        connected_names: Optional[set[str]] = None
        if conn:
            connected_names = set()
            for edge in graph.edges:
                src = edge.get("source") or edge.get("src_id") or ""
                tgt = edge.get("target") or edge.get("tgt_id") or ""
                if src == conn:
                    connected_names.add(tgt)
                elif tgt == conn:
                    connected_names.add(src)

        entities: list[EntityInfo] = []
        for n in graph.nodes:
            props = n.get("properties") or {}
            node_id = n.get("id") or n.get("name") or ""
            entity_name = props.get("entity_name") or n.get("name") or node_id or ""
            node_type = props.get("entity_type", "")
            node_fp = n.get("file_path") or props.get("file_path", "")
            node_fp_clean = node_fp.lower().replace("\\", "/") if node_fp else ""

            if etype_lower and node_type.lower() != etype_lower:
                continue
            if name_lower and name_lower not in entity_name.lower():
                continue
            if fpath_lower and fpath_lower not in node_fp_clean:
                continue
            if connected_names is not None:
                candidates = {node_id, entity_name, props.get("name", "")}
                if not (candidates & connected_names):
                    continue

            fallback_name = (
                entity_name
                or f"entity_{node_id or hashlib.md5(str(n).encode()).hexdigest()[:8]}"
            )
            entity_data = _safe_entity_data(n, fallback_name)
            try:
                entities.append(EntityInfo(**entity_data))
            except Exception:
                entity_data["name"] = fallback_name
                entities.append(EntityInfo(**entity_data))

        total = len(entities)
        paginated = entities[offset : offset + limit]
        return GetNodesResponse(
            entities=paginated,
            total=total,
            returned=len(paginated),
            filters={
                k: v
                for k, v in {
                    "entity_type": entity_type,
                    "name": name,
                    "file_path": file_path,
                    "connected_to": connected_to,
                }.items()
                if v
            },
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
                    node_id = node.get("id") or node.get("name") or ""
                    fallback_name = (
                        f"entity_{node_id}"
                        if node_id
                        else f"file_entity_{hashlib.md5(str(node).encode()).hexdigest()[:8]}"
                    )
                    entity_data = _safe_entity_data(node, fallback_name)
                    try:
                        entities.append(EntityInfo(**entity_data))
                    except Exception:
                        entity_data["name"] = fallback_name
                        entities.append(EntityInfo(**entity_data))

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
            node_id = node.get("id") or node.get("name") or ""
            node_name = (
                node.get("properties", {}).get("entity_name")
                or node.get("name")
                or node_id
            )
            desc = node.get("properties", {}).get("description", "")

            if compiled.search(node_name) or compiled.search(desc):
                if entity_type:
                    node_type = node.get("properties", {}).get("entity_type", "")
                    if node_type.lower() != entity_type.lower():
                        continue
                fallback_name = (
                    node_name
                    if node_name
                    else f"search_{hashlib.md5(str(node).encode()).hexdigest()[:8]}"
                )
                entity_data = _safe_entity_data(node, fallback_name)
                try:
                    matches.append(EntityInfo(**entity_data))
                except Exception:
                    entity_data["name"] = fallback_name
                    matches.append(EntityInfo(**entity_data))
                if len(matches) >= limit:
                    break

        return SearchEntitiesResponse(
            pattern=pattern,
            matches=matches,
            total=len(matches),
        )

    async def get_relation_detail(
        self, entity_name: str, relation_type: str = ""
    ) -> RelationDetailResponse:
        """Get relations for an entity. If relation_type is omitted, all relations are returned."""
        try:
            graph = await self.get_knowledge_graph(label="*")
        except Exception as e:
            self.logger.error(f"Error fetching graph for relation detail: {e}")
            return RelationDetailResponse(
                entity_name=entity_name,
                relation_type=relation_type or "*",
                relations=[],
                total=0,
            )

        relations: list[RelationInfo] = []
        for edge in graph.edges:
            src = edge.get("source") or edge.get("src_id") or ""
            tgt = edge.get("target") or edge.get("tgt_id") or ""
            kw = edge.get("properties", {}).get("keywords", "")
            if src != entity_name and tgt != entity_name:
                continue
            if relation_type and relation_type.lower() not in kw.lower():
                continue
            rel_data = {
                "id": edge.get("id", f"{src}-{tgt}"),
                "source_entity": src,
                "target_entity": tgt,
                "type": kw,
                "properties": edge.get("properties", {}),
                "weight": edge.get("weight"),
            }
            try:
                relations.append(RelationInfo(**rel_data))
            except Exception:
                pass

        return RelationDetailResponse(
            entity_name=entity_name,
            relation_type=relation_type,
            relations=relations,
            total=len(relations),
        )

    async def get_node_details(
        self,
        entity_name: str,
        view: str = "details",
        relation_type: str = "",
        depth: int = 2,
    ):
        """Unified node lookup. `view` selects the response shape:
        - "details"   : node + incoming + outgoing + neighbors (depth-filtered)
        - "relations" : relations involving the node (optionally filtered)
        - "neighbors" : 1-hop neighbor entities + connecting relations
        """
        view_norm = (view or "details").lower()
        if view_norm == "relations":
            return await self.get_relation_detail(
                entity_name=entity_name, relation_type=relation_type
            )
        if view_norm == "neighbors":
            return await self.get_entity_neighbors(entity_name=entity_name)
        return await self.get_entity_details(entity_name=entity_name, depth=depth)

    async def find_related_files(self, entity_or_file: str) -> RelatedFilesResponse:
        """Find all files related to an entity or file path."""
        try:
            graph = await self.get_knowledge_graph(label="*")
        except Exception as e:
            self.logger.error(f"Error fetching graph for related files: {e}")
            return RelatedFilesResponse(
                query=entity_or_file, files=[], entities_per_file={}, total_files=0
            )

        related_files: dict[str, set[str]] = {}
        query_lower = entity_or_file.lower().replace("\\", "/")

        for node in graph.nodes:
            props = node.get("properties") or {}
            file_path = props.get("file_path", "")
            if not file_path:
                continue
            file_lower = file_path.lower().replace("\\", "/")

            match = False
            # Match by exact file path
            if file_lower == query_lower:
                match = True
            # Match by entity name
            node_name = props.get("entity_name") or node.get("id") or ""
            if query_lower in node_name.lower() or node_name.lower() == query_lower:
                match = True

            if match:
                related_files.setdefault(file_path, set()).add(node_name)

        # Also check edges: find entities connected to the query entity/file
        for edge in graph.edges:
            src = edge.get("source") or edge.get("src_id") or ""
            tgt = edge.get("target") or edge.get("tgt_id") or ""
            props = edge.get("properties") or {}
            src_fp = props.get("file_path", "")
            tgt_fp = props.get("file_path", "")

            for candidate in [src, tgt, src_fp, tgt_fp]:
                cand_lower = candidate.lower().replace("\\", "/")
                if query_lower in cand_lower or cand_lower == query_lower:
                    for node in graph.nodes:
                        n_props = node.get("properties") or {}
                        n_fp = n_props.get("file_path", "")
                        if n_fp:
                            n_fp_lower = n_fp.lower().replace("\\", "/")
                            if n_fp_lower not in {
                                f.lower().replace("\\", "/") for f in related_files
                            }:
                                n_name = (
                                    n_props.get("entity_name") or node.get("id") or ""
                                )
                                related_files.setdefault(n_fp, set()).add(n_name)

        files_list = sorted(related_files.keys())
        entities_per_file = {
            f: sorted(list(entities)) for f, entities in related_files.items()
        }

        return RelatedFilesResponse(
            query=entity_or_file,
            files=files_list,
            entities_per_file=entities_per_file,
            total_files=len(files_list),
        )

    async def get_interface_methods(
        self, interface_name: str
    ) -> InterfaceMethodsResponse:
        """Get methods declared by an interface and classes that implement it."""
        try:
            graph = await self.get_knowledge_graph(label="*")
        except Exception as e:
            self.logger.error(f"Error fetching graph for interface methods: {e}")
            return InterfaceMethodsResponse(
                interface_name=interface_name,
                methods=[],
                implementing_classes=[],
            )

        methods: list[InterfaceMethodInfo] = []
        implementing_classes: list[str] = []

        for edge in graph.edges:
            src = edge.get("source") or edge.get("src_id") or ""
            tgt = edge.get("target") or edge.get("tgt_id") or ""
            kw = edge.get("properties", {}).get("keywords", "")

            # Interface declares method: interface -> method (via defines)
            if src == interface_name and kw == "defines,method":
                methods.append(
                    InterfaceMethodInfo(
                        method_name=tgt,
                        description=edge.get("properties", {}).get("description", ""),
                    )
                )

            # Class implements interface: class --implements--> interface
            if tgt == interface_name and kw == "implements":
                implementing_classes.append(src)

        return InterfaceMethodsResponse(
            interface_name=interface_name,
            methods=methods,
            implementing_classes=implementing_classes,
        )

    _VIS_SUFFIX_RE = re.compile(
        r"\[(public|private|internal|protected|protected internal)\]\s*$"
    )

    async def find_dead_code(
        self,
        min_age_days: int = 0,
        entity_type: str = "",
    ) -> DeadCodeResponse:
        """Find entities that are unreferenced and not exported.

        An entity is dead when ALL of:
          - it has zero incoming relations,
          - AND (its visibility is `private` or `internal`,
                 OR it is public but not in its file's # EXPORTS list),
          - AND it is not an extension method, abstract method,
            or interface member.

        Extension methods are detected by a `this ` modifier before
        the first parameter type in the description (e.g.
        ``... -> RETURNS -> int [public]`` for a regular method vs.
        ``this string s, int times`` for an extension method).

        Args:
            min_age_days: 0 = include all files; >0 = require file mtime
                older than `now - min_age_days*86400`. Files without a
                known mtime are excluded when this filter is active.
            entity_type: optional filter (case-insensitive). Empty =
                all types. Matches LightRAG's `entity_type` property.

        Returns:
            DeadCodeResponse with one row per dead candidate, sorted by
            file_path then name.
        """
        try:
            graph = await self.get_knowledge_graph(label="*")
        except Exception as e:
            self.logger.error(f"Error fetching graph for find_dead_code: {e}")
            return DeadCodeResponse(
                entities=[],
                total=0,
                filters={
                    "min_age_days": min_age_days,
                    "entity_type": entity_type,
                },
            )

        etype_filter = entity_type.strip().lower()

        # Build per-file EXPORTS map and per-file mtime map from the
        # EXPORTS metadata entities emitted by the C# walker.
        exports_by_file: dict[str, set[str]] = {}
        mtime_by_file: dict[str, int] = {}
        for n in graph.nodes:
            props = n.get("properties") or {}
            node_type = (props.get("entity_type") or "").upper()
            if node_type != "EXPORTS":
                continue
            fp_raw = n.get("file_path") or props.get("file_path", "")
            fp = fp_raw.lower().replace("\\", "/") if fp_raw else ""
            if not fp:
                continue
            desc = props.get("description") or n.get("description") or ""
            exports = {
                name.strip()
                for name in desc.split(",")
                if name.strip() and "[MTIME=" not in name
            }
            # Strip the [MTIME=...] tail that may be glued onto the
            # last export name.
            exports = {
                re.sub(r"\s*\[MTIME=\d+\]\s*$", "", name).strip() for name in exports
            }
            exports_by_file[fp] = {x for x in exports if x}
            m = re.search(r"\[MTIME=(\d+)\]", desc)
            if m:
                try:
                    mtime_by_file[fp] = int(m.group(1))
                except ValueError:
                    pass

        # Incoming-edge count per entity (only "real" usages, not
        # structural definitions). Structural relations like
        # "method, defines" (class -> method) and "property, has"
        # (class -> property) are ownership, not usage; they should
        # not count as "the method is used". Similarly "contains,
        # nested", "inheritance, extends", "implements, interface",
        # "has_note, annotation", and "depends_on, references" are
        # structural. Only edges whose keywords contain "calls" or
        # similar real-use signals count as a usage.
        structural_keywords = (
            "method, defines",
            "property, has",
            "contains, nested",
            "inheritance, extends",
            "implements, interface",
            "has_note, annotation",
            "depends_on, references",
        )
        incoming: dict[str, int] = {}
        for edge in graph.edges:
            tgt = edge.get("target") or edge.get("tgt_id") or ""
            if not tgt:
                continue
            kw = (edge.get("properties") or {}).get("keywords") or ""
            if kw in structural_keywords:
                continue
            incoming[tgt] = incoming.get(tgt, 0) + 1

        now = int(time.time())
        # Build a set of names that are INTERFACE declarations so we
        # can skip methods/properties declared inside them (they're
        # required to be implemented, not dead).
        interface_names: set[str] = set()
        for n in graph.nodes:
            if (n.get("properties") or {}).get(
                "entity_type", ""
            ).upper() == "INTERFACE":
                nm = (
                    (n.get("properties") or {}).get("entity_name")
                    or n.get("name")
                    or n.get("id")
                    or ""
                )
                if nm:
                    interface_names.add(nm)

        candidates: list[DeadCodeEntity] = []
        for n in graph.nodes:
            props = n.get("properties") or {}
            node_type = props.get("entity_type") or ""
            if node_type.upper() == "EXPORTS":
                continue
            if etype_filter and node_type.lower() != etype_filter:
                continue

            node_id = n.get("id") or n.get("name") or ""
            entity_name = props.get("entity_name") or n.get("name") or node_id or ""
            if not entity_name:
                continue

            fp_raw = n.get("file_path") or props.get("file_path", "")
            fp = fp_raw.lower().replace("\\", "/") if fp_raw else ""
            if not fp:
                continue

            description = props.get("description") or ""

            vis_match = self._VIS_SUFFIX_RE.search(description)
            visibility = vis_match.group(1) if vis_match else ""
            if not visibility:
                # Unknown visibility -> exclude (safer than guessing).
                continue

            # Strip the [vis] suffix before further regex checks so
            # they don't accidentally match the bracket text.
            description_clean = self._VIS_SUFFIX_RE.sub("", description).strip()

            # Skip interface members — they're required to be implemented.
            if node_type.upper() == "INTERFACE":
                continue
            # Skip methods/properties whose parent entity is an
            # interface (they must be implemented by something).
            if "." in entity_name:
                parent_name = entity_name.rsplit(".", 1)[0]
                if parent_name in interface_names:
                    continue
            # Skip abstract methods — they must be overridden.
            if "abstract" in description_clean.lower():
                continue
            # Skip extension methods (heuristic: "this " appears right
            # after the method name in the description).
            if self._is_extension_method(entity_name, description_clean):
                continue

            # Incoming-edge count.
            if incoming.get(entity_name, 0) > 0:
                continue

            # EXPORTS check.
            in_exports = entity_name in exports_by_file.get(fp, set())
            exports_list = exports_by_file.get(fp, set())
            # Also try the unqualified name for methods/properties:
            # an entry like "TestApp.OrderService.PlaceOrder" should
            # match the file's export "TestApp.OrderService.PlaceOrder".
            if not in_exports and "." in entity_name:
                short = entity_name.rsplit(".", 1)[-1]
                if short in exports_list:
                    in_exports = True

            is_private_or_internal = visibility in {"private", "internal"}
            is_public_unexported = visibility == "public" and not in_exports
            if not (is_private_or_internal or is_public_unexported):
                continue

            # Age filter (mtime is per file).
            if min_age_days > 0:
                mtime = mtime_by_file.get(fp, 0)
                if mtime == 0:
                    continue
                if (now - mtime) < min_age_days * 86400:
                    continue

            if is_private_or_internal:
                reason = visibility
            else:
                reason = "not_exported"

            line_no = 0
            try:
                line_no = int(props.get("line_no") or 0)
            except (TypeError, ValueError):
                line_no = 0

            candidates.append(
                DeadCodeEntity(
                    name=entity_name,
                    type=node_type,
                    file_path=fp_raw,
                    line_no=line_no,
                    visibility=visibility,
                    reason=reason,
                    in_exports=in_exports,
                )
            )

        candidates.sort(key=lambda e: (e.file_path, e.name))
        return DeadCodeResponse(
            entities=candidates,
            total=len(candidates),
            filters={
                "min_age_days": min_age_days,
                "entity_type": entity_type,
            },
        )

    @staticmethod
    def _is_extension_method(entity_name: str, description: str) -> bool:
        """Return True if the method's description starts with
        `<receiver> this <Type>` pattern (i.e. it's an extension method).

        The walker's descriptions for extension methods look like
        ``TestApp.X.ExtendTo(this string s, int times) -> RETURNS -> int``.
        """
        # After the method name and `(`, the first parameter for an
        # extension method starts with "this ".
        if "this " not in description:
            return False
        # Quick guard: "this " must appear within the parameter list,
        # not in arbitrary prose. The walker always emits the method
        # name followed by an opening paren.
        idx = description.find(entity_name.rsplit(".", 1)[-1])
        if idx < 0:
            return False
        paren = description.find("(", idx)
        if paren < 0:
            return False
        closing = description.find(")", paren)
        if closing < 0:
            return False
        param_block = description[paren + 1 : closing]
        stripped = param_block.lstrip()
        return stripped.startswith("this ")
