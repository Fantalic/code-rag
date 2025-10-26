using System;
using System.Threading.Tasks;
using Microsoft.Build.Locator;
using Microsoft.CodeAnalysis.MSBuild;
using System.IO;

class Program
{
    static async Task Main(string[] args)
    {
        MSBuildLocator.RegisterDefaults();

        using (var workspace = MSBuildWorkspace.Create())
        {
            string solutionPath = @"C:\path\to\your\solution.sln";
            Console.WriteLine("Loading solution...");
            var solution = await workspace.OpenSolutionAsync(solutionPath);

            Console.WriteLine($"Solution loaded with {solution.Projects.Count} projects.");

            foreach (var project in solution.Projects)
            {
                string projectInfo = $"Project: {project.Name}, Files: {project.DocumentIds.Count}";
                Console.WriteLine(projectInfo);

                // Save basic project info to a text file
                string outputDir = @"C:\Path\To\Output";
                Directory.CreateDirectory(outputDir);

                string projectFile = Path.Combine(outputDir, $"{project.Name}_info.txt");
                await File.WriteAllTextAsync(projectFile, projectInfo);
            }
        }
    }
}
