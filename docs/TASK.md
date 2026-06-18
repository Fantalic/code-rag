# Current Task: Phase 4 — Modernise

## Objective
Modernise the codebase to Python 3.10+ idioms, Pydantic V2 patterns, and clean async patterns.

## Steps

### 4.1 — Pydantic V2: Replace `class Config` with `ConfigDict`
- [ ] Replace all `class Config:` with `model_config = ConfigDict(...)` in `document_routes.py` (16 occurrences)
- [ ] Check for other Pydantic V1 patterns across codebase (`@validator` → `@field_validator`, etc.)

### 4.2 — Python 3.10+ Type Annotations
- [ ] Replace `Optional[X]` → `X | None` across codebase
- [ ] Replace `Union[X, Y]` → `X | Y`
- [ ] Replace `List[X]` → `list[X]`, `Dict[K, V]` → `dict[K, V]`, `Tuple` → `tuple`, `Set` → `set`

### 4.3 — Async Patterns
- [ ] Replace bare `asyncio.gather(...)` with `async with asyncio.TaskGroup() as tg:` where feasible
- [ ] Ensure all async context managers use `async with`

### 4.4 — Match/Case (if applicable)
- [ ] Replace long if-elif chains with `match/case` in key locations (document_routes.py, kg/__init__.py, etc.)

## Rules
- Verify with `ruff check .` and `pytest tests/test_smoke_*.py` after each step
- No breaking changes to public API
- Each change should be purely syntactic — no behavior changes
- Commit after each step (or batch of similar steps)
