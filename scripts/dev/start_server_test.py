#!/usr/bin/env python3
"""Start LightRAG server and verify it works"""
import os
import sys
import time
import threading
import subprocess

# Fix UTF-8 encoding for Windows
if sys.platform == 'win32':
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

PROJECT_ROOT = os.path.dirname(os.path.abspath(__file__))
WORKING_DIR = os.path.join(PROJECT_ROOT, ".coderag", "rag_storage")

os.environ["LIGHTRAG_WORKING_DIR"] = WORKING_DIR

print(f"Working dir: {WORKING_DIR}")
print(f"Storage exists: {os.path.exists(WORKING_DIR)}")

# Start server process
proc = subprocess.Popen(
    [os.path.join(PROJECT_ROOT, "venv", "Scripts", "python.exe"), "-c", f"""
import os
os.environ['LIGHTRAG_WORKING_DIR'] = r'{WORKING_DIR}'
os.chdir(r'{PROJECT_ROOT}')
from lightrag.api.lightrag_server import main
sys.argv = ['', '--working-dir', r'{WORKING_DIR}', '--port', '9621', '--host', '0.0.0.0']
main()
"""],
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    text=True
)

print(f"Started server process: {proc.pid}")

# Wait for server to start
time.sleep(15)

# Test if server is responding
try:
    import httpx
    client = httpx.Client(base_url='http://localhost:9621', follow_redirects=True)
    r = client.get('/graph/label/list')
    print(f"Server responding! Status: {r.status_code}")
    print(f"Labels: {r.json()}")
except Exception as e:
    print(f"Server not responding: {e}")

# Keep server running
print("Server running. Press Ctrl+C to stop.")
try:
    proc.wait()
except KeyboardInterrupt:
    proc.terminate()
    proc.wait()