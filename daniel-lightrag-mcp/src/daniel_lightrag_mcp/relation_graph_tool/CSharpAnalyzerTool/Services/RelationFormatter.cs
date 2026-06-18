using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Microsoft.CodeAnalysis;

namespace CSharpAnalyzerTool.Services;

public class RelationFormatter
{
    public static string FormatModuleHeader(string moduleName)
    {
        return $"MODULE: {moduleName}";
    }

    public static string FormatFileEntry(string relativePath, string fileName, string extension)
    {
        return $"FILE: {relativePath} [LINK: {fileName}.{extension}]";
    }

    public static string FormatClassEntry(string className)
    {
        return $"  CLASS: {className}";
    }

    public static string FormatInheritsFrom(string baseType)
    {
        return $"    INHERITS_FROM -> {baseType}";
    }

    public static string FormatDependsOn(string dependency)
    {
        return $"    DEPENDS_ON -> {dependency}";
    }

    public static string FormatContains(string member)
    {
        return $"    CONTAINS -> {member}";
    }

    public static string FormatMethodEntry(string methodName, string returnType)
    {
        return $"    METHOD: {methodName} -> RETURNS -> {returnType}";
    }

    public static string FormatPropertyEntry(string propertyName, string returnType)
    {
        return $"    PROPERTY: {propertyName} -> RETURNS -> {returnType}";
    }

    public static string FormatMethodCall(string methodName, string target, string link)
    {
        return $"      METHOD: {methodName} -> CALLS -> {target} [LINK: {link}]";
    }

    public static string FormatMethodUses(string methodName, string dependency, string reference)
    {
        return $"      METHOD: {methodName} -> USES -> {dependency} [REF: {reference}]";
    }

    public static string FormatPropertyUses(string propertyName, string dependency, string reference)
    {
        return $"    PROPERTY: {propertyName} -> USES -> {dependency} [REF: {reference}]";
    }

    public static string FormatModuleDependency(string moduleName)
    {
        return $"DEPENDS_ON -> {moduleName}";
    }

    public static string FormatOutboundReferencesHeader()
    {
        return "OUTBOUND REFERENCES:";
    }

    public static string FormatReference(string reference)
    {
        return $"  - [REF: {reference}]";
    }
}