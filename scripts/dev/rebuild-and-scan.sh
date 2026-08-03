#!/usr/bin/env bash
set -euo pipefail

# codeRAG — build the C# analyser + coderag CLI, then run a scan
# Usage:
#   bash scripts/dev/rebuild-and-scan.sh                     # scan testdata sample
#   CLEAN=1 bash scripts/dev/rebuild-and-scan.sh              # wipe graph first
#   bash scripts/dev/rebuild-and-scan.sh /path/to/MyApp
#   SKIP_BUILD=1 bash scripts/dev/rebuild-and-scan.sh         # reuse existing binaries

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_DIR="${1:-$ROOT/coderag/cli/analysers/csharp/testdata}"
CLEAN="${CLEAN:-0}"
SKIP_BUILD="${SKIP_BUILD:-0}"

if [ "$SKIP_BUILD" != "1" ]; then
    echo "[..] Building C# analyser ..."
    go build -C "$ROOT/coderag/cli/analysers/csharp" -o "$ROOT/coderag/cli/analysers/csharp/csharp_analyser.exe" ./cmd/csharp_analyser
    echo "[OK] C# analyser built"

    echo "[..] Building coderag CLI ..."
    go build -C "$ROOT/coderag/cli" -o "$ROOT/coderag" .
    echo "[OK] coderag built"
fi

if [ ! -d "$PROJECT_DIR" ]; then
    echo "[XX] Project directory not found: $PROJECT_DIR" >&2
    exit 1
fi

ARGS=("scan")
[ "$CLEAN" = "1" ] && ARGS+=("--clean")
ARGS+=("$PROJECT_DIR")

echo "[..] Scanning $PROJECT_DIR ..."
"$ROOT/coderag" "${ARGS[@]}" 2>&1 |
    grep -E "Extracted|Entities:|Relations:|Skipping|Scan complete|Analyser produced|ERROR"
