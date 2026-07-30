# Query Endpoint Plan: Logging, Timeout & Prompt Optimization

## Overview

This document covers three improvements to the query endpoints in `LightRAG/lightrag/api/routers/query_routes.py`:

1. **Structured logging** for debugging prompt content, retrieval stats, and response analysis
2. **Configurable timeout** with env-var defaults per endpoint
3. **Prompt optimization** for the local phi3.5 model

## 1. Structured Logging

### Dev Logs Flag

A `dev_logs` parameter (boolean) is added to `QueryRequest`. When enabled:

- Full prompt content (system prompt + user prompt) is logged
- Retrieval context size (entity/relation/chunk counts) is logged
- Token count estimate (via tokenizer) is logged before LLM call
- Response size (character count) is logged after LLM call
- Per-stage timing (retrieval + LLM generation) is logged
- Cache hit/miss is logged with key details
- Request ID (UUID) is generated per request for correlation

### Logging Levels

| Scenario | Level |
|---|---|
| Request received with params | `INFO` |
| Retrieval stats (entities, relations, chunks) | `INFO` |
| Token count estimate | `DEBUG` |
| Full prompt content (dev_logs only) | `DEBUG` |
| LLM response size | `INFO` (dev_logs: full content at DEBUG) |
| Cache hit/miss | `INFO` |
| Per-stage timing | `DEBUG` |
| Errors | `ERROR` |

### Log Format

```python
logger.info(f"[{request_id}] Query received: mode={mode}, query_len={len(query)}")
logger.debug(f"[{request_id}] System prompt ({len(sys_prompt)} chars): {sys_prompt[:500]}...")
logger.debug(f"[{request_id}] User prompt: {user_query}")
logger.info(f"[{request_id}] Retrieval: {entities_count} entities, {relations_count} relations, {chunks_count} chunks")
logger.info(f"[{request_id}] LLM response: {len(response)} chars in {elapsed:.2f}s")
```

## 2. Timeout Parameter

### Design

A `timeout` parameter is added to `QueryRequest` (in seconds). Default comes from `LLM_TIMEOUT` env var (`DEFAULT_LLM_TIMEOUT = 180` in `constants.py`).

### Behavior

- `timeout=None` (default): Uses the `LLM_TIMEOUT` env var (180s default)
- `timeout=<positive int>`: Overrides the env var for this specific request
- `timeout=0`: No timeout (equivalent to None)
- Timeout is applied via `asyncio.wait_for()` wrapping the `rag.aquery_llm()` call

### Timeout Enforcement

```python
effective_timeout = request.timeout if request.timeout is not None else int(os.getenv("LLM_TIMEOUT", str(DEFAULT_LLM_TIMEOUT)))
try:
    result = await asyncio.wait_for(rag.aquery_llm(request.query, param=param), timeout=effective_timeout)
except asyncio.TimeoutError:
    logger.warning(f"[{request_id}] Query timed out after {effective_timeout}s")
    raise HTTPException(status_code=504, detail=f"Query timed out after {effective_timeout} seconds")
```

### Endpoints Affected

- `POST /query` — non-streaming
- `POST /query/stream` — streaming
- `POST /query/data` — data retrieval (lower timeout default: 30s)

## 3. Prompt Optimization

### Current Issues

| Issue | Impact on phi3.5 |
|---|---|
| `code_rag_response` prompt is verbose (57 lines) | Wastes context window on small model |
| `num_predict` defaults to 128 tokens | Severely truncates phi3.5 output |
| `temperature=1.0` default | Too creative for deterministic code analysis |
| No model-aware prompt selection | Same prompt regardless of model capability |

### Proposed Changes

1. **Increase `num_predict`**: Default to 2048 for query responses (was 128)
2. **Add `temperature` parameter**: Default to 0.2 for code analysis (was 1.0)
3. **Create lightweight prompt variant**: When using phi3.5 or similar small models, use a condensed prompt template
4. **Add `response_type` alternatives**: Add "Structured Analysis" and "Trace" response types better suited for code analysis

## 4. Death-Loop Prevention

### Current State

- Ollama HTTP client has no generation timeout
- `num_predict=128` limits output tokens (already too low, now increased to 2048)
- No streaming chunk timeout

### New Safeguards

1. **`asyncio.wait_for()`** wrapping all LLM calls (timeout from config)
2. **Streaming chunk timeout**: If no chunk arrives within 30s, cancel the stream and return partial results
3. **`num_predict` upper bound**: Capped at 4096 even if user configures higher
4. **Client disconnect handling**: If the client disconnects during streaming, cancel the LLM task

## File Changes

| File | Change |
|---|---|
| `LightRAG/lightrag/api/routers/query_routes.py` | Add `dev_logs`, `timeout` fields to `QueryRequest`; add logging; add timeout enforcement |
| `LightRAG/lightrag/constants.py` | Rename `DEFAULT_LLM_TIMEOUT` to be clearer; add `DEFAULT_QUERY_TIMEOUT` |
| `LightRAG/lightrag/prompt.py` | Add condensed prompt variant; update temperature defaults |
| `LightRAG/lightrag/llm/ollama.py` | Increase `num_predict` default; add generation timeout |
| `LightRAG/lightrag/llm/binding_options.py` | Update OllamaLLMOptions defaults |
| `LightRAG/lightrag/api/lightrag_server.py` | Wire timeout through to Ollama client |