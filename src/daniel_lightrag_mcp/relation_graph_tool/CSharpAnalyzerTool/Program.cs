using Microsoft.Build.Locator;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.MSBuild;
using CSharpAnalyzerTool.Models;
using CSharpAnalyzerTool.Services;
using System.Text;

class Program
{
    static async Task<int> Main(string[] args)
    {
        if (args.Length == 0)
        {
            Console.WriteLine("Usage: CSharpAnalyzerTool <path-to-solution.sln> [output-directory]");
            return 1;
        }

        string solutionPath = args[0];
        
        if (!File.Exists(solutionPath))
        {
            Console.WriteLine($"Error: Solution file not found: {solutionPath}");
            return 1;
        }

        // Load configuration
        var options = ConfigurationService.LoadConfiguration(solutionPath);
        if (args.Length > 1)
        {
            options.OutputDir = args[1];
        }
        
        Directory.CreateDirectory(options.OutputDir);

        var instance = MSBuildLocator.QueryVisualStudioInstances().FirstOrDefault();
        if (instance == null)
        {
            Console.WriteLine("No Visual Studio/MSBuild instances found.");
            return 1;
        }
        MSBuildLocator.RegisterInstance(instance);

        using var workspace = MSBuildWorkspace.Create();
        var solution = await workspace.OpenSolutionAsync(solutionPath);
        Console.WriteLine($"Loaded {solution.Projects.Count()} projects.");

        var fileWriterService = new FileWriterService(options);

// Create a single list for all file entries across all projects
        var allFileEntries = new List<string>();
        var solutionDir = Path.GetDirectoryName(solutionPath) ?? Directory.GetCurrentDirectory();
        var totalIgnoredFilesCount = 0;
        
        foreach (var project in solution.Projects)
        {
            if (project.Language != LanguageNames.CSharp)
                continue;

            Console.WriteLine($"Analyzing project: {project.Name}");
            var compilation = await project.GetCompilationAsync();
            if (compilation == null) continue;

            // Process documents directly in output directory, no subdirectories
            var ignoredFilesCount = 0;
            foreach (var document in project.Documents)
            {
                var relativePath = Path.GetRelativePath(solutionDir, document.FilePath ?? "").Replace('\\', '/');
                // Debug logging
                // Console.WriteLine($"Checking file: {relativePath}");
                if (ConfigurationService.IsIgnored(relativePath, options.IgnorePatterns))
                {
                    Console.WriteLine($"  -> Ignoring {relativePath}");
                    ignoredFilesCount++;
                    totalIgnoredFilesCount++;
                    continue;
                }

                if (document.SourceCodeKind != SourceCodeKind.Regular)
                    continue;

                // Create unique filename by combining project name and file name to avoid conflicts
                var fileName = Path.GetFileNameWithoutExtension(document.FilePath ?? "unknown");
                var uniqueFileName = $"{project.Name}_{fileName}";
                var relFilePath = Path.Combine(options.OutputDir, $"{uniqueFileName}.{options.FileExtension}");

                // Add to global index
                var relativeDocumentPath = Path.GetRelativePath(solutionDir, document.FilePath ?? "");
                allFileEntries.Add(RelationFormatter.FormatFileEntry(relativeDocumentPath, uniqueFileName, options.FileExtension));

                var lines = new List<string>();
                lines.Add($"FILE: {relativeDocumentPath}");
                lines.Add("");

                var tree = await document.GetSyntaxTreeAsync();
                if (tree == null) continue;
                var root = await tree.GetRootAsync();
                var model = compilation.GetSemanticModel(tree);

                var typeDecls = root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>();
                foreach (var typeDecl in typeDecls)
                {
                    var symbol = model.GetDeclaredSymbol(typeDecl) as INamedTypeSymbol;
                    if (symbol == null) continue;

                    lines.Add(RelationFormatter.FormatClassEntry(symbol.Name));

                    // Focus on usage rather than inheritance/package relationships
                    // Only include inheritance if it's within the same solution
                    if (symbol.BaseType != null && symbol.BaseType.Name != "Object")
                    {
                        // Check if base type is from the same solution
                        var baseTypeProject = solution.Projects.FirstOrDefault(p => 
                            p.AssemblyName == symbol.BaseType.ContainingAssembly?.Name);
                        if (baseTypeProject != null)
                        {
                            lines.Add(RelationFormatter.FormatInheritsFrom(symbol.BaseType.Name));
                        }
                    }

                    // Focus on interfaces from the same solution
                    foreach (var iface in symbol.Interfaces)
                    {
                        var ifaceProject = solution.Projects.FirstOrDefault(p => 
                            p.AssemblyName == iface.ContainingAssembly?.Name);
                        if (ifaceProject != null)
                        {
                            lines.Add(RelationFormatter.FormatDependsOn(iface.Name));
                        }
                    }

                    // Properties - focus on usage within solution
                    foreach (var prop in symbol.GetMembers().OfType<IPropertySymbol>())
                    {
                        lines.Add(RelationFormatter.FormatPropertyEntry(prop.Name, prop.Type.ToDisplayString()));
                        foreach (var dep in ExtractTypeDependencies(prop.Type))
                        {
                            // Only track dependencies on types within the same solution
                            var depProject = solution.Projects.FirstOrDefault(p => 
                                p.AssemblyName == GetAssemblyNameFromType(dep));
                            if (depProject != null)
                            {
                                lines.Add(RelationFormatter.FormatPropertyUses(prop.Name, dep, $"{depProject.Name}_{dep}.{options.FileExtension}"));
                            }
                        }
                    }

                    // Methods - focus on usage rather than package relationships
                    foreach (var method in symbol.GetMembers().OfType<IMethodSymbol>())
                    {
                        if (method.MethodKind == MethodKind.Constructor || method.DeclaringSyntaxReferences.Length == 0)
                            continue;

                        lines.Add(RelationFormatter.FormatMethodEntry(method.Name, method.ReturnType.ToDisplayString()));

                        var syntaxRef = method.DeclaringSyntaxReferences.FirstOrDefault();
                        if (syntaxRef == null)
                            continue;

                        var syntax = syntaxRef.GetSyntax() as MethodDeclarationSyntax;
                        if (syntax == null)
                            continue;

                        var syntaxTree = syntax.SyntaxTree;
                        var localModel = syntaxTree == tree ? model : compilation.GetSemanticModel(syntaxTree);

                        // Focus on method calls within the same solution
                        foreach (var inv in syntax.DescendantNodes().OfType<InvocationExpressionSyntax>())
                        {
                            try
                            {
                                var symbolInfo = localModel.GetSymbolInfo(inv);
                                var target = symbolInfo.Symbol as IMethodSymbol ?? symbolInfo.CandidateSymbols.FirstOrDefault() as IMethodSymbol;
                                if (target != null)
                                {
                                    // Only process calls to methods within the same solution
                                    var targetProject = solution.Projects.FirstOrDefault(p => 
                                        p.AssemblyName == target.ContainingAssembly?.Name);
                                    if (targetProject != null)
                                    {
                                        var callee = target.ToDisplayString(SymbolDisplayFormat.MinimallyQualifiedFormat);
                                        var targetFile = target.ContainingType?.Name ?? callee.Split('.').FirstOrDefault() ?? "unknown";
                                        var uniqueTargetFileName = $"{targetProject.Name}_{targetFile}";
                                        lines.Add(RelationFormatter.FormatMethodCall(method.Name, callee, $"{uniqueTargetFileName}.{options.FileExtension}"));
                                    }
                                }
                            }
                            catch { }
                        }

                        // Method parameters - focus on usage within solution
                        foreach (var param in method.Parameters)
                        {
                            foreach (var dep in ExtractTypeDependencies(param.Type))
                            {
                                // Only track dependencies on types within the same solution
                                var depProject = solution.Projects.FirstOrDefault(p => 
                                    p.AssemblyName == GetAssemblyNameFromType(dep));
                                if (depProject != null)
                                {
                                    lines.Add(RelationFormatter.FormatMethodUses(method.Name, dep, $"{depProject.Name}_{dep}.{options.FileExtension}"));
                                }
                            }
                        }
                    }
                }

                // Skip outbound references to external packages
                lines.Add("");
                lines.Add(RelationFormatter.FormatOutboundReferencesHeader());
                // Only include references to projects within the same solution
                foreach (var projectReference in project.ProjectReferences)
                {
                    var referencedProject = solution.GetProject(projectReference.ProjectId);
                    if (referencedProject != null)
                    {
                        lines.Add(RelationFormatter.FormatReference($"{referencedProject.Name}.{options.FileExtension}"));
                    }
                }

                fileWriterService.WriteFileRelations(relFilePath, lines);
            }
        }

        // Write a single global index file instead of per-module indexes
        WriteGlobalIndex(options.OutputDir, solution.Projects.Select(p => p.Name).ToList(), allFileEntries);
        Console.WriteLine($"  -> Saved all relation files in {options.OutputDir}");
        Console.WriteLine($"  -> Ignored {totalIgnoredFilesCount} files based on .relignore patterns");

        Console.WriteLine("All relation files generated successfully.");
        return 0;
    }

    static IEnumerable<string> ExtractTypeDependencies(ITypeSymbol typeSymbol)
    {
        var results = new HashSet<string>();
        void collect(ITypeSymbol t)
        {
            if (t == null) return;
            if (!string.IsNullOrEmpty(t.Name))
                results.Add(t.ToDisplayString(SymbolDisplayFormat.MinimallyQualifiedFormat));
            if (t is INamedTypeSymbol named)
                foreach (var arg in named.TypeArguments) collect(arg);
        }
        collect(typeSymbol);
        return results;
    }

    static string GetAssemblyNameFromType(string typeName)
    {
        // Extract assembly name from fully qualified type name
        // This is a simplified approach - in practice, you might need more sophisticated parsing
        var parts = typeName.Split(',');
        if (parts.Length > 0)
        {
            var nameParts = parts[0].Split('.');
            return nameParts.Length > 0 ? nameParts[0] : "unknown";
        }
        return "unknown";
    }

    static void WriteGlobalIndex(string outputDir, List<string> projectNames, List<string> fileEntries)
    {
        var indexPath = Path.Combine(outputDir, "_index.txt");
        
        using var writer = new StreamWriter(indexPath, false, new UTF8Encoding(true)); // UTF-8 with BOM
        writer.WriteLine("GLOBAL INDEX");
        writer.WriteLine();
        writer.WriteLine("PROJECTS:");
        foreach (var projectName in projectNames)
        {
            writer.WriteLine($"  - {projectName}");
        }
        writer.WriteLine();
        
        writer.WriteLine("FILES:");
        foreach (var entry in fileEntries)
        {
            writer.WriteLine(entry);
        }
    }
}
