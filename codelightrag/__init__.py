"""
CodeLightRAG — Knowledge graph for AI agents.

Re-exports key components from all three sub-packages.
"""

from lightrag import LightRAG
from lightrag.base import DocProcessingStatus, DocStatus, QueryParam

try:
    from daniel_lightrag_mcp import LightRAGClient, LightRAGError, mcp_server
except ModuleNotFoundError:
    LightRAGClient = None  # type: ignore
    LightRAGError = None   # type: ignore
    mcp_server = None      # type: ignore

__version__ = "0.1.1"
