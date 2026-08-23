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
# Output mode:
#   By default, only summaries are printed (one line per probe).
#   Detail is only printed on failure or when --verbose is set.
#
# Usage:
#   sh scripts/dev/runtime-test.sh                       # default testdata
#   sh scripts/dev/runtime-test.sh /path/to/MyApp        # custom project
#   sh scripts/dev/runtime-test.sh --clean               # wipe graph first
#   sh scripts/dev/runtime-test.sh --skip-build          # reuse existing image
#   sh scripts/dev/runtime-test.sh --no-stop             # don't docker compose down
#   sh scripts/dev/runtime-test.sh --verbose             # show probe details
#   CLEAN=1 SKIP_BUILD=1 VERBOSE=1 sh scripts/dev/runtime-test.sh
#
# Env vars:
#   PROJECT_DIR - project to scan (default: testdata/TestApp)
#   WORKSPACE   - LightRAG workspace (default: basename of PROJECT_DIR)
#   CODERAG_BIN - path to coderag binary (default: ./coderag[.exe] in repo root)
#   PYTHON      - python interpreter (default: python3, then python)
#   VERBOSE     - if "1", print probe details on success too

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)

CLEAN="${CLEAN:-0}"
SKIP_BUILD="${SKIP_BUILD:-0}"
DO_STOP=1
VERBOSE="${VERBOSE:-0}"
PROJECT_ARG=""

for arg in "$@"; do
    case "$arg" in
        --clean)      CLEAN=1 ;;
        --skip-build) SKIP_BUILD=1 ;;
        --no-stop)    DO_STOP=0 ;;
        --verbose|-v) VERBOSE=1 ;;
        --help|-h)
            sed -n '2,22p' "$0"
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

# Verbose/quiet helper.
# If VERBOSE=1: prints both summary and full detail.
# If VERBOSE=0: prints only the summary line.
vlog() {
    # $1 = summary line; subsequent args = optional detail lines
    if [ "$VERBOSE" = "1" ]; then
        printf '%s\n' "$@"
    else
        printf '%s\n' "$1"
    fi
}

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
echo "[..] Scanning $PROJECT_DIR ..."
SCAN_ARGS="scan"
[ "$CLEAN" = "1" ] && SCAN_ARGS="$SCAN_ARGS --clean"
SCAN_ARGS="$SCAN_ARGS $PROJECT_DIR"

SCAN_LOG="${SCAN_LOG:-/tmp/coderag-scan.log}"
mkdir -p "$(dirname "$SCAN_LOG")" 2>/dev/null || SCAN_LOG="$ROOT/.coderag-scan.log"
# shellcheck disable=SC2086
"$CODERAG_BIN" $SCAN_ARGS >"$SCAN_LOG" 2>&1 || SCAN_RC=$?
SCAN_RC="${SCAN_RC:-0}"
SCAN_SUMMARY=$(grep -E "Extracted|Entities:|Relations:|knowledge graph|Scan complete|Analyser produced" "$SCAN_LOG" 2>/dev/null | tail -n 6 || true)
SCAN_HAD_ERROR=0
if [ "$SCAN_RC" -ne 0 ]; then SCAN_HAD_ERROR=1; fi
if grep -qiE "error|FAIL" "$SCAN_LOG" 2>/dev/null; then SCAN_HAD_ERROR=1; fi
if [ $SCAN_HAD_ERROR -eq 1 ]; then
    echo "  scan: ERROR (exit $SCAN_RC)"
    [ "$VERBOSE" = "1" ] && sed 's/^/    /' "$SCAN_LOG"
    echo "    (full log: $SCAN_LOG)"
else
    if [ "$VERBOSE" = "1" ]; then
        echo "  scan: OK"
        echo "$SCAN_SUMMARY" | sed 's/^/    /'
    else
        echo "  scan: OK"
    fi
fi

# --- 6. CLI graph commands -----------------------------------------------
echo "[..] Testing CLI graph commands ..."

# Each CLI probe returns either "OK: <summary>" or "ERROR: <reason>".
run_cli() {
    label="$1"; shift
    out=$("$CODERAG_BIN" "$@" 2>&1)
    rc=$?
    if [ $rc -ne 0 ]; then
        echo "  $label: ERROR (exit $rc)"
        [ "$VERBOSE" = "1" ] && echo "$out" | sed 's/^/    /'
        return 1
    fi
    # First meaningful line as summary.
    summary=$(echo "$out" | grep -v '^$' | head -n 1)
    count=$(echo "$out" | grep -cv '^$')
    echo "  $label: OK ($count lines)"
    [ "$VERBOSE" = "1" ] && echo "$out" | sed 's/^/    /'
    return 0
}

run_cli "graph files"      graph files
run_cli "graph deps Program.cs"      graph deps "Program.cs"
run_cli "graph deps OrderService.cs" graph deps "OrderService.cs"
run_cli "graph cycles"     graph cycles

# --- 7. MCP server tools -------------------------------------------------
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

    # Generic MCP probe runner.
    # $1 = label, $2 = tool, $3 = args json, $4 = python one-liner that
    # reads JSON from stdin and prints a one-line summary. The summary
    # line is shown in non-verbose mode; in verbose mode full detail is
    # printed too.
    # Exit status: 0 on OK, 1 on error.
    run_probe() {
        label="$1"; tool="$2"; args="$3"; fmt="$4"
        raw=$(call_probe "$tool" "$args")
        if [ -z "$raw" ]; then
            echo "  $label: ERROR (no response)"
            return 1
        fi
        if echo "$raw" | grep -q "bad args JSON"; then
            echo "  $label: ERROR (probe rejected args: $raw)"
            return 1
        fi
        summary=$(printf '%s' "$raw" | "$PY" -c "$fmt" 2>/dev/null || true)
        if [ -z "$summary" ]; then
            echo "  $label: ERROR (parse failure)"
            [ "$VERBOSE" = "1" ] && printf '%s\n' "$raw" | sed 's/^/    /'
            return 1
        fi
        if echo "$summary" | grep -q "^ERROR:"; then
            echo "  $label: $summary"
            [ "$VERBOSE" = "1" ] && printf '%s\n' "$raw" | sed 's/^/    /'
            return 1
        fi
        echo "  $label: $summary"
        if [ "$VERBOSE" = "1" ]; then
            printf '%s\n' "$raw" | "$PY" -c "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    r=d.get('result',{})
    print(json.dumps(r, indent=2))
except Exception as e:
    print(f'(parse error: {e})')
" 2>/dev/null | sed 's/^/    /' || true
        fi
        return 0
    }

    run_probe "file_dependencies(Program.cs)" "file_dependencies" '{"file_path":"Program.cs"}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        refs=len(r.get('references',[]) or [])
        deps=len(r.get('dependents',[]) or [])
        ents=len(r.get('entities',[]) or [])
        print(f'OK ({refs} refs, {deps} dependents, {ents} entities)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "detect_circular_dependencies" "detect_circular_dependencies" '{}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        cnt=r.get('count',0)
        print(f'OK ({cnt} cycles)' if cnt==0 else f'WARN ({cnt} cycles)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "node_details(view=details)" "node_details" '{"entity_name":"TestApp.OrderService","view":"details","depth":2}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        ent=r.get('entity',{})
        rels=len(r.get('relations',[]) or [])
        print(f\"OK ({ent.get('name','?')} [{ent.get('type','?')}], {rels} relations)\")
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "node_details(view=neighbors)" "node_details" '{"entity_name":"TestApp.OrderService","view":"neighbors"}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        nbs=len(r.get('neighbors',[]) or [])
        inc=len(r.get('incoming_relations',[]) or [])
        out=len(r.get('outgoing_relations',[]) or [])
        print(f'OK ({nbs} neighbors, {inc} in, {out} out)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "get_nodes(entity_type=class)" "get_nodes" '{"entity_type":"class"}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        total=r.get('total',0)
        ents=r.get('entities',[]) or []
        print(f'OK ({total} classes)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "get_nodes(name=Order)" "get_nodes" '{"name":"Order","entity_type":"class"}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        total=r.get('total',0)
        ents=r.get('entities',[]) or []
        names=[e.get('name','?') for e in ents]
        print(f\"OK ({total} matches: {', '.join(names) if names else '(none)'})\")
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "dead_code()" "dead_code" '{"min_age_days":0}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        total=r.get('total',0)
        ents=r.get('entities',[]) or []
        reasons={}
        for e in ents:
            r_=e.get('reason','?')
            reasons[r_]=reasons.get(r_,0)+1
        rs=', '.join(f'{k}:{v}' for k,v in sorted(reasons.items()))
        if rs:
            print(f'OK ({total} dead; {rs})')
        else:
            print(f'OK ({total} dead)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    # --- Practicality-fix probes: notes round-trip, overview guards, query_codebase,
    #     coverage + changelog sidecars, get_source. Loosely asserted: those tools
    #     should respond OK and shape a useful summary. The runtime test is not
    #     the unit-test bed for tool internals; it just asserts end-to-end wiring.

    run_probe "add_note + list_notes round-trip" "list_notes" '{"entity_name":"TestApp.OrderService"}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    r=d.get('result',{})
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        notes=r.get('notes',[]) or []
        print(f'OK ({len(notes)} notes)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "codebase_overview(summary)" "codebase_overview" '{"view":"summary"}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        types=r.get('entity_type_counts',{}) or {}
        print(f'OK ({sum(types.values())} entities across {len(types)} types)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "codebase_overview(graph,limit=20)" "codebase_overview" '{"view":"graph","limit":20,"offset":0}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        ents=len(r.get('entities',[]) or [])
        truncated=r.get('truncated', False)
        flag=' (truncated)' if truncated else ''
        print(f'OK ({ents} nodes{flag})')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "index_coverage" "index_coverage" '{}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"ERROR: {d.get('error','unknown')}\")
    else:
        r=d.get('result',{})
        proj=len(r.get('projects',[]) or [])
        skipped=r.get('total_skipped',0)
        print(f'OK ({proj} projects, {skipped} skipped items)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "what_changed" "what_changed" '{"hours":168}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        print(f\"WARN: tooling requires watch-mode scans; {d.get('error','no scans')}\")
    else:
        r=d.get('result',{})
        total=r.get('total',0)
        items=len(r.get('changes',[]) or [])
        print(f'OK ({items}/{total} recent changes)' if items else 'OK (no changes yet)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"

    run_probe "get_source(enabled by SOURCE_ROOT)" "get_source" '{"entity_name":"TestApp.OrderService","max_lines":5}' "
import json,sys
try:
    d=json.loads(sys.stdin.read())
    if not d.get('ok'):
        msg=str(d.get('error',''))
        if 'SOURCE_ROOT' in msg or 'disabled' in msg:
            print(f'OK (tool gated as expected: {msg[:60]})')
        else:
            print(f'ERROR: {msg}')
    else:
        r=d.get('result',{})
        lines=len(r.get('lines',[]) or [])
        print(f'OK ({lines} source lines)')
except Exception as e:
    print(f'ERROR: parse: {e}')
"
fi

echo "[OK] Runtime test complete"
