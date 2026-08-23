package csharpanalyser

// tree-sitter based C# extractor. Replaces the previous line/regex scanner:
// declarations are read from the concrete syntax tree, so control-flow
// keywords can no longer become entities, strings/comments can't desync the
// brace state, and every node carries exact start/end line positions.

import (
	"fmt"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

// typeDeclInfo describes one kind of type declaration node.
type typeDeclInfo struct {
	kind       string // keyword used in descriptions ("class", "record", ...)
	entityType EntityType
}

var typeDeclKinds = map[string]typeDeclInfo{
	"class_declaration":     {kind: "class", entityType: EntityClass},
	"struct_declaration":    {kind: "struct", entityType: EntityStruct},
	"interface_declaration": {kind: "interface", entityType: EntityInterface},
	"enum_declaration":      {kind: "enum", entityType: EntityEnum},
	// Records surface as CLASS so existing consumers (dead_code filters,
	// index builder) treat them like any other reference type.
	"record_declaration": {kind: "record", entityType: EntityClass},
}

var modifierKeywords = map[string]bool{
	"public": true, "private": true, "internal": true, "protected": true,
	"static": true, "abstract": true, "sealed": true, "partial": true,
	"readonly": true, "new": true, "unsafe": true, "async": true,
	"extern": true, "virtual": true, "override": true, "const": true,
	"volatile": true, "required": true, "file": true, "ref": true,
}

// methodScope carries the context while walking inside a method body.
type methodScope struct {
	qualifiedName string // Ns.Class.Method (empty = not inside a callable)
	className     string // enclosing qualified class
	vars          map[string]string
}

func (p *Parser) parse() {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(sitter.NewLanguage(csharp.Language())); err != nil {
		// Grammar ABI mismatch â€” nothing sensible to extract.
		return
	}
	tree := parser.Parse(p.contentBytes, nil)
	if tree == nil {
		return
	}
	defer tree.Close()
	p.walkCompilationUnit(tree.RootNode())
}

func (p *Parser) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Utf8Text(p.contentBytes)
}

func (p *Parser) field(n *sitter.Node, name string) *sitter.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName(name)
}

// startLine/endLine convert tree-sitter's zero-based rows to 1-based lines.
func startLine(n *sitter.Node) int {
	if n == nil {
		return 0
	}
	return int(n.StartPosition().Row) + 1
}

func endLine(n *sitter.Node) int {
	if n == nil {
		return 0
	}
	return int(n.EndPosition().Row) + 1
}

// visibilityFrom extracts the first access modifier from a declaration's
// modifier list. Mirrors the old line-prefix behaviour: only explicit
// modifiers produce a value; default accessibility stays "".
func (p *Parser) visibilityFrom(decl *sitter.Node) string {
	for i := uint(0); i < decl.NamedChildCount(); i++ {
		child := decl.NamedChild(i)
		if child == nil {
			continue
		}
		kind := child.Kind()
		switch kind {
		case "modifier":
			// Some grammar revisions wrap keywords in a modifier node.
			if child.ChildCount() > 0 && child.Child(0) != nil {
				kw := child.Child(0).Kind()
				if isAccessModifier(kw) {
					return kw
				}
			}
		default:
			if isAccessModifier(kind) {
				return kind
			}
		}
	}
	return ""
}

func isAccessModifier(kind string) bool {
	switch kind {
	case "public", "private", "internal", "protected":
		return true
	}
	return false
}

func (p *Parser) walkCompilationUnit(root *sitter.Node) {
	for i := uint(0); i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "using_directive":
			p.handleUsing(child)
		case "namespace_declaration":
			name := p.text(p.field(child, "name"))
			prev := p.namespace
			p.namespace = joinQualified(prev, name)
			p.walkNamespaceBody(child)
			p.namespace = prev
		case "file_scoped_namespace_declaration":
			// Quirk: members after a file-scoped namespace are SIBLINGS in
			// the compilation unit, not children — remember it for them.
			name := p.text(p.field(child, "name"))
			p.fileNamespace = joinQualified(p.fileNamespace, name)
		default:
			if info, ok := typeDeclKinds[child.Kind()]; ok {
				p.handleTypeDecl(child, info, "")
			}
		}
	}
}

func (p *Parser) handleUsing(n *sitter.Node) {
	text := p.text(n)
	isGlobal := strings.Contains(text, "global")
	ns := ""
	// using_directive fields: (global)? static? name
	nameNode := p.field(n, "name")
	if nameNode != nil {
		ns = p.text(nameNode)
	} else {
		// Fallback: last identifier segment chain in the raw text.
		fields := strings.Fields(strings.TrimSuffix(strings.TrimSpace(text), ";"))
		if len(fields) > 0 {
			ns = fields[len(fields)-1]
		}
	}
	if ns == "" {
		return
	}
	p.usings = append(p.usings, ns)
	p.imports = append(p.imports, ns)
	if isGlobal {
		p.globalUsings = append(p.globalUsings, ns)
	}
}

// walkNamespaceBody handles children of a block namespace_declaration.
func (p *Parser) walkNamespaceBody(n *sitter.Node) {
	body := p.field(n, "body")
	if body == nil {
		return
	}
	p.walkTypeContainer(body, "")
}

// walkTypeContainer walks a node whose named children are types/members
// (namespace body or class declaration_list).
func (p *Parser) walkTypeContainer(container *sitter.Node, parentClass string) {
	for i := uint(0); i < container.NamedChildCount(); i++ {
		child := container.NamedChild(i)
		if child == nil {
			continue
		}
		if info, ok := typeDeclKinds[child.Kind()]; ok {
			p.handleTypeDecl(child, info, parentClass)
		}
	}
}

func joinQualified(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func (p *Parser) handleTypeDecl(n *sitter.Node, info typeDeclInfo, parentClass string) {
	nameNode := p.field(n, "name")
	if nameNode == nil {
		return
	}
	name := p.text(nameNode)
	qualified := joinQualified(parentClass, name)
	if parentClass == "" {
		ns := p.namespace
		if ns == "" {
			ns = p.fileNamespace
		}
		qualified = joinQualified(ns, name)
	}

	vis := p.visibilityFrom(n)
	visSuffix := ""
	if vis != "" {
		visSuffix = " [" + vis + "]"
	}

	ctx := classContext{
		name:   qualified,
		depth:  0,
		lineNo: startLine(n),
	}

	// Base list: first non-interface entry is the base type, the rest are
	// interfaces. Keeps the historical I-prefix heuristic for compat.
	// Note: base_list carries no field label in this grammar revision, so
	// find it by node kind.
	var baseList *sitter.Node
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil && c.Kind() == "base_list" {
			baseList = c
			break
		}
	}
	if baseList != nil {
		for _, part := range p.baseListEntries(baseList) {
			simplePart := part
			if idx := strings.IndexAny(simplePart, "<"); idx >= 0 {
				simplePart = simplePart[:idx]
			}
			if idx := strings.LastIndex(simplePart, "."); idx >= 0 {
				simplePart = simplePart[idx+1:]
			}
			isInterface := len(simplePart) >= 2 && simplePart[0] == 'I' && simplePart[1] >= 'A' && simplePart[1] <= 'Z'
			if isInterface {
				if simplePart != name {
					ctx.interfaces = append(ctx.interfaces, simplePart)
					p.imports = append(p.imports, simplePart)
				}
			} else if simplePart != "" && simplePart != name {
				ctx.baseType = simplePart
				p.imports = append(p.imports, simplePart)
			}
		}
	}

	p.addEntity(Entity{
		Name:        qualified,
		Type:        info.entityType,
		Description: fmt.Sprintf("%s %s%s", info.kind, name, visSuffix),
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
		Visibility:  vis,
	})
	p.exports = append(p.exports, qualified)

	if parentClass != "" {
		p.addRelation(Relation{
			Source:      parentClass,
			Target:      qualified,
			Description: fmt.Sprintf("%s has nested type %s", parentClass, name),
			Keywords:    "contains, nested",
			Weight:      0.7,
		})
	}

	if p.index != nil {
		if ctx.baseType != "" {
			resolved := p.resolveType(ctx.baseType)
			if _, ok := p.index.Classes[resolved]; ok {
				p.addRelation(Relation{
					Source:      qualified,
					Target:      resolved,
					Description: fmt.Sprintf("%s extends %s", qualified, resolved),
					Keywords:    "inheritance, extends",
					Weight:      1.0,
				})
			}
		}
		for _, iface := range ctx.interfaces {
			resolved := p.resolveType(iface)
			if _, ok := p.index.Classes[resolved]; ok {
				p.addRelation(Relation{
					Source:      qualified,
					Target:      resolved,
					Description: fmt.Sprintf("%s implements %s", qualified, resolved),
					Keywords:    "implements",
					Weight:      1.0,
				})
			}
		}
	}

	// Enum members: each constant becomes its own entity so member ordering
	// and names are queryable (previously invisible).
	if info.entityType == EntityEnum {
		if body := p.field(n, "body"); body != nil {
			for i := uint(0); i < body.NamedChildCount(); i++ {
				member := body.NamedChild(i)
				if member == nil || member.Kind() != "enum_member_declaration" {
					continue
				}
				p.handleEnumMember(member, qualified)
			}
		}
		return
	}

	if body := p.field(n, "body"); body != nil {
		p.walkClassBody(body, qualified)
	}
}

func (p *Parser) baseListEntries(baseList *sitter.Node) []string {
	var entries []string
	for i := uint(0); i < baseList.NamedChildCount(); i++ {
		child := baseList.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "identifier", "generic_name", "qualified_name":
			entries = append(entries, p.text(child))
		}
	}
	return entries
}

func (p *Parser) handleEnumMember(member *sitter.Node, enumName string) {
	nameNode := p.field(member, "name")
	if nameNode == nil {
		return
	}
	memberName := p.text(nameNode)
	qualified := joinQualified(enumName, memberName)

	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityEnumMember,
		Description: fmt.Sprintf("%s.%s", enumName, memberName),
		FilePath:    p.sourcePath,
		LineNo:      startLine(member),
		EndLine:     endLine(member),
	})

	p.addRelation(Relation{
		Source:      enumName,
		Target:      qualified,
		Description: fmt.Sprintf("%s.%s", enumName, memberName),
		Keywords:    "property, has",
		Weight:      0.8,
	})
}

func (p *Parser) walkClassBody(body *sitter.Node, className string) {
	for i := uint(0); i < body.NamedChildCount(); i++ {
		member := body.NamedChild(i)
		if member == nil {
			continue
		}
		switch member.Kind() {
		case "method_declaration":
			p.handleMethod(member, className)
		case "constructor_declaration":
			p.handleConstructor(member, className)
		case "property_declaration":
			p.handleProperty(member, className)
		case "field_declaration":
			p.handleField(member, className)
		default:
			if info, ok := typeDeclKinds[member.Kind()]; ok {
				p.handleTypeDecl(member, info, className)
			}
		}
	}
}

// paramInfo captures a single parameter's type/name pair.
type paramInfo struct {
	typ  string
	name string
}

// listParams walks a parameter_list, returning one entry per declared
// parameter. Tolerates two emission styles in the v0.23.x grammar:
//   (parameter_list (parameter ...))                       — most parameters
//   (parameter_list type: ... name: ...)                  — `params` modifier
func (p *Parser) listParams(paramsNode *sitter.Node) []paramInfo {
	if paramsNode == nil {
		return nil
	}
	var out []paramInfo
	// Case 1: explicit parameter / parameter_array children.
	for i := uint(0); i < paramsNode.NamedChildCount(); i++ {
		c := paramsNode.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "parameter":
			typ := p.compactText(p.field(c, "type"))
			name := p.text(p.field(c, "name"))
			if typ != "" && name != "" {
				out = append(out, paramInfo{typ: typ, name: name})
			}
		case "parameter_array":
			typ := p.compactText(p.field(c, "type"))
			name := p.text(p.field(c, "name"))
			if typ != "" && name != "" {
				out = append(out, paramInfo{typ: typ, name: name})
			}
		}
	}
	// Case 2: type/name sit directly on parameter_list (the `params` quirk).
	if len(out) == 0 {
		typ := p.compactText(p.field(paramsNode, "type"))
		name := p.text(p.field(paramsNode, "name"))
		if typ != "" && name != "" {
			out = append(out, paramInfo{typ: typ, name: name})
		}
	}
	return out
}

// paramSignature rebuilds the "(a, b)" fragment used in descriptions.
// Preserves modifiers (`ref`, `out`, `in`, `this`, `params`), attributes
// (`[NotNull]`) and default values (`int x = 5`) exactly as written.
func (p *Parser) paramSignature(paramsNode *sitter.Node) string {
	if paramsNode == nil {
		return "()"
	}
	var parts []string
	// Case 1: explicit per-param children.
	for i := uint(0); i < paramsNode.NamedChildCount(); i++ {
		c := paramsNode.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "parameter" || c.Kind() == "parameter_array" {
			parts = append(parts, p.compactText(c))
		}
	}
	// Case 2: type/name sit directly on parameter_list (the `params` quirk).
	if len(parts) == 0 {
		typ := p.compactText(p.field(paramsNode, "type"))
		name := p.text(p.field(paramsNode, "name"))
		if typ != "" && name != "" {
			parts = append(parts, strings.TrimSpace(p.text(paramsNode)))
		}
	}
	if len(parts) == 0 {
		return "()"
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// compactText collapses whitespace/newlines inside a node's text.
func (p *Parser) compactText(n *sitter.Node) string {
	return strings.Join(strings.Fields(p.text(n)), " ")
}

func (p *Parser) handleMethod(n *sitter.Node, className string) {
	nameNode := p.field(n, "name")
	if nameNode == nil || className == "" {
		return
	}
	methodName := p.text(nameNode)
	qualified := joinQualified(className, methodName)
	p.emitCallable(n, className, methodName, qualified, "")

	scope := &methodScope{
		qualifiedName: qualified,
		className:     className,
		vars:          make(map[string]string),
	}
	p.collectParams(n, scope)

	if returnType := p.compactText(p.field(n, "returns")); returnType != "" &&
		returnType != "void" && returnType != "Void" {
		p.imports = append(p.imports, returnType)
	}

	if body := p.field(n, "body"); body != nil {
		p.walkMethodBody(body, scope)
	} else if arrow := p.field(n, "arrow_expression_clause"); arrow != nil {
		p.walkMethodBody(arrow, scope)
	}
}

// emitCallable creates the METHOD entity + defining relation + type refs.
// owner is the entity that defines it (class for methods, enclosing method
// for local functions); kindLabel only affects nothing today but keeps the
// description builder in one place.
func (p *Parser) emitCallable(n *sitter.Node, owner string, methodName, qualified, _ string) {
	paramsStr := p.paramSignature(p.field(n, "parameters"))
	returnType := p.compactText(p.field(n, "returns"))
	vis := p.visibilityFrom(n)
	visSuffix := ""
	if vis != "" {
		visSuffix = " [" + vis + "]"
	}

	desc := fmt.Sprintf("%s.%s%s%s", owner, methodName, paramsStr, visSuffix)
	if returnType != "" {
		desc = fmt.Sprintf("%s.%s%s -> RETURNS -> %s%s", owner, methodName, paramsStr, returnType, visSuffix)
	}

	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityMethod,
		Description: desc,
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
		Visibility:  vis,
	})

	p.addRelation(Relation{
		Source:      owner,
		Target:      qualified,
		Description: fmt.Sprintf("%s.%s%s", owner, methodName, paramsStr),
		Keywords:    "method, defines",
		Weight:      1.0,
	})

	if p.index != nil && returnType != "" && returnType != "void" && returnType != "Void" {
		resolvedType := p.resolveType(returnType)
		if resolvedType != returnType {
			if _, ok := p.index.Classes[resolvedType]; ok {
				p.addRelation(Relation{
					Source:      qualified,
					Target:      resolvedType,
					Description: fmt.Sprintf("%s.%s%s -> RETURNS -> %s", owner, methodName, paramsStr, resolvedType),
					Keywords:    "depends_on, references",
					Weight:      0.5,
				})
			}
		}
	}

	p.exports = append(p.exports, qualified)

	// depends_on edges for parameter types (resolvable ones only).
	for _, pi := range p.listParams(p.field(n, "parameters")) {
		if p.index == nil {
			continue
		}
		cleanType := stripModifiers(pi.typ)
		resolvedType := p.resolveType(cleanType)
		if resolvedType == pi.typ || resolvedType == cleanType {
			continue
		}
		if _, ok := p.index.Classes[resolvedType]; ok {
			p.addRelation(Relation{
				Source:      qualified,
				Target:      resolvedType,
				Description: fmt.Sprintf("%s.%s%s uses %s param %s", owner, methodName, paramsStr, resolvedType, pi.name),
				Keywords:    "depends_on",
				Weight:      0.3,
			})
		}
	}
}

// stripModifiers removes parameter modifiers that tree-sitter includes in
// the type field text ("ref string s" -> "string").
func stripModifiers(t string) string {
	fields := strings.Fields(t)
	for len(fields) > 1 {
		switch fields[0] {
		case "ref", "out", "in", "this", "params", "readonly":
			fields = fields[1:]
			continue
		}
		break
	}
	return strings.Join(fields, " ")
}

func (p *Parser) collectParams(n *sitter.Node, scope *methodScope) {
	for _, pi := range p.listParams(p.field(n, "parameters")) {
		cleanType := stripModifiers(pi.typ)
		p.imports = append(p.imports, cleanType)
		if scope.vars != nil {
			scope.vars[pi.name] = p.resolveType(cleanType)
		}
	}
}

func (p *Parser) handleConstructor(n *sitter.Node, className string) {
	nameNode := p.field(n, "name")
	if nameNode == nil || className == "" {
		return
	}
	ctorName := p.text(nameNode)
	qualified := joinQualified(className, ctorName)
	vis := p.visibilityFrom(n)
	visSuffix := ""
	if vis != "" {
		visSuffix = " [" + vis + "]"
	}
	paramsStr := p.paramSignature(p.field(n, "parameters"))

	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityConstructor,
		Description: fmt.Sprintf("%s%s%s", qualified, paramsStr, visSuffix),
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
		Visibility:  vis,
	})

	p.addRelation(Relation{
		Source:      className,
		Target:      qualified,
		Description: fmt.Sprintf("%s%s", qualified, paramsStr),
		Keywords:    "method, defines",
		Weight:      1.0,
	})

	scope := &methodScope{qualifiedName: qualified, className: className, vars: make(map[string]string)}
	p.collectParams(n, scope)
	if body := p.field(n, "body"); body != nil {
		p.walkMethodBody(body, scope)
	}
}

func (p *Parser) handleProperty(n *sitter.Node, className string) {
	nameNode := p.field(n, "name")
	if nameNode == nil || className == "" {
		return
	}
	propName := p.text(nameNode)
	qualified := joinQualified(className, propName)

	propType := p.compactText(p.field(n, "type"))
	vis := p.visibilityFrom(n)
	visSuffix := ""
	if vis != "" {
		visSuffix = " [" + vis + "]"
	}
	desc := fmt.Sprintf("%s.%s%s", className, propName, visSuffix)
	if propType != "" {
		desc = fmt.Sprintf("%s.%s -> RETURNS -> %s%s", className, propName, propType, visSuffix)
	}

	p.addEntity(Entity{
		Name:        qualified,
		Type:        EntityProperty,
		Description: desc,
		FilePath:    p.sourcePath,
		LineNo:      startLine(n),
		EndLine:     endLine(n),
		Visibility:  vis,
	})

	p.addRelation(Relation{
		Source:      className,
		Target:      qualified,
		Description: fmt.Sprintf("%s.%s", className, propName),
		Keywords:    "property, has",
		Weight:      0.8,
	})

	cleanType := stripModifiers(propType)
	if p.index != nil && cleanType != "" {
		resolvedType := p.resolveType(cleanType)
		if resolvedType != cleanType {
			if _, ok := p.index.Classes[resolvedType]; ok {
				p.addRelation(Relation{
					Source:      qualified,
					Target:      resolvedType,
					Description: fmt.Sprintf("%s.%s -> REFERENCES -> %s", className, propName, resolvedType),
					Keywords:    "depends_on, references",
					Weight:      0.5,
				})
			}
		}
	}
	if cleanType != "" {
		p.imports = append(p.imports, cleanType)
	}
}

func (p *Parser) handleField(n *sitter.Node, className string) {
	if className == "" {
		return
	}
	fieldType := p.compactText(p.field(n, "type"))
	vis := p.visibilityFrom(n)
	visSuffix := ""
	if vis != "" {
		visSuffix = " [" + vis + "]"
	}

	// One entity per declarator: "int a, b" produces two FIELD entities.
	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if child == nil || child.Kind() != "variable_declaration" {
			continue
		}
		for j := uint(0); j < child.NamedChildCount(); j++ {
			decl := child.NamedChild(j)
			if decl == nil || decl.Kind() != "variable_declarator" {
				continue
			}
			nameNode := p.field(decl, "name")
			if nameNode == nil {
				continue
			}
			fieldName := p.text(nameNode)
			qualified := joinQualified(className, fieldName)

			desc := fmt.Sprintf("%s.%s%s", className, fieldName, visSuffix)
			if fieldType != "" {
				desc = fmt.Sprintf("%s.%s -> RETURNS -> %s%s", className, fieldName, fieldType, visSuffix)
			}
			p.addEntity(Entity{
				Name:        qualified,
				Type:        EntityField,
				Description: desc,
				FilePath:    p.sourcePath,
				LineNo:      startLine(decl),
				EndLine:     endLine(decl),
				Visibility:  vis,
			})
			p.addRelation(Relation{
				Source:      className,
				Target:      qualified,
				Description: fmt.Sprintf("%s.%s", className, fieldName),
				Keywords:    "property, has",
				Weight:      0.8,
			})
			if p.index != nil && fieldType != "" {
				cleanType := stripModifiers(fieldType)
				resolvedType := p.resolveType(cleanType)
				if resolvedType != cleanType {
					if _, ok := p.index.Classes[resolvedType]; ok {
						p.addRelation(Relation{
							Source:      qualified,
							Target:      resolvedType,
							Description: fmt.Sprintf("%s.%s -> REFERENCES -> %s", className, fieldName, resolvedType),
							Keywords:    "depends_on, references",
							Weight:      0.5,
						})
					}
				}
			}
			if fieldType != "" {
				p.imports = append(p.imports, stripModifiers(fieldType))
			}
		}
	}
}

// walkMethodBody traverses everything inside a method/constructor/local
// function body: local functions become entities, invocations become call
// sites, local declarations feed the variable->type scope used to resolve
// `obj.Method()` calls.
func (p *Parser) walkMethodBody(n *sitter.Node, scope *methodScope) {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "local_function_statement":
			p.handleLocalFunction(child, scope)
		case "local_declaration_statement":
			p.registerLocals(child, scope)
		case "variable_declaration":
			p.registerVariableDeclaration(child, scope)
		case "for_each_statement":
			p.forEachVar(child, scope)
		case "invocation_expression":
			p.handleInvocation(child, scope)
		default:
			p.walkMethodBody(child, scope)
		}
	}
}

func (p *Parser) handleLocalFunction(n *sitter.Node, scope *methodScope) {
	nameNode := p.field(n, "name")
	if nameNode == nil || scope.qualifiedName == "" {
		return
	}
	fnName := p.text(nameNode)
	localQualified := joinQualified(scope.qualifiedName, fnName)

	p.emitCallable(n, scope.qualifiedName, fnName, localQualified, "")

	localScope := &methodScope{
		qualifiedName: localQualified,
		className:     scope.className,
		vars:          make(map[string]string),
	}
	p.collectParams(n, localScope)
	if body := p.field(n, "body"); body != nil {
		p.walkMethodBody(body, localScope)
	} else if arrow := p.field(n, "arrow_expression_clause"); arrow != nil {
		p.walkMethodBody(arrow, localScope)
	}
}

func (p *Parser) registerLocals(stmt *sitter.Node, scope *methodScope) {
	for i := uint(0); i < stmt.NamedChildCount(); i++ {
		child := stmt.NamedChild(i)
		if child != nil && child.Kind() == "variable_declaration" {
			p.registerVariableDeclaration(child, scope)
		}
	}
}

func (p *Parser) registerVariableDeclaration(decl *sitter.Node, scope *methodScope) {
	typeText := p.compactText(p.field(decl, "type"))
	if typeText == "" {
		return
	}
	resolved := p.resolveType(stripModifiers(typeText))
	for i := uint(0); i < decl.NamedChildCount(); i++ {
		declarator := decl.NamedChild(i)
		if declarator == nil || declarator.Kind() != "variable_declarator" {
			continue
		}
		nameNode := p.field(declarator, "name")
		if nameNode == nil {
			continue
		}
		varType := resolved
		// One-level `var` inference: var x = other; copies the declared
		// type of `other` so subsequent x.Method() calls resolve.
		if stripModifiers(typeText) == "var" {
			value := p.field(declarator, "value")
			if value == nil && declarator.NamedChildCount() > 1 {
				// Grammar quirk: the initializer is an unlabeled named child
				// after the name node.
				value = declarator.NamedChild(declarator.NamedChildCount() - 1)
			}
			if value != nil && value.Kind() == "identifier" {
				if t, ok := scope.vars[p.text(value)]; ok {
					varType = t
				}
			}
		}
		scope.vars[p.text(nameNode)] = varType
	}
}

// forEachVar records the loop variable of a foreach statement. Three
// shapes are possible:
//   - "foreach (var x in y)"          -> left is identifier, no known type
//   - "foreach (int x in y)"          -> left is declaration_expression
//   - "foreach (var (a, b) in pairs)" -> left is tuple_pattern (skipped)
func (p *Parser) forEachVar(stmt *sitter.Node, scope *methodScope) {
	left := p.field(stmt, "left")
	if left == nil {
		return
	}
	switch left.Kind() {
	case "declaration_expression":
		typeText := p.compactText(p.field(left, "type"))
		nameText := p.text(p.field(left, "name"))
		if typeText == "" || nameText == "" {
			return
		}
		scope.vars[nameText] = p.resolveType(stripModifiers(typeText))
	case "identifier":
		// Cannot resolve the inferred collection-element type without
		// type info; the variable is still tracked as "var" so a later
		// .Method() call won't make a wild guess about the receiver class.
		scope.vars[p.text(left)] = "var"
	}
}

// handleInvocation converts one invocation_expression into CallSites.
// Resolution rules (mirroring the old regex walker, now AST-exact):
//   - this.X()/base.X(): always recorded against the enclosing class
//   - this.X() with blank expression member_access → same class
//   - obj.X(): recorded when obj's declared type is known (params/locals)
//   - plain X(): treated as a same-class call
func (p *Parser) handleInvocation(inv *sitter.Node, scope *methodScope) {
	fn := p.field(inv, "function")
	if fn == nil {
		return
	}
	callee := ""
	calleeClass := ""

	switch fn.Kind() {
	case "member_access_expression":
		nameNode := p.field(fn, "name")
		if nameNode == nil {
			return
		}
		callee = p.text(nameNode)
		objNode := p.field(fn, "expression")
		switch {
		case objNode == nil:
			// this.X() / M() folded into the member_access form
			calleeClass = scope.className
		case objNode.Kind() == "this", objNode.Kind() == "base":
			calleeClass = scope.className
		default:
			obj := p.text(objNode)
			if isKeyword(obj) {
				return
			}
			if t, ok := scope.vars[obj]; ok {
				calleeClass = t
			} else {
				return // unknown receiver (BCL static, chained call, ...) - skip
			}
		}
	case "identifier":
		if scope.className == "" {
			return
		}
		callee = p.text(fn)
		if _, ok := scope.vars[callee]; ok {
			// Local function — emit anyway (already a METHOD entity).
		}
		calleeClass = scope.className
	default:
		// Conditional/other receivers: still descend into arguments below.
	}

	if callee != "" && calleeClass != "" && scope.qualifiedName != "" {
		p.calls = append(p.calls, CallSite{
			CallerFile:   p.sourcePath,
			CallerMethod: scope.qualifiedName,
			CalleeName:   calleeClass + "." + callee,
			CalleeClass:  calleeClass,
			LineNo:       startLine(inv),
		})
	}

	// Walk argument expressions for nested/chained invocations.
	if args := p.field(inv, "arguments"); args != nil {
		p.walkMethodBody(args, scope)
	}
}
