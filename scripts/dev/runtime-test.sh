#!/bin/sh
# codeRAG - full runtime test (cross-platform POSIX sh).
# Runs on Linux/macOS directly, on Windows via Git's sh.exe (e.g.
# E:\programme\Git\usr\bin\sh.exe) or any POSIX shell.
#
# Behaviour:
#   1. docker compose down (kills any running stack)
#   2. docker compose build  (unless --skip-build)
#   3. docker compose up -d  with WORKSPACE=<project basename>
#   4. wait for LightRAG /health
#   5. run `coderag scan` against PROJECT_DIR
#   6. test CLI graph files | deps | cycles
#   7. test MCP tools via scripts/dev/mcp_probe.py (SSE JSON-RPC)
#
# Usage:
#   sh scripts/dev/runtime-test.sh                       # default testdata
#   sh scripts/dev/runtime-test.sh /path/to/MyApp        # custom project
#   sh scripts/dev/runtime-test.sh --clean               # wipe graph first
#   sh scripts/dev/runtime-test.sh --skip-build          # reuse existing image
#   sh scripts/dev/runtime-test.sh --no-stop             # don't docker compose down
#   CLEAN=1 SKIP_BUILD=1 sh scripts/dev/runtime-test.sh  # via env vars
#
# Env vars:
#   PROJECT_DIR - project to scan (default: testdata/TestApp)
#   WORKSPACE   - LightRAG workspace (default: basename of PROJECT_DIR)
#   CODERAG_BIN - path to coderag binary (default: ./coderag[.exe] in repo root)
#   PYTHON      - python interpreter (default: python3, then python)

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)

CLEAN="${CLEAN:-0}"
SKIP_BUILD="${SKIP_BUILD:-0}"
DO_STOP=1
PROJECT_ARG=""

for arg in "$@"; do
    case "$arg" in
        --clean)      CLEAN=1 ;;
        --skip-build) SKIP_BUILD=1 ;;
        --no-stop)    DO_STOP=0 ;;
        --help|-h)
            sed -n '2,18p' "$0"
            exit 0
            ;;
        -*) echo "[XX] Unknown flag: $arg" >&2; exit 2 ;;
        *)  PROJECT_ARG="$arg" ;;
    esac
done

PROJECT_DIR="${PROJECT_DIR:-${PROJECT_ARG:-$ROOT/coderag/cli/analysers/csharp/testdata/TestApp}}"
if [ ! -d "$PROJECT_DIR" ]; then
    echo "[XX] Project directory not found: $PROJECT_DIR" >&2
    exit 1
fi

WORKSPACE="${WORKSPACE:-$(basename "$PROJECT_DIR")}"

# Pick coderag binary
CODERAG_BIN="${CODERAG_BIN:-}"
if [ -z "$CODERAG_BIN" ]; then
    if [ -x "$ROOT/coderag.exe" ]; then CODERAG_BIN="$ROOT/coderag.exe"
    elif [ -x "$ROOT/coderag" ]; then CODERAG_BIN="$ROOT/coderag"
    else
        echo "[XX] coderag binary not found in $ROOT" >&2
        exit 1
    fi
fi

# Pick python interpreter
PY="${PYTHON:-}"
if [ -z "$PY" ]; then
    if command -v python3 >/dev/null 2>&1; then PY=python3
    elif command -v python >/dev/null 2>&1; then PY=python
    fi
fi

MCP_PROBE="$SCRIPT_DIR/mcp_probe.py"

# --- 1. Stop existing containers ------------------------------------------
if [ "$DO_STOP" = "1" ]; then
    echo "[..] Stopping Docker services ..."
    WORKSPACE="$WORKSPACE" docker compose down 2>&1 | tail -n 5 || true
fi

# --- 2. Build image -------------------------------------------------------
if [ "$SKIP_BUILD" != "1" ]; then
    echo "[..] Rebuilding Docker image ..."
    docker compose build 2>&1 | grep -E "DONE|ERROR|FAIL" || true
    echo "[OK] Docker image rebuilt"
fi

# --- 3. Start containers with WORKSPACE -----------------------------------
echo "[..] Starting Docker services (WORKSPACE=$WORKSPACE) ..."
WORKSPACE="$WORKSPACE" docker compose up -d 2>&1 | tail -n 5 || true

# --- 4. Wait for LightRAG health ------------------------------------------
echo "[..] Waiting for LightRAG ..."
i=0
ok=0
while [ $i -lt 60 ]; do
    if curl -sf http://localhost:9621/health >/dev/null 2>&1; then
        ok=1
        break
    fi
    sleep 1
    i=$((i + 1))
done
if [ $ok -ne 1 ]; then
    echo "[XX] LightRAG not healthy after 60s" >&2
    exit 1
fi
echo "[OK] LightRAG healthy"

# --- 4b. Pre-warm bge-m3 (cold load can take ~35s; LightRAG worker timeout is 60s) ---
WS=$(curl -sf http://localhost:9621/health 2>/dev/null | "$PY" -c "import json,sys; print(json.load(sys.stdin).get('configuration',{}).get('embedding_model','bge-m3:latest'))" 2>/dev/null || echo "bge-m3:latest")
echo "[..] Pre-warming embedding model $WS (avoids LightRAG 60s worker timeout on cold load) ..."
SW=$(date +%s)
curl -sf -m 120 -X POST -H "Content-Type: application/json" \
    -d "{\"model\":\"$WS\",\"prompt\":\"warmup\"}" \
    http://localhost:11434/api/embeddings >/dev/null 2>&1 || true
EW=$(date +%s)
echo "[OK] pre-warm took $((EW - SW))s"

# --- 5. Scan --------------------------------------------------------------
echo
echo "[..] Scanning $PROJECT_DIR ..."
SCAN_ARGS="scan"
[ "$CLEAN" = "1" ] && SCAN_ARGS="$SCAN_ARGS --clean"
SCAN_ARGS="$SCAN_ARGS $PROJECT_DIR"

SCAN_LOG="${SCAN_LOG:-/tmp/coderag-scan.log}"
# shellcheck disable=SC2086
"$CODERAG_BIN" $SCAN_ARGS >"$SCAN_LOG" 2>&1 || true
grep -E "Extracted|Entities:|Relations:|knowledge graph|Scan complete|ERROR|Analyser produced" "$SCAN_LOG" || true

# --- 6. CLI graph commands -----------------------------------------------
echo
echo "[..] Testing CLI graph commands ..."

echo "  graph files:"
"$CODERAG_BIN" graph files 2>&1 | tail -n 20

echo
echo "  graph deps Program.cs:"
"$CODERAG_BIN" graph deps "Program.cs" 2>&1 | tail -n 20

echo
echo "  graph deps OrderService.cs:"
"$CODERAG_BIN" graph deps "OrderService.cs" 2>&1 | tail -n 20

echo
echo "  graph cycles:"
"$CODERAG_BIN" graph cycles 2>&1 | tail -n 20

# --- 7. MCP server tools -------------------------------------------------
echo
echo "[..] Testing MCP server tools ..."

# 7a. /health
MCP_HEALTH=$(curl -sf http://localhost:9622/health 2>&1 || echo "unreachable")
echo "  MCP health: $MCP_HEALTH"

# 7b. Tool calls via SSE probe
if [ -z "$PY" ]; then
    echo "  [!!] no python interpreter; skipping MCP tool probes"
else
    call_probe() {
        tool="$1"
        args_json="$2"
        "$PY" "$MCP_PROBE" "$tool" "$args_json" 2>/dev/null
    }

    if [ -n "$PY" ]; then
        echo
        echo "  file_dependencies(Program.cs):"
        deps_raw=$(call_probe "file_dependencies" '{"file_path":"Program.cs"}')
        if [ -n "$deps_raw" ]; then
            printf '%s' "$deps_raw" | "$PY" -c "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"    ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        refs=r.get('references',[]) or []
        deps=r.get('dependents',[]) or []
        ents=r.get('entities',[]) or []
        print(f\"    references: {', '.join(refs) if refs else '(none)'}\")
        print(f\"    dependents: {', '.join(deps) if deps else '(none)'}\")
        print(f\"    entities:   {', '.join(ents) if ents else '(none)'}\")
except Exception as e:
    print(f'    (parse error: {e})')
" 2>/dev/null || echo "    (raw): $deps_raw"
        fi

        echo
        echo "  detect_circular_dependencies:"
        cyc_raw=$(call_probe "detect_circular_dependencies" "{}")
        if [ -n "$cyc_raw" ]; then
            printf '%s' "$cyc_raw" | "$PY" -c "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"    ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        cnt=r.get('count',0)
        cycles=r.get('cycles',[]) or []
        print(f'    count: {cnt}')
        if cycles:
            for c in cycles: print(f\"    cycle: {' -> '.join(c)}\")
        else:
            print('    no cycles')
except Exception as e:
    print(f'    (parse error: {e})')
" 2>/dev/null || echo "    (raw): $cyc_raw"
        fi
    fi
fi

echo
echo "[OK] Runtime test complete"
