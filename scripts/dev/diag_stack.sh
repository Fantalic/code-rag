#!/bin/sh
# diag_stack.sh - one-shot diagnostic dump of the codeRAG stack.
#
# Useful when something is broken: runs multiple diagnostics in sequence
# and prints a clear summary at the end.
#
# Checks:
#   - LightRAG /health (config.workspace)
#   - LightRAG /health/pipeline status
#   - MCP /health
#   - Ollama reachability + models + embedding latency
#   - Container ↔ host network reachability
#   - Knowledge graph contents (entities / relations / docs)
#   - MCP SSE round-trip (one tool call)
#
# Usage:
#   sh scripts/dev/diag_stack.sh
#   CONTAINER=lightrag-server sh scripts/dev/diag_stack.sh
#   WORKSPACE=TestApp sh scripts/dev/diag_stack.sh

set -u

CONTAINER="${CONTAINER:-lightrag-server}"
WORKSPACE="${WORKSPACE:-}"
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)

PY="${PYTHON:-}"
if [ -z "$PY" ]; then
    if command -v python3 >/dev/null 2>&1; then PY=python3
    elif command -v python >/dev/null 2>&1; then PY=python
    fi
fi

section() { echo; echo "================================================================"; echo "## $1"; echo "================================================================"; }

section "LightRAG /health"
curl -sS -m 5 http://localhost:9621/health | $PY -c "
import json,sys
try:
    d = json.loads(sys.stdin.read())
    print(' status           :', d.get('status'))
    print(' core_version     :', d.get('core_version'))
    print(' api_version      :', d.get('api_version'))
    print(' pipeline_busy    :', d.get('pipeline_busy'))
    cfg = d.get('configuration') or {}
    print(' workspace        :', cfg.get('workspace'))
    print(' embedding_model  :', cfg.get('embedding_model'))
    print(' llm_model        :', cfg.get('llm_model'))
    print(' embedding_host   :', cfg.get('embedding_binding_host'))
    print(' max_async        :', cfg.get('max_async'))
    print(' emb_func_max_async:', cfg.get('embedding_func_max_async'))
    print(' embedding_batch  :', cfg.get('embedding_batch_num'))
    print(' cosine_threshold :', cfg.get('cosine_threshold'))
except Exception as e:
    print('  PARSE ERROR:', e)
" 2>/dev/null

section "LightRAG /documents pipeline status"
curl -sS -m 5 "http://localhost:9621/documents/pipeline_status" 2>&1 | head -c 500 | $PY -c "
import sys
try:
    import json
    d = json.loads(sys.stdin.read())
    if isinstance(d, dict):
        for k, v in d.items(): print(f' {k}: {v}')
    else:
        print(d)
except Exception:
    import sys as s
    s.stdout.write(s.stdin.read())
" 2>/dev/null

section "MCP /health"
curl -sS -m 5 http://localhost:9622/health 2>&1

section "Ollama (host)"
"$SCRIPT_DIR/diag_ollama.sh" 2>&1 | grep -E "^\[|reachable|present|cold|warm|FAIL|OK|elapsed" | head -25

section "Ollama (inside container)"
"$SCRIPT_DIR/diag_ollama.sh" --container "$CONTAINER" 2>&1 | grep -E "^\[|reachable|present|cold|warm|FAIL|OK|elapsed" | head -25

section "Knowledge graph dump"
if [ -n "$PY" ]; then
    WS_ARG=""
    [ -n "$WORKSPACE" ] && WS_ARG="--workspace $WORKSPACE"
    $PY "$SCRIPT_DIR/diag_lightrag.py" --container "$CONTAINER" $WS_ARG 2>&1 | head -50
else
    echo "(no python, skipping)"
fi

section "MCP SSE round-trip"
if [ -n "$PY" ]; then
    RESP=$($PY "$SCRIPT_DIR/mcp_probe.py" get_file_stats "{}" 2>&1)
    echo "  get_file_stats -> $RESP" | head -c 300
    echo
else
    echo "(no python, skipping)"
fi

echo
echo "[OK] diag_stack complete"
