"""
Unit tests for the C# analyser runner module.
"""

import pytest
import os
import re
from pathlib import Path
from unittest.mock import patch, MagicMock, mock_open

from daniel_lightrag_mcp.analyser_runner import (
    _should_skip,
    _filter_output_files,
    _read_output_files,
    _find_analyser_binary,
    _find_solution_file,
    run_analysis,
    SKIP_FILE_PATTERNS,
)


class TestSkipPatterns:
    """Test auto-generated file skip patterns."""

    def test_skip_index_txt(self):
        assert _should_skip("_index.txt") is True

    def test_skip_assembly_info(self):
        assert _should_skip("TestConsole_TestConsole.AssemblyInfo.txt") is True

    def test_skip_global_usings(self):
        assert _should_skip("TestConsole_TestConsole.GlobalUsings.g.txt") is True

    def test_skip_assembly_attributes(self):
        assert _should_skip("TestConsole_.NETCoreApp,Version=v7.0.AssemblyAttributes.txt") is True

    def test_keep_normal_output(self):
        assert _should_skip("TestLib_Calculator.txt") is False

    def test_keep_program_output(self):
        assert _should_skip("TestConsole_Program.txt") is False


class TestFilterOutputFiles:
    """Test filtering analyser output files."""

    def test_filter_empty_directory(self, tmp_path):
        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        result = _filter_output_files(output_dir)
        assert result == []

    def test_filter_only_noise_files(self, tmp_path):
        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        (output_dir / "_index.txt").write_text("index", encoding="utf-8")
        (output_dir / "AssemblyInfo.txt").write_text("info", encoding="utf-8")
        result = _filter_output_files(output_dir)
        assert result == []

    def test_filter_keeps_valid_files(self, tmp_path):
        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        (output_dir / "_index.txt").write_text("index", encoding="utf-8")
        (output_dir / "Calculator.txt").write_text("class Calc", encoding="utf-8")
        (output_dir / "Program.txt").write_text("class Program", encoding="utf-8")
        result = _filter_output_files(output_dir)
        assert len(result) == 2
        assert result[0].name == "Calculator.txt"
        assert result[1].name == "Program.txt"

    def test_filter_skips_non_txt_files(self, tmp_path):
        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        (output_dir / "Calculator.txt").write_text("content", encoding="utf-8")
        (output_dir / "notes.md").write_text("note", encoding="utf-8")
        result = _filter_output_files(output_dir)
        assert len(result) == 1
        assert result[0].name == "Calculator.txt"

    def test_filter_nonexistent_directory(self, tmp_path):
        output_dir = tmp_path / "nonexistent"
        result = _filter_output_files(output_dir)
        assert result == []


class TestReadOutputFiles:
    """Test reading analyser output files."""

    def test_read_files_returns_content(self, tmp_path):
        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        (output_dir / "Calc.txt").write_text("class Calc", encoding="utf-8")
        (output_dir / "Prog.txt").write_text("class Program", encoding="utf-8")
        results = _read_output_files(output_dir)
        assert len(results) == 2
        assert ("Calc.txt", "class Calc") in results
        assert ("Prog.txt", "class Program") in results

    def test_read_skips_noise_files(self, tmp_path):
        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        (output_dir / "Calc.txt").write_text("content", encoding="utf-8")
        (output_dir / "_index.txt").write_text("index", encoding="utf-8")
        results = _read_output_files(output_dir)
        assert len(results) == 1
        assert results[0][0] == "Calc.txt"

    def test_read_handles_read_error_gracefully(self, tmp_path):
        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        (output_dir / "Calc.txt").write_text("content", encoding="utf-8")
        bad_file = output_dir / "bad.txt"
        bad_file.write_text("bad", encoding="utf-8")
        with patch.object(Path, "read_text", side_effect=[PermissionError("denied"), "content"]):
            results = _read_output_files(output_dir / "nonexistent")
            assert results == []


class TestFindAnalyserBinary:
    """Test locating the C# analyser binary."""

    def test_finds_binary_when_exists(self, tmp_path):
        module_dir = tmp_path / "daniel_lightrag_mcp"
        module_dir.mkdir(parents=True)
        analyser_dir = tmp_path / "analysers" / "c_sharp"
        analyser_dir.mkdir(parents=True)
        binary = analyser_dir / "CSharpAnalyzerTool.exe"
        binary.write_text("fake binary")
        relative = Path("../analysers/c_sharp/CSharpAnalyzerTool.exe")
        with patch("daniel_lightrag_mcp.analyser_runner.__file__", str(module_dir / "__init__.py")):
            with patch("daniel_lightrag_mcp.analyser_runner.ANALYSER_RELATIVE_PATH", str(relative)):
                result = _find_analyser_binary()
                assert result.exists()

    def test_raises_when_binary_missing(self, tmp_path):
        module_dir = tmp_path / "daniel_lightrag_mcp"
        module_dir.mkdir(parents=True)
        with patch("daniel_lightrag_mcp.analyser_runner.__file__", str(module_dir / "__init__.py")):
            with pytest.raises(FileNotFoundError, match="C# analyser binary not found"):
                _find_analyser_binary()


class TestFindSolutionFile:
    """Test locating .sln files."""

    def test_finds_sln_file_direct(self, tmp_path):
        sln = tmp_path / "Test.sln"
        sln.write_text("solution", encoding="utf-8")
        result = _find_solution_file(str(sln))
        assert result == sln

    def test_finds_sln_in_directory(self, tmp_path):
        sln = tmp_path / "Test.sln"
        sln.write_text("solution", encoding="utf-8")
        result = _find_solution_file(str(tmp_path))
        assert result == sln

    def test_raises_when_no_sln(self, tmp_path):
        with pytest.raises(FileNotFoundError, match="No .sln file found"):
            _find_solution_file(str(tmp_path))

    def test_raises_when_path_nonexistent(self):
        with pytest.raises(FileNotFoundError, match="does not exist"):
            _find_solution_file("/nonexistent/path")

    def test_raises_for_csproj(self, tmp_path):
        csproj = tmp_path / "Test.csproj"
        csproj.write_text("project", encoding="utf-8")
        with pytest.raises(ValueError, match="requires a .sln file"):
            _find_solution_file(str(csproj))

    def test_selects_first_sln_when_multiple(self, tmp_path):
        sln1 = tmp_path / "A.sln"
        sln1.write_text("a", encoding="utf-8")
        sln2 = tmp_path / "B.sln"
        sln2.write_text("b", encoding="utf-8")
        result = _find_solution_file(str(tmp_path))
        assert result == sln1


class TestRunAnalysis:
    """Test the main run_analysis function."""

    @patch("daniel_lightrag_mcp.analyser_runner.subprocess.run")
    @patch("daniel_lightrag_mcp.analyser_runner._find_analyser_binary")
    def test_run_analysis_success(self, mock_find_binary, mock_subprocess_run, tmp_path):
        sln = tmp_path / "Test.sln"
        sln.write_text("solution", encoding="utf-8")
        mock_find_binary.return_value = Path("/fake/analyser.exe")
        mock_subprocess_run.return_value = MagicMock(returncode=0, stdout="Done", stderr="")

        output_dir = tmp_path / "relations_output"
        output_dir.mkdir()
        (output_dir / "Calc.txt").write_text("class Calc", encoding="utf-8")

        results = run_analysis(str(sln), output_dir=str(tmp_path))
        assert len(results) == 1
        assert results[0][0] == "Calc.txt"
        assert results[0][1] == "class Calc"

    @patch("daniel_lightrag_mcp.analyser_runner.subprocess.run")
    @patch("daniel_lightrag_mcp.analyser_runner._find_analyser_binary")
    def test_run_analysis_analyser_fails(self, mock_find_binary, mock_subprocess_run, tmp_path):
        sln = tmp_path / "Test.sln"
        sln.write_text("solution", encoding="utf-8")
        mock_find_binary.return_value = Path("/fake/analyser.exe")
        mock_subprocess_run.return_value = MagicMock(returncode=1, stdout="", stderr="error")

        with pytest.raises(RuntimeError, match="C# analyser exited with code 1"):
            run_analysis(str(sln), output_dir=str(tmp_path))

    @patch("daniel_lightrag_mcp.analyser_runner.subprocess.run")
    @patch("daniel_lightrag_mcp.analyser_runner._find_analyser_binary")
    def test_run_analysis_no_output_dir(self, mock_find_binary, mock_subprocess_run, tmp_path):
        sln = tmp_path / "Test.sln"
        sln.write_text("solution", encoding="utf-8")
        mock_find_binary.return_value = Path("/fake/analyser.exe")
        mock_subprocess_run.return_value = MagicMock(returncode=0, stdout="Done", stderr="")

        results = run_analysis(str(sln), output_dir=str(tmp_path))
        assert results == []

    @patch("daniel_lightrag_mcp.analyser_runner.subprocess.run")
    @patch("daniel_lightrag_mcp.analyser_runner._find_analyser_binary")
    def test_run_analysis_timeout(self, mock_find_binary, mock_subprocess_run, tmp_path):
        sln = tmp_path / "Test.sln"
        sln.write_text("solution", encoding="utf-8")
        mock_find_binary.return_value = Path("/fake/analyser.exe")
        mock_subprocess_run.side_effect = TimeoutError("timed out")

        with pytest.raises(TimeoutError, match="timed out"):
            run_analysis(str(sln), output_dir=str(tmp_path))

    @patch("daniel_lightrag_mcp.analyser_runner.subprocess.run")
    @patch("daniel_lightrag_mcp.analyser_runner._find_analyser_binary")
    def test_run_analysis_binary_not_found(self, mock_find_binary, mock_subprocess_run, tmp_path):
        sln = tmp_path / "Test.sln"
        sln.write_text("solution", encoding="utf-8")
        mock_find_binary.return_value = Path("/fake/analyser.exe")
        mock_subprocess_run.side_effect = FileNotFoundError("binary not found")

        with pytest.raises(FileNotFoundError, match="binary not found"):
            run_analysis(str(sln), output_dir=str(tmp_path))
