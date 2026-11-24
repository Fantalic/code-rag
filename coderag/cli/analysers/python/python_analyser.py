#!/usr/bin/env python3
"""AST-based Python analyser producing format-2 output compatible with the
existing Go pipeline (C#/Zig analysers).

Walks a project directory, parses every .py file with ast.parse, and writes
a flat .txt sidecar per file plus _coverage.txt, _index.txt, and optionally
_changelog.txt.

Output format (same as the C#/Zig analysers):
    # FORMAT: 2
    # EXPORTS: ClassA, func_b
    # MTIME: <unix>              # optional
    # IMPORTS: os, sys
    CLASS|ClassA|pkg/mod.py|10|20|description...
    FUNCTION|func_b|pkg/mod.py|5|8|description...
    RELATION|ClassA|method_x|...|method, defines|pkg/mod.py|1.0
    CALL|pkg/mod.py|ClassA.method_x|func_b|pkg.mod|15
"""

from __future__ import annotations

import ast
import json
import os
import sys
import time
import tokenize
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

SKIPPED_DIRS = {"__pycache__", ".git", ".venv", "venv", "site-packages",
                ".mypy_cache", ".pytest_cache", "build", "dist"}


def parse_cli(argv: list[str]) -> dict[str, Any]:
    """Parse command-line arguments (scan / update with --dir, --output, --changed)."""
    if len(argv) < 2:
        print(f"Usage: python_analyser.py <scan|update> [flags]", file=sys.stderr)
        return {"error": "no command"}

    command = argv[1]
    cfg: dict[str, Any] = {"command": command, "dir": ".", "output": "",
                           "changed": ""}

    i = 2
    while i < len(argv):
        if argv[i] == "--dir":
            cfg["dir"] = argv[i + 1] if i + 1 < len(argv) else "."
            i += 2
        elif argv[i] == "--output":
            cfg["output"] = argv[i + 1] if i + 1 < len(argv) else ""
            i += 2
        elif argv[i] == "--changed":
            cfg["changed"] = argv[i + 1] if i + 1 < len(argv) else ""
            i += 2
        else:
            i += 1

    return cfg


# ---------------------------------------------------------------------------
# File walking
# ---------------------------------------------------------------------------

def collect_py_files(project_dir: str) -> list[str]:
    """Recursively collect .py files, skipping cache directories. Returns
    a deterministically sorted list of absolute paths."""
    files: list[str] = []
    for root, dirs, filenames in os.walk(project_dir):
        # Skip ignored directories in-place
        dirs[:] = sorted(d for d in dirs if d not in SKIPPED_DIRS)
        for fname in sorted(filenames):
            if fname.endswith(".py"):
                files.append(os.path.join(root, fname))
    files.sort()
    return files


# ---------------------------------------------------------------------------
# AST helpers
# ---------------------------------------------------------------------------

def _first_docstring(node: ast.AST) -> str:
    """Return the first line of a docstring, or empty string.

    ast.get_docstring only works on Module/ClassDef/FunctionDef/AsyncFunctionDef;
    guard against other node types to avoid TypeError.
    """
    # Nodes that can have docstrings in Python
    docstring_types = (ast.Module, ast.ClassDef,
                       ast.FunctionDef, ast.AsyncFunctionDef)
    if not isinstance(node, docstring_types):
        return ""
    ds = ast.get_docstring(node)
    if ds:
        return ds.splitlines()[0].strip()
    return ""


def _sig_string(node: ast.FunctionDef | ast.AsyncFunctionDef) -> str:
    """Build a human-readable signature from a function AST node."""
    args = node.args
    parts: list[str] = []

    # Positional-only
    if args.posonlyargs:
        for a in args.posonlyargs:
            p = a.arg
            if a.annotation:
                p += ": " + ast.unparse(a.annotation)
            parts.append(p)
    # Regular positional
    for a in args.args:
        p = a.arg
        if a.annotation:
            p += ": " + ast.unparse(a.annotation)
        parts.append(p)
    # *args
    if args.vararg:
        p = "*" + args.vararg.arg
        if args.vararg.annotation:
            p += ": " + ast.unparse(args.vararg.annotation)
        parts.append(p)
    # Keyword-only
    for a in args.kwonlyargs:
        p = a.arg
        if a.annotation:
            p += ": " + ast.unparse(a.annotation)
        parts.append(p)
    # **kwargs
    if args.kwarg:
        p = "**" + args.kwarg.arg
        if args.kwarg.annotation:
            p += ": " + ast.unparse(args.kwarg.annotation)
        parts.append(p)

    ret = ""
    if node.returns:
        ret = " -> " + ast.unparse(node.returns)

    return "(" + ", ".join(parts) + ")" + ret


def _decorators(node: ast.FunctionDef | ast.AsyncFunctionDef) -> list[str]:
    """Return decorator names for a function node."""
    names: list[str] = []
    for dec in node.decorator_list:
        names.append(ast.unparse(dec))
    return names


def _base_classes(node: ast.ClassDef) -> list[str]:
    """Return base class names."""
    return [ast.unparse(b) for b in node.bases]


def _sanitize(text: str) -> str:
    """Remove pipe, carriage-return, and newline so row schemas stay valid."""
    text = text.replace("|", " ")
    text = text.replace("\r", " ")
    text = text.replace("\n", " ")
    return text.strip()


def _add_visibility_suffix(desc: str, entity: dict[str, Any]) -> str:
    """Append a visibility marker to the description when the entity is a
    symbol (not an IMPORT).

    The marker is ``[private]`` when the last dot-separated segment of the
    entity name starts with ``_``, otherwise ``[public]``.  IMPORT entities
    are returned unchanged.
    """
    if entity.get("type") == "IMPORT":
        return desc
    name = entity.get("name", "")
    last_segment = name.rsplit(".", 1)[-1]
    suffix = "[private]" if last_segment.startswith("_") else "[public]"
    return f"{desc} {suffix}"


def _module_path(rel_path: str) -> str:
    """Convert a relative .py path to a dotted module path.
    e.g. 'pkg/mod.py' -> 'pkg.mod', 'pkg/sub/mod.py' -> 'pkg.sub.mod'
    """
    parts = Path(rel_path).with_suffix("").parts
    return ".".join(parts)


# ---------------------------------------------------------------------------
# Global callable index (built before per-file analysis for CALL resolution)
# ---------------------------------------------------------------------------

class CallableInfo:
    """A callable that can be referenced by CALL rows."""

    def __init__(self, name: str, owner: str, file_path: str, line: int) -> None:
        self.name = name
        self.owner = owner      # e.g. 'ClassName' or '' for module-level
        self.file_path = file_path
        self.line = line


def build_global_index(py_files: list[str], project_dir: str) -> list[CallableInfo]:
    """Parse every file and collect all function/method names so CALL rows
    only reference known callees."""
    index: list[CallableInfo] = []
    for fpath in py_files:
        try:
            src = open(fpath, encoding="utf-8").read()
            tree = ast.parse(src, filename=fpath, type_comments=True)
        except SyntaxError:
            continue
        rel = os.path.relpath(fpath, project_dir)
        mod = _module_path(rel)

        # Module-level functions
        for node in ast.iter_child_nodes(tree):
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                index.append(CallableInfo(node.name, "", fpath, node.lineno))

        # Methods inside classes
        for node in ast.iter_child_nodes(tree):
            if isinstance(node, ast.ClassDef):
                cls_name = node.name
                for item in ast.iter_child_nodes(node):
                    if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
                        index.append(CallableInfo(item.name, cls_name, fpath,
                                                  item.lineno))
    return index


# ---------------------------------------------------------------------------
# Per-file analysis
# ---------------------------------------------------------------------------

class FileResult:
    """Holds the parsed entities, relations, and metadata for one file."""

    def __init__(self, file_path: str, rel_path: str, module_path: str) -> None:
        self.file_path = file_path
        self.rel_path = rel_path
        self.module_path = module_path
        self.entities: list[dict] = []
        self.relations: list[dict] = []
        self.calls: list[dict] = []
        self.exports: list[str] = []
        self.imports: set[str] = set()


def _parse_imports(tree: ast.Module) -> set[str]:
    """Collect imported module names (deduped)."""
    mods: set[str] = set()
    for node in ast.iter_child_nodes(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                mods.add(alias.name.split(".")[0])
        elif isinstance(node, ast.ImportFrom):
            if node.module:
                mods.add(node.module.split(".")[0])
    return mods


def _is_property_decorator(node: ast.FunctionDef) -> bool:
    """Check if a method is decorated with @property or @<cls>.setter / @<cls>.deleter."""
    for dec in node.decorator_list:
        unparsed = ast.unparse(dec)
        if unparsed == "property" or unparsed.endswith(".setter") or \
                unparsed.endswith(".deleter"):
            return True
    return False


def analyse_file(file_path: str, project_dir: str,
                 global_index: list[CallableInfo]) -> Tuple[FileResult, Optional[str]]:
    """Parse a single .py file and return (result, error_or_none).

    The global_index is used to resolve CALL targets so we never emit
    phantom UNKNOWN callee rows.
    """
    rel_path = os.path.relpath(file_path, project_dir)
    mod_path = _module_path(rel_path)
    result = FileResult(file_path, rel_path, mod_path)

    try:
        src = open(file_path, encoding="utf-8").read()
        tree = ast.parse(src, filename=file_path, type_comments=True)
    except SyntaxError as exc:
        return result, f"{rel_path}: {exc}"

    # --- Imports -----------------------------------------------------------
    result.imports = _parse_imports(tree)
    # Emit IMPORT entities for each imported module
    for mod_name in sorted(result.imports):
        result.entities.append({
            "type": "IMPORT", "name": mod_name, "file_path": rel_path,
            "start_line": 1, "end_line": 1,
            "description": _sanitize(f"IMPORT {mod_name} in {mod_path}"),
        })

    # --- Module-level entities ---------------------------------------------
    for node in ast.iter_child_nodes(tree):
        if isinstance(node, ast.ClassDef):
            bases = _base_classes(node)
            base_str = " (" + ", ".join(bases) + ")" if bases else ""
            ds = _first_docstring(node)
            desc = (f"CLASS {node.name}{base_str} in {mod_path}"
                    + (f": {ds}" if ds else ""))
            result.entities.append({
                "type": "CLASS", "name": node.name, "file_path": rel_path,
                "start_line": node.lineno, "end_line": getattr(node, "end_lineno", node.lineno),
                "description": _sanitize(desc),
            })
            result.exports.append(node.name)

        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            sig = _sig_string(node)
            decs = _decorators(node)
            ds = _first_docstring(node)
            parts = [f"FUNCTION {node.name}{sig} in {mod_path}"]
            if decs:
                parts.append(f"decorators: {', '.join(decs)}")
            if ds:
                parts.append(f"- {ds}")
            desc = "; ".join(parts)
            result.entities.append({
                "type": "FUNCTION", "name": node.name, "file_path": rel_path,
                "start_line": node.lineno, "end_line": getattr(node, "end_lineno", node.lineno),
                "description": _sanitize(desc),
            })
            result.exports.append(node.name)

        elif isinstance(node, ast.Assign):
            # Module-level assignments: CONST for UPPER_CASE targets
            for target in node.targets:
                if isinstance(target, ast.Name) and target.id.isupper():
                    ds = _first_docstring(node)
                    desc = (f"CONST {target.id} in {mod_path}"
                            + (f": {ds}" if ds else ""))
                    result.entities.append({
                        "type": "CONST", "name": target.id, "file_path": rel_path,
                        "start_line": node.lineno, "end_line": getattr(node, "end_lineno", node.lineno),
                        "description": _sanitize(desc),
                    })
                    result.exports.append(target.id)

        elif isinstance(node, ast.AnnAssign):
            # Annotated assignments: CONST for UPPER_CASE, VAR for others
            if isinstance(node.target, ast.Name):
                name = node.target.id
                if name.isupper() and not name.startswith("_"):
                    ds = _first_docstring(node)
                    desc = (f"CONST {name} in {mod_path}"
                            + (f": {ds}" if ds else ""))
                    result.entities.append({
                        "type": "CONST", "name": name, "file_path": rel_path,
                        "start_line": node.lineno, "end_line": getattr(node, "end_lineno", node.lineno),
                        "description": _sanitize(desc),
                    })
                    result.exports.append(name)
                elif not name.startswith("_"):
                    ann = ""
                    if node.annotation:
                        ann = ": " + ast.unparse(node.annotation)
                    ds = _first_docstring(node)
                    parts = [f"VAR {name}{ann} in {mod_path}"]
                    if ds:
                        parts.append(f"- {ds}")
                    desc = "; ".join(parts)
                    result.entities.append({
                        "type": "VAR", "name": name, "file_path": rel_path,
                        "start_line": node.lineno, "end_line": getattr(node, "end_lineno", node.lineno),
                        "description": _sanitize(desc),
                    })

    # --- Class-level entities & relations ----------------------------------
    for node in ast.iter_child_nodes(tree):
        if not isinstance(node, ast.ClassDef):
            continue
        cls_name = node.name
        for item in ast.iter_child_nodes(node):
            if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
                sig = _sig_string(item)
                decs = _decorators(item)
                ds = _first_docstring(item)
                is_prop = _is_property_decorator(item)
                etype = "PROPERTY" if is_prop else "METHOD"
                parts = [f"{etype} {cls_name}.{item.name}{sig} in {mod_path}"]
                if decs:
                    parts.append(f"decorators: {', '.join(decs)}")
                if ds:
                    parts.append(f"- {ds}")
                desc = "; ".join(parts)
                result.entities.append({
                    "type": etype, "name": f"{cls_name}.{item.name}",
                    "file_path": rel_path,
                    "start_line": item.lineno,
                    "end_line": getattr(item, "end_lineno", item.lineno),
                    "description": _sanitize(desc),
                })
                # Class -> method/property relation
                kw = "property, has" if is_prop else "method, defines"
                result.relations.append({
                    "source": cls_name, "target": f"{cls_name}.{item.name}",
                    "description": f"{cls_name} has {item.name}",
                    "keywords": kw, "file_path": rel_path, "weight": 1.0,
                })
            elif isinstance(item, ast.Assign):
                # Class-level assignments -> PROPERTY when single target
                if len(item.targets) == 1 and isinstance(item.targets[0], ast.Name):
                    attr_name = item.targets[0].id
                    result.entities.append({
                        "type": "PROPERTY", "name": f"{cls_name}.{attr_name}",
                        "file_path": rel_path,
                        "start_line": item.lineno,
                        "end_line": getattr(item, "end_lineno", item.lineno),
                        "description": _sanitize(
                            f"PROPERTY {cls_name}.{attr_name} in {mod_path}"),
                    })
                    result.relations.append({
                        "source": cls_name, "target": f"{cls_name}.{attr_name}",
                        "description": f"{cls_name} has {attr_name}",
                        "keywords": "property, has", "file_path": rel_path, "weight": 1.0,
                    })

        # Class -> base class relation
        for base in _base_classes(node):
            result.relations.append({
                "source": cls_name, "target": base,
                "description": f"{cls_name} inherits from {base}",
                "keywords": "inherits, extends", "file_path": rel_path, "weight": 1.0,
            })

    # --- CALL rows ---------------------------------------------------------
    # Build a lookup of callable names for resolution
    callee_map: dict[str, CallableInfo] = {}
    for ci in global_index:
        callee_map[ci.name] = ci

    # Walk all function/method bodies for call expressions
    for node in ast.iter_child_nodes(tree):
        owner = ""
        if isinstance(node, ast.ClassDef):
            for item in ast.iter_child_nodes(node):
                if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
                    _walk_calls(item, f"{node.name}.{item.name}",
                                rel_path, callee_map, result)
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            _walk_calls(node, node.name, rel_path, callee_map, result)

    return result, None


def _walk_calls(func_node: ast.FunctionDef | ast.AsyncFunctionDef,
                caller_name: str, caller_file: str,
                callee_map: dict[str, CallableInfo],
                result: FileResult) -> None:
    """Walk a function body and emit CALL rows for resolvable calls."""
    for node in ast.walk(func_node):
        if isinstance(node, ast.Call):
            # Try to resolve the callee name
            callee_name = ""
            if isinstance(node.func, ast.Name):
                callee_name = node.func.id
            elif isinstance(node.func, ast.Attribute):
                callee_name = node.func.attr

            if callee_name and callee_name in callee_map:
                ci = callee_map[callee_name]
                result.calls.append({
                    "caller_file": caller_file,
                    "caller_method": caller_name,
                    "callee_name": ci.name,
                    "callee_owner": ci.owner,
                    "line_no": node.lineno,
                })


# ---------------------------------------------------------------------------
# Output writer
# ---------------------------------------------------------------------------

def _flat_name(rel_path: str) -> str:
    """Convert a relative file path to a flat filename: replace / and \\
    with __, then append .txt.  e.g. 'pkg/mod.py' -> 'pkg__mod.py.txt'."""
    flat = rel_path.replace("\\", "__").replace("/", "__")
    return flat + ".txt"


def write_file_result(result: FileResult, output_dir: str) -> str:
    """Write one file's result to <output_dir>/<flat>.txt and return the path."""
    os.makedirs(output_dir, exist_ok=True)
    flat = _flat_name(result.rel_path)
    out_path = os.path.join(output_dir, flat)

    exports_sorted = sorted(result.exports)
    imports_sorted = sorted(result.imports)

    with open(out_path, "w", encoding="utf-8") as f:
        f.write("# FORMAT: 2\n")
        f.write("# EXPORTS: " + ", ".join(exports_sorted) + "\n")
        f.write("# IMPORTS: " + ", ".join(imports_sorted) + "\n")

        for ent in result.entities:
            end = ent["end_line"] if ent["end_line"] >= ent["start_line"] else ent["start_line"]
            desc = _add_visibility_suffix(ent["description"], ent)
            f.write(f'{ent["type"]}|{ent["name"]}|{ent["file_path"]}'
                    f'|{ent["start_line"]}|{end}|{desc}\n')

        for rel in result.relations:
            f.write(f'RELATION|{rel["source"]}|{rel["target"]}|{rel["description"]}'
                    f'|{rel["keywords"]}|{rel["file_path"]}|{rel["weight"]:.1f}\n')

        for call in result.calls:
            f.write(f'CALL|{call["caller_file"]}|{call["caller_method"]}'
                    f'|{call["callee_name"]}|{call["callee_owner"]}'
                    f'|{call["line_no"]}\n')

    return out_path


def _mtime(file_path: str) -> Optional[int]:
    """Return file mtime as unix seconds, or None."""
    try:
        return int(os.path.getmtime(file_path))
    except OSError:
        return None


def write_file_result_with_mtime(result: FileResult, output_dir: str) -> str:
    """Write file result with optional MTIME header."""
    os.makedirs(output_dir, exist_ok=True)
    flat = _flat_name(result.rel_path)
    out_path = os.path.join(output_dir, flat)

    exports_sorted = sorted(result.exports)
    imports_sorted = sorted(result.imports)

    mtime = _mtime(result.file_path)

    with open(out_path, "w", encoding="utf-8") as f:
        f.write("# FORMAT: 2\n")
        f.write("# EXPORTS: " + ", ".join(exports_sorted) + "\n")
        if mtime is not None:
            f.write(f"# MTIME: {mtime}\n")
        f.write("# IMPORTS: " + ", ".join(imports_sorted) + "\n")

        for ent in result.entities:
            end = ent["end_line"] if ent["end_line"] >= ent["start_line"] else ent["start_line"]
            desc = _add_visibility_suffix(ent["description"], ent)
            f.write(f'{ent["type"]}|{ent["name"]}|{ent["file_path"]}'
                    f'|{ent["start_line"]}|{end}|{desc}\n')

        for rel in result.relations:
            f.write(f'RELATION|{rel["source"]}|{rel["target"]}|{rel["description"]}'
                    f'|{rel["keywords"]}|{rel["file_path"]}|{rel["weight"]:.1f}\n')

        for call in result.calls:
            f.write(f'CALL|{call["caller_file"]}|{call["caller_method"]}'
                    f'|{call["callee_name"]}|{call["callee_owner"]}'
                    f'|{call["line_no"]}\n')

    return out_path


def write_coverage(output_dir: str, py_files: list[str],
                   skipped: list[str]) -> str:
    """Write _coverage.txt sidecar."""
    os.makedirs(output_dir, exist_ok=True)
    out_path = os.path.join(output_dir, "_coverage.txt")
    ts = int(time.time())
    skipped_json = json.dumps(skipped)
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(f"# COVERAGE REPORT\n")
        f.write(f'# Total Projects: 1\n')
        f.write(f'COVERAGE|[project]|{{"files":{len(py_files)}'
                f',"skipped":{skipped_json},"scanned_at":{ts},"warnings":"-"}}\n')
    return out_path


def write_index(output_dir: str, py_files: list[str],
                global_index: list[CallableInfo]) -> str:
    """Write _index.txt sidecar with GLOBAL INDEX sections."""
    os.makedirs(output_dir, exist_ok=True)
    out_path = os.path.join(output_dir, "_index.txt")

    functions: dict[str, list[dict]] = {}
    types: dict[str, dict] = {}

    for ci in global_index:
        if ci.owner:
            # Method
            key = f"{ci.owner}.{ci.name}"
        else:
            key = ci.name
        if key not in functions:
            functions[key] = []
        functions[key].append({
            "file_path": ci.file_path, "line": ci.line, "owner": ci.owner,
        })

    # Collect types (classes) from the index
    for ci in global_index:
        if ci.owner and not functions.get(ci.owner, []):
            # This is a class name used as owner
            pass
        if ci.owner and ci.name not in functions.get(ci.owner, []):
            pass

    with open(out_path, "w", encoding="utf-8") as f:
        f.write("# GLOBAL INDEX\n")
        f.write(f"# Total files: {len(py_files)}\n\n")
        f.write("# FUNCTIONS\n")
        for name in sorted(functions):
            for info in functions[name]:
                owner_str = f"|{info['owner']}" if info["owner"] else ""
                f.write(f"{name}|{info['file_path']}|{info['line']}{owner_str}\n")
        f.write("\n# TYPES\n")
        # Extract unique class types
        seen_types: dict[str, str] = {}
        for ci in global_index:
            if ci.owner and ci.owner not in seen_types:
                seen_types[ci.owner] = ci.owner
        for name in sorted(seen_types):
            f.write(f"{name}|{name}|0|CLASS\n")
        f.write("\n# FIELDS\n")
        f.write("# (no fields in format-2 Python output)\n")

    return out_path


def read_output_exports(output_path: str) -> list[str]:
    """Read EXPORTS header from a previously written .txt sidecar."""
    try:
        with open(output_path, "r", encoding="utf-8") as f:
            for line in f:
                stripped = line.strip()
                if stripped.startswith("# EXPORTS:"):
                    exports_str = stripped[len("# EXPORTS:"):]
                    return [e.strip() for e in exports_str.split(",") if e.strip()]
    except OSError:
        pass
    return []


def compute_diff(old_exports: list[str], new_exports: list[str]) -> dict[str, list[str]]:
    """Compute added/removed exports between two export lists."""
    old_set = set(old_exports)
    new_set = set(new_exports)
    return {
        "added": sorted(new_set - old_set),
        "removed": sorted(old_set - new_set),
    }


def write_changelog(output_dir: str, rel_path: str, diff: dict[str, list[str]]) -> str:
    """Append CHANGELOG rows for added/removed symbols."""
    if not diff.get("added") and not diff.get("removed"):
        return ""
    os.makedirs(output_dir, exist_ok=True)
    out_path = os.path.join(output_dir, "_changelog.txt")
    ts = int(time.time())
    with open(out_path, "a", encoding="utf-8") as f:
        for sym in diff["added"]:
            entry = json.dumps({"file": rel_path, "action": "added",
                                "symbol": sym, "changed_at": ts})
            f.write(f"CHANGELOG|{rel_path}|{entry}\n")
        for sym in diff["removed"]:
            entry = json.dumps({"file": rel_path, "action": "removed",
                                "symbol": sym, "changed_at": ts})
            f.write(f"CHANGELOG|{rel_path}|{entry}\n")
    return out_path


# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------

def run_scan(project_dir: str, output_dir: str) -> int:
    """Full scan: collect files, parse, write outputs."""
    py_files = collect_py_files(project_dir)
    if not py_files:
        print(f"warning: no .py files found in {project_dir}", file=sys.stderr)

    # Build global callable index first
    global_index = build_global_index(py_files, project_dir)

    skipped: list[str] = []
    written = 0

    for fpath in py_files:
        result, err = analyse_file(fpath, project_dir, global_index)
        if err:
            skipped.append(err)
            print(f"warning: {err}", file=sys.stderr)
            # Still write an empty output so the file appears in the output dir
            path = write_file_result_with_mtime(result, output_dir)
            print(f"wrote {path}", file=sys.stderr)
            written += 1
            continue

        path = write_file_result_with_mtime(result, output_dir)
        print(f"wrote {path}", file=sys.stderr)
        written += 1

    write_coverage(output_dir, py_files, skipped)
    print(f"wrote {os.path.join(output_dir, '_coverage.txt')}", file=sys.stderr)

    write_index(output_dir, py_files, global_index)
    print(f"wrote {os.path.join(output_dir, '_index.txt')}", file=sys.stderr)

    print(f"OK: scanned {written} files")
    return 0


def run_update(project_dir: str, output_dir: str,
               changed_str: str) -> int:
    """Incremental update: re-parse changed files, write diff to changelog."""
    changed_files: list[str] = []
    for p in changed_str.split(","):
        p = p.strip()
        if not p:
            continue
        if not os.path.isabs(p):
            p = os.path.abspath(p)
        changed_files.append(p)

    if not changed_files:
        print("error: no changed files specified (use --changed flag)", file=sys.stderr)
        return 1

    py_files = collect_py_files(project_dir)
    global_index = build_global_index(py_files, project_dir)

    for cf in changed_files:
        if not os.path.isfile(cf):
            print(f"warning: file not found: {cf}", file=sys.stderr)
            continue

        result, err = analyse_file(cf, project_dir, global_index)
        if err:
            print(f"warning: {err}", file=sys.stderr)
            continue

        rel_path = os.path.relpath(cf, project_dir)
        flat = _flat_name(rel_path)
        old_output_path = os.path.join(output_dir, flat)

        old_exports = read_output_exports(old_output_path)
        path = write_file_result_with_mtime(result, output_dir)
        print(f"wrote {path}", file=sys.stderr)

        diff = compute_diff(old_exports, result.exports)
        if diff["added"] or diff["removed"]:
            print(f"diff: {diff}", file=sys.stderr)
            write_changelog(output_dir, rel_path, diff)

    return 0


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> int:
    cfg = parse_cli(sys.argv)
    if "error" in cfg:
        return 1

    command = cfg["command"]
    project_dir = os.path.abspath(cfg["dir"])
    output_dir = cfg["output"]
    if not output_dir:
        output_dir = os.path.join(project_dir, ".lightrag")

    if command == "scan":
        return run_scan(project_dir, output_dir)
    elif command == "update":
        return run_update(project_dir, output_dir, cfg["changed"])
    else:
        print(f"error: unknown command: {command} (use scan or update)",
              file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
