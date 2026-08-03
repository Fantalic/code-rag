#!/usr/bin/env python3
"""Proper server startup with correct working directory"""
import os
import sys
import subprocess

PROJECT_ROOT = r"E:\_dev\_Projects\code-lightrag"
WORKING_DIR = os.path.join(PROJECT_ROOT, ".coderag", "rag_storage")
PYTHON = os.path.join(PROJECT_ROOT, "venv", "Scripts", "python.exe")

# Verify working dir exists and has data
if os.path.exists(WORKING_DIR):
    graph_file = os.path.join(WORKING_DIR, "graph_chunk_entity_relation.graphml")
    vdb_file = os.path.join(WORKING_DIR, "vdb_entities.json")
    print(f"Working dir: {WORKING_DIR}")
    print(f"Graph file exists: {os.path.exists(graph_file)}")
    if os.path.exists(graph_file):
        with open(graph_file, 'r') as f:
            content = f.read()
            print(f"Graph file size: {len(content)} bytes")
else:
    print(f"Working dir does not exist: {WORKING_DIR}")

# Set environment
env = os.environ.copy()
env["LIGHTRAG_WORKING_DIR"] = WORKING_DIR

# Create startup script
startup_script = os.path.join(PROJECT_ROOT, "testing", "_server_startup.py")
with open(startup_script, "w") as f:
    f.write(f'''
import os
os.environ["LIGHTRAG_WORKING_DIR"] = r"{WORKING_DIR}"
os.chdir(r"{PROJECT_ROOT}")
from lightrag.api.lightrag_server import main
import sys
sys.argv = [
    "",
    "--working-dir", r"{WORKING_DIR}",
    "--port", "9621",
    "--host", "0.0.0.0"
]
main()
''')

print(f"Created startup script: {startup_script}")

# Run server
print("Starting server...")
proc = subprocess.Popen(
    [PYTHON, startup_script],
    env=env,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    text=True
)

print(f"Server PID: {proc.pid}")

# Wait for startup
time.sleep(15)

# Check if server is running
try:
    import httpx
    client = httpx.Client(base_url="http://localhost:9621", timeout=None)
    r = client.get("/graph/label/list")
    labels = r.json()
    print(f"Server running! Labels: {labels}")

    # Check graph
    r = client.get("/graphs?label=*&max_depth=1&max_nodes=1000")
    data = r.json()
    print(f"Nodes: {{len(data.get('nodes', []))}}")
    print(f"Edges: {{len(data.get('edges', []))}}")
except Exception as e:
    print(f"Server check failed: {{e}}")
    # Show stderr if available
    if proc.stderr:
        stderr = proc.stderr.read(500)
        if stderr:
            print(f"Stderr: {{stderr}}")

print("Server process running in background.")
print("Press Ctrl+C to stop.")