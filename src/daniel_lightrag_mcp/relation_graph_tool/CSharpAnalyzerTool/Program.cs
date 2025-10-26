// CSharpAnalyzerTool: Relation-based .rel file generator
// Usage: CSharpAnalyzerTool.exe <path-to-solution.sln> [output-directory]

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.Build.Locator;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.MSBuild;
using Microsoft.CodeAnalysis.CSharp.Syntax;

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
        string outputDir = args.Length > 1 ? args[1] : Path.Combine(Directory.GetCurrentDirectory(), "relations_output");

        if (!File.Exists(solutionPath))
        {
            Console.WriteLine($"Error: Solution file not found: {solutionPath}");
            return 1;
        }

        Directory.CreateDirectory(outputDir);

        var instance = MSBuildLocator.QueryVisualStudioInstances().FirstOrDefault();
        if (instance == null)
        {
            Console.WriteLine("No Visual Studio/MSBuild instances found.");
            return 1;
        }
        MSBuildLocator.RegisterInstance(instance);

        using (var workspace = MSBuildWorkspace.Create())
        {
            Console.WriteLine($"Loading solution from {solutionPath} ...");
            var solution = await workspace.OpenSolutionAsync(solutionPath);
            Console.WriteLine($"Loaded {solution.Projects.Count()} projects.");

            foreach (var project in solution.Projects)
            {
                if (project.Language != LanguageNames.CSharp)
                    continue;

                Console.WriteLine($"Analyzing project: {project.Name}");
                var compilation = await project.GetCompilationAsync();
                if (compilation == null) continue;

                // Each project or main folder is treated as a module
                string moduleName = Path.GetFileNameWithoutExtension(project.FilePath) ?? project.Name;
                string relFilePath = Path.Combine(outputDir, moduleName + ".rel");

                using (var writer = new StreamWriter(relFilePath, append: false))
                {
                    writer.WriteLine($"MODULE: {moduleName}\n");

                    foreach (var document in project.Documents)
                    {
                        if (document.SourceCodeKind != SourceCodeKind.Regular)
                            continue;

                        var filePath = document.FilePath ?? "<in-memory>";
                        writer.WriteLine($"  FILE: {filePath}");

                        var tree = await document.GetSyntaxTreeAsync();
                        if (tree == null) continue;

                        var root = await tree.GetRootAsync();
                        var model = compilation.GetSemanticModel(tree);

                        // Process class/struct/interface declarations
                        var typeDecls = root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>();
                        foreach (var typeDecl in typeDecls)
                        {
                            var symbol = model.GetDeclaredSymbol(typeDecl) as INamedTypeSymbol;
                            if (symbol == null) continue;

                            writer.WriteLine($"    CLASS: {symbol.Name}");

                            // Inheritance
                            if (symbol.BaseType != null && symbol.BaseType.Name != "Object")
                                writer.WriteLine($"      INHERITS_FROM -> {symbol.BaseType.ToDisplayString()}");

                            // Interfaces
                            foreach (var iface in symbol.Interfaces)
                                writer.WriteLine($"      DEPENDS_ON -> {iface.ToDisplayString()}");

                            // Properties
                            foreach (var prop in symbol.GetMembers().OfType<IPropertySymbol>())
                            {
                                writer.WriteLine($"      PROPERTY: {prop.Name} -> RETURNS -> {prop.Type.ToDisplayString()}");
                                foreach (var dep in ExtractTypeDependencies(prop.Type))
                                    writer.WriteLine($"      PROPERTY: {prop.Name} -> USES -> {dep}");
                            }

                            // Methods
                            foreach (var method in symbol.GetMembers().OfType<IMethodSymbol>())
                            {
                                if (method.MethodKind == MethodKind.Constructor || method.DeclaringSyntaxReferences.Length == 0)
                                    continue;

                                writer.WriteLine($"      METHOD: {method.Name} -> RETURNS -> {method.ReturnType.ToDisplayString()}");

                                // Analyze method body for calls and usages
                                var syntax = method.DeclaringSyntaxReferences.First().GetSyntax() as MethodDeclarationSyntax;
                                if (syntax == null) continue;

                                var invocations = syntax.DescendantNodes().OfType<InvocationExpressionSyntax>();
                                foreach (var inv in invocations)
                                {
                                    var symbolInfo = model.GetSymbolInfo(inv);
                                    var target = symbolInfo.Symbol as IMethodSymbol ?? symbolInfo.CandidateSymbols.FirstOrDefault() as IMethodSymbol;
                                    if (target != null)
                                    {
                                        var callee = target.ToDisplayString(SymbolDisplayFormat.MinimallyQualifiedFormat);
                                        writer.WriteLine($"        METHOD: {method.Name} -> CALLS -> {callee}");
                                    }
                                }

                                // Dependencies from parameter types
                                foreach (var param in method.Parameters)
                                {
                                    foreach (var dep in ExtractTypeDependencies(param.Type))
                                        writer.WriteLine($"        METHOD: {method.Name} -> USES -> {dep}");
                                }
                            }
                        }

                        writer.WriteLine();
                    }

                    // At the end, list outbound references (simplified)
                    writer.WriteLine("OUTBOUND REFERENCES:");
                    foreach (var reference in project.MetadataReferences)
                    {
                        var name = Path.GetFileNameWithoutExtension(reference.Display ?? "<unknown>");
                        writer.WriteLine($"  - [REF: {name}.rel]");
                    }
                }

                Console.WriteLine($"  -> Saved relations to {relFilePath}");
            }
        }

        Console.WriteLine("All module relation files generated successfully.");
        return 0;
    }

    // Helper: Extract type names used by a symbol type
    static IEnumerable<string> ExtractTypeDependencies(ITypeSymbol typeSymbol)
    {
        var results = new HashSet<string>();

        void collect(ITypeSymbol t)
        {
            if (t == null) return;
            if (!string.IsNullOrEmpty(t.Name)) results.Add(t.ToDisplayString(SymbolDisplayFormat.MinimallyQualifiedFormat));
            if (t is INamedTypeSymbol named)
            {
                foreach (var arg in named.TypeArguments)
                    collect(arg);
            }
        }

        collect(typeSymbol);
        return results;
    }
}
