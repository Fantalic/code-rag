"""
LightRAG API client for MCP server integration.
"""

import json
import logging
from typing import Any, Optional

import httpx

from .analyser_runner import run_analysis
from .models import (
    DocumentInfo,
    EntityCreateRequest,
    EntityCreateResponse,
    EntityExistsResponse,
    EntityUpdateRequest,
    EntityUpdateResponse,
    GraphResponse,
    InsertResponse,
    InsertTextRequest,
    LabelsResponse,
    QueryRequest,
    QueryResponse,
    RelatedFilesResponse,
    RelationCreateRequest,
    RelationCreateResponse,
    RelationUpdateRequest,
    RelationUpdateResponse,
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
        """Get files related to a query or keywords."""
        query_result = await self.query_text(query, mode=mode)
        related_files = []
        if hasattr(query_result, "results") and query_result.results:
            for r in query_result.results[:limit]:
                related_files.append(
                    DocumentInfo(
                        id=r.document_id,
                        title=r.document_id,
                        snippet=r.snippet,
                    )
                )
        return RelatedFilesResponse(
            related_files=related_files,
            query=query,
            total_found=len(related_files),
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
                self.logger.info(
                    f"Ingested {filename} -> track_id={tid}"
                )
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
