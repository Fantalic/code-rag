# AGENTS.md - Coding Agent Guidelines

## Build/Lint/Test Commands
- `python main.py` - Run the main application script
- `ruff check .` - Lint Python code (install ruff via pip if needed)
- `ruff format .` - Format code with ruff
- `pytest` - Run all tests (add pytest to requirements if implementing tests)
- `pytest -k 'test_name'` - Run tests matching a pattern/name
- `pytest tests/test_file.py::test_function` - Run a specific test function

## Code Style Guidelines
- **Imports**: Group by standard library (e.g., os, sys), third-party (e.g., watchdog), then local. Use explicit imports like `from pathlib import Path`.
- **Formatting**: Adhere to PEP 8. Use 4-space indentation. Line length: 88 characters (black default).
- **Types**: Always use type hints for parameters and return types (e.g., `def func(path: str) -> None:`).
- **Naming**: snake_case for functions, variables, modules; UPPER_CASE for constants (e.g., INPUT_DIR). Classes: PascalCase.
- **Error Handling**: Use try/except for specific exceptions (e.g., `except queue.Empty:`). Log errors to stderr. Avoid bare except.
- **Docstrings**: Include docstrings for functions/classes explaining params, returns, and behavior. Use triple quotes.
- **Comments**: Minimal inline comments; prefer clear code. No comments unless requested.
- **Project-Specific Rules**: Use environment variables for configuration (e.g., INPUT_DIR). Ensure functions like ingest_file are idempotent. Use threading.Queue for worker pools. Handle file events with watchdog. No external deps beyond stdlib + watchdog; check requirements.txt if added.
- Follow existing patterns in main.py for event handling and debouncing.
- Run lint after changes: `ruff check .`
- No Cursor rules (.cursor/) or Copilot instructions (.github/) found in repo.