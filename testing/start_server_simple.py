#!/usr/bin/env python3
"""Start LightRAG server and verify it works"""
import os
import sys
import time

PROJECT_ROOT = r"E:\_dev\_Projects\code-lightrag"
WORKING_DIR = os.path.join(PROJECT_ROOT, ".coderag", "rag_storage")
PYTHON_EXE = os.path.join(PROJECT_ROOT, "venv", "Scripts", "python.exe")

print(f"Working dir: {WORKING_DIR}")
print(f"Storage exists: {os.path.exists(WORKING_DIR)}")
print(f"Python: {PYTHON_EXE}")

os.environ["LIGHTRAG_WORKING_DIR"] = WORKING_DIR
os.chdir(PROJECT_ROOT)

# Import after chdir so lightrag can find its modules
from lightrag.api.lightrag_server import main

print("Starting server...")
sys.argv = ["", "--working-dir", WORKING_DIR, "--port", "9621", "--host", "0.0.0.0"]
main()