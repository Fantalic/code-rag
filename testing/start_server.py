#!/usr/bin/env python3
"""Start LightRAG server with proper configuration"""
import os
import sys
import threading
import time

# Fix UTF-8 encoding for Windows
if sys.platform == 'win32':
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

# Set working directory to project root
PROJECT_ROOT = os.path.dirname(os.path.abspath(__file__))
os.chdir(PROJECT_ROOT)

# Set environment
WORKING_DIR = os.path.join(PROJECT_ROOT, ".coderag", "rag_storage")
os.environ["LIGHTRAG_WORKING_DIR"] = WORKING_DIR

print(f"Starting LightRAG with working_dir: {WORKING_DIR}")

# Import and start server
from lightrag.api.lightrag_server import main

if __name__ == "__main__":
    sys.argv = [
        sys.argv[0],
        "--working-dir", WORKING_DIR,
        "--port", "9621",
        "--host", "0.0.0.0",
    ]
    main()