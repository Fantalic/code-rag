"""
C# analyser runner — invokes CSharpAnalyzerTool.exe and collects output.

DEPRECATED: This module is no longer used for analysis in production.
The C# analyser is now run by the Go watchdog (pkg/watchdog/pipeline/run.go)
which runs as a background goroutine when `coderag mcp` is invoked.
The `coderag scan` command provides explicit analysis trigger.

This module is kept for potential future use (e.g., manual analysis from MCP).
"""

from __future__ import annotations

import logging
import re
import subprocess
from pathlib import Path
from typing import List, Tuple

logger = logging.getLogger(__name__)

# Patterns for auto-generated files to skip during ingestion
SKIP_FILE_PATTERNS: List[str] = [
    r"_index\.txt$",
    r"AssemblyInfo",
    r"GlobalUsings",
    r"AssemblyAttributes",
]

# NOTE: The analyser binary has been moved to project root analysers/c_sharp/
# The Go watchdog handles analysis using the host-side binary.
# This path is unused in production but kept for potential future MCP-side analysis.
ANALYSER_RELATIVE_PATH = "../../analysers/c_sharp/CSharpAnalyzerTool.exe"

ANALYSER_OUTPUT_DIR = "relations_output"

ANALYSER_TIMEOUT_SECONDS = 300


def _find_analyser_binary() -> Path:
    """Resolve path to CSharpAnalyzerTool.exe relative to this module."""
    module_dir = Path(__file__).resolve().parent
    candidate = (module_dir / ANALYSER_RELATIVE_PATH).resolve()
    if not candidate.exists():
        raise FileNotFoundError(
            f"C# analyser binary not found at {candidate}. "
            f"Expected relative to {module_dir}"
        )
    return candidate


def _find_solution_file(project_path: str) -> Path:
    """Resolve a .sln file from a path (file or directory)."""
    path = Path(project_path).resolve()
    if not path.exists():
        raise FileNotFoundError(f"Project path does not exist: {path}")

    if path.is_file():
        if path.suffix == ".sln":
            return path
        if path.suffix == ".csproj":
            raise ValueError(f"C# analyser requires a .sln file, not a .csproj: {path}")
        raise ValueError(f"Unsupported file type: {path.suffix}")

    sln_files = list(path.glob("*.sln"))
    if not sln_files:
        raise FileNotFoundError(f"No .sln file found in directory: {path}")
    if len(sln_files) > 1:
        logger.warning(
            f"Multiple .sln files found in {path}, using first: {sln_files[0]}"
        )
    return sln_files[0]


def _should_skip(filename: str) -> bool:
    """Check if a filename matches auto-generated skip patterns."""
    return any(re.search(p, filename) for p in SKIP_FILE_PATTERNS)


def _filter_output_files(output_dir: Path) -> List[Path]:
    """Return analyser output files that should be ingested (skip noise)."""
    if not output_dir.is_dir():
        logger.warning(f"Analyser output directory not found: {output_dir}")
        return []

    files = sorted(output_dir.iterdir())
    kept = []
    skipped = []
    for f in files:
        if not f.is_file() or f.suffix != ".txt":
            continue
        if _should_skip(f.name):
            skipped.append(f.name)
        else:
            kept.append(f)

    if skipped:
        logger.info(f"Skipped {len(skipped)} auto-generated files: {skipped}")
    logger.info(f"Keeping {len(kept)} analyser output files for ingestion")
    return kept


def _read_output_files(output_dir: Path) -> List[Tuple[str, str]]:
    """Read analyser output files and return (filename, content) pairs."""
    files = _filter_output_files(output_dir)
    results: List[Tuple[str, str]] = []
    for f in files:
        try:
            content = f.read_text(encoding="utf-8")
            results.append((f.name, content))
        except Exception as e:
            logger.error(f"Failed to read {f}: {e}")
    return results


def run_analysis(
    project_path: str, output_dir: str | None = None
) -> List[Tuple[str, str]]:
    """
    Run the C# analyser on a project and return (filename, content) pairs.

    The analyser always writes to a ``relations_output/`` subdirectory relative to
    its working directory.  We set the working directory to ``output_dir`` (which
    defaults to the .sln's parent directory) and then read from
    ``output_dir/relations_output/``.

    Args:
        project_path: Path to a .sln file or a directory containing one.
        output_dir: Working directory for the analyser.  Defaults to the .sln's parent.

    Returns:
        List of (filename, content) tuples for ingestion.
    """
    analyser_path = _find_analyser_binary()
    solution_path = _find_solution_file(project_path)

    if output_dir is None:
        output_dir = str(solution_path.parent)
    work_dir = Path(output_dir).resolve()
    work_dir.mkdir(parents=True, exist_ok=True)

    logger.info(f"Running C# analyser on {solution_path}, cwd={work_dir}")

    try:
        result = subprocess.run(
            [str(analyser_path), str(solution_path)],
            cwd=str(work_dir),
            capture_output=True,
            text=True,
            timeout=ANALYSER_TIMEOUT_SECONDS,
        )
    except subprocess.TimeoutExpired:
        raise TimeoutError(
            f"C# analyser timed out after {ANALYSER_TIMEOUT_SECONDS}s "
            f"on {solution_path}"
        )
    except FileNotFoundError:
        raise FileNotFoundError(
            f"C# analyser binary not found at {analyser_path}. "
            f"Is .NET 9 runtime installed?"
        )

    if result.returncode != 0:
        stderr_preview = result.stderr[:500] if result.stderr else "(none)"
        raise RuntimeError(
            f"C# analyser exited with code {result.returncode}:\n"
            f"stdout: {result.stdout[:300]}\n"
            f"stderr: {stderr_preview}"
        )

    logger.info(f"C# analyser stdout:\n{result.stdout}")

    # The analyser always writes to a subdirectory 'relations_output' within cwd
    analyser_out = work_dir / ANALYSER_OUTPUT_DIR
    if analyser_out.is_dir():
        return _read_output_files(analyser_out)

    # Fallback: check cwd itself (analyser might have changed behaviour)
    logger.warning(f"Analyser output dir {analyser_out} not found, checking {work_dir}")
    return _read_output_files(work_dir)
