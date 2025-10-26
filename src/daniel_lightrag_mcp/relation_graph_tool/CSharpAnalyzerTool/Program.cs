// CSharpAnalyzerTool: Relation extractor
// Usage: CSharpAnalyzerTool.exe <path-to-solution.sln> [output-directory]
// Example: CSharpAnalyzerTool.exe "C:\Projects\MyApp\MyApp.sln" "C:\temp\relations"

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

        // Register MSBuild instance
        var instance = MSBuildLocator.QueryVisualStudioInstances().FirstOrDefault();
        if (instance == null)
        {
            Console.WriteLine("No Visual Studio/MSBuild instances found. Ensure Visual Studio or Build Tools are installed.");
            return 1;
        }

        MSBuildLocator.RegisterInstance(instance);

        using (var workspace = MSBuildWorkspace.Create())
        {
            Console.WriteLine($"Loading solution from {solutionPath} ...");
            var solution = await workspace.OpenSolutionAsync(solutionPath);
            Console.WriteLine($"Solution loaded with {solution.Projects.Count()} projects.");

            // Data containers
            var files = new HashSet<string>();
            var types = new Dictionary<INamedTypeSymbol, string>(SymbolEqualityComparer.Default); // symbol -> file
            var typeInfos = new List<TypeInfoRecord>();
            var memberInfos = new List<MemberInfoRecord>();
            var callEdges = new List<CallEdgeRecord>();

            foreach (var project in solution.Projects)
            {
                Console.WriteLine($"Processing project: {project.Name} ({project.Language})");

                if (project.Language != LanguageNames.CSharp && project.Language != LanguageNames.VisualBasic)
                {
                    Console.WriteLine(" - Skipping non-C#/VB project");
                    continue;
                }

                var compilation = await project.GetCompilationAsync();
                if (compilation == null)
                {
                    Console.WriteLine(" - Failed to get compilation for project");
                    continue;
                }

                // Process each document
                foreach (var document in project.Documents)
                {
                    if (document.SourceCodeKind != SourceCodeKind.Regular)
                        continue; // skip generated / other

                    var filePath = document.FilePath ?? "<in-memory>";
                    files.Add(filePath);

                    var syntaxTree = await document.GetSyntaxTreeAsync();
                    if (syntaxTree == null) continue;
                    var root = await syntaxTree.GetRootAsync();

                    var semanticModel = compilation.GetSemanticModel(syntaxTree);

                    // Find declared named types
                    var typeDecls = root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>();
                    foreach (var td in typeDecls)
                    {
                        var namedSymbol = semanticModel.GetDeclaredSymbol(td) as INamedTypeSymbol;
                        if (namedSymbol == null) continue;

                        types[namedSymbol] = filePath;

                        var rec = new TypeInfoRecord
                        {
                            Symbol = namedSymbol,
                            Name = namedSymbol.ToDisplayString(SymbolDisplayFormat.CSharpErrorMessageFormat),
                            Kind = namedSymbol.TypeKind.ToString(),
                            Namespace = namedSymbol.ContainingNamespace?.ToDisplayString() ?? "<global>",
                            FilePath = filePath,
                            BaseType = namedSymbol.BaseType?.ToDisplayString() ?? null,
                            Interfaces = namedSymbol.Interfaces.Select(i => i.ToDisplayString()).ToList()
                        };

                        typeInfos.Add(rec);

                        // Members
                        foreach (var member in namedSymbol.GetMembers())
                        {
                            if (member.DeclaredAccessibility == Accessibility.Private && member.Kind == SymbolKind.Field)
                            {
                                // still include fields; optional to filter
                            }

                            var mrec = new MemberInfoRecord
                            {
                                ContainingType = namedSymbol.ToDisplayString(SymbolDisplayFormat.CSharpErrorMessageFormat),
                                MemberKind = member.Kind.ToString(),
                                Name = member.ToDisplayString(SymbolDisplayFormat.CSharpErrorMessageFormat),
                                FilePath = filePath
                            };

                            memberInfos.Add(mrec);
                        }

                        // Analyze method bodies for calls inside this file's types
                        var methodDecls = td.DescendantNodes().OfType<BaseMethodDeclarationSyntax>();
                        foreach (var md in methodDecls)
                        {
                            var methodSymbol = semanticModel.GetDeclaredSymbol(md) as IMethodSymbol;
                            if (methodSymbol == null) continue;

                            var invocations = md.DescendantNodes().OfType<InvocationExpressionSyntax>();
                            foreach (var inv in invocations)
                            {
                                var symbolInfo = semanticModel.GetSymbolInfo(inv);
                                var target = symbolInfo.Symbol as IMethodSymbol ?? symbolInfo.CandidateSymbols.FirstOrDefault() as IMethodSymbol;
                                if (target == null) continue;

                                var caller = methodSymbol.ToDisplayString(SymbolDisplayFormat.CSharpErrorMessageFormat);
                                var callee = target.ToDisplayString(SymbolDisplayFormat.CSharpErrorMessageFormat);

                                callEdges.Add(new CallEdgeRecord
                                {
                                    Caller = caller,
                                    Callee = callee,
                                    CallerFile = filePath,
                                    CalleeFile = target.Locations.FirstOrDefault()?.SourceTree?.FilePath ?? "<external>"
                                });
                            }
                        }
                    }

                    // Also capture top-level functions (C# 9+)
                    var topLevelInvocations = root.DescendantNodes().OfType<InvocationExpressionSyntax>();
                    foreach (var inv in topLevelInvocations)
                    {
                        var symbolInfo = semanticModel.GetSymbolInfo(inv);
                        var target = symbolInfo.Symbol as IMethodSymbol ?? symbolInfo.CandidateSymbols.FirstOrDefault() as IMethodSymbol;
                        if (target == null) continue;
                        // We don't have a caller method symbol here easily; tag caller as <top-level> + file
                        callEdges.Add(new CallEdgeRecord
                        {
                            Caller = "<top-level> in " + filePath,
                            Callee = target.ToDisplayString(SymbolDisplayFormat.CSharpErrorMessageFormat),
                            CallerFile = filePath,
                            CalleeFile = target.Locations.FirstOrDefault()?.SourceTree?.FilePath ?? "<external>"
                        });
                    }
                }
            }

            // --- Write outputs ---
            // 1) files.txt
            File.WriteAllLines(Path.Combine(outputDir, "files.txt"), files.OrderBy(x => x));

            // 2) types.txt
            using (var tw = new StreamWriter(Path.Combine(outputDir, "types.txt")))
            {
                foreach (var t in typeInfos.OrderBy(t => t.Namespace).ThenBy(t => t.Name))
                {
                    tw.WriteLine($"Type: {t.Name}");
                    tw.WriteLine($"  Kind: {t.Kind}");
                    tw.WriteLine($"  Namespace: {t.Namespace}");
                    tw.WriteLine($"  File: {t.FilePath}");
                    if (!string.IsNullOrEmpty(t.BaseType)) tw.WriteLine($"  Base: {t.BaseType}");
                    if (t.Interfaces.Any()) tw.WriteLine($"  Interfaces: {string.Join(", ", t.Interfaces)}");
                    tw.WriteLine();
                }
            }

            // 3) members.txt
            using (var mw = new StreamWriter(Path.Combine(outputDir, "members.txt")))
            {
                foreach (var m in memberInfos.OrderBy(m => m.ContainingType).ThenBy(m => m.Name))
                {
                    mw.WriteLine($"{m.ContainingType} :: {m.MemberKind} :: {m.Name} :: File: {m.FilePath}");
                }
            }

            // 4) calls.txt
            using (var cw = new StreamWriter(Path.Combine(outputDir, "calls.txt")))
            {
                foreach (var c in callEdges)
                {
                    cw.WriteLine($"{c.Caller} -> {c.Callee}    CallerFile: {c.CallerFile}    CalleeFile: {c.CalleeFile}");
                }
            }

            // 5) file_type_map.txt
            using (var ft = new StreamWriter(Path.Combine(outputDir, "file_type_map.txt")))
            {
                var byFile = typeInfos.GroupBy(t => t.FilePath).OrderBy(g => g.Key);
                foreach (var g in byFile)
                {
                    ft.WriteLine($"File: {g.Key}");
                    foreach (var t in g)
                    {
                        ft.WriteLine($"  - {t.Name} ({t.Kind})");
                    }
                    ft.WriteLine();
                }
            }

            // 6) graph_edges.txt (simplified edges for graph tools)
            using (var gw = new StreamWriter(Path.Combine(outputDir, "graph_edges.txt")))
            {
                // type inheritance and implements
                foreach (var t in typeInfos)
                {
                    if (!string.IsNullOrEmpty(t.BaseType))
                        gw.WriteLine($"{t.Name} -> {t.BaseType} [rel=\":base\"]");
                    foreach (var i in t.Interfaces)
                        gw.WriteLine($"{t.Name} -> {i} [rel=\":implements\"]");
                }

                // call edges
                foreach (var c in callEdges)
                {
                    gw.WriteLine($"{c.Caller} -> {c.Callee} [rel=\":calls\"]");
                }
            }

            Console.WriteLine($"Analysis complete. Outputs written to: {outputDir}");
        }

        return 0;
    }

    // --- helper record types ---
    class TypeInfoRecord
    {
        public INamedTypeSymbol Symbol { get; set; }
        public string Name { get; set; }
        public string Kind { get; set; }
        public string Namespace { get; set; }
        public string FilePath { get; set; }
        public string BaseType { get; set; }
        public List<string> Interfaces { get; set; } = new List<string>();
    }

    class MemberInfoRecord
    {
        public string ContainingType { get; set; }
        public string MemberKind { get; set; }
        public string Name { get; set; }
        public string FilePath { get; set; }
    }

    class CallEdgeRecord
    {
        public string Caller { get; set; }
        public string Callee { get; set; }
        public string CallerFile { get; set; }
        public string CalleeFile { get; set; }
    }
}
