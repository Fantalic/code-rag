"""
E2E: C# analyser -> LightRAG -> query with REAL Ollama LLM + real embedding.

Run with Docker:                    pytest tests/test_e2e_csharp.py -v
Run standalone (starts own server): pytest tests/test_e2e_csharp.py --standalone -v
"""
import sys, os, json, time, asyncio, shutil

LIGHTRAG_PORT = int(os.environ.get("LIGHTRAG_PORT", "9621"))
STANDALONE = "--standalone" in sys.argv
LLM_MODEL = os.environ.get("LLM_MODEL", "qwen3-coder:480b-cloud")
EMBEDDING_MODEL = os.environ.get("EMBEDDING_MODEL", "bge-m3:latest")

if STANDALONE:
    sys.argv = ["lightrag-server", "--host", "127.0.0.1", "--port", str(LIGHTRAG_PORT)]
    os.environ["LLM_MODEL"] = LLM_MODEL
    os.environ["EMBEDDING_MODEL"] = EMBEDDING_MODEL
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

    import lightrag.api.lightrag_server as ls_mod
    ls_mod.check_frontend_build = lambda: None

    from lightrag.api.lightrag_server import create_app
    from lightrag.api.config import get_global_args

    app = create_app(get_global_args())

    import uvicorn
    from threading import Thread
    t = Thread(target=lambda: uvicorn.run(app, host="127.0.0.1", port=LIGHTRAG_PORT, log_level="warning"), daemon=True)
    t.start()
    time.sleep(5)

import httpx
cli = httpx.Client(base_url=f"http://127.0.0.1:{LIGHTRAG_PORT}", timeout=120)
results = []

def check(label, ok, detail=""):
    status = "PASS" if ok else "FAIL"
    results.append((status, label, detail))

# 1. Health
r = cli.get("/health")
check("GET /health", r.status_code == 200, r.status_code)
if r.status_code == 200:
    print(f"  LLM: {r.json().get('configuration', {}).get('llm_model', '?')}", flush=True)

# 2. Run C# analyser and ingest output
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from daniel_lightrag_mcp.analyser_runner import run_analysis

sln = os.path.join(os.path.dirname(__file__), "..", "test-csharp-project", "TestCodebase.sln")
analysis_results = run_analysis(sln)
check(f"C# analyser generated {len(analysis_results)} files", True)

track_ids = []
for filename, content in analysis_results:
    r = cli.post("/documents/text", json={"text": content, "file_source": filename})
    ok = r.status_code == 200
    check(f"POST /documents/text ({filename})", ok, r.status_code)
    if ok:
        track_ids.append(r.json().get("track_id", ""))
check(f"Ingested {len(track_ids)} analyser output files", True)

# 3. Wait for async processing (extraction + merge)
print("Waiting for document processing...", flush=True)
for _ in range(12):
    r = cli.get("/documents")
    data = r.json()
    statuses = data.get("statuses", {})
    processed = len(statuses.get("processed", []))
    failed = len(statuses.get("failed", []))
    processing = len(statuses.get("processing", []))
    print(f"  processed={processed} failed={failed} processing={processing}", flush=True)
    if processing == 0 and (processed + failed) >= len(analysis_results):
        break
    time.sleep(10)

# 4. Status counts
r = cli.get("/documents/status_counts")
check("GET /documents/status_counts", r.status_code == 200, r.status_code)
if r.status_code == 200:
    check("Status counts data", True, str(r.json())[:400])

# 5. Track status
if track_ids:
    r = cli.get(f"/documents/track_status/{track_ids[0]}")
    check("GET /documents/track_status/<id>", r.status_code == 200, r.status_code)
    if r.status_code == 200:
        check("Track detail", True, str(r.json())[:500])

# 6. Graph labels (verify entities were extracted)
r = cli.get("/graph/label/list")
check("GET /graph/label/list", r.status_code == 200, r.status_code)
if r.status_code == 200:
    labels = r.json()
    check(f"Graph has {len(labels)} labels", len(labels) > 0, str(labels)[:300])
    has_class = any("Calculator" in str(l) or "DataStore" in str(l) or "Class1" in str(l) for l in labels)
    check("Graph contains extracted classes", has_class, str(labels)[:300])

# 7. Query
r = cli.post("/query", json={"query": "What classes are in the codebase?", "mode": "local"})
check("POST /query", r.status_code == 200, r.status_code)
if r.status_code == 200:
    resp = r.json()
    check("Query has response", bool(resp.get("response")), (resp.get("response") or "")[:300])
    check("Query has references", bool(resp.get("references")), f"{len(resp.get('references', []))} refs")

# 8. Pipeline status
r = cli.get("/documents/pipeline_status")
check("GET /documents/pipeline_status", r.status_code == 200, r.status_code)
if r.status_code == 200:
    check("Pipeline data", True, str(r.json())[:400])

# 9. Clean up: clear documents via API
r = cli.delete("/documents")
check("DELETE /documents (cleanup)", r.status_code in (200, 204), r.status_code)

print()
print("=" * 60)
print(f"  E2E TEST RESULTS ({LLM_MODEL}, port {LIGHTRAG_PORT})")
print("=" * 60)
for status, label, detail in results:
    print(f"  {status:6s} | {label}")
    if detail and not isinstance(detail, bool):
        print(f"         {detail}")
print("=" * 60)

failed = sum(1 for s, _, _ in results if s == "FAIL")
if failed:
    sys.exit(1)
