# code-lightrag single-command launcher
# Run from repo root: .\scripts\start_all.ps1

$ROOT = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

# Default models (override via env vars before running)
$env:LLM_MODEL = if ($env:LLM_MODEL) { $env:LLM_MODEL } else { "qwen2.5-coder:1.5b" }
$env:EMBEDDING_MODEL = if ($env:EMBEDDING_MODEL) { $env:EMBEDDING_MODEL } else { "bge-m3:latest" }
$env:LIGHTRAG_EMBEDDING_DIM = if ($env:LIGHTRAG_EMBEDDING_DIM) { $env:LIGHTRAG_EMBEDDING_DIM } else { "1024" }

Write-Host "Starting LightRAG server..."
Write-Host "  LLM model:       $env:LLM_MODEL"
Write-Host "  Embedding model: $env:EMBEDDING_MODEL"
Write-Host "  Embedding dim:   $env:LIGHTRAG_EMBEDDING_DIM"
Write-Host ""

# Start LightRAG server in this process
lightrag-server --host 0.0.0.0 --port 9621
