using System;
using System.IO;
using System.Linq;
using CSharpAnalyzerTool.Models;

namespace CSharpAnalyzerTool.Services;

public class ConfigurationService
{
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
            options.IgnorePatterns = File.ReadAllLines(ignorePath)
                .Where(line => !string.IsNullOrWhiteSpace(line) && !line.StartsWith("#"))
                .Select(line => line.Trim())
                .ToArray();
        }

        return options;
    }

    public static bool IsIgnored(string relativePath, string[] ignorePatterns)
    {
        foreach (var pattern in ignorePatterns)
        {
            if (SimpleGlobMatch(relativePath, pattern))
                return true;
        }
        return false;
    }

    private static bool SimpleGlobMatch(string path, string pattern)
    {
        // Handle simple glob patterns
        if (pattern == "**")
            return true;

        // Split path and pattern into segments
        var pathSegments = path.Split(new[] { '/', '\\' }, StringSplitOptions.RemoveEmptyEntries);
        var patternSegments = pattern.Split(new[] { '/', '\\' }, StringSplitOptions.RemoveEmptyEntries);

        return MatchSegments(pathSegments, patternSegments, 0, 0);
    }

    private static bool MatchSegments(string[] pathSegments, string[] patternSegments, int pathIndex, int patternIndex)
    {
        // If we've matched all pattern segments, we're done
        if (patternIndex >= patternSegments.Length)
            return pathIndex >= pathSegments.Length;

        // If we've run out of path segments but have pattern segments left
        if (pathIndex >= pathSegments.Length)
        {
            // Only match if all remaining pattern segments are "*"
            for (int i = patternIndex; i < patternSegments.Length; i++)
            {
                if (patternSegments[i] != "*" && patternSegments[i] != "**")
                    return false;
            }
            return true;
        }

        var patternSegment = patternSegments[patternIndex];

        // Handle "**" (matches any number of path segments)
        if (patternSegment == "**")
        {
            // Try matching the rest of the pattern at this position or any subsequent position
            for (int i = pathIndex; i <= pathSegments.Length; i++)
            {
                if (MatchSegments(pathSegments, patternSegments, i, patternIndex + 1))
                    return true;
            }
            return false;
        }

        // Handle "*" (matches a single path segment)
        if (patternSegment == "*")
        {
            return MatchSegments(pathSegments, patternSegments, pathIndex + 1, patternIndex + 1);
        }

        // Handle exact match
        if (string.Equals(pathSegments[pathIndex], patternSegment, StringComparison.OrdinalIgnoreCase))
        {
            return MatchSegments(pathSegments, patternSegments, pathIndex + 1, patternIndex + 1);
        }

        return false;
    }
}