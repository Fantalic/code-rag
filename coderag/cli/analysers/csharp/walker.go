package csharpanalyser

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	classRegex       = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|abstract|sealed|partial|readonly|new|unsafe|async|extern|virtual|override)\s+)*(class|struct|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::\s*([A-Za-z_][A-Za-z0-9_<>,\[\].\s]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_<>,\[\].]*)*))?\s*\{?\s*$`)
	classSimpleRegex = regexp.MustCompile(`^\s*(class|struct|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::\s*([A-Za-z_][A-Za-z0-9_<>,\[\].\s]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_<>,\[\].]*)*))?\s*\{?\s*$`)
	methodRegex      = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|abstract|sealed|partial|readonly|new|unsafe|async|extern|virtual|override)\s+)*(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*(?:where\s|=>|\s*[{;])?`)
	propRegex        = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|new|virtual|override|abstract|sealed|readonly)\s+)*(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\{\s*(?:get|set)`)
	autoPropRegex    = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|new|virtual|override|abstract|sealed|readonly)\s+)*(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\{[^}]*\}`)
	callRegex        = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	thisCallRegex    = regexp.MustCompile(`(?:this|base)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	nsRegex          = regexp.MustCompile(`^\s*namespace\s+([A-Za-z_][A-Za-z0-9_.]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)
	usingRegex       = regexp.MustCompile(`^\s*(global\s+)?using\s+(?:static\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*;`)
	varDeclRegex     = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|readonly|new|volatile|async)\s+)*(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*[=;]`)

	reAttr       = regexp.MustCompile(`^\s*\[`)
	reComment    = regexp.MustCompile(`^\s*//`)
	reRegion     = regexp.MustCompile(`^\s*#`)
	reInheritSep = regexp.MustCompile(`\s*,\s*`)
)

type classContext struct {
	name       string
	depth      int
	baseType   string
	interfaces []string
	lineNo     int
}

type Parser struct {
	sourcePath string
	content    string
	lines      []string

	entities  []Entity
	relations []Relation
	calls     []CallSite
	exports   []string
	imports   []string

	namespace    string
	usings       []string
	globalUsings []string

	braceDepth int
	classStack []classContext

	inMethod    bool
	methodName  string
	methodLine  int
	methodDepth int
	methodBody  string

	scopeVars []map[string]string

	index *GlobalIndex
}

func (p *Parser) currentClass() string {
	if len(p.classStack) == 0 {
		return ""
	}
	return p.classStack[len(p.classStack)-1].name
}

func (p *Parser) resolveType(simpleName string) string {
	if p.index == nil {
		return simpleName
	}
	// 1. Current namespace
	if p.namespace != "" {
		candidate := p.namespace + "." + simpleName
		if _, ok := p.index.Classes[candidate]; ok {
			return candidate
		}
	}
	// 2. File-level usings
	for _, u := range p.usings {
		candidate := u + "." + simpleName
		if _, ok := p.index.Classes[candidate]; ok {
			return candidate
		}
	}
	// 3. Global usings (from index)
	for _, u := range p.index.GlobalUsings {
		candidate := u + "." + simpleName
		if _, ok := p.index.Classes[candidate]; ok {
			return candidate
		}
	}
	// 4. Exact match (already fully-qualified or global namespace)
	if _, ok := p.index.Classes[simpleName]; ok {
		return simpleName
	}
	// 5. Unique simple-name match across index (fallback)
	if matches, ok := p.index.SimpleNames[simpleName]; ok && len(matches) == 1 {
		return matches[0]
	}
	return simpleName
}

func ParseFile(filePath string, index *GlobalIndex) (*FileResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	// Strip UTF-8 BOM
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	content := string(data)
	p := &Parser{
		sourcePath: filePath,
		content:    content,
		lines:      strings.Split(content, "\n"),
		scopeVars:  []map[string]string{{}},
		index:      index,
	}

	p.parse()
	return &FileResult{
		FilePath:     filePath,
		Entities:     p.entities,
		Relations:    p.relations,
		Exports:      p.exports,
		Imports:      p.imports,
		Calls:        p.calls,
		GlobalUsings: p.globalUsings,
	}, nil
}

func (p *Parser) parse() {
	for i := 0; i < len(p.lines); i++ {
		line := p.lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || reComment.MatchString(trimmed) || reAttr.MatchString(trimmed) || reRegion.MatchString(trimmed) {
			p.countBraces(trimmed)
			continue
		}

		if nsMatch := nsRegex.FindStringSubmatch(trimmed); nsMatch != nil {
			p.namespace = nsMatch[1]
			continue
		}

		if usingMatch := usingRegex.FindStringSubmatch(trimmed); usingMatch != nil {
			ns := usingMatch[2]
			p.usings = append(p.usings, ns)
			p.imports = append(p.imports, ns)
			if usingMatch[1] != "" {
				p.globalUsings = append(p.globalUsings, ns)
			}
			continue
		}

		if !p.inMethod {
			if m := classRegex.FindStringSubmatch(trimmed); m != nil {
				p.handleClassDecl(m, i, trimmed)
				continue
			}
			if m := classSimpleRegex.FindStringSubmatch(trimmed); m != nil {
				p.handleClassDecl(append([]string{""}, m[1:]...), i, trimmed)
				continue
			}
		}

		if len(p.classStack) > 0 && !p.inMethod {
			if m := methodRegex.FindStringSubmatch(trimmed); m != nil && p.looksLikeMethod(trimmed) {
				p.handleMethodDecl(m, i)
				continue
			}
			if m := propRegex.FindStringSubmatch(trimmed); m != nil {
				p.handlePropDecl(m, i)
				continue
			}
			if m := autoPropRegex.FindStringSubmatch(trimmed); m != nil {
				p.handlePropDecl(m, i)
				continue
			}
			if v := varDeclRegex.FindStringSubmatch(trimmed); v != nil && v[1] != "" && v[2] != "" && !isKeyword(v[1]) {
				// class-level field -> treat as property
				p.handlePropDecl(v, i)
				continue
			}
		}

		if p.inMethod {
			if v := varDeclRegex.FindStringSubmatch(trimmed); v != nil && v[1] != "" {
				p.scopeVars[len(p.scopeVars)-1][v[2]] = p.resolveType(v[1])
			}
			p.extractCalls(trimmed, i)
		}

		p.countBraces(trimmed)
	}
}

func (p *Parser) handleClassDecl(m []string, lineNo int, line string) {
	kind := m[1]
	name := m[2]
	baseClause := ""
	if len(m) >= 4 && m[3] != "" {
		baseClause = strings.TrimSpace(m[3])
	}

	// Build fully-qualified name: prefix top-level types with namespace, nest others under parent
	qualifiedClassName := name
	if len(p.classStack) >= 1 {
		parent := p.classStack[len(p.classStack)-1].name
		qualifiedClassName = parent + "." + name
	} else if p.namespace != "" {
		qualifiedClassName = p.namespace + "." + name
	}

	ctx := classContext{
		name:       qualifiedClassName,
		depth:      p.braceDepth,
		baseType:   "",
		interfaces: nil,
		lineNo:     lineNo,
	}

	if baseClause != "" {
		parts := reInheritSep.Split(baseClause, -1)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			// Strip generic suffixes for name matching
			simplePart := part
			if idx := strings.IndexAny(simplePart, "<"); idx >= 0 {
				simplePart = simplePart[:idx]
			}
			// Detect interfaces by "I" prefix convention + uppercase second char
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

	p.classStack = append(p.classStack, ctx)

	entityType := EntityClass
	switch kind {
	case "interface":
		entityType = EntityInterface
	case "struct":
		entityType = EntityStruct
	case "enum":
		entityType = EntityEnum
	}

	entityName := qualifiedClassName

	p.addEntity(Entity{
		Name:        entityName,
		Type:        entityType,
		Description: fmt.Sprintf("%s %s", kind, name),
		FilePath:    p.sourcePath,
		LineNo:      lineNo + 1,
	})
	p.exports = append(p.exports, entityName)

	// Add relation from parent class to nested type
	if len(p.classStack) >= 2 {
		parent := p.classStack[len(p.classStack)-2].name
		p.addRelation(Relation{
			Source:      parent,
			Target:      entityName,
			Description: fmt.Sprintf("%s has nested type %s", parent, name),
			Keywords:    "contains, nested",
			Weight:      0.7,
		})
	}

	// Emit inheritance/implements relations via GlobalIndex
	if p.index != nil {
		if ctx.baseType != "" {
			resolved := p.resolveType(ctx.baseType)
			if _, ok := p.index.Classes[resolved]; ok {
				p.addRelation(Relation{
					Source:      entityName,
					Target:      resolved,
					Description: fmt.Sprintf("%s extends %s", entityName, resolved),
					Keywords:    "inheritance, extends",
					Weight:      1.0,
				})
			}
		}
		for _, iface := range ctx.interfaces {
			resolved := p.resolveType(iface)
			if _, ok := p.index.Classes[resolved]; ok {
				p.addRelation(Relation{
					Source:      entityName,
					Target:      resolved,
					Description: fmt.Sprintf("%s implements %s", entityName, resolved),
					Keywords:    "implements",
					Weight:      1.0,
				})
			}
		}
	}

	p.countBraces(line)
}

func (p *Parser) looksLikeMethod(line string) bool {
	if strings.Contains(line, "=>") || strings.HasSuffix(strings.TrimSpace(line), "{") {
		return true
	}
	return strings.Contains(line, "(") && strings.Contains(line, ")")
}

func extractVisibility(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "public"):
		return "public"
	case strings.HasPrefix(trimmed, "private"):
		return "private"
	case strings.HasPrefix(trimmed, "protected internal"):
		return "protected internal"
	case strings.HasPrefix(trimmed, "protected"):
		return "protected"
	case strings.HasPrefix(trimmed, "internal"):
		return "internal"
	default:
		return ""
	}
}

func (p *Parser) handleMethodDecl(m []string, lineNo int) {
	returnType := strings.TrimSpace(m[1])
	methodName := strings.TrimSpace(m[2])
	params := strings.TrimSpace(m[3])

	cls := p.currentClass()
	if cls == "" {
		return
	}

	if methodName == "" {
		return
	}

	// Skip constructors (method name matches unqualified class name)
	unqualifiedCls := cls
	if idx := strings.LastIndex(cls, "."); idx >= 0 {
		unqualifiedCls = cls[idx+1:]
	}
	if methodName == unqualifiedCls {
		return
	}

	qualifiedName := cls + "." + methodName

	p.methodName = qualifiedName
	p.methodLine = lineNo
	p.methodBody = ""

	vis := extractVisibility(p.lines[lineNo])
	visSuffix := ""
	if vis != "" {
		visSuffix = " [" + vis + "]"
	}
	paramStr := "()"
	if params != "" {
		paramStr = "(" + params + ")"
	}
	desc := fmt.Sprintf("%s.%s%s%s", cls, methodName, paramStr, visSuffix)
	if returnType != "" {
		desc = fmt.Sprintf("%s.%s%s -> RETURNS -> %s%s", cls, methodName, paramStr, returnType, visSuffix)
	}

	p.addEntity(Entity{
		Name:        qualifiedName,
		Type:        EntityMethod,
		Description: desc,
		FilePath:    p.sourcePath,
		LineNo:      lineNo + 1,
	})

	p.addRelation(Relation{
		Source:      cls,
		Target:      qualifiedName,
		Description: fmt.Sprintf("%s.%s%s", cls, methodName, paramStr),
		Keywords:    "method, defines",
		Weight:      1.0,
	})

	if returnType != "" && returnType != "void" && returnType != "Void" && p.index != nil {
		resolvedType := p.resolveType(returnType)
		if resolvedType != returnType {
			if _, ok := p.index.Classes[resolvedType]; ok {
				p.addRelation(Relation{
					Source:      qualifiedName,
					Target:      resolvedType,
					Description: fmt.Sprintf("%s.%s%s -> RETURNS -> %s", cls, methodName, paramStr, resolvedType),
					Keywords:    "depends_on, references",
					Weight:      0.5,
				})
			}
		}
	}

	p.exports = append(p.exports, qualifiedName)

	isExpressionBody := strings.Contains(p.lines[lineNo], "=>")

	p.scopeVars = append(p.scopeVars, make(map[string]string))
	if params != "" {
		for _, pair := range splitParams(params) {
			paramType, paramName, ok := parseParamDecl(pair)
			if !ok || paramName == "" {
				continue
			}
			resolvedType := p.resolveType(paramType)
			p.scopeVars[len(p.scopeVars)-1][paramName] = resolvedType
			p.imports = append(p.imports, paramType)
			if p.index != nil && resolvedType != paramType {
				if _, ok := p.index.Classes[resolvedType]; ok {
					p.addRelation(Relation{
						Source:      qualifiedName,
						Target:      resolvedType,
						Description: fmt.Sprintf("%s.%s%s uses %s param %s", cls, methodName, paramStr, resolvedType, paramName),
						Keywords:    "depends_on",
						Weight:      0.3,
					})
				}
			}
		}
	}

	if returnType != "" && returnType != "void" && returnType != "Void" {
		p.imports = append(p.imports, returnType)
	}

	if isExpressionBody {
		p.scopeVars = p.scopeVars[:len(p.scopeVars)-1]
		return
	}

	p.inMethod = true
}

func (p *Parser) handlePropDecl(m []string, lineNo int) {
	propType := strings.TrimSpace(m[1])
	propName := strings.TrimSpace(m[2])
	cls := p.currentClass()

	if propName == "" || cls == "" {
		return
	}

	qualifiedName := cls + "." + propName

	vis := extractVisibility(p.lines[lineNo])
	visSuffix := ""
	if vis != "" {
		visSuffix = " [" + vis + "]"
	}
	desc := fmt.Sprintf("%s.%s%s", cls, propName, visSuffix)
	if propType != "" {
		desc = fmt.Sprintf("%s.%s -> RETURNS -> %s%s", cls, propName, propType, visSuffix)
	}

	p.addEntity(Entity{
		Name:        qualifiedName,
		Type:        EntityProperty,
		Description: desc,
		FilePath:    p.sourcePath,
		LineNo:      lineNo + 1,
	})

	p.addRelation(Relation{
		Source:      cls,
		Target:      qualifiedName,
		Description: fmt.Sprintf("%s.%s", cls, propName),
		Keywords:    "property, has",
		Weight:      0.8,
	})

	if propType != "" && p.index != nil {
		resolvedType := p.resolveType(propType)
		if resolvedType != propType {
			if _, ok := p.index.Classes[resolvedType]; ok {
				p.addRelation(Relation{
					Source:      qualifiedName,
					Target:      resolvedType,
					Description: fmt.Sprintf("%s.%s -> REFERENCES -> %s", cls, propName, resolvedType),
					Keywords:    "depends_on, references",
					Weight:      0.5,
				})
			}
		}
	}

	if propType != "" {
		p.imports = append(p.imports, propType)
	}
}

func (p *Parser) extractCalls(line string, lineNo int) {
	if strings.HasPrefix(strings.TrimSpace(line), "//") {
		return
	}

	currentCls := p.currentClass()

	if tm := thisCallRegex.FindStringSubmatch(line); tm != nil {
		callee := tm[1]
		qualifiedCallee := currentCls + "." + callee
		p.calls = append(p.calls, CallSite{
			CallerFile:   p.sourcePath,
			CallerMethod: p.methodName,
			CalleeName:   qualifiedCallee,
			CalleeClass:  currentCls,
			LineNo:       lineNo + 1,
		})
	}

	cm := callRegex.FindAllStringSubmatch(line, -1)
	for _, c := range cm {
		objName := c[1]
		callee := c[2]

		if objName == "this" || objName == "base" {
			continue
		}

		if isKeyword(objName) {
			continue
		}

		calleeClass := ""
		for i := len(p.scopeVars) - 1; i >= 0; i-- {
			if t, ok := p.scopeVars[i][objName]; ok {
				calleeClass = t
				break
			}
		}

		if calleeClass == "" {
			continue
		}

		qualifiedCallee := calleeClass + "." + callee
		p.calls = append(p.calls, CallSite{
			CallerFile:   p.sourcePath,
			CallerMethod: p.methodName,
			CalleeName:   qualifiedCallee,
			CalleeClass:  calleeClass,
			LineNo:       lineNo + 1,
		})
	}
}

func (p *Parser) countBraces(line string) {
	for _, ch := range line {
		switch ch {
		case '{':
			p.braceDepth++
			if p.inMethod {
				p.methodDepth++
			}
		case '}':
			p.braceDepth--
			if p.inMethod {
				p.methodDepth--
				if p.methodDepth <= 0 {
					p.inMethod = false
					p.scopeVars = p.scopeVars[:len(p.scopeVars)-1]
				}
			}
			if p.braceDepth < 0 {
				p.braceDepth = 0
			}
			if len(p.classStack) > 0 && p.braceDepth == p.classStack[len(p.classStack)-1].depth {
				p.classStack = p.classStack[:len(p.classStack)-1]
			}
		}
	}
}

func (p *Parser) addEntity(e Entity) {
	p.entities = append(p.entities, e)
}

func (p *Parser) addRelation(r Relation) {
	p.relations = append(p.relations, r)
}

func (p *Parser) isInImports(s string) bool {
	for _, u := range p.usings {
		if strings.Contains(u, s) {
			return true
		}
	}
	return p.namespace != "" && strings.Contains(p.namespace, s)
}

func balancedClose(s string, openChar, closeChar byte) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case openChar:
			depth++
		case closeChar:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func findTopLevelChar(s string, c byte) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '<', '[':
			depth++
		case ')', '>', ']':
			if depth > 0 {
				depth--
			}
		case c:
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitParams(s string) []string {
	var result []string
	start := 0
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '<', '[':
			depth++
		case ')', '>', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	rest := strings.TrimSpace(s[start:])
	if rest != "" {
		result = append(result, rest)
	}
	return result
}

func parseParamDecl(s string) (typeName, paramName string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}
	for strings.HasPrefix(s, "[") {
		end := balancedClose(s, '[', ']')
		if end < 0 {
			return "", "", false
		}
		s = strings.TrimSpace(s[end+1:])
	}
	lower := strings.ToLower(s)
	for _, mod := range []string{"out ", "ref ", "in ", "params ", "this "} {
		if strings.HasPrefix(lower, mod) {
			s = strings.TrimSpace(s[len(mod):])
			lower = strings.ToLower(s)
			break
		}
	}
	if eq := findTopLevelChar(s, '='); eq >= 0 {
		s = strings.TrimSpace(s[:eq])
	}
	if strings.TrimSpace(s) == "" {
		return "", "", false
	}
	re := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*\s*$`)
	loc := re.FindStringIndex(s)
	if loc == nil {
		return "", "", false
	}
	paramName = s[loc[0]:loc[1]]
	typeName = strings.TrimSpace(s[:loc[0]])
	if typeName == "" {
		return "", "", false
	}
	return typeName, paramName, true
}

var keywords = map[string]bool{
	"if": true, "else": true, "for": true, "foreach": true, "while": true,
	"do": true, "switch": true, "return": true, "throw": true, "yield": true,
	"await": true, "new": true, "var": true, "base": true, "this": true,
	"in": true, "out": true, "ref": true, "sizeof": true, "typeof": true,
	"nameof": true, "true": true, "false": true, "null": true, "default": true,
}

func isKeyword(s string) bool {
	return keywords[s]
}

func ReadSourceFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func FileNameOnly(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".cs")
}
