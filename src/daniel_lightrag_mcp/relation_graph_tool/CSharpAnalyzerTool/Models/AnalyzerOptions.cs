namespace CSharpAnalyzerTool.Models;

public class AnalyzerOptions
{
    public string SolutionPath { get; set; } = string.Empty;
    public string OutputDir { get; set; } = string.Empty;
    public string FileExtension { get; set; } = "txt";
    public long MaxFileSize { get; set; } = 10000;
    public string[] IgnorePatterns { get; set; } = [];
}