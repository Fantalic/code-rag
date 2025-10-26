using System;
using System.IO;
using System.Linq;
using CSharpAnalyzerTool.Models;
using Ignore;

namespace CSharpAnalyzerTool.Services;

public class ConfigurationService
{
    private static string EscapeBackslashes(string pattern)
    {
        // Escape backslashes that are not already escaped
        return System.Text.RegularExpressions.Regex.Replace(pattern, @"(?<!\\)\\(?!\\)", @"\\");
    }

    public static AnalyzerOptions LoadConfiguration(string solutionPath)
    {
        var options = new AnalyzerOptions
        {
            SolutionPath = solutionPath,
            OutputDir = Path.Combine(Directory.GetCurrentDirectory(), "relations_output")
        };

        var solutionDir = Path.GetDirectoryName(solutionPath);
        if (string.IsNullOrEmpty(solutionDir))
            return options;

        // Load .env configuration
        var envPath = Path.Combine(solutionDir, ".env");
        if (File.Exists(envPath))
        {
            var lines = File.ReadAllLines(envPath);
            foreach (var line in lines)
            {
                if (line.StartsWith("REL_FILE_TYPE="))
                {
                    options.FileExtension = line.Split('=')[1]?.Trim() ?? "txt";
                }
                else if (line.StartsWith("REL_MAX_SIZE="))
                {
                    if (long.TryParse(line.Split('=')[1]?.Trim() ?? "10000", out var maxSize))
                    {
                        options.MaxFileSize = maxSize;
                    }
                }
            }
        }

// Load .relignore patterns
        var ignorePath = Path.Combine(solutionDir, ".relignore");
        if (File.Exists(ignorePath))
        {
            Console.WriteLine($"Found .relignore file at: {ignorePath}");
            var ignoreLines = File.ReadAllLines(ignorePath)
                .Where(line => !string.IsNullOrWhiteSpace(line) && !line.StartsWith("#"))
                .Select(line => line.Trim().Replace('\\', '/')) // Normalize path separators
                .Select(line => 
                {
                    // Convert simple case-insensitive patterns like "test_*.cs" to proper regex
                    if (line.StartsWith("test_", StringComparison.OrdinalIgnoreCase) && line.EndsWith(".cs"))
                    {
                        // Convert to case-insensitive pattern
                        return "[Tt][Ee][Ss][Tt]_" + line.Substring(5);
                    }
                    // If the line looks like a directory name (no wildcards, no slashes at end), 
                    // treat it as a directory pattern by adding a trailing slash
                    else if (!line.Contains("*") && !line.EndsWith("/") && !line.Contains(".") && !line.Contains("["))
                    {
                        return line + "/";
                    }
                    return line;
                })
                .Select(EscapeBackslashes)
                .ToArray();
            options.IgnorePatterns = ignoreLines;
            Console.WriteLine($"Loaded {ignoreLines.Length} ignore patterns from .relignore file");
        }
        else
        {
            Console.WriteLine("No .relignore file found, using default settings");
        }

        return options;
    }

    public static bool IsIgnored(string relativePath, string[] ignorePatterns)
    {
        try
        {
            // Normalize path separators
            var normalizedPath = relativePath.Replace('\\', '/');
            
            // Debug logging
            // Console.WriteLine($"Checking if '{normalizedPath}' is ignored by patterns: [{string.Join(", ", ignorePatterns)}]");
            
            // Use the Ignore library to check if the path should be ignored
            var ignore = new Ignore.Ignore();
            ignore.Add(ignorePatterns);
            var isIgnored = ignore.IsIgnored(normalizedPath);
            
            // Debug logging - only show when a path is ignored
            if (isIgnored)
            {
                Console.WriteLine($"  -> '{normalizedPath}' is ignored by .relignore patterns");
            }
            
            return isIgnored;
        }
        catch (System.Text.RegularExpressions.RegexParseException ex)
        {
            Console.WriteLine($"Warning: Error parsing ignore pattern: {ex.Message}");
            Console.WriteLine("Falling back to simple string matching for this pattern");
            
            // Fallback to simple string matching if regex parsing fails
            foreach (var pattern in ignorePatterns)
            {
                var cleanPattern = pattern.Split('#')[0].Trim(); // Remove comments
                if (string.IsNullOrEmpty(cleanPattern)) continue;
                
                // Simple string matching - check if the relative path contains the pattern
                if (relativePath.Contains(cleanPattern))
                    return true;
            }
            return false;
        }
    }
}