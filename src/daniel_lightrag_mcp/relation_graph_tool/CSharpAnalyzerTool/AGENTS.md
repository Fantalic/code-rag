# AGENTS.md

This is a C# code analysis tool that generates relation graphs for C# solutions using Roslyn.

## Build Commands
- Build: `dotnet build`
- Run: `dotnet run -- <solution-path> [output-directory]`
- Publish: `dotnet publish -c Release -o ./publish`

## Test Commands
- Run specific tests: Not applicable (no test framework configured)

## Code Style Guidelines
- **Language**: C# 9.0+ with .NET 9
- **Formatting**: Default .NET formatting conventions
- **Imports**: Use standard C# using statements, group system libraries first
- **Naming**: PascalCase for classes, methods, properties; camelCase for variables
- **Types**: Use explicit typing where clarity is needed, var where type is obvious
- **Error Handling**: Try-catch for file operations and Roslyn analysis
- **Structure**: Follow standard C# project structure with Models/Services separation
- **Dependencies**: Only use configured NuGet packages (Roslyn, DotNetEnv, Ignore)

## Additional Notes
- The tool analyzes C# solutions to extract code relationships
- Configuration via .env (REL_FILE_TYPE, REL_MAX_SIZE) and .relignore files
- Outputs structured relation files with CALLS, USES, INHERITS_FROM, DEPENDS_ON relationships
- Always build with `dotnet build` after changes