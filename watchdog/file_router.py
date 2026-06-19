"""File extension to analyser mapping.

Maps file extensions to the appropriate analyser module.
Analysers extract entities + relations from source code.
"""

from __future__ import annotations

import logging
from typing import Callable

logger = logging.getLogger(__name__)

# Extension -> (analyser_name, supported)
# When Zig analysers are added, they'll appear here.
EXTENSION_MAP: dict[str, str] = {
    ".cs": "c_sharp",
}

# Future analysers (to be written in Zig):
# ".py":  "python",
# ".js":  "javascript",
# ".ts":  "typescript",
# ".go":  "golang",
# ".rs":  "rust",
# ".md":  "markdown",
# ".txt": "text",
# ".yaml": "yaml",
# ".yml":  "yaml",
# ".ini":  "ini",
# ".toml": "toml",
# ".json": "json",


# Placeholder analyser signatures -- real ones will be loaded dynamically
# Type: (file_path: str) -> tuple[list[dict], list[dict]]
# Returns: (entities, relations) where each is a list of dicts
AnalyserFunc = Callable[[str], tuple[list[dict], list[dict]]]


def get_analyser(extension: str) -> str | None:
    """Return the analyser name for a file extension, or None if unsupported."""
    return EXTENSION_MAP.get(extension.lower())


def is_supported(path: str) -> bool:
    """Check if a file path has a supported extension."""
    import os

    _, ext = os.path.splitext(path)
    return ext.lower() in EXTENSION_MAP
