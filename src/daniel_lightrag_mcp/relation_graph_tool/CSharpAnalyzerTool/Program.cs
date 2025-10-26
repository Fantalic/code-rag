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

        foreach (var project in solution.Projects)
        {
            if (project.Language != LanguageNames.CSharp)
                continue;

            Console.WriteLine($"Analyzing project: {project.Name}");
            var compilation = await project.GetCompilationAsync();
            if (compilation == null) continue;

            string moduleDir = Path.Combine(options.OutputDir, Path.GetFileNameWithoutExtension(project.FilePath) ?? project.Name);
            Directory.CreateDirectory(moduleDir);

            var fileEntries = new List<string>();
            var moduleDependencies = new List<string>();

            // Collect project dependencies
            foreach (var projectReference in project.ProjectReferences)
            {
                var referencedProject = solution.GetProject(projectReference.ProjectId);
                if (referencedProject != null)
                {
                    moduleDependencies.Add(referencedProject.Name);
                }
            }

            var solutionDir = Path.GetDirectoryName(solutionPath) ?? Directory.GetCurrentDirectory();
            
            foreach (var document in project.Documents)
            {
                var relativePath = Path.GetRelativePath(solutionDir, document.FilePath ?? "");
                if (ConfigurationService.IsIgnored(relativePath, options.IgnorePatterns))
                {
                    Console.WriteLine($"  -> Ignoring {relativePath}");
                    continue;
                }

                if (document.SourceCodeKind != SourceCodeKind.Regular)
                    continue;

                var fileName = Path.GetFileNameWithoutExtension(document.FilePath ?? "unknown");
                var relFilePath = Path.Combine(moduleDir, $"{fileName}.{options.FileExtension}");
                
                // Add to index
                var relativeDocumentPath = Path.GetRelativePath(solutionDir, document.FilePath ?? "");
                fileEntries.Add(RelationFormatter.FormatFileEntry(relativeDocumentPath, fileName, options.FileExtension));

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

                    if (symbol.BaseType != null && symbol.BaseType.Name != "Object")
                        lines.Add(RelationFormatter.FormatInheritsFrom(symbol.BaseType.ToDisplayString()));

                    foreach (var iface in symbol.Interfaces)
                        lines.Add(RelationFormatter.FormatDependsOn(iface.ToDisplayString()));

                    // Properties
                    foreach (var prop in symbol.GetMembers().OfType<IPropertySymbol>())
                    {
                        lines.Add(RelationFormatter.FormatPropertyEntry(prop.Name, prop.Type.ToDisplayString()));
                        foreach (var dep in ExtractTypeDependencies(prop.Type))
                        {
                            // For now, we'll use a simple reference format
                            lines.Add(RelationFormatter.FormatPropertyUses(prop.Name, dep, $"{dep}.{options.FileExtension}"));
                        }
                    }

                    // Methods
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

                        foreach (var inv in syntax.DescendantNodes().OfType<InvocationExpressionSyntax>())
                        {
                            try
                            {
                                var symbolInfo = localModel.GetSymbolInfo(inv);
                                var target = symbolInfo.Symbol as IMethodSymbol ?? symbolInfo.CandidateSymbols.FirstOrDefault() as IMethodSymbol;
                                if (target != null)
                                {
                                    var callee = target.ToDisplayString(SymbolDisplayFormat.MinimallyQualifiedFormat);
                                    var targetModule = target.ContainingAssembly?.Name ?? "unknown";
                                    var targetFile = target.ContainingType?.Name ?? callee.Split('.').FirstOrDefault() ?? "unknown";
                                    lines.Add(RelationFormatter.FormatMethodCall(method.Name, callee, $"{targetModule}/{targetFile}.{options.FileExtension}"));
                                }
                            }
                            catch { }
                        }

                        foreach (var param in method.Parameters)
                        {
                            foreach (var dep in ExtractTypeDependencies(param.Type))
                            {
                                // For now, we'll use a simple reference format
                                lines.Add(RelationFormatter.FormatMethodUses(method.Name, dep, $"{dep}.{options.FileExtension}"));
                            }
                        }
                    }
                }

                lines.Add("");
                lines.Add(RelationFormatter.FormatOutboundReferencesHeader());
                foreach (var reference in project.MetadataReferences)
                {
                    var name = Path.GetFileNameWithoutExtension(reference.Display ?? "<unknown>");
                    lines.Add(RelationFormatter.FormatReference($"{name}.{options.FileExtension}"));
                }

                fileWriterService.WriteFileRelations(relFilePath, lines);
            }

            // Write module index
            fileWriterService.WriteModuleIndex(moduleDir, project.Name, fileEntries, moduleDependencies);
            Console.WriteLine($"  -> Saved module relations in {moduleDir}");
        }

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
}
