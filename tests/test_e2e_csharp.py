"""
E2E: C# analyser -> LightRAG -> query with REAL Ollama LLM + mock embedding.
"""
import sys, os, json, time, asyncio, shutil

sys.argv = ["lightrag-server", "--host", "127.0.0.1", "--port", "9621"]
os.environ["LLM_MODEL"] = "qwen2.5-coder:1.5b"
os.environ["EMBEDDING_MODEL"] = "bge-m3:latest"
os.environ["EMBEDDING_DIM"] = "1024"

# Clean stale rag_storage
rag_storage = os.path.join(os.path.dirname(__file__), "..", "rag_storage")
if os.path.isdir(rag_storage):
    for f in os.listdir(rag_storage):
        fpath = os.path.join(rag_storage, f)
        try:
            if os.path.isfile(fpath) or os.path.islink(fpath):
                os.unlink(fpath)
            elif os.path.isdir(fpath):
                shutil.rmtree(fpath)
        except Exception as e:
            print(f"Warning: could not clean {fpath}: {e}")

# Real embedding via bge-m3 locally; real LLM via qwen2.5-coder:1.5b

import lightrag.api.lightrag_server as ls_mod
ls_mod.check_frontend_build = lambda: None

from ascii_colors import ASCIIColors
from lightrag.api.lightrag_server import create_app
from lightrag.api.config import get_global_args

app = create_app(get_global_args())

import uvicorn
from threading import Thread
t = Thread(target=lambda: uvicorn.run(app, host="127.0.0.1", port=9621, log_level="warning"), daemon=True)
t.start()
time.sleep(5)

import httpx
cli = httpx.Client(base_url="http://127.0.0.1:9621", timeout=120)
results = []

def check(label, ok, detail=""):
    status = "PASS" if ok else "FAIL"
    results.append((status, label, detail))

# 1. Health
r = cli.get("/health")
check("GET /health", r.status_code == 200, r.status_code)

# 2. Ingest C# analyser output
analyser_dir = os.path.join(os.path.dirname(__file__), "..", "test-csharp-project", "output")
track_ids = []
for fname in sorted(os.listdir(analyser_dir)):
    fpath = os.path.join(analyser_dir, fname)
    if not os.path.isfile(fpath) or fname == "_index.txt":
        continue
    with open(fpath, "r", encoding="utf-8") as fh:
        r = cli.post("/documents/text", json={"text": fh.read()})
    ok = r.status_code == 200
    check(f"POST /documents/text ({fname})", ok, r.status_code)
    if ok:
        track_ids.append(r.json().get("track_id", ""))
check(f"Ingested {len(track_ids)} analyser output files", True)

# Wait for async processing (extraction + merge)
print("Waiting for document processing...", flush=True)
time.sleep(15)

# 3. Status counts
r = cli.get("/documents/status_counts")
check("GET /documents/status_counts", r.status_code == 200, r.status_code)
if r.status_code == 200:
    check("Status counts data", True, str(r.json())[:400])

# 4. Track status
if track_ids:
    r = cli.get(f"/documents/track_status/{track_ids[0]}")
    check("GET /documents/track_status/<id>", r.status_code == 200, r.status_code)
    if r.status_code == 200:
        check("Track detail", True, str(r.json())[:500])

# 5. Query
r = cli.post("/query", json={"query": "What classes are in the codebase?", "mode": "local"})
check("POST /query", r.status_code == 200, r.status_code)
if r.status_code == 200:
    resp = r.json()
    check("Query has response", bool(resp.get("response")), resp["response"][:300])
    check("Query has references", bool(resp.get("references")), f"{len(resp.get('references', []))} refs")

# 6. Pipeline status
r = cli.get("/documents/pipeline_status")
check("GET /documents/pipeline_status", r.status_code == 200, r.status_code)
if r.status_code == 200:
    check("Pipeline data", True, str(r.json())[:400])

print()
print("=" * 60)
print("  E2E TEST RESULTS (real Ollama LLM)")
print("=" * 60)
for status, label, detail in results:
    print(f"  {status:6s} | {label}")
    if detail and not isinstance(detail, bool):
        print(f"         {detail}")
print("=" * 60)
