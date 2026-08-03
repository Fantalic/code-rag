FROM oven/bun:1 AS frontend
WORKDIR /app
COPY coderag/lightrag/lightrag_webui ./lightrag_webui
RUN cd lightrag_webui && bun install --frozen-lockfile && bun run build

FROM python:3.12-slim AS base
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY coderag/lightrag ./LightRAG
COPY coderag/mcp ./mcp

# Install LightRAG with API extras
RUN pip install --no-cache-dir -e "LightRAG[api]"

# Install MCP server
RUN pip install --no-cache-dir -e mcp

FROM base AS final
# Copy WebUI build output (vite outDir: ../lightrag/api/webui from lightrag_webui/)
COPY --from=frontend /app/lightrag/api/webui /app/LightRAG/lightrag/api/webui

ENV LLM_BINDING_HOST=http://host.docker.internal:11434
ENV EMBEDDING_BINDING_HOST=http://host.docker.internal:11434
ENV LLM_BINDING=ollama
ENV EMBEDDING_BINDING=ollama
ENV LLM_MODEL=phi3.5:latest
ENV EMBEDDING_MODEL=bge-m3:latest
ENV EMBEDDING_DIM=1024
ENV WORKING_DIR=/app/data/rag_storage
ENV INPUT_DIR=/app/data/inputs

RUN mkdir -p /app/data/rag_storage /app/data/inputs

EXPOSE 9621

CMD ["python", "-m", "lightrag.api.lightrag_server", "--host", "0.0.0.0", "--port", "9621"]
