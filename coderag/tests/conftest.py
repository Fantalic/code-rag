import pytest
import sys


def pytest_configure(config):
    config.addinivalue_line("markers", "smoke: smoke/startup tests")
    config.addinivalue_line("markers", "integration: integration tests")


@pytest.fixture
def temp_working_dir(tmp_path):
    d = tmp_path / "lightrag_storage"
    d.mkdir()
    return str(d)


# ── Integration test helpers ────────────────────────────────────────────────

@pytest.fixture(scope="session")
def mock_lightrag_server():
    """Create a fully mocked LightRAG FastAPI app (session-scoped)."""
    old_argv = sys.argv
    sys.argv = ["lightrag-server"]
    try:
        import lightrag.api.lightrag_server as server_mod
        server_mod.check_frontend_build = lambda: None

        import ascii_colors
        ascii_colors.ASCIIColors.green = lambda t, **k: None

        import lightrag.llm.ollama as ollama_mod
        import numpy as np

        async def _mock_llm(prompt, system_prompt=None, history_messages=None, **kwargs):
            return "mock response"

        async def _mock_embed(texts, embed_model, **kwargs):
            return np.array([[0.1] * 1024 for _ in texts])

        ollama_mod.ollama_model_complete = _mock_llm
        ollama_mod.ollama_embed = _mock_embed

        from lightrag.api.lightrag_server import get_application
        app = get_application()
        yield app
    finally:
        sys.argv = old_argv


@pytest.fixture(scope="session")
async def lifespan(mock_lightrag_server):
    """Enter the server lifespan once for the whole session."""
    async with mock_lightrag_server.router.lifespan_context(mock_lightrag_server):
        yield


@pytest.fixture
async def client(mock_lightrag_server, lifespan):
    """Return an ``httpx.AsyncClient`` wired to the mocked app.

    Lifespan is already active (session-scoped), so state persists
    across test functions.
    """
    import httpx
    from httpx import ASGITransport

    transport = ASGITransport(app=mock_lightrag_server)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
