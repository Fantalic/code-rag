FROM python:3.12-slim AS base

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY . .

# Install Python packages (editable)
RUN pip install --no-cache-dir -e "LightRAG[api]" \
    && pip install --no-cache-dir -e daniel-lightrag-mcp \
    && pip install --no-cache-dir -e watchdog

# Build WebUI
FROM oven/bun:1 AS frontend
WORKDIR /app
COPY LightRAG/lightrag_webui ./lightrag_webui
RUN cd lightrag_webui && bun install --frozen-lockfile && bun run build

FROM base AS final
COPY --from=frontend /app/lightrag/api/webui /app/LightRAG/lightrag/api/webui

# Default Ollama host
ENV LLM_BINDING_HOST=http://ollama:11434
ENV EMBEDDING_BINDING_HOST=http://ollama:11434
ENV LLM_BINDING=ollama
ENV EMBEDDING_BINDING=ollama
ENV LLM_MODEL=qwen3-coder:480b-cloud
ENV EMBEDDING_MODEL=bge-m3:latest
ENV EMBEDDING_DIM=1024
ENV WORKING_DIR=/app/data/rag_storage
ENV INPUT_DIR=/app/data/inputs

RUN mkdir -p /app/data/rag_storage /app/data/inputs

EXPOSE 9621

ENTRYPOINT ["python", "-m", "lightrag.api.lightrag_server", "--host", "0.0.0.0", "--port", "9621"]
