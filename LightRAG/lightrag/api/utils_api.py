"""
Utility functions for the LightRAG API.
"""

import os
import argparse
from typing import Optional, List, Tuple
import sys
from ascii_colors import ASCIIColors
from lightrag.api import __api_version__ as api_version
from lightrag import __version__ as core_version
from lightrag.constants import (
    DEFAULT_FORCE_LLM_SUMMARY_ON_MERGE,
)
from fastapi import HTTPException, Security, Request, status
from fastapi.security import APIKeyHeader, OAuth2PasswordBearer
from starlette.status import HTTP_403_FORBIDDEN
from .auth import auth_handler
from .config import ollama_server_infos, get_global_args, get_env_value


def check_env_file():
    """
    Check if .env file exists. For single-instance deployments, .env is optional.
    Always returns True to allow startup.
    """
    return True


# Get whitelist paths from global_args, only once during initialization
whitelist_paths = get_global_args().whitelist_paths.split(",")

# Pre-compile path matching patterns
whitelist_patterns: list[tuple[str, bool]] = []
for path in whitelist_paths:
    path = path.strip()
    if path:
        # If path ends with /*, match all paths with that prefix
        if path.endswith("/*"):
            prefix = path[:-2]
            whitelist_patterns.append((prefix, True))  # (prefix, is_prefix_match)
        else:
            whitelist_patterns.append((path, False))  # (exact_path, is_prefix_match)

# Global authentication configuration
auth_configured = bool(auth_handler.accounts)


def get_combined_auth_dependency(api_key: Optional[str] = None):
    """
    Create a combined authentication dependency that implements authentication logic
    based on API key, OAuth2 token, and whitelist paths.

    Args:
        api_key (Optional[str]): API key for validation

    Returns:
        Callable: A dependency function that implements the authentication logic
    """
    # Use global whitelist_patterns and auth_configured variables
    # whitelist_patterns and auth_configured are already initialized at module level

    # Only calculate api_key_configured as it depends on the function parameter
    api_key_configured = bool(api_key)

    # CodeLightRAG: local-only Ollama mode.
    # When api_key is empty/None, api_key_configured = False.
    # Combined with auth_configured = False (no accounts), all requests pass at step 3 below.
    # See lightrag_server.py line 318 — LIGHTRAG_API_KEY is optional.
    # Create security dependencies with proper descriptions for Swagger UI
    oauth2_scheme = OAuth2PasswordBearer(
        tokenUrl="login", auto_error=False, description="OAuth2 Password Authentication"
    )

    # If API key is configured, create an API key header security
    api_key_header = None
    if api_key_configured:
        api_key_header = APIKeyHeader(
            name="X-API-Key", auto_error=False, description="API Key Authentication"
        )

    async def combined_dependency(
        request: Request,
        token: str = Security(oauth2_scheme),
        api_key_header_value: Optional[str] = None
        if api_key_header is None
        else Security(api_key_header),
    ):
        # 1. Check if path is in whitelist
        path = request.url.path
        for pattern, is_prefix in whitelist_patterns:
            if (is_prefix and path.startswith(pattern)) or (
                not is_prefix and path == pattern
            ):
                return  # Whitelist path, allow access

        # 2. Validate token first if provided in the request (Ensure 401 error if token is invalid)
        if token:
            try:
                token_info = auth_handler.validate_token(token)
                # Accept guest token if no auth is configured
                if not auth_configured and token_info.get("role") == "guest":
                    return
                # Accept non-guest token if auth is configured
                if auth_configured and token_info.get("role") != "guest":
                    return

                # Token validation failed, immediately return 401 error
                raise HTTPException(
                    status_code=status.HTTP_401_UNAUTHORIZED,
                    detail="Invalid token. Please login again.",
                )
            except HTTPException as e:
                # If already a 401 error, re-raise it
                if e.status_code == status.HTTP_401_UNAUTHORIZED:
                    raise
                # For other exceptions, continue processing

        # 3. Acept all request if no API protection needed
        if not auth_configured and not api_key_configured:
            return

        # 4. Validate API key if provided and API-Key authentication is configured
        if (
            api_key_configured
            and api_key_header_value
            and api_key_header_value == api_key
        ):
            return  # API key validation successful

        ### Authentication failed ####

        # if password authentication is configured but not provided, ensure 401 error if auth_configured
        if auth_configured and not token:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="No credentials provided. Please login.",
            )

        # if api key is provided but validation failed
        if api_key_header_value:
            raise HTTPException(
                status_code=HTTP_403_FORBIDDEN,
                detail="Invalid API Key",
            )

        # if api_key_configured but not provided
        if api_key_configured and not api_key_header_value:
            raise HTTPException(
                status_code=HTTP_403_FORBIDDEN,
                detail="API Key required",
            )

        # Otherwise: refuse access and return 403 error
        raise HTTPException(
            status_code=HTTP_403_FORBIDDEN,
            detail="API Key required or login authentication required.",
        )

    return combined_dependency


def display_splash_screen(args: argparse.Namespace) -> None:
    """
    Display a colorful splash screen showing LightRAG server configuration

    Args:
        args: Parsed command line arguments
    """
    def safe_print(msg):
        try:
            print(msg)
        except UnicodeEncodeError:
            # Replace Unicode chars with ASCII equivalents for Windows console
            ascii_msg = msg.replace("\u250c", "+").replace("\u2510", "+")
            ascii_msg = ascii_msg.replace("\u2514", "+").replace("\u2518", "+")
            ascii_msg = ascii_msg.replace("\u2500", "-").replace("\u2502", "|")
            ascii_msg = ascii_msg.replace("\u256d", "+").replace("\u256e", "+")
            ascii_msg = ascii_msg.replace("\u256f", "+").replace("\u2570", "+")
            ascii_msg = ascii_msg.replace("\U0001f4e1", "[Phone]")
            ascii_msg = ascii_msg.replace("\U0001f4c1", "[Folder]")
            ascii_msg = ascii_msg.replace("\U0001f916", "[Bot]")
            ascii_msg = ascii_msg.replace("\U0001f4ca", "[Chart]")
            ascii_msg = ascii_msg.replace("\u2699", "[Gear]")
            ascii_msg = ascii_msg.replace("\U0001f4be", "[Disk]")
            ascii_msg = ascii_msg.replace("\u2728", "[Sparkle]")
            ascii_msg = ascii_msg.replace("\U0001f310", "[Globe]")
            ascii_msg = ascii_msg.replace("\U0001f4dd", "[Note]")
            ascii_msg = ascii_msg.replace("\u26a0", "[!]")
            print(ascii_msg)

    try:
        # Banner
        top_border = "+" + "=" * 62 + "+"
        bottom_border = "+" + "=" * 62 + "+"
        width = 62

        line1_text = f"LightRAG Server v{core_version}/{api_version}"
        line2_text = "Fast, Lightweight RAG Server Implementation"

        line1 = f"| {line1_text.center(width)} |"
        line2 = f"| {line2_text.center(width)} |"

        banner = f"""
    {top_border}
    {line1}
    {line2}
    {bottom_border}
    """
        ASCIIColors.cyan(banner)
        print()
    except UnicodeEncodeError:
        safe_print(f"LightRAG Server v{core_version}/{api_version} starting on {args.host}:{args.port}")

    # Server Configuration - use safe_print for all
    safe_print("\n[Server Configuration]")
    safe_print(f"    Host: {args.host}")
    safe_print(f"    Port: {args.port}")
    safe_print(f"    Workers: {args.workers}")
    safe_print(f"    Timeout: {args.timeout}")
    safe_print(f"    CORS Origins: {args.cors_origins}")
    safe_print(f"    SSL Enabled: {args.ssl}")
    if args.ssl:
        safe_print(f"    SSL Cert: {args.ssl_certfile}")
        safe_print(f"    SSL Key: {args.ssl_keyfile}")
    safe_print(f"    Ollama Emulating Model: {ollama_server_infos.LIGHTRAG_MODEL}")
    safe_print(f"    Log Level: {args.log_level}")
    safe_print(f"    Verbose Debug: {args.verbose}")
    safe_print(f"    API Key: {'Set' if args.key else 'Not Set'}")
    safe_print(f"    JWT Auth: {'Enabled' if args.auth_accounts else 'Disabled'}")

    # Directory Configuration
    safe_print("\n[Directory Configuration]")
    safe_print(f"    Working Directory: {args.working_dir}")
    safe_print(f"    Input Directory: {args.input_dir}")

    # LLM Configuration
    safe_print("\n[LLM Configuration]")
    safe_print(f"    Binding: {args.llm_binding}")
    safe_print(f"    Host: {args.llm_binding_host}")
    safe_print(f"    Model: {args.llm_model}")
    safe_print(f"    Max Async for LLM: {args.max_async}")
    safe_print(f"    Summary Context Size: {args.summary_context_size}")
    safe_print(f"    LLM Cache Enabled: {args.enable_llm_cache}")
    safe_print(f"    LLM Cache for Extraction Enabled: {args.enable_llm_cache_for_extract}")

    # Embedding Configuration
    safe_print("\n[Embedding Configuration]")
    safe_print(f"    Binding: {args.embedding_binding}")
    safe_print(f"    Host: {args.embedding_binding_host}")
    safe_print(f"    Model: {args.embedding_model}")
    safe_print(f"    Dimensions: {args.embedding_dim}")

    # RAG Configuration
    safe_print("\n[RAG Configuration]")
    safe_print(f"    Summary Language: {args.summary_language}")
    safe_print(f"    Entity Types: {args.entity_types}")
    safe_print(f"    Max Parallel Insert: {args.max_parallel_insert}")
    safe_print(f"    Chunk Size: {args.chunk_size}")
    safe_print(f"    Chunk Overlap Size: {args.chunk_overlap_size}")
    safe_print(f"    Cosine Threshold: {args.cosine_threshold}")
    safe_print(f"    Top-K: {args.top_k}")
    safe_print(f"    Force LLM Summary on Merge: {get_env_value('FORCE_LLM_SUMMARY_ON_MERGE', DEFAULT_FORCE_LLM_SUMMARY_ON_MERGE, int)}")

    # System Configuration
    safe_print("\n[Storage Configuration]")
    safe_print(f"    KV Storage: {args.kv_storage}")
    safe_print(f"    Vector Storage: {args.vector_storage}")
    safe_print(f"    Graph Storage: {args.graph_storage}")
    safe_print(f"    Document Status Storage: {args.doc_status_storage}")
    safe_print(f"    Workspace: {args.workspace if args.workspace else '-'}")

    # Server Status
    safe_print("\n[Server starting up...]\n")

    # Server Access Information
    protocol = "https" if args.ssl else "http"
    if args.host == "0.0.0.0":
        safe_print("\n[Server Access Information]")
        safe_print(f"    WebUI (local): {protocol}://localhost:{args.port}")
        safe_print(f"    Remote Access: {protocol}://<your-ip-address>:{args.port}")
        safe_print(f"    API Documentation (local): {protocol}://localhost:{args.port}/docs")
        safe_print(f"    Alternative Documentation (local): {protocol}://localhost:{args.port}/redoc")

        safe_print("\n[Note]")
        safe_print("""    Since the server is running on 0.0.0.0:
    - Use 'localhost' or '127.0.0.1' for local access
    - Use your machine's IP address for remote access
    """)
    else:
        base_url = f"{protocol}://{args.host}:{args.port}"
        safe_print("\n[Server Access Information]")
        safe_print(f"    WebUI (local): {base_url}")
        safe_print(f"    API Documentation: {base_url}/docs")
        safe_print(f"    Alternative Documentation: {base_url}/redoc")

    # Security Notice
    if args.key:
        safe_print("\n[!] Security Notice:")
        safe_print("""    API Key authentication is enabled.
    Make sure to include the X-API-Key header in all your requests.
    """)
    if args.auth_accounts:
        safe_print("\n[!] Security Notice:")
        safe_print("""    JWT authentication is enabled.
    Make sure to login before making the request, and include the 'Authorization' in the header.
    """)

    # Ensure splash output flush to system log
    sys.stdout.flush()
