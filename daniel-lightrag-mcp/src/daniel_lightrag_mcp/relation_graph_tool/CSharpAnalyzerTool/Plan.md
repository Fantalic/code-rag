# Modification Plan for CSharpAnalyzerTool

## Overview
This plan outlines updates to `Program.cs` to better align with the specified relation file format, incorporate configurable file extensions via `.env`, add ignore functionality via `.relignore`, and manage file sizes with splitting and enhanced linking. The tool analyzes C# solutions using Roslyn to extract relationships (CALLS, USES, CONTAINS, INHERITS_FROM, DEPENDS_ON, RETURNS) into modular `.txt` files (configurable).

## Key Changes

### 1. Adhere to Specified Format
- **Structure**:
  - `_index.txt` (per project/module): Start with `MODULE: {project.Name}`, list `FILE: {relative_path}` with `[LINK: {fileName}.{extension}]`, add module-level `DEPENDS_ON -> {other_module}` based on project references.
  - Per-file `{fileName}.{extension}`: `FILE: {relative_path}`, then `  CLASS: {name}` with `    CONTAINS -> {methods/properties}`, `    INHERITS_FROM -> {base}`, `    DEPENDS_ON -> {interfaces}`. For methods/properties: `    METHOD/PROPERTY: {name} -> RETURNS -> {type}`, `      METHOD: {name} -> CALLS -> {target} [LINK: {target_module}/{target_file}.{extension}]`, `      METHOD: {name} -> USES -> {dep} [REF: {dep_module}.{extension}]`.
  - End each file with `OUTBOUND REFERENCES: - [REF: {external}.{extension}]` for imports/metadata refs.
- **Relations**:
  - Add `CONTAINS` for classes under FILE.
  - Use consistent indentation: 2 spaces for FILE under MODULE, 4 for CLASS under FILE, 6 for METHOD under CLASS.
  - For external: `[REF: {namespace or assembly}.{extension}]`.
  - Relative paths: Use `Path.GetRelativePath(solutionDir, document.FilePath)` for FILE paths.
- **Module-Level**: Scan `solution.Projects` for inter-project deps, add to `_index.txt` as `DEPENDS_ON -> {otherProject.Name}`.

### 2. Configurable File Type (.env)
- Read `.env` from `solutionDir = Path.GetDirectoryName(solutionPath)`.
- Parse: `string extension = "txt"; if (File.Exists(envPath)) { var lines = File.ReadAllLines(envPath); foreach (var line in lines) { if (line.StartsWith("REL_FILE_TYPE=")) { extension = line.Split('=')[1]?.Trim() ?? "txt"; break; } } }`
- Use `extension` in all file paths: `{fileName}.{extension}`, links/refs: `[LINK: ...{extension}]`, `[REF: ...{extension}]`.
- Handle missing `.env`: Default to "txt".

### 3. Ignore Files/Directories (.relignore)
- Read `.relignore` from `solutionDir` (similar to .env).
- Parse lines as simple patterns (e.g., dirs like "bin/", files like "*.tmp", globs like "**/node_modules/*").
- Implement basic matcher: `bool IsIgnored(string relPath, string[] ignorePatterns) { foreach (var p in ignorePatterns) { if (SimpleGlobMatch(relPath, p)) return true; } return false; }`
- `SimpleGlobMatch`: Handle `*` as wildcard, `**` for recursive dirs (basic impl: split path, check segments).
- Before processing document: `string relPath = Path.GetRelativePath(solutionDir, document.FilePath ?? ""); if (IsIgnored(relPath, ignorePatterns)) continue;`
- Also ignore in project refs if needed.
- Skip if `.relignore` missing.

### 4. File Size Management & Enhanced Links
- Threshold: Configurable via `.env` (e.g., `REL_MAX_SIZE=10000` bytes; parse similarly, default 10000).
- Per-file: Collect all relations in memory (e.g., List<string> lines), then check `string content = string.Join("\n", lines); if (content.Length > maxSize) { split by class: create `{className}.{extension}` files, update _index.txt with [LINK: {className}.{extension}], add cross-links like `CONTAINS -> {otherClass} [LINK: {otherClass}.{extension}]` in split files. }`
- Links:
  - Internal CALLS/USES: Derive target module/file from symbol.ContainingNamespace/ContainingAssembly, use `[LINK: {targetModule}/{targetFile}.{extension}]`.
  - Unknown/external: `[REF: {symbol.ContainingAssembly.Name}.{extension}]`.
  - Ensure relative: From moduleDir perspective.
- For large modules: If > N files, consider sub-modules, but keep simple (per-project as is).

### 5. General Improvements
- **Error Handling**: Log skipped/ignored files to console. Catch Roslyn exceptions per document.
- **Relative Paths**: All FILE/LINK use relative to solutionDir.
- **Maintenance**: Overwrite files on re-run (no append). Consistent names: Use symbol.Name, no comments.
- **Dependencies**: No new NuGets; use existing Roslyn.
- **Verification**: After changes, suggest running tool on a sample solution. Add console output for generated files count.
- **Edge Cases**: Handle null symbols/trees gracefully. Skip non-C# projects. For constructors: Include if explicit, but skip as per current.

## Implementation Steps
1. Add .env and .relignore parsing at start (after solutionPath).
2. Load ignorePatterns.
3. In project loop: Compute relative paths, skip ignored.
4. Update writing logic: Collect lines in lists, add CONTAINS/INHERITS_FROM/DEPENDS_ON.
5. Enhance CALLS/USES with better link derivation (use symbol.ContainingType.ContainingNamespace.ToDisplayString() for module).
6. Add size check and splitting logic before writing.
7. Update _index.txt with module deps and file links.
8. Test: Run on self-project if possible (provide sln path).

This plan ensures the tool generates a modular, link-rich relation graph in configurable `.txt` format, ignoring specified patterns, while keeping files manageable.