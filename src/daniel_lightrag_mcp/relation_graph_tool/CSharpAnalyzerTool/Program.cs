using System;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.Build.Locator;
using Microsoft.CodeAnalysis.MSBuild;

class Program
{
    static async Task Main(string[] args)
    {
        // Validate arguments
        if (args.Length == 0)
        {
            Console.WriteLine("Usage: CSharpAnalyzerTool <path-to-solution.sln>");
            return;
        }

        string solutionPath = args[0];

        if (!File.Exists(solutionPath))
        {
            Console.WriteLine($"Error: The specified solution file does not exist:\n{solutionPath}");
            return;
        }

        // Register MSBuild
        var instance = MSBuildLocator.QueryVisualStudioInstances().First();
        MSBuildLocator.RegisterInstance(instance);

        using (var workspace = MSBuildWorkspace.Create())
        {
            Console.WriteLine($"Loading solution from {solutionPath} ...");

            try
            {
                var solution = await workspace.OpenSolutionAsync(solutionPath);

                Console.WriteLine($"Solution loaded with {solution.Projects.Count()} projects:");
                foreach (var project in solution.Projects)
                {
                    Console.WriteLine($" - Project: {project.Name}, Language: {project.Language}, Documents: {project.DocumentIds.Count}");
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Failed to load solution: {ex.Message}");
            }
        }

        Console.WriteLine("Analysis complete. Press any key to exit.");
        Console.ReadKey();
    }
}
