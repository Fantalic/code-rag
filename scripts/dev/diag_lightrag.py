"""diag_lightrag.py - dump contents of a LightRAG workspace storage.

Reads JSON storage files from inside (or outside) the lightrag-server
container and prints a human-readable summary.

Usage:
    python scripts/dev/diag_lightrag.py [--container NAME] [--workspace NAME] [--filter TEXT]

Examples:
    python scripts/dev/diag_lightrag.py
    python scripts/dev/diag_lightrag.py --container lightrag-server --workspace TestApp
    python scripts/dev/diag_lightrag.py --filter OrderService
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

DEFAULT_STORAGE = "/app/data/rag_storage"
DEFAULT_CONTAINER = os.environ.get("LIGHTRAG_CONTAINER", "lightrag-server")


def read_remote(container: str, path: str) -> str:
    r = subprocess.run(
        ["docker", "exec", container, "cat", path],
        capture_output=True, text=True, check=False,
    )
    if r.returncode != 0:
        raise FileNotFoundError(f"cannot read {container}:{path}: {r.stderr.strip()}")
    return r.stdout


def load_json(text: str) -> object:
    try:
        return json.loads(text)
    except json.JSONDecodeError as e:
        return {"_parse_error": str(e), "_raw_head": text[:200]}


def summarize_kv(name: str, data: object, flt: str) -> None:
    if not isinstance(data, dict):
        print(f"  {name}: {type(data).__name__} (len={len(data) if hasattr(data,'__len__') else '?'})")
        return
    keys = list(data.keys())
    matched = [k for k in keys if not flt or flt.lower() in k.lower()]
    print(f"  {name}: {len(keys)} entries ({len(matched)} match filter)")
    for k in matched[:20]:
        v = data[k]
        if isinstance(v, dict):
            preview = ", ".join(f"{kk}={str(vv)[:60]}" for kk, vv in list(v.items())[:5])
        else:
            preview = str(v)[:120]
        print(f"    {k[:80]:80s} | {preview[:120]}")
    if len(matched) > 20:
        print(f"    ... ({len(matched) - 20} more)")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--container", default=DEFAULT_CONTAINER)
    ap.add_argument("--workspace", default=None, help="workspace name (default: autodetect)")
    ap.add_argument("--storage", default=DEFAULT_STORAGE)
    ap.add_argument("--filter", dest="flt", default="", help="substring filter on keys/names")
    args = ap.parse_args()

    storage = args.storage
    ws = args.workspace
    if not ws:
        idx_path = f"{storage}/_index.json"
        try:
            idx = load_json(read_remote(args.container, idx_path))
            if isinstance(idx, dict) and "workspaces" in idx:
                wss = [w for w in idx["workspaces"] if not args.flt or args.flt.lower() in w.lower()]
                print(f"[..] workspaces in index: {wss}")
                if wss:
                    ws = wss[0]
            elif isinstance(idx, dict):
                ws = next(iter(idx.keys()), None)
        except FileNotFoundError:
            print(f"[..] no _index.json, scanning workspace dirs")
        if not ws:
            r = subprocess.run(
                ["docker", "exec", args.container, "sh", "-c", f"ls {storage}"],
                capture_output=True, text=True, check=False,
            )
            candidates = [d for d in r.stdout.split() if d and not d.startswith(".") and d != "_index.json"]
            print(f"[..] candidate workspaces: {candidates}")
            if candidates:
                ws = candidates[0]

    if not ws:
        print("[XX] no workspace found")
        return 1
    print(f"[..] workspace: {ws}")
    print(f"[..] container: {args.container}")
    print()

    files = [
        "kv_store_doc_status.json",
        "kv_store_full_docs.json",
        "kv_store_text_chunks.json",
        "kv_store_full_entities.json",
        "kv_store_entity_chunks.json",
        "kv_store_full_relations.json",
        "kv_store_relation_chunks.json",
    ]

    for f in files:
        path = f"{storage}/{ws}/{f}"
        print(f"[{f}]")
        try:
            text = read_remote(args.container, path)
        except FileNotFoundError:
            print("  (not present)")
            continue
        data = load_json(text)
        if "_parse_error" in data:
            print(f"  parse error: {data['_parse_error']}")
            print(f"  raw head: {data['_raw_head']}")
            continue
        summarize_kv(f.replace("kv_store_", "").replace(".json", ""), data, args.flt)
        print()

    print("[OK] diag_lightrag complete")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
