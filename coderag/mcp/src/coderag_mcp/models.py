"""
Pydantic models for LightRAG API requests and responses.
"""

from enum import Enum
from typing import Any, Optional

from pydantic import BaseModel, Field


# Enums for status types and mode parameters
class DocStatus(str, Enum):
    """Document status enumeration."""

    PENDING = "pending"
    PROCESSING = "processing"
    PROCESSED = "processed"
    FAILED = "failed"
    DELETED = "deleted"


class QueryMode(str, Enum):
    """Query mode enumeration."""

    NAIVE = "naive"
    LOCAL = "local"
    GLOBAL = "global"
    HYBRID = "hybrid"


class PipelineStatus(str, Enum):
    """Pipeline status enumeration."""

    IDLE = "idle"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"


# Common Models
class TextDocument(BaseModel):
    """Text document model."""

    title: Optional[str] = None
    content: str = Field(..., description="Document content")
    metadata: Optional[dict[str, Any]] = None


class PaginationInfo(BaseModel):
    """Pagination information model."""

    page: int = Field(1, ge=1, description="Page number")
    page_size: int = Field(10, ge=1, le=100, description="Number of items per page")
    total_items: Optional[int] = Field(None, description="Total number of items")
    total_pages: Optional[int] = Field(None, ge=1, description="Total number of pages")
    has_next: bool = Field(False, description="Whether there is a next page")
    has_prev: bool = Field(False, description="Whether there is a previous page")


class ValidationError(BaseModel):
    """Validation error model."""

    loc: list[str | int]
    msg: str
    type: str


class HTTPValidationError(BaseModel):
    """HTTP validation error model."""

    detail: list[ValidationError]


# Document Management Request Models
class InsertTextRequest(BaseModel):
    """Request model for inserting a single text document."""

    text: str = Field(..., description="Text content to insert")
    file_source: str = Field(
        default="text_input.txt", description="Source file name for the text"
    )


class InsertTextsRequest(BaseModel):
    """Request model for inserting multiple text documents."""

    texts: list[str] = Field(..., description="List of text strings to insert")
    file_sources: list[str] = Field(
        default_factory=list, description="List of file sources for the texts"
    )


class DeleteDocRequest(BaseModel):
    """Request model for deleting a document by ID."""

    doc_ids: list[str] = Field(..., description="List of document IDs to delete")


class DeleteEntityRequest(BaseModel):
    """Request model for deleting an entity."""

    entity_id: str = Field(..., description="ID of the entity to delete")
    entity_name: str = Field(..., description="Name of the entity to delete")


class DeleteRelationRequest(BaseModel):
    """Request model for deleting a relation."""

    relation_id: str = Field(..., description="ID of the relation to delete")
    source_entity: str = Field(..., description="Source entity name")
    target_entity: str = Field(..., description="Target entity name")


class DocumentsRequest(BaseModel):
    """Request model for paginated documents."""

    page: int = Field(1, ge=1, description="Page number")
    page_size: int = Field(10, ge=1, le=100, description="Number of items per page")
    status_filter: Optional[DocStatus] = None


class ClearCacheRequest(BaseModel):
    """Request model for clearing cache."""

    cache_type: Optional[str] = None


# Query Request Models
class QueryRequest(BaseModel):
    """Request model for text queries."""

    query: str = Field(..., description="Query text")
    mode: QueryMode = Field(QueryMode.HYBRID, description="Query mode")
    only_need_context: bool = Field(False, description="Whether to only return context")
    stream: bool = Field(False, description="Whether to stream results")


# Knowledge Graph Request Models
class EntityCreateRequest(BaseModel):
    """Request model for creating an entity."""

    entity_name: str = Field(..., description="Name of the entity to create")
    entity_data: dict[str, Any] = Field(
        default_factory=dict,
        description="Entity properties (description, entity_type, etc.)",
    )


class EntityUpdateRequest(BaseModel):
    """Request model for updating an entity."""

    entity_id: str = Field(..., description="ID of the entity to update")
    entity_name: str = Field(..., description="Name of the entity to update")
    updated_data: dict[str, Any] = Field(..., description="Updated data for the entity")


class RelationCreateRequest(BaseModel):
    """Request model for creating a relation."""

    source_entity: str = Field(..., description="Source entity name")
    target_entity: str = Field(..., description="Target entity name")
    relation_data: dict[str, Any] = Field(
        default_factory=dict,
        description="Relation properties (description, keywords, weight, etc.)",
    )


class RelationUpdateRequest(BaseModel):
    """Request model for updating a relation."""

    source_id: str = Field(..., description="Source entity ID")
    target_id: str = Field(..., description="Target entity ID")
    updated_data: dict[str, Any] = Field(
        ..., description="Updated data for the relation"
    )


class BatchRelationUpdateRequest(BaseModel):
    """Request model for batch updating relations."""

    updates: list[RelationUpdateRequest] = Field(
        ..., description="List of relation updates"
    )


class RelatedFilesRequest(BaseModel):
    """Request model for getting related files."""

    query: str = Field(..., description="Query or keywords to find related files")
    mode: QueryMode = Field(QueryMode.HYBRID, description="Query mode")
    limit: int = Field(
        10, ge=1, le=100, description="Maximum number of related files to return"
    )


class RelatedFilesResponse(BaseModel):
    """Response model for related files."""

    related_files: list["DocumentInfo"] = Field(
        default_factory=list, description="List of related document info"
    )
    query: str = Field(..., description="Original query")
    total_found: int = Field(0, description="Total number of related files found")


class EntityExistsRequest(BaseModel):
    """Request model for checking if entity exists."""

    entity_name: str = Field(..., description="Name of the entity to check")


# Authentication Request Models
class LoginRequest(BaseModel):
    """Request model for login."""

    username: str = Field(..., description="Username")
    password: str = Field(..., description="Password")


# Document Management Response Models
class InsertResponse(BaseModel):
    """Response model for document insertion."""

    status: str = "success"
    message: str = ""
    track_id: str = ""
    id: Optional[str] = None


class ScanResponse(BaseModel):
    """Response model for document scanning."""

    status: str = "success"
    message: str = ""
    track_id: str = ""
    scanned: int = 0
    new_documents: list[str] = Field(
        default_factory=list, description="List of new document names"
    )


class UploadResponse(BaseModel):
    """Response model for file upload."""

    status: str = "success"
    filename: Optional[str] = None
    message: Optional[str] = None
    track_id: Optional[str] = None


class DocumentInfo(BaseModel):
    """Document information model."""

    id: str = Field(..., description="Document ID")
    title: Optional[str] = None
    status: DocStatus = Field(DocStatus.PROCESSED, description="Document status")
    created_at: Optional[str] = None
    updated_at: Optional[str] = None
    metadata: Optional[dict[str, Any]] = None
    snippet: Optional[str] = None


class DocumentsResponse(BaseModel):
    """Response model for retrieving documents."""

    documents: list[DocumentInfo] = Field(
        default_factory=list, description="Document list"
    )
    statuses: dict[str, Any] = Field(
        default_factory=dict, description="Document statuses"
    )


class PaginatedDocsResponse(BaseModel):
    """Response model for paginated documents."""

    documents: list[DocumentInfo] = Field(default_factory=list)
    pagination: PaginationInfo = Field(..., description="Pagination information")
    status_counts: dict[str, int] = Field(
        default_factory=dict, description="Status counts"
    )


class DeleteDocByIdResponse(BaseModel):
    """Response model for document deletion by ID."""

    status: str = "success"
    deleted: bool = False
    doc_id: Optional[str] = None
    document_id: Optional[str] = None
    message: Optional[str] = None


class ClearDocumentsResponse(BaseModel):
    """Response model for clearing all documents."""

    status: str = Field(..., description="Clearing status")
    message: Optional[str] = None


class PipelineStatusResponse(BaseModel):
    """Response model for pipeline status."""

    status: str = "idle"
    autoscanned: bool = False
    busy: bool = False
    progress: Optional[float] = Field(
        None, ge=0, le=100, description="Progress percentage"
    )
    current_task: Optional[str] = None
    job_name: Optional[str] = None
    job_start: Optional[str] = None
    docs: Optional[int] = None
    batchs: Optional[int] = None
    cur_batch: Optional[int] = None
    request_pending: Optional[bool] = None
    latest_message: Optional[str] = None
    history_messages: Optional[list[str]] = None
    update_status: Optional[dict[str, Any]] = None
    message: Optional[str] = None


class TrackStatusResponse(BaseModel):
    """Response model for track status."""

    track_id: str = ""
    status: str = ""
    progress: Optional[float] = None
    documents: list[dict[str, Any]] = Field(
        default_factory=list, description="Documents in track"
    )
    total_count: int = Field(0, description="Total document count")
    status_summary: dict[str, Any] = Field(
        default_factory=dict, description="Status summary"
    )


class StatusCountsResponse(BaseModel):
    """Response model for document status counts."""

    pending: int = Field(0, ge=0)
    processing: int = Field(0, ge=0)
    processed: int = Field(0, ge=0)
    failed: int = Field(0, ge=0)
    total: int = Field(0, ge=0)
    status_counts: dict[str, int] = Field(
        default_factory=dict, description="Status counts mapping"
    )


class ClearCacheResponse(BaseModel):
    """Response model for cache clearing."""

    status: str = "success"
    message: Optional[str] = None
    cache_type: Optional[str] = None


class DeletionResult(BaseModel):
    """Response model for entity/relation deletion."""

    deleted: bool = False
    id: str = ""
    type: str = ""
    message: Optional[str] = None


# Query Response Models
class QueryResult(BaseModel):
    """Query result model."""

    document_id: str = Field(..., description="Document ID")
    snippet: str = Field(..., description="Text snippet")
    score: Optional[float] = Field(None, ge=0, le=1)
    metadata: Optional[dict[str, Any]] = None


class QueryResponse(BaseModel):
    """Response model for text queries."""

    response: str = ""
    query: Optional[str] = None
    results: Optional[list[QueryResult]] = None
    total_results: Optional[int] = None
    processing_time: Optional[float] = None
    context: Optional[str] = None


# Knowledge Graph Response Models
class EntityInfo(BaseModel):
    """Entity information model."""

    id: str = Field(..., description="Entity ID")
    name: str = Field(..., description="Entity name")
    type: Optional[str] = None
    properties: dict[str, Any] = Field(default_factory=dict)
    created_at: Optional[str] = None
    updated_at: Optional[str] = None


class RelationInfo(BaseModel):
    """Relation information model."""

    id: str = Field(..., description="Relation ID")
    source_entity: str = Field(..., description="Source entity ID")
    target_entity: str = Field(..., description="Target entity ID")
    type: str = Field(..., description="Relation type")
    properties: dict[str, Any] = Field(default_factory=dict)
    weight: Optional[float] = Field(None, ge=0, le=1)
    created_at: Optional[str] = None
    updated_at: Optional[str] = None


class GraphResponse(BaseModel):
    """Response model for knowledge graph."""

    nodes: list[dict[str, Any]] = Field(
        default_factory=list, description="Graph nodes (entities)"
    )
    edges: list[dict[str, Any]] = Field(
        default_factory=list, description="Graph edges (relations)"
    )
    is_truncated: bool = Field(False, description="Whether the graph is truncated")

    @property
    def entities(self) -> list[EntityInfo]:
        """Entities parsed from nodes."""
        return [EntityInfo(**n) for n in self.nodes]

    @property
    def relations(self) -> list[RelationInfo]:
        """Relations parsed from edges."""
        return [RelationInfo(**e) for e in self.edges]


class LabelsResponse(BaseModel):
    """Response model for codebase_types (entity + relation labels)."""

    entity_labels: list[str] = Field(default_factory=list)
    relation_labels: list[str] = Field(default_factory=list)
    node: Optional[str] = Field(
        None,
        description="If set, lists are restricted to types connected to this member.",
    )
    entity_labels_total: int = Field(
        0, description="Total entity labels before pagination"
    )
    relation_labels_total: int = Field(
        0, description="Total relation labels before pagination"
    )


class EntityExistsResponse(BaseModel):
    """Response model for entity existence check."""

    exists: bool = False
    entity_name: Optional[str] = None
    entity_id: Optional[str] = None


class EntityCreateResponse(BaseModel):
    """Response model for entity creation."""

    status: str = "success"
    created: bool = False
    entity_name: str = ""
    message: str = ""
    data: dict[str, Any] = Field(default_factory=dict)


class RelationCreateResponse(BaseModel):
    """Response model for relation creation."""

    status: str = "success"
    created: bool = False
    source_entity: str = ""
    target_entity: str = ""
    message: str = ""
    data: dict[str, Any] = Field(default_factory=dict)


class AddNoteResponse(BaseModel):
    """Response model for attach_note_to_node."""

    status: str = "success"
    entity_name: str = ""
    note_entity_name: str = ""
    note_text: str = ""
    relationship_created: bool = False
    message: str = ""


class NoteInfo(BaseModel):
    """One note attached to a node."""

    note_entity_name: str = Field(..., description="Graph name of the NOTE entity")
    description: str = Field("", description="Note text including provenance header")
    created_at: Optional[str] = Field(
        None, description="ISO timestamp when the note was attached"
    )


class NotesResponse(BaseModel):
    """Response model for get_notes (notes attached to a node)."""

    node: str = Field(..., description="Member/node the notes are attached to")
    notes: list[NoteInfo] = Field(default_factory=list)
    total: int = Field(0, description="Number of notes returned")


class EntityUpdateResponse(BaseModel):
    """Response model for entity update."""

    status: str = "success"
    updated: bool = False
    entity_id: str = ""
    message: str = ""
    data: dict[str, Any] = Field(default_factory=dict)


class RelationUpdateResponse(BaseModel):
    """Response model for relation update."""

    status: str = "success"
    updated: bool = False
    relation_id: str = ""
    message: str = ""
    data: dict[str, Any] = Field(default_factory=dict)


# System Management Response Models
class HealthResponse(BaseModel):
    """Response model for health check."""

    status: str = "healthy"
    version: Optional[str] = None
    uptime: Optional[float] = None
    database_status: Optional[str] = None
    cache_status: Optional[str] = None
    message: Optional[str] = None


# Authentication Response Models
class AuthStatusResponse(BaseModel):
    """Response model for authentication status."""

    authenticated: bool = Field(..., description="Whether user is authenticated")
    user: Optional[str] = None


class LoginResponse(BaseModel):
    """Response model for login."""

    success: bool = Field(..., description="Whether login was successful")
    token: Optional[str] = None
    user: Optional[str] = None
    message: Optional[str] = None


# Ollama API Models (for completeness)
class OllamaVersionResponse(BaseModel):
    """Response model for Ollama version."""

    version: str = Field(..., description="Ollama version")


class OllamaTagsResponse(BaseModel):
    """Response model for Ollama tags."""

    models: list[dict[str, Any]] = Field(default_factory=list)


class OllamaProcessResponse(BaseModel):
    """Response model for Ollama running processes."""

    models: list[dict[str, Any]] = Field(default_factory=list)


class OllamaGenerateRequest(BaseModel):
    """Request model for Ollama generate."""

    model: str = Field(..., description="Model name")
    prompt: str = Field(..., description="Prompt text")
    stream: bool = Field(False, description="Whether to stream response")


class OllamaChatMessage(BaseModel):
    """Chat message model for Ollama."""

    role: str = Field(..., description="Message role (user/assistant)")
    content: str = Field(..., description="Message content")


class OllamaChatRequest(BaseModel):
    """Request model for Ollama chat."""

    model: str = Field(..., description="Model name")
    messages: list[OllamaChatMessage] = Field(..., description="Chat messages")
    stream: bool = Field(False, description="Whether to stream response")


# File upload models
class Body_upload_to_input_dir_documents_upload_post(BaseModel):
    """Request body for file upload."""

    file: bytes = Field(..., description="File content")


class Body_login_login_post(BaseModel):
    """Request body for login."""

    username: str = Field(..., description="Username")
    password: str = Field(..., description="Password")


# Status response models
class DocStatusResponse(BaseModel):
    """Response model for document status."""

    document_id: str = Field(..., description="Document ID")
    status: DocStatus = Field(..., description="Document status")
    message: Optional[str] = None


class DocsStatusesResponse(BaseModel):
    """Response model for multiple document statuses."""

    statuses: list[DocStatusResponse] = Field(default_factory=list)
    total: int = Field(0, ge=0)


# Phase 22: MCP Tool Improvement Models
# ---------------------------------------------------------------------------


class QueryDataResponse(BaseModel):
    """Response model for /query/data structured queries."""

    status: str = Field(..., description="Query execution status")
    message: str = Field(default="", description="Status message")
    data: dict[str, Any] = Field(
        default_factory=dict,
        description="Query result data: entities, relationships, chunks, references",
    )
    metadata: dict[str, Any] = Field(
        default_factory=dict, description="Query metadata: mode, keywords, tokens"
    )


class EntityNeighborsResponse(BaseModel):
    """Response model for entity neighbor queries."""

    entity_name: str = Field(..., description="Source entity name")
    neighbors: list[EntityInfo] = Field(
        default_factory=list, description="Directly connected entities"
    )
    incoming_relations: list[RelationInfo] = Field(
        default_factory=list, description="Relations pointing TO this entity"
    )
    outgoing_relations: list[RelationInfo] = Field(
        default_factory=list, description="Relations from this entity to others"
    )
    total_neighbors: int = Field(0, description="Total neighbor count")
    total_incoming: int = Field(0, description="Total incoming relation count")
    total_outgoing: int = Field(0, description="Total outgoing relation count")


class EntityDetailsResponse(BaseModel):
    """Response model for full entity details with relations."""

    entity: EntityInfo = Field(..., description="Entity information")
    relations: list[RelationInfo] = Field(
        default_factory=list, description="All relations involving this entity"
    )
    incoming_relations: list[RelationInfo] = Field(
        default_factory=list, description="Relations pointing TO this entity"
    )
    outgoing_relations: list[RelationInfo] = Field(
        default_factory=list, description="Relations from this entity"
    )


class ListEntitiesRequest(BaseModel):
    """Request model for listing entities by type."""

    entity_type: str = Field(
        ..., description="Entity type to list (e.g. CLASS, METHOD)"
    )
    limit: int = Field(100, ge=1, le=500, description="Maximum entities to return")
    offset: int = Field(0, ge=0, description="Offset for pagination")


class ListEntitiesResponse(BaseModel):
    """Response model for entity listing."""

    entity_type: str = Field(..., description="Entity type requested")
    node: Optional[str] = Field(
        None, description="If set, only nodes connected to this one are listed"
    )
    entities: list[EntityInfo] = Field(default_factory=list)
    total: int = Field(0, description="Total entities of this type (after node filter)")
    returned: int = Field(0, description="Entities returned in this response")


class GetNodesResponse(BaseModel):
    """Response model for get_nodes (filter-by-anything)."""

    entities: list[EntityInfo] = Field(
        default_factory=list, description="Matching entities"
    )
    total: int = Field(0, description="Total matches before pagination")
    returned: int = Field(0, description="Entities returned in this page")
    filters: dict[str, str] = Field(
        default_factory=dict, description="Echo of non-empty filters applied"
    )


class SearchEntitiesRequest(BaseModel):
    """Request model for searching entities by pattern."""

    pattern: str = Field(..., description="Regex pattern to match against entity names")
    entity_type: Optional[str] = Field(None, description="Optional entity type filter")
    limit: int = Field(50, ge=1, le=200, description="Maximum results to return")


class SearchEntitiesResponse(BaseModel):
    """Response model for entity search."""

    pattern: str = Field(..., description="Search pattern used")
    matches: list[EntityInfo] = Field(default_factory=list)
    total: int = Field(0, description="Total matches found")


class GetEntitiesInFileRequest(BaseModel):
    """Request model for getting entities from a specific file."""

    file_path: str = Field(..., description="File path to filter by")
    entity_type: Optional[str] = Field(None, description="Optional entity type filter")


class GetEntitiesInFileResponse(BaseModel):
    """Response model for entities in a file."""

    file_path: str = Field(..., description="File path queried")
    entities: list[EntityInfo] = Field(default_factory=list)
    total: int = Field(0, description="Total entities from this file")


class RelatedFileInfo(BaseModel):
    """File info for related files responses."""

    file_path: str = Field(..., description="File path")
    chunk_id: Optional[str] = None
    snippet: Optional[str] = None
    score: Optional[float] = None


class RelatedFilesResponseV2(BaseModel):
    """Response model for get_related_files (fixed to use /query/data)."""

    files: list[RelatedFileInfo] = Field(default_factory=list)
    query: str = Field(..., description="Original query")
    total_found: int = Field(0, description="Total files found")


class RelationDetailResponse(BaseModel):
    """Response model for get_relation_detail tool."""

    entity_name: str = Field(..., description="Entity queried")
    relation_type: str = Field(..., description="Relation type queried")
    relations: list[RelationInfo] = Field(
        default_factory=list, description="Matching relations"
    )
    total: int = Field(0, description="Total matching relations")


class RelatedFilesResponse(BaseModel):
    """Response model for find_related_files tool."""

    query: str = Field(..., description="Entity or file path queried")
    files: list[str] = Field(default_factory=list, description="Related file paths")
    entities_per_file: dict[str, list[str]] = Field(
        default_factory=dict, description="Entities found in each file"
    )
    total_files: int = Field(0, description="Total related files found")


class InterfaceMethodInfo(BaseModel):
    """Method info for an interface."""

    method_name: str = Field(..., description="Method name")
    description: str = Field("", description="Method description")


class InterfaceMethodsResponse(BaseModel):
    """Response model for get_interface_methods tool."""

    interface_name: str = Field(..., description="Interface queried")
    methods: list[InterfaceMethodInfo] = Field(
        default_factory=list, description="Methods declared by this interface"
    )
    implementing_classes: list[str] = Field(
        default_factory=list, description="Classes that implement this interface"
    )


class FileDependenciesResponse(BaseModel):
    """Response model for get_file_dependencies tool."""

    file_path: str = Field(..., description="Resolved file path")
    references: list[str] = Field(
        default_factory=list, description="Files this file references"
    )
    dependents: list[str] = Field(
        default_factory=list, description="Files that depend on this file"
    )
    entities: list[str] = Field(
        default_factory=list, description="Entity ids declared in this file"
    )
    not_found: bool = Field(
        False, description="True if the file was not found in the graph"
    )


class FileStatInfo(BaseModel):
    """Per-file stats for the get_file_stats tool."""

    file_path: str = Field(..., description="File path")
    nodes: int = Field(0, description="Number of graph entities in this file")
    fan_in: int = Field(0, description="Number of files that depend on this file")
    fan_out: int = Field(0, description="Number of files this file references")


class FileStatsResponse(BaseModel):
    """Response model for get_file_stats tool."""

    files: list[FileStatInfo] = Field(
        default_factory=list, description="Per-file stats"
    )
    total_files: int = Field(0, description="Total number of files")


class FileCyclesResponse(BaseModel):
    """Response model for file_dependency_cycles tool."""

    cycles: list[list[str]] = Field(
        default_factory=list, description="Dependency cycles between files"
    )
    count: int = Field(0, description="Number of dependency cycles found")
