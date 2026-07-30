import os
import sys

os.environ["WORKING_DIR"] = r"E:\_dev\_Projects\code-lightrag\.coderag\rag_storage"
os.environ["LIGHTRAG_WORKING_DIR"] = r"E:\_dev\_Projects\code-lightrag\.coderag\rag_storage"
os.chdir(r"E:\_dev\_Projects\code-lightrag")

from lightrag.api.lightrag_server import main

sys.argv = [
    "",
    "--working-dir", r"E:\_dev\_Projects\code-lightrag\.coderag\rag_storage",
    "--port", "9621",
    "--host", "0.0.0.0"
]

main()