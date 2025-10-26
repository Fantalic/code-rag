using Microsoft.Build.Locator;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.MSBuild;

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

        using var workspace = MSBuildWorkspace.Create();
        var solution = await workspace.OpenSolutionAsync(solutionPath);
        Console.WriteLine($"Loaded {solution.Projects.Count()} projects.");

        foreach (var project in solution.Projects)
        {
            if (project.Language != LanguageNames.CSharp)
                continue;

            Console.WriteLine($"Analyzing project: {project.Name}");
            var compilation = await project.GetCompilationAsync();
            if (compilation == null) continue;

            string moduleDir = Path.Combine(outputDir, Path.GetFileNameWithoutExtension(project.FilePath) ?? project.Name);
            Directory.CreateDirectory(moduleDir);

            var relIndexPath = Path.Combine(moduleDir, "_index.txt");
            using var indexWriter = new StreamWriter(relIndexPath, append: false);
            indexWriter.WriteLine($"MODULE: {project.Name}");
            indexWriter.WriteLine();

            foreach (var document in project.Documents)
            {
                if (document.SourceCodeKind != SourceCodeKind.Regular)
                    continue;

                var fileName = Path.GetFileNameWithoutExtension(document.FilePath ?? "unknown");
                string relFilePath = Path.Combine(moduleDir, fileName + ".txt");
                indexWriter.WriteLine($"[LINK: {fileName}.txt]");

                using var writer = new StreamWriter(relFilePath, append: false);
                writer.WriteLine($"FILE: {document.FilePath}");
                writer.WriteLine();

                var tree = await document.GetSyntaxTreeAsync();
                if (tree == null) continue;
                var root = await tree.GetRootAsync();
                var model = compilation.GetSemanticModel(tree);

                var typeDecls = root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>();
                foreach (var typeDecl in typeDecls)
                {
                    var symbol = model.GetDeclaredSymbol(typeDecl) as INamedTypeSymbol;
                    if (symbol == null) continue;

                    writer.WriteLine($"  CLASS: {symbol.Name}");

                    if (symbol.BaseType != null && symbol.BaseType.Name != "Object")
                        writer.WriteLine($"    INHERITS_FROM -> {symbol.BaseType.ToDisplayString()}");

                    foreach (var iface in symbol.Interfaces)
                        writer.WriteLine($"    DEPENDS_ON -> {iface.ToDisplayString()}");

                    // Properties
                    foreach (var prop in symbol.GetMembers().OfType<IPropertySymbol>())
                    {
                        writer.WriteLine($"    PROPERTY: {prop.Name} -> RETURNS -> {prop.Type.ToDisplayString()}");
                        foreach (var dep in ExtractTypeDependencies(prop.Type))
                            writer.WriteLine($"    PROPERTY: {prop.Name} -> USES -> {dep} [LINK: maybe:{dep}.txt]");
                    }

                    // Methods
                    foreach (var method in symbol.GetMembers().OfType<IMethodSymbol>())
                    {
                        if (method.MethodKind == MethodKind.Constructor || method.DeclaringSyntaxReferences.Length == 0)
                            continue;

                        writer.WriteLine($"    METHOD: {method.Name} -> RETURNS -> {method.ReturnType.ToDisplayString()}");

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
                                    var linkFile = $"{callee.Split('.').FirstOrDefault()}.txt";
                                    writer.WriteLine($"      METHOD: {method.Name} -> CALLS -> {callee} [LINK: {linkFile}]");
                                }
                            }
                            catch { }
                        }

                        foreach (var param in method.Parameters)
                        {
                            foreach (var dep in ExtractTypeDependencies(param.Type))
                                writer.WriteLine($"      METHOD: {method.Name} -> USES -> {dep} [LINK: maybe:{dep}.txt]");
                        }
                    }
                }

                writer.WriteLine();
                writer.WriteLine("OUTBOUND REFERENCES:");
                foreach (var reference in project.MetadataReferences)
                {
                    var name = Path.GetFileNameWithoutExtension(reference.Display ?? "<unknown>");
                    writer.WriteLine($"  - [REF: {name}.txt]");
                }
            }

            Console.WriteLine($"  -> Saved module chunked relations in {moduleDir}");
        }

        Console.WriteLine("All relation chunks generated successfully.");
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
