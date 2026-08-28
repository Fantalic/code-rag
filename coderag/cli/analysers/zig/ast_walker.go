package ziganalyser

// tree-sitter-zig based extractor. The walker mirrors the structure of
// the C# analyser's ast_walker.go: drive the visitor from the root,
// detect declaration kinds by node type, and dispatch to per-kind
// handlers. Resolution of call sites and type references goes through
// the GlobalIndex populated in index.go.
//
// Notable Zig-AST facts this file relies on (verified by inspecting
// tree-sitter-zig output):
//   - Modifiers like `pub` and declaration keywords like `const`/`fn`
//     are anonymous TOKEN children, not named nodes.
//   - For `pub const X = struct {...};` the identifier X is a direct
//     sibling of the inline struct_declaration under
//     variable_declaration — there's no `_variable_declaration_header`
//     wrapper named node.
//   - For `pub fn name(...) RetType { ... }` the identifier is a direct
//     sibling of parameters / return-type / body — no
//     `_function_prototype` named wrapper in the current grammar
//     revision.

import (
	"fmt"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	zig "github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go"
)

// visibilityKeywords gates the EXPORTS list. Only `pub` is exposed in
// the entity model for now.
var visibilityKeywords = map[string]bool{
	"pub": true,
}

func (p *Parser) parse() {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(sitter.NewLanguage(zig.Language())); err != nil {
		// Grammar ABI mismatch — nothing sensible to extract.
		return
	}
	tree := parser.Parse(p.contentBytes, nil)
	if tree == nil {
		return
	}
	defer tree.Close()
	p.walkRoot(tree.RootNode())
}

// walkRoot visits the top-level container of a Zig source file.
func (p *Parser) walkRoot(root *sitter.Node) {
	p.walkContainerMembers(root, "")
}

// walkContainerMembers walks a node whose children are top-level
// declarations / container fields. parentType is the enclosing type
// name (empty at file scope).
func (p *Parser) walkContainerMembers(node *sitter.Node, parentType string) {
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		p.walkDecl(child, parentType)
	}
}

// walkDecl dispatches a single declaration node to the right handler.
// Returns the qualified name it produced ("" for non-declarations).
func (p *Parser) walkDecl(n *sitter.Node, parentType string) string {
	if n == nil {
		return ""
	}

	switch n.Kind() {
	case "function_declaration":
		return p.handleFunction(n, parentType)
	case "variable_declaration":
		return p.handleVariable(n, parentType)
	case "test_declaration":
		p.handleTest(n, parentType)
		return ""
	case "comptime_declaration":
		return p.handleComptime(n, parentType)
	case "using_namespace_declaration":
		p.handleUsing(n)
		return ""
	case "container_field":
		p.handleContainerField(n, parentType)
		return ""
	case "struct_declaration",
		"enum_declaration",
		"union_declaration",
		"opaque_declaration":
		// Standalone inline type declarations have no enclosing
		// variable_declaration so we can't read the modifier here;
		// leave visibility empty (the parser still records the entity).
		return p.handleInlineTypeDecl(n, parentType, "")
	case "error_set_declaration":
		return p.handleErrorSet(n, parentType)
	}
	return ""
}

// handleFunction processes `fn name(...) [body]`.
// Visibility is detected by scanning ALL children (including anonymous
// tokens) for the `pub` keyword.
func (p *Parser) handleFunction(n *sitter.Node, parentType string) string {
	name := p.firstIdentifierText(n)
	if name == "" {
		return ""
	}

	qualified := qualifiedName(parentType, name)
	vis := p.visibilityFromAllChildren(n)
	body := findChildByKind(n, "block")

	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityFunction,
		Description: descriptionWithVis("fn "+name, vis),
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
		Visibility:  vis,
	})

	if parentType != "" {
		p.addRelation(Relation{
			Source:      parentType,
			Target:      qualified,
			Description: parentType + " has method " + name,
			Keywords:    "method, defines",
			Weight:      0.8,
		})
	}

	if vis == "pub" {
		p.exports = append(p.exports, qualified)
	}

	// Walk body for call sites — only when the function is well-formed.
	if body != nil {
		prevFn := p.currentFn
		prevScope := p.scope
		p.currentFn = qualified
		p.scope = p.scope.Push()
		p.recordParameters(findChildByKind(n, "parameters"))
		p.walkBlock(body)
		p.scope = prevScope
		p.currentFn = prevFn
	}

	return qualified
}

// recordParameters registers every fn parameter name in the current
// scope so a later call expression reusing the parameter name is
// correctly suppressed from cross-file resolution.
func (p *Parser) recordParameters(params *sitter.Node) {
	if params == nil {
		return
	}
	for i := uint(0); i < params.NamedChildCount(); i++ {
		child := params.NamedChild(i)
		if child == nil || child.Kind() != "parameter" {
			continue
		}
		nameNode := findChildByKind(child, "identifier")
		if nameNode != nil {
			p.scope.Declare(p.text(nameNode), "")
		}
	}
}

// handleVariable processes `const|var name [: type] = expr;`.
// When the RHS is an inline type declaration (struct/enum/union/opaque/
// error_set), emit ONLY the type entity (not a separate CONST) so the
// knowledge graph sees one node per declaration. Walk the RHS for
// `@import` calls so the IMPORTS header is populated.
func (p *Parser) handleVariable(n *sitter.Node, parentType string) string {
	name := p.firstIdentifierText(n)
	if name == "" {
		return ""
	}

	qualified := qualifiedName(parentType, name)
	vis := p.visibilityFromAllChildren(n)

	inlineType := findInlineType(n)
	if inlineType != nil {
		// Inline type declaration: emit the type entity (not a separate
		// CONST) so callers see exactly one entity per declaration.
		if inlineType.Kind() == "error_set_declaration" {
			p.handleErrorSet(inlineType, qualified)
		} else {
			p.handleInlineTypeDecl(inlineType, qualified, vis)
		}
	} else {
		p.addEntity(Entity{
			Name:        qualified,
			Type:        EntityConst,
			Description: descriptionWithVis("const "+name, vis),
			FilePath:    p.sourcePath,
			LineNo:      startLine(n),
			EndLine:     endLine(n),
			Visibility:  vis,
		})

		if parentType != "" {
			p.addRelation(Relation{
				Source:      parentType,
				Target:      qualified,
				Description: parentType + " has field " + name,
				Keywords:    "field, defines",
				Weight:      0.7,
			})
		}
	}

	if vis == "pub" {
		p.exports = append(p.exports, qualified)
	}

	// Walk the RHS expression for @import calls and other resolution-
	// worthy expressions (assignments, call expressions, etc.).
	p.walkVariableRHS(n)

	return qualified
}

// walkVariableRHS locates the expression to the right of `=` in a
// variable_declaration and walks it for call sites and imports.
// Inline type declarations (struct/enum/union/opaque/error_set) are
// skipped here — they have their own dedicated handler in
// handleVariable. Otherwise the walker would descend into function
// bodies a second time and emit duplicate CALL rows.
func (p *Parser) walkVariableRHS(n *sitter.Node) {
	if n == nil {
		return
	}
	sawIdent := false
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if !sawIdent {
			if c.Kind() == "identifier" {
				sawIdent = true
			}
			continue
		}
		// Skip the inline type — it's already handled by handleVariable.
		if isInlineTypeKind(c.Kind()) {
			continue
		}
		p.walkExpression(c)
	}
}

// isInlineTypeKind returns true for node kinds that represent a type
// declaration rather than an expression. Used to keep walkVariableRHS
// from re-walking declaration bodies.
func isInlineTypeKind(kind string) bool {
	switch kind {
	case "struct_declaration",
		"enum_declaration",
		"union_declaration",
		"opaque_declaration",
		"error_set_declaration":
		return true
	}
	return false
}

// handleTest processes `test "name" { ... }`.
func (p *Parser) handleTest(n *sitter.Node, parentType string) {
	name := ""
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "string" || c.Kind() == "multiline_string" || c.Kind() == "identifier" {
			name = p.text(c)
			break
		}
	}
	// Strip surrounding quotes from string literals so the entity name
	// doesn't carry the source's punctuation into the knowledge graph.
	name = strings.Trim(name, "\"")
	name = strings.Trim(name, "'")
	if name == "" {
		name = "anon"
	}

	qualified := qualifiedName(parentType, "test:"+name)
	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityTest,
		Description: "test " + name,
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
	})

	if body := findChildByKind(n, "block"); body != nil {
		// Test bodies don't produce call edges — they're verification
		// scaffolding rather than real callers. Bump isInsideTest so
		// handleCallExpression drops any calls the walk encounters.
		p.isInsideTest++
		p.walkBlock(body)
		p.isInsideTest--
	}
}

// handleComptime processes `comptime { ... }` blocks. We don't drill
// into them for calls (comptime evaluation order is ambiguous) but we
// still emit the entity so the name shows up in queries.
func (p *Parser) handleComptime(n *sitter.Node, parentType string) string {
	name := "comptime@" + fmt.Sprint(startLine(n))
	qualified := qualifiedName(parentType, name)
	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityConst,
		Description: "comptime block",
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
	})
	if body := findChildByKind(n, "block"); body != nil {
		p.walkBlock(body)
	}
	return qualified
}

// handleUsing processes `usingnamespace @import("foo")`.
func (p *Parser) handleUsing(n *sitter.Node) {
	path := extractImportPath(n)
	if path == "" {
		return
	}
	p.imports = append(p.imports, path)
	p.addEntity(Entity{
		Name:        "using:" + path,
		Type:        EntityUsing,
		Description: "usingnamespace " + path,
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
	})
}

// extractImportPath pulls the string literal out of a `usingnamespace`
// node. Zig's grammar only allows `@import("…")` here so we look for
// the first string child of the first builtin_function descendant.
func extractImportPath(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	builtin := findChildByKind(n, "builtin_function")
	if builtin == nil {
		return ""
	}
	args := findChildByKind(builtin, "arguments")
	if args == nil {
		return ""
	}
	for i := uint(0); i < args.NamedChildCount(); i++ {
		arg := args.NamedChild(i)
		if arg == nil {
			continue
		}
		if arg.Kind() == "string" || arg.Kind() == "multiline_string" {
			return strings.Trim(arg.Utf8Text(nil), "\"")
		}
	}
	return ""
}

// handleContainerField processes a field declaration inside a struct
// / union / opaque body.
func (p *Parser) handleContainerField(n *sitter.Node, parentType string) {
	if parentType == "" {
		return
	}
	nameNode := findChildByKind(n, "identifier")
	if nameNode == nil {
		return
	}
	name := p.text(nameNode)
	if name == "" {
		return
	}
	qualified := qualifiedName(parentType, name)
	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityConst,
		Description: descriptionWithVis("field "+name, ""),
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
	})
	p.addRelation(Relation{
		Source:      parentType,
		Target:      qualified,
		Description: parentType + " has field " + name,
		Keywords:    "field, defines",
		Weight:      0.7,
	})
}

// handleInlineTypeDecl emits an entity for an inline type declaration
// (struct / enum / union / opaque). vis is the visibility recovered
// from the wrapping variable_declaration so the export list reflects
// the `pub const X = ...` modifier.
//
// Zig's tree-sitter grammar does NOT wrap the body of these nodes in
// a `body` field — the `{` token and `_container_members` are direct
// children — so we walk the node itself when there's no body wrapper.
func (p *Parser) handleInlineTypeDecl(n *sitter.Node, parentType string, vis string) string {
	info := struct {
		entityType EntityType
		kind       string
	}{
		EntityStruct, "struct",
	}
	switch n.Kind() {
	case "struct_declaration":
		info = struct{ entityType EntityType; kind string }{EntityStruct, "struct"}
	case "enum_declaration":
		info = struct{ entityType EntityType; kind string }{EntityEnum, "enum"}
	case "union_declaration":
		info = struct{ entityType EntityType; kind string }{EntityUnion, "union"}
	case "opaque_declaration":
		info = struct{ entityType EntityType; kind string }{EntityOpaque, "opaque"}
	default:
		return ""
	}

	qualified := parentType
	if qualified == "" {
		// Truly anonymous inline type (e.g. `struct {...}` as an
		// argument). No stable identity — skip.
		return ""
	}

	p.addEntity(Entity{
		Name:        qualified,
		Type:        info.entityType,
		Description: descriptionWithVis(info.kind, vis),
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
		Visibility:  vis,
	})

	// Walk for nested members. Zig's grammar nests members directly
	// inside the type declaration, not in a `body` wrapper.
	switch info.entityType {
	case EntityEnum:
		p.walkEnumMembers(n, qualified)
	default:
		p.walkContainerMembers(n, qualified)
	}

	return qualified
}

// walkEnumMembers treats every container_field inside an enum body
// as a member (Zig enums list their variants the same way structs
// list fields).
func (p *Parser) walkEnumMembers(body *sitter.Node, enumName string) {
	for i := uint(0); i < body.NamedChildCount(); i++ {
		c := body.NamedChild(i)
		if c == nil || c.Kind() != "container_field" {
			continue
		}
		nameNode := findChildByKind(c, "identifier")
		if nameNode == nil {
			continue
		}
		name := p.text(nameNode)
		if name == "" {
			continue
		}
		qualified := qualifiedName(enumName, name)
		p.addEntity(Entity{
			Name:        qualified,
			Type:        EntityEnumMember,
			Description: enumName + "." + name,
			FilePath:    p.sourcePath,
			LineNo:      startLine(c),
			EndLine:     endLine(c),
		})
		p.addRelation(Relation{
			Source:      enumName,
			Target:      qualified,
			Description: enumName + "." + name,
			Keywords:    "member, has",
			Weight:      0.7,
		})
	}
}

// handleErrorSet processes `error { Name1, Name2, ... }`. Each name
// becomes an ENUM_MEMBER so callers can resolve error references.
// When called from handleVariable with a non-empty parentType, emit
// the ERROR_SET entity (not a separate CONST).
func (p *Parser) handleErrorSet(n *sitter.Node, parentType string) string {
	qualified := parentType
	if qualified == "" {
		// Standalone error set — synthesise a name from the source line.
		qualified = "error@" + fmt.Sprint(startLine(n))
		p.addEntity(Entity{
			Name:        qualified,
			Type:        EntityErrorSet,
			Description: "error set",
			FilePath:    p.sourcePath,
			LineNo:      startLine(n),
			EndLine:     endLine(n),
		})
	} else {
		// Called from handleVariable; emit only the error-set entity
		// (the wrapping const name is the parent type, so we don't
		// duplicate it as CONST here).
		p.addEntity(Entity{
			Name:        qualified,
			Type:        EntityErrorSet,
			Description: "error set",
			FilePath:    p.sourcePath,
			LineNo:      startLine(n),
			EndLine:     endLine(n),
		})
	}

	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil || c.Kind() != "identifier" {
			continue
		}
		memberName := p.text(c)
		if memberName == "" {
			continue
		}
		memberQualified := qualifiedName(qualified, memberName)
		p.addEntity(Entity{
			Name:        memberQualified,
			Type:        EntityEnumMember,
			Description: qualified + "." + memberName,
			FilePath:    p.sourcePath,
			LineNo:      startLine(c),
			EndLine:     endLine(c),
		})
		p.addRelation(Relation{
			Source:      qualified,
			Target:      memberQualified,
			Description: qualified + "." + memberName,
			Keywords:    "member, has",
			Weight:      0.7,
		})
	}
	return qualified
}

// walkBlock walks a `{ ... }` body for local declarations and call
// expressions. Opens a new scope on entry, restores it on exit.
func (p *Parser) walkBlock(block *sitter.Node) {
	if block == nil {
		return
	}
	prev := p.scope
	p.scope = p.scope.Push()
	defer func() { p.scope = prev }()

	for i := uint(0); i < block.NamedChildCount(); i++ {
		child := block.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "variable_declaration":
			p.handleLocalVar(child)
		case "expression_statement":
			p.walkExpression(child)
		case "if_statement", "for_statement", "while_statement",
			"labeled_statement", "switch_expression", "block",
			"block_expression", "comptime_statement", "defer_statement",
			"errdefer_statement", "nosuspend_statement", "suspend_statement":
			p.walkNestedStatement(child)
		}
	}
}

// handleLocalVar registers a local-scope name and then descends into
// the initializer expression for call-site detection.
func (p *Parser) handleLocalVar(n *sitter.Node) {
	name := p.firstIdentifierText(n)
	if name == "" {
		return
	}
	p.scope.Declare(name, "")
	// Walk the RHS for call sites and imports — same logic as
	// top-level variable declarations.
	p.walkVariableRHS(n)
}

// walkNestedStatement recurses into a nested control-flow block,
// pushing a new scope for the body.
func (p *Parser) walkNestedStatement(n *sitter.Node) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "block", "block_expression":
		p.walkBlock(n)
	case "if_statement":
		p.walkExpression(p.field(n, "condition"))
		if body := findChildByKind(n, "block"); body != nil {
			p.walkBlock(body)
		} else if body := p.field(n, "body"); body != nil {
			p.walkExpression(body)
		}
		if alt := findChildByKind(n, "else_clause"); alt != nil {
			p.walkNestedStatement(alt)
		}
	case "for_statement", "while_statement":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil && (c.Kind() == "block" || c.Kind() == "block_expression") {
				p.walkBlock(c)
			}
		}
	case "labeled_statement", "switch_expression":
		if body := findChildByKind(n, "block"); body != nil {
			p.walkBlock(body)
		}
	default:
		p.walkExpression(n)
	}
}

// walkExpression inspects an expression node for call sites.
// Unknown node kinds fall through to a generic "walk every named
// child" pass so deeply nested expressions (assignment, struct
// initialisers, etc.) still get their call sites found.
func (p *Parser) walkExpression(n *sitter.Node) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "call_expression":
		p.handleCallExpression(n)
		p.walkExpression(p.field(n, "function"))
		// Also walk the argument list so nested calls are detected.
		if args := findChildByKind(n, "arguments"); args != nil {
			for i := uint(0); i < args.NamedChildCount(); i++ {
				c := args.NamedChild(i)
				if c != nil {
					p.walkExpression(c)
				}
			}
		}
	case "field_expression":
		p.walkExpression(p.field(n, "object"))
	case "assignment_expression":
		p.walkExpression(p.field(n, "left"))
		p.walkExpression(p.field(n, "right"))
	case "binary_expression":
		p.walkExpression(p.field(n, "left"))
		p.walkExpression(p.field(n, "right"))
	case "unary_expression":
		p.walkExpression(p.field(n, "argument"))
	case "expression_statement", "return_expression", "defer_statement",
		"errdefer_statement", "labeled_statement":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil {
				p.walkExpression(c)
			}
		}
	case "if_expression", "for_expression", "while_expression":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil {
				p.walkExpression(c)
			}
		}
	case "switch_expression":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c == nil {
				continue
			}
			if c.Kind() == "switch_case" {
				for j := uint(0); j < c.NamedChildCount(); j++ {
					sc := c.NamedChild(j)
					if sc != nil {
						p.walkExpression(sc)
					}
				}
			}
		}
	case "try_expression", "catch_expression", "async_expression",
		"await_expression", "nosuspend_expression", "comptime_expression":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil {
				p.walkExpression(c)
			}
		}
	case "builtin_function":
		p.handleBuiltinCall(n)
	case "struct_initializer", "anonymous_struct_initializer":
		// Struct initialisers wrap call expressions; walk every child.
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil {
				p.walkExpression(c)
			}
		}
	default:
		// Generic descent: walk every named child so call sites buried
		// inside unknown expression wrappers still surface.
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil {
				p.walkExpression(c)
			}
		}
	}
}

// handleCallExpression resolves a `callee(args)` site. Resolution
// order:
//  1. If the callee is a field expression `obj.name`, look it up as
//     `obj.name` in the field index.
//  2. If the callee is a plain identifier, look it up as a free
//     function in the index.
//  3. Otherwise drop the call (no phantom UNKNOWN edges).
func (p *Parser) handleCallExpression(n *sitter.Node) {
	if n == nil {
		return
	}
	fnNode := p.field(n, "function")
	callee := ""
	owner := ""

	switch fnNode.Kind() {
	case "identifier":
		callee = p.text(fnNode)
	case "field_expression":
		callee = p.text(p.field(fnNode, "member"))
		obj := p.field(fnNode, "object")
		if obj != nil && obj.Kind() == "identifier" {
			owner = p.text(obj)
		}
	case "builtin_function":
		// @import("std") etc. — handled by handleBuiltinCall.
		p.handleBuiltinCall(n)
		return
	}

	if callee == "" {
		return
	}

	// Suppress calls that resolve to a local name.
	if _, isLocal := p.scope.LookupLocal(callee); isLocal {
		return
	}

	// Suppress calls inside test bodies.
	if p.isInsideTest > 0 {
		return
	}

	resolved := ""
	if p.index != nil {
		if owner != "" {
			if cands, ok := p.index.Fields[owner+"."+callee]; ok && len(cands) > 0 {
				resolved = cands[0].FilePath
			} else if matches, ok := p.index.SimpleNames[callee]; ok && len(matches) == 1 {
				resolved = matches[0]
			}
		} else {
			if matches, ok := p.index.SimpleNames[callee]; ok && len(matches) == 1 {
				resolved = matches[0]
			} else if cands, ok := p.index.Functions[callee]; ok && len(cands) == 1 {
				resolved = cands[0].FilePath
			}
		}
	}

	if resolved == "" {
		// Unresolved — drop. Avoids phantom UNKNOWN nodes downstream.
		return
	}

	p.addCall(CallSite{
		CallerFile:   p.sourcePath,
		CallerMethod: p.currentFn,
		CalleeName:   callee,
		CalleeOwner:  owner,
		LineNo:       startLine(n),
		ResolvedFile: resolved,
	})
}

// handleBuiltinCall treats a few well-known builtins as imports.
// Other builtins are ignored — they're standard-library utilities, not
// user-defined code.
func (p *Parser) handleBuiltinCall(n *sitter.Node) {
	name := ""
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c != nil && c.Kind() == "builtin_identifier" {
			name = strings.TrimPrefix(p.text(c), "@")
			break
		}
	}
	if name != "import" {
		return
	}

	args := findChildByKind(n, "arguments")
	if args == nil {
		return
	}
	for i := uint(0); i < args.NamedChildCount(); i++ {
		arg := args.NamedChild(i)
		if arg == nil {
			continue
		}
		if arg.Kind() == "string" || arg.Kind() == "multiline_string" {
			path := strings.Trim(p.text(arg), "\"")
			p.imports = append(p.imports, path)
			p.addEntity(Entity{
				Name:        "import:" + path,
				Type:        EntityImport,
				Description: "@import(\"" + path + "\")",
				FilePath:    p.sourcePath,
				LineNo:      startLine(n),
				EndLine:     endLine(n),
			})
			return
		}
	}
}

// --- low-level tree-sitter helpers ---------------------------------

// text returns the source bytes for a node, or "" if the node is nil.
func (p *Parser) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Utf8Text(p.contentBytes)
}

// field returns the named child of n with the given field name, or nil.
func (p *Parser) field(n *sitter.Node, name string) *sitter.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName(name)
}

// findChildByKind returns the first named child of n whose kind
// matches, or nil.
func findChildByKind(n *sitter.Node, kind string) *sitter.Node {
	if n == nil {
		return nil
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c != nil && c.Kind() == kind {
			return c
		}
	}
	return nil
}

// firstIdentifierText returns the text of the first identifier named
// child of n, or "" when none exists. Used for variable/function
// declarations whose identifier is a direct sibling of the modifier
// keywords.
func (p *Parser) firstIdentifierText(n *sitter.Node) string {
	child := findChildByKind(n, "identifier")
	if child == nil {
		return ""
	}
	return p.text(child)
}

// findInlineType returns the inline type declaration child of a
// variable_declaration node, if any. nil otherwise.
func findInlineType(n *sitter.Node) *sitter.Node {
	if n == nil {
		return nil
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "struct_declaration",
			"enum_declaration",
			"union_declaration",
			"opaque_declaration",
			"error_set_declaration":
			return c
		}
	}
	return nil
}

// startLine converts tree-sitter's zero-based rows to 1-based lines.
func startLine(n *sitter.Node) int {
	if n == nil {
		return 0
	}
	return int(n.StartPosition().Row) + 1
}

// endLine converts tree-sitter's zero-based rows to 1-based lines for
// the end position of the node.
func endLine(n *sitter.Node) int {
	if n == nil {
		return 0
	}
	return int(n.EndPosition().Row) + 1
}

// visibilityFromAllChildren scans EVERY child of n (named + anonymous
// tokens) for the `pub` keyword. Zig's grammar emits visibility
// modifiers as anonymous tokens so we can't rely on a named child.
// Implemented as a method so it has access to the source bytes for
// anonymous-token text lookup.
func (p *Parser) visibilityFromAllChildren(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		if visibilityKeywords[c.Kind()] {
			return c.Kind()
		}
		// Defensive: anonymous token text might be "pub" if the
		// grammar version emits it as a literal token.
		if !c.IsNamed() {
			if c.Utf8Text(p.contentBytes) == "pub" {
				return "pub"
			}
		}
	}
	return ""
}

// visibilityFromAncestors recovers the `pub` modifier when the AST
// wrapper (e.g. a parent variable_declaration) is what carries it.
// Inline type declarations wrapped by `pub const X = struct {...};`
// lose visibility when walked standalone, so we look up one level via
// the parent. Returns "" when no parent carries `pub`.
func visibilityFromAncestors() string {
	// Walk-up is non-trivial with tree-sitter's API; the wrapping
	// variable_declaration is visited first and emits the export, so
	// the inline type entity doesn't need to repeat the visibility.
	return ""
}

// qualifiedName builds a "Parent.Name" or "Name" string depending on
// whether parentType is set. Matches the C# analyser's dot-naming.
func qualifiedName(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// descriptionWithVis formats a declaration description with optional
// visibility suffix.
func descriptionWithVis(base, vis string) string {
	if vis == "" {
		return base
	}
	return base + " [" + vis + "]"
}
