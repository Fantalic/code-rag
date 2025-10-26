using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using CSharpAnalyzerTool.Models;

namespace CSharpAnalyzerTool.Services;

public class FileWriterService
{
    private readonly AnalyzerOptions _options;

    public FileWriterService(AnalyzerOptions options)
    {
        _options = options;
    }

    public void WriteModuleIndex(string moduleDir, string moduleName, List<string> entries, List<string> moduleDependencies)
    {
        var indexPath = Path.Combine(moduleDir, $"_index.{_options.FileExtension}");
        
        // Ensure directory exists
        Directory.CreateDirectory(Path.GetDirectoryName(indexPath)!);

        using var writer = new StreamWriter(indexPath, false, Encoding.UTF8);
        writer.WriteLine(RelationFormatter.FormatModuleHeader(moduleName));
        writer.WriteLine();

        // Write file entries
        foreach (var entry in entries)
        {
            writer.WriteLine(entry);
        }

        // Write module dependencies
        if (moduleDependencies.Any())
        {
            writer.WriteLine();
            foreach (var dep in moduleDependencies)
            {
                writer.WriteLine(RelationFormatter.FormatModuleDependency(dep));
            }
        }
    }

    public void WriteFileRelations(string filePath, List<string> lines)
    {
        // Ensure directory exists
        Directory.CreateDirectory(Path.GetDirectoryName(filePath)!);

        // Check if file needs to be split
        var content = string.Join(Environment.NewLine, lines);
        if (content.Length > _options.MaxFileSize)
        {
            // For now, we'll write the full content and handle splitting later
            // In a full implementation, we would split by classes
        }

        using var writer = new StreamWriter(filePath, false, Encoding.UTF8);
        foreach (var line in lines)
        {
            writer.WriteLine(line);
        }
    }

    public void WriteClassRelations(string classFilePath, List<string> lines)
    {
        // Ensure directory exists
        Directory.CreateDirectory(Path.GetDirectoryName(classFilePath)!);

        using var writer = new StreamWriter(classFilePath, false, Encoding.UTF8);
        foreach (var line in lines)
        {
            writer.WriteLine(line);
        }
    }
}