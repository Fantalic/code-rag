import subprocess
import os
import sys
import time

PROJECT_ROOT = r"E:\_dev\_Projects\code-lightrag"
WORKING_DIR = os.path.join(PROJECT_ROOT, ".coderag", "rag_storage")
PYTHON = os.path.join(PROJECT_ROOT, "venv", "Scripts", "python.exe")

# Set environment before starting process
env = os.environ.copy()
env["LIGHTRAG_WORKING_DIR"] = WORKING_DIR
env["PYTHONIOENCODING"] = "utf-8:replace"

# Create the command as a single file to avoid quoting issues
server_code = f'''
import os
os.environ["LIGHTRAG_WORKING_DIR"] = r"{WORKING_DIR}"
os.chdir(r"{PROJECT_ROOT}")
from lightrag.api.lightrag_server import main
import sys
sys.argv=["", "--working-dir", r"{WORKING_DIR}", "--port", "9621", "--host", "0.0.0.0"]
main()
'''

# Write to temp file
temp_file = os.path.join(PROJECT_ROOT, "testing", "_temp_server.py")
with open(temp_file, "w", encoding="utf-8") as f:
    f.write(server_code)

# Run with UTF-8 mode
proc = subprocess.Popen(
    [PYTHON, "-X", "utf8", temp_file],
    env=env,
    stdout=subprocess.PIPE,
    stderr=subprocess.STDOUT,
    text=True,
    bufsize=1
)

print(f"Started server with PID {proc.pid}")

# Read output line by line for 5 seconds
start = time.time()
while time.time() - start < 5:
    line = proc.stdout.readline()
    if line:
        print(line.rstrip())
    if proc.poll() is not None:
        print("Process exited!")
        break

# Check if server is running
import httpx
try:
    client = httpx.Client(base_url="http://localhost:9621", timeout=10)
    r = client.get("/graph/label/list")
    print(f"\nServer is running! Labels: {r.json()}")
except Exception as e:
    print(f"\nServer not responding: {e}")

# Clean up temp file
if os.path.exists(temp_file):
    os.remove(temp_file)