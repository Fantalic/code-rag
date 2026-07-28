package csharpanalyser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	classRegex       = regexp.MustCompile(`^\s*(?:public|private|protected|internal|static|abstract|sealed|partial|readonly|new|unsafe|async|extern|virtual|override)\s+(class|struct|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::\s*([A-Za-z_][A-Za-z0-9_<>,\[\].\s]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_<>,\[\].]*)*))?\s*\{?\s*$`)
	classSimpleRegex = regexp.MustCompile(`^\s*(class|struct|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::\s*([A-Za-z_][A-Za-z0-9_<>,\[\].\s]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_<>,\[\].]*)*))?\s*\{?\s*$`)
	methodRegex      = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|abstract|sealed|partial|readonly|new|unsafe|async|extern|virtual|override)\s+)*(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*(?:where\s|=>|\s*[{;])?`)
	propRegex        = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|new|virtual|override|abstract|sealed|readonly)\s+)*(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\{\s*(?:get|set)`)
	autoPropRegex    = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|new|virtual|override|abstract|sealed|readonly)\s+)*(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\{[^}]*\}`)
	callRegex        = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	thisCallRegex    = regexp.MustCompile(`(?:this|base)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	nsRegex          = regexp.MustCompile(`^\s*namespace\s+([A-Za-z_][A-Za-z0-9_.]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)
	usingRegex       = regexp.MustCompile(`^\s*using\s+(?:static\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*;`)
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

	namespace string
	usings    []string

	braceDepth int
	classStack []classContext

	inMethod    bool
	methodName  string
	methodLine  int
	methodDepth int
	methodBody  string

	scopeVars []map[string]string
}

func (p *Parser) currentClass() string {
	if len(p.classStack) == 0 {
		return ""
	}
	return p.classStack[len(p.classStack)-1].name
}

func ParseFile(filePath string) (*FileResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	content := string(data)
	p := &Parser{
		sourcePath: filePath,
		content:    content,
		lines:      strings.Split(content, "\n"),
		scopeVars:  []map[string]string{{}},
	}

	p.parse()
	return &FileResult{
		FilePath:  filePath,
		Entities:  p.entities,
		Relations: p.relations,
		Exports:   p.exports,
		Imports:   p.imports,
		Calls:     p.calls,
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
			p.usings = append(p.usings, usingMatch[1])
			p.imports = append(p.imports, usingMatch[1])
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
			if v := varDeclRegex.FindStringSubmatch(trimmed); v != nil && v[1] != "" && v[2] != "" {
				// class-level field -> treat as property
				p.handlePropDecl(v, i)
				continue
			}
		}

		if p.inMethod {
			if v := varDeclRegex.FindStringSubmatch(trimmed); v != nil && v[1] != "" {
				p.scopeVars[len(p.scopeVars)-1][v[2]] = v[1]
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

	qualifiedClassName := name
	if len(p.classStack) >= 1 {
		parent := p.classStack[len(p.classStack)-1].name
		qualifiedClassName = parent + "." + name
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
			part = strings.TrimPrefix(part, "I")
			if strings.HasPrefix(part, "I") && part != "" && part[0] >= 'A' && part[0] <= 'Z' {
				rPart := "I" + part
				if !p.isInImports(part) && part != name {
					ctx.interfaces = append(ctx.interfaces, rPart)
					p.imports = append(p.imports, rPart)
				}
			} else if part != "" && part != name {
				ctx.baseType = part
				p.imports = append(p.imports, part)
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

	entityName := name
	if len(p.classStack) >= 2 {
		parent := p.classStack[len(p.classStack)-2].name
		entityName = parent + "." + name
	}

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

	p.countBraces(line)
}

func (p *Parser) looksLikeMethod(line string) bool {
	if strings.Contains(line, "=>") || strings.HasSuffix(strings.TrimSpace(line), "{") {
		return true
	}
	return strings.Contains(line, "(") && strings.Contains(line, ")")
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

	desc := fmt.Sprintf("%s.%s()", cls, methodName)
	if returnType != "" {
		desc = fmt.Sprintf("%s.%s() -> RETURNS -> %s", cls, methodName, returnType)
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
		Description: fmt.Sprintf("%s.%s()", cls, methodName),
		Keywords:    "method, defines",
		Weight:      1.0,
	})

	p.exports = append(p.exports, qualifiedName)

	isExpressionBody := strings.Contains(p.lines[lineNo], "=>")

	p.scopeVars = append(p.scopeVars, make(map[string]string))
	if params != "" {
		paramPairs := strings.Split(params, ",")
		for _, pair := range paramPairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			parts := strings.Fields(pair)
			if len(parts) >= 2 {
				paramType := parts[0]
				paramName := parts[len(parts)-1]
				p.scopeVars[len(p.scopeVars)-1][paramName] = paramType
				p.imports = append(p.imports, paramType)
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

	desc := fmt.Sprintf("%s.%s", cls, propName)
	if propType != "" {
		desc = fmt.Sprintf("%s.%s -> RETURNS -> %s", cls, propName, propType)
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

		qualifiedCallee := callee
		if calleeClass != "" {
			qualifiedCallee = calleeClass + "." + callee
		}

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
