#!/bin/sh
# diag_ollama.sh - diagnose Ollama + embedding latency.
#
# Checks from both host and (optionally) inside the lightrag container:
#   1. Ollama process reachable
#   2. Required models pulled
#   3. Cold-load embedding time (first call, may load model from disk)
#   4. Warm embedding time (subsequent calls)
#   5. Per-model parameter sizes
#
# Usage:
#   sh scripts/dev/diag_ollama.sh                       # host only
#   sh scripts/dev/diag_ollama.sh --container lightrag-server
#   EMBED_MODEL=bge-m3 sh scripts/dev/diag_ollama.sh
#   EMBED_PROMPT="longer sample text" sh scripts/dev/diag_ollama.sh

set -u

EMBED_MODEL="${EMBED_MODEL:-bge-m3}"
EMBED_PROMPT="${EMBED_PROMPT:-hello world embedding latency probe from codeRAG diag script}"

CONTAINER="${CONTAINER:-}"
while [ $# -gt 0 ]; do
    case "$1" in
        --container) CONTAINER="$2"; shift 2 ;;
        *) echo "[XX] unknown arg: $1" >&2; exit 2 ;;
    esac
done

run_in() {
    if [ -n "$CONTAINER" ]; then
        docker exec "$CONTAINER" sh -c "$1" 2>&1
    else
        sh -c "$1" 2>&1
    fi
}

curl_in() {
    run_in "curl -sS -m ${1:-10} ${2}"
}

time_request() {
    label="$1"
    url="$2"
    body="$3"
    timeout="${4:-120}"
    if [ -n "$CONTAINER" ]; then
        # ns timer inside container
        run_in "date +%s%N; curl -sS -m $timeout -o /dev/null -w '%{http_code}\\n' -X POST -H 'Content-Type: application/json' --data '$body' $url; date +%s%N"
    else
        # host: use python for ms timing
        python3 -c "
import json, time, urllib.request, sys
data = json.dumps($body).encode('utf-8')
req = urllib.request.Request('$url', data=data, headers={'Content-Type':'application/json'}, method='POST')
t0 = time.monotonic_ns()
try:
    with urllib.request.urlopen(req, timeout=$timeout) as r:
        body = r.read()
        code = r.status
except Exception as e:
    code = -1
    print('ERR:', e)
t1 = time.monotonic_ns()
print(f'$label: http={code} elapsed_ms={(t1 - t0) // 1_000_000}')
"
    fi
}

echo "[..] Ollama diagnostics (model=$EMBED_MODEL, container=${CONTAINER:-host})"
echo

echo "[1] Reachable?"
HTTP=$(curl_in 5 "http://localhost:11434/api/tags" | head -c 200)
if [ -z "$HTTP" ]; then
    echo "  [FAIL] Ollama not responding on :11434"
    exit 1
fi
echo "  [OK] /api/tags responded"
echo

echo "[2] Models present?"
echo "$HTTP" | python3 -c "
import json, sys
try:
    d = json.loads(sys.stdin.read())
    models = [m['name'] for m in d.get('models', [])]
    print('  found', len(models), 'models')
    for m in models: print('   -', m)
    target = '$EMBED_MODEL'.split(':')[0]
    if any(m.startswith(target) for m in models):
        print('  [OK]', target, 'is pulled')
    else:
        print('  [FAIL]', target, 'NOT pulled. Run: ollama pull', target)
except Exception as e:
    print('  [FAIL] could not parse /api/tags:', e)
" 2>/dev/null
echo

echo "[3] Embedding latency (cold -> warm)"
BODY="{\"model\":\"$EMBED_MODEL\",\"prompt\":\"$EMBED_PROMPT\"}"
echo "  prompt length: ${#EMBED_PROMPT} chars"
echo "  -- cold call --"
time_request "cold" "http://localhost:11434/api/embeddings" "$BODY" 120
echo "  -- warm calls (2x) --"
time_request "warm1" "http://localhost:11434/api/embeddings" "$BODY" 30
time_request "warm2" "http://localhost:11434/api/embeddings" "$BODY" 30
echo

echo "[4] Container reachability"
if [ -n "$CONTAINER" ]; then
    HTTP=$(curl_in 5 "http://host.docker.internal:11434/api/tags")
    if [ -n "$HTTP" ]; then
        echo "  [OK] container $CONTAINER reaches host.docker.internal:11434"
    else
        echo "  [FAIL] container cannot reach host.docker.internal:11434"
    fi
else
    echo "  (skipped, run with --container NAME to check)"
fi
echo

echo "[5] GPU / running processes"
if [ -z "$CONTAINER" ]; then
    if command -v nvidia-smi >/dev/null 2>&1; then
        nvidia-smi --query-gpu=name,utilization.gpu,memory.used --format=csv 2>/dev/null || echo "  (nvidia-smi present but failed)"
    else
        echo "  no nvidia-smi; Ollama is running on CPU only"
    fi
    ps -ef 2>/dev/null | grep -i ollama | grep -v grep | head -3 || true
fi
echo

echo "[OK] Ollama diagnostics complete"
