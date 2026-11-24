"""Tests for the Python analyser (format-2 output).

Creates a temp project with a package, class, method, @property, module
function, constant, imports and an intra-file call.  Runs the analyser
via subprocess and asserts the expected output.

Also tests that a syntax-error file is reported in coverage skipped while
other files still produce output.
"""

import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

# Resolve the analyser path relative to the repo root
_REPO_ROOT = Path(__file__).resolve().parent.parent.parent.parent.parent
ANALYSER = _REPO_ROOT / "cli" / "analysers" / "python" / "python_analyser.py"


# ---------------------------------------------------------------------------
# Fixture: a temp project with typical Python code
# ---------------------------------------------------------------------------

def _create_project() -> str:
    """Create a temporary project directory with sample Python code.

    Returns the absolute path to the project root.
    """
    root = tempfile.mkdtemp()

    # -- Package layout --------------------------------------------------
    pkg = Path(root) / "mypkg"
    pkg.mkdir()
    (pkg / "__init__.py").write_text(
        '"""My package."""\nfrom . import core\n')

    # -- core.py: class, methods, property, constant, imports, calls ------
    core = pkg / "core.py"
    core.write_text(
        '"""Core module."""\n'
        'import os\n'
        'import sys\n'
        '\n'
        'DEFAULT_VALUE = 42\n'
        '\n'
        'def helper(x: int) -> str:\n'
        '    """A helper function."""\n'
        '    return str(x)\n'
        '\n'
        'class MyClass:\n'
        '    """A sample class."""\n'
        '\n'
        '    def __init__(self, name: str) -> None:\n'
        '        self.name = name\n'
        '\n'
        '    def process(self) -> str:\n'
        '        """Process something."""\n'
        '        return helper(self.name)\n'
        '\n'
        '    @property\n'
        '    def label(self) -> str:\n'
        '        """The label property."""\n'
        '        return self.name.upper()\n'
        '\n'
        'class BaseClass:\n'
        '    """Base class with no methods."""\n'
    )

    # -- bad.py: syntax error --------------------------------------------
    bad = pkg / "bad.py"
    bad.write_text('def broken(\n')  # unclosed paren

    return root


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _run_analyser(project_dir: str, *extra_args: str) -> subprocess.CompletedProcess:
    """Run the analyser via subprocess and return the CompletedProcess."""
    cmd = [sys.executable, str(ANALYSER), "scan", "--dir", project_dir] + list(extra_args)
    return subprocess.run(cmd, capture_output=True, text=True)


def _read_output_files(project_dir: str) -> dict[str, str]:
    """Read all .txt files from the .lightrag output directory."""
    output_dir = os.path.join(project_dir, ".lightrag")
    result: dict[str, str] = {}
    if os.path.isdir(output_dir):
        for fname in sorted(os.listdir(output_dir)):
            fpath = os.path.join(output_dir, fname)
            if os.path.isfile(fpath) and fname.endswith(".txt"):
                result[fname] = open(fpath, encoding="utf-8").read()
    return result


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_scan_format_and_entities() -> None:
    """Full scan produces correct format-2 output with expected entities."""
    project_dir = _create_project()

    cp = _run_analyser(project_dir)
    assert cp.returncode == 0, f"stderr: {cp.stderr}"

    files = _read_output_files(project_dir)

    # -- Verify _coverage.txt exists and has a COVERAGE row ----------------
    assert "_coverage.txt" in files, "Missing _coverage.txt"
    cov_lines = files["_coverage.txt"].strip().splitlines()
    coverage_found = False
    for line in cov_lines:
        if line.startswith("COVERAGE|"):
            coverage_found = True
            # Parse the JSON portion
            json_part = line.split("|", 2)[2]
            data = json.loads(json_part)
            assert "files" in data, "Missing 'files' in coverage JSON"
            assert data["files"] >= 3, f"Expected >= 3 files, got {data['files']}"
            break
    assert coverage_found, "No COVERAGE row found in _coverage.txt"

    # -- Verify _index.txt exists -----------------------------------------
    assert "_index.txt" in files, "Missing _index.txt"
    assert "# GLOBAL INDEX" in files["_index.txt"]

    # -- Find the core.py output file -------------------------------------
    core_flat = "mypkg__core.py.txt"
    assert core_flat in files, f"Missing output file {core_flat}"
    content = files[core_flat]
    lines = content.strip().splitlines()

    # Header lines
    assert lines[0] == "# FORMAT: 2", f"First line: {lines[0]}"
    assert lines[1].startswith("# EXPORTS:"), f"EXPORTS header: {lines[1]}"
    # IMPORTS is always the last header (may have optional MTIME in between)
    import_line = next((l for l in lines if l.startswith("# IMPORTS:")), None)
    assert import_line is not None, "IMPORTS header missing"

    # Check for MTIME header
    has_mtime = any(l.startswith("# MTIME:") for l in lines)
    # MTIME is optional; if present it should be between EXPORTS and IMPORTS

    # Entity types present
    entity_types = set()
    for line in lines:
        if line.startswith("#") or not line:
            continue
        parts = line.split("|", 1)
        if parts and parts[0] in ("CLASS", "FUNCTION", "METHOD", "PROPERTY",
                                   "CONST", "VAR", "IMPORT", "RELATION", "CALL"):
            entity_types.add(parts[0])

    assert "CLASS" in entity_types, "Missing CLASS entity"
    assert "FUNCTION" in entity_types, "Missing FUNCTION entity"
    assert "METHOD" in entity_types, "Missing METHOD entity"
    assert "PROPERTY" in entity_types, "Missing PROPERTY entity"
    assert "CONST" in entity_types, "Missing CONST entity"

    # No pipe inside description columns (entity rows have 6 cols, no extra |)
    for line in lines:
        if line.startswith("#") or not line:
            continue
        if line.startswith("RELATION|") or line.startswith("CALL|"):
            # These have different column counts, skip pipe check
            continue
        parts = line.split("|")
        if len(parts) != 6:
            # RELATION has 7, CALL has 5 — but we already filtered those
            # Entity rows should have exactly 6 columns
            assert len(parts) == 6, f"Wrong column count ({len(parts)}) in: {line}"

    # RELATION row with "method, defines"
    has_method_def = any("RELATION|" in l and "method, defines" in l for l in lines)
    assert has_method_def, "Missing RELATION|...|method, defines row"

    # CALL row
    has_call = any(l.startswith("CALL|") for l in lines)
    assert has_call, "Missing CALL row"

    # -- Verify bad.py produced output but is in coverage skipped -----------
    bad_flat = "mypkg__bad.py.txt"
    assert bad_flat in files, "Missing output for bad.py"
    # Its content should only have headers (no entities)
    bad_content = files[bad_flat]
    bad_lines = bad_content.strip().splitlines()
    assert bad_lines[0] == "# FORMAT: 2"

    # bad.py should be in coverage skipped
    cov_json = None
    for line in cov_lines:
        if line.startswith("COVERAGE|"):
            cov_json = json.loads(line.split("|", 2)[2])
    assert cov_json is not None
    skipped = cov_json.get("skipped", [])
    assert any("bad.py" in s for s in skipped), \
        f"bad.py not in coverage skipped: {skipped}"


def test_scan_no_pipes_in_descriptions() -> None:
    """Entity description columns must not contain pipe characters."""
    project_dir = _create_project()

    cp = _run_analyser(project_dir)
    assert cp.returncode == 0

    files = _read_output_files(project_dir)

    for fname, content in files.items():
        if fname == "_index.txt":
            # _index.txt uses a different format (not entity rows)
            continue
        for line in content.strip().splitlines():
            if line.startswith("#") or not line:
                continue
            if line.startswith("RELATION|") or line.startswith("CALL|") or \
                    line.startswith("COVERAGE|"):
                continue
            # Entity rows: split only on first 5 pipes to get 6 columns
            parts = line.split("|")
            assert len(parts) == 6, f"Pipe in description for {fname}: {line!r}"
            # The 6th column (description) must not contain a pipe
            desc = parts[5]
            assert "|" not in desc, f"Pipe in description: {fname} {line!r}"


def test_syntax_error_does_not_abort_scan() -> None:
    """A syntax-error file is reported in coverage skipped; other files
    still produce normal output."""
    project_dir = _create_project()

    cp = _run_analyser(project_dir)
    assert cp.returncode == 0, f"stderr: {cp.stderr}"

    files = _read_output_files(project_dir)

    # The good file should still have full output
    core_flat = "mypkg__core.py.txt"
    assert core_flat in files
    core_content = files[core_flat]
    assert "CLASS|MyClass|" in core_content
    assert "FUNCTION|helper|" in core_content

    # Coverage should list the bad file
    cov_content = files.get("_coverage.txt", "")
    assert "bad.py" in cov_content


def test_scan_default_output_dir() -> None:
    """Without --output, files go to <project>/.lightrag."""
    project_dir = _create_project()

    cp = _run_analyser(project_dir)
    assert cp.returncode == 0

    output_dir = os.path.join(project_dir, ".lightrag")
    assert os.path.isdir(output_dir), "Default .lightrag dir not created"
    assert os.path.isfile(os.path.join(output_dir, "_coverage.txt"))
    assert os.path.isfile(os.path.join(output_dir, "_index.txt"))


def test_scan_update_command() -> None:
    """Update command rewrites changed files and appends changelog rows."""
    project_dir = _create_project()

    # First, do a full scan
    cp = _run_analyser(project_dir)
    assert cp.returncode == 0

    # Now modify core.py to add a new top-level class
    core_path = Path(project_dir) / "mypkg" / "core.py"
    core_path.write_text(
        core_path.read_text() + '\n\nclass NewClass:\n    """A new class."""\n'
    )

    # Run update on just core.py
    core_abs = str(core_path.resolve())
    cp2 = subprocess.run(
        [sys.executable, str(ANALYSER), "update", "--dir", project_dir,
         "--changed", core_abs],
        capture_output=True, text=True,
    )
    assert cp2.returncode == 0, f"stderr: {cp2.stderr}"

    # Changelog should have been written
    output_dir = os.path.join(project_dir, ".lightrag")
    changelog_path = os.path.join(output_dir, "_changelog.txt")
    assert os.path.isfile(changelog_path), "Missing _changelog.txt after update"
    changelog = open(changelog_path, encoding="utf-8").read()
    assert "NewClass" in changelog, "NewClass not in changelog"
    assert "added" in changelog, "No 'added' action in changelog"


def test_scan_with_output_flag() -> None:
    """--output flag directs output to a custom directory."""
    project_dir = _create_project()
    custom_output = os.path.join(project_dir, "my_output")

    cp = _run_analyser(project_dir, "--output", custom_output)
    assert cp.returncode == 0
    assert os.path.isdir(custom_output)
    assert os.path.isfile(os.path.join(custom_output, "_coverage.txt"))

    # Default .lightrag should NOT have been created
    default_output = os.path.join(project_dir, ".lightrag")
    assert not os.path.isdir(default_output), \
        f"Default .lightrag created despite --output: {default_output}"


def test_entity_visibility_suffixes() -> None:
    """Every CLASS / FUNCTION / METHOD / PROPERTY / CONST / VAR description
    must end with ``[public]`` or ``[private]``.  Symbols whose last
    dot-separated name segment starts with ``_`` get ``[private]``; all
    others get ``[public]``.  IMPORT entities are excluded."""
    root = tempfile.mkdtemp()

    # -- Package layout --------------------------------------------------
    pkg = Path(root) / "mypkg"
    pkg.mkdir()
    (pkg / "__init__.py").write_text('')

    code = pkg / "mod.py"
    code.write_text(
        '"""Module."""\n'
        'import os\n'
        '\n'
        'PUBLIC_CONST = 1\n'
        '\n'
        'class PublicClass:\n'
        '    """Public class."""\n'
        '\n'
        '    def public_method(self) -> None:\n'
        '        """A public method."""\n'
        '\n'
        '    @property\n'
        '    def _private_prop(self) -> None:\n'
        '        """A private property."""\n'
        '\n'
        'class _PrivateClass:\n'
        '    """A private class."""\n'
        '\n'
        '    def method(self) -> None:\n'
        '        """Method inside private class."""\n'
        '\n'
        'def _private_helper() -> None:\n'
        '    """A private helper function."""\n'
        '\n'
        'def public_func() -> None:\n'
        '    """A public function."""\n'
    )

    cp = subprocess.run(
        [sys.executable, str(ANALYSER), "scan", "--dir", root],
        capture_output=True, text=True,
    )
    assert cp.returncode == 0, f"stderr: {cp.stderr}"

    output_dir = os.path.join(root, ".lightrag")
    flat = "mypkg__mod.py.txt"
    content = open(os.path.join(output_dir, flat), encoding="utf-8").read()
    lines = content.strip().splitlines()

    # Collect entity rows (skip headers, RELATION, CALL)
    symbol_types = {"CLASS", "FUNCTION", "METHOD", "PROPERTY", "CONST", "VAR"}
    entity_re = re.compile(
        r"^(CLASS|FUNCTION|METHOD|PROPERTY|CONST|VAR|IMPORT)\|"
    )

    public_seen: list[str] = []
    private_seen: list[str] = []

    for line in lines:
        if line.startswith("#") or not line:
            continue
        m = entity_re.match(line)
        if not m:
            continue
        etype = m.group(1)
        if etype not in symbol_types:
            continue
        parts = line.split("|")
        assert len(parts) == 6, f"Expected 6 columns: {line}"
        name = parts[1]
        desc = parts[5]
        last_segment = name.rsplit(".", 1)[-1]
        is_private = last_segment.startswith("_")
        suffix = "[private]" if is_private else "[public]"
        assert desc.endswith(suffix), (
            f"{etype} {name!r} desc {desc!r} should end with {suffix!r}"
        )
        if is_private:
            private_seen.append(name)
        else:
            public_seen.append(name)

    # Verify we got both public and private
    assert len(public_seen) > 0, "Expected at least one [public] entity"
    assert len(private_seen) > 0, "Expected at least one [private] entity"

    # Specific checks
    assert "public_func" in public_seen
    assert "_private_helper" in private_seen
    assert "PublicClass._private_prop" in private_seen
    assert "PublicClass" in public_seen
    assert "_PrivateClass" in private_seen
    assert "PublicClass.public_method" in public_seen
    # method inside _PrivateClass: last segment is "method" (no leading _)
    assert "_PrivateClass.method" in public_seen


if __name__ == "__main__":
    import pytest
    pytest.main([__file__, "-v"])
