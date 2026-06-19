# AGENTS.md

This is an MCP server for LightRAG.

## Build Commands

## Build Commands
- Build package: python -m build
- Install in dev mode: pip install -e ".[dev]"

## Lint and Format Commands
- Format code: black src/ tests/ --line-length 88
- Sort imports: isort src/ tests/ --profile black
- Type checking: mypy src/ --python-version 3.10 --disallow-untyped-defs --warn-return-any
- Run all lint: black --check src/ tests/ && isort --check-only src/ tests/ && mypy src/

## Test Commands
- Run all tests: pytest tests/
- Coverage: pytest --cov=src/daniel_lightrag_mcp --cov-report=html tests/
- Single test file: pytest tests/test_client.py
- Single test: pytest tests/test_client.py::TestClient::test_specific_method -v
- Async tests: pytest -m asyncio tests/

## Code Style Guidelines
- **Formatting**: Use Black (line-length=88, target Python 3.10+). No manual formatting.
- **Imports**: isort with Black profile. Group standard lib, third-party, local imports. Absolute imports preferred.
- **Typing**: Full type annotations using typing/Pydantic. Enums for statuses/modes. Strict mypy: disallow untyped defs, warn on unused configs.
- **Naming**: snake_case for functions/variables, PascalCase for classes/models. Descriptive names (e.g., InsertTextRequest).
- **Error Handling**: Use Pydantic validation for inputs. Try-except for runtime errors (e.g., KeyboardInterrupt, HTTP errors). Log exceptions with context.
- **Structure**: Models in models.py with BaseModel. CLI in cli.py. Use asyncio for async ops. No inline comments unless necessary.
- **Conventions**: Python 3.10+. Follow PEP 8. Pydantic for API models. Test with pytest-asyncio for async.

## Additional Notes
- Always run lint/typecheck after changes: black, isort, mypy.
- Tests in tests/ with markers: unit, integration, slow, asyncio.
- No Cursor/Copilot rules found.
