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
	varDeclRegex     = regexp.MustCompile(`^\s*(?:public|private|protected|internal|static|readonly|new|volatile|async)\s+(?:([A-Za-z_][A-Za-z0-9_<>,\[\]]*(?:\?)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*[=;]`)

	reAttr       = regexp.MustCompile(`^\s*\[`)
	reComment    = regexp.MustCompile(`^\s*//`)
	reRegion     = regexp.MustCompile(`^\s*#`)
	reInheritSep = regexp.MustCompile(`\s*,\s*`)
)

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
	inClass    bool
	classDepth int
	className  string
	baseType   string
	interfaces []string
	classLine  int

	inMethod    bool
	methodName  string
	methodLine  int
	methodDepth int
	methodBody  string

	scopeVars []map[string]string
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

		if !p.inClass && !p.inMethod {
			if m := classRegex.FindStringSubmatch(trimmed); m != nil {
				p.handleClassDecl(m, i, trimmed)
				continue
			}
			if m := classSimpleRegex.FindStringSubmatch(trimmed); m != nil {
				p.handleClassDecl(append([]string{""}, m[1:]...), i, trimmed)
				continue
			}
		}

		if p.inClass && !p.inMethod {
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
	if p.inClass {
		return
	}

	kind := m[1]
	name := m[2]
	baseClause := ""
	if len(m) >= 4 && m[3] != "" {
		baseClause = strings.TrimSpace(m[3])
	}

	p.className = name
	p.baseType = ""
	p.interfaces = nil
	p.classLine = lineNo
	p.inClass = true
	p.classDepth = p.braceDepth

	if baseClause != "" {
		parts := reInheritSep.Split(baseClause, -1)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			part = strings.TrimPrefix(part, "I")
			if strings.HasPrefix(part, "I") && part != "" && part[0] >= 'A' && part[0] <= 'Z' {
				rPart := "I" + part
				if !p.isInImports(part) && part != name {
					p.interfaces = append(p.interfaces, rPart)
					p.imports = append(p.imports, rPart)
				}
			} else if part != "" && part != name {
				p.baseType = part
				p.imports = append(p.imports, part)
			}
		}
	}

	entityType := EntityClass
	switch kind {
	case "interface":
		entityType = EntityInterface
	case "struct":
		entityType = EntityStruct
	case "enum":
		entityType = EntityEnum
	}

	p.addEntity(Entity{
		Name:        name,
		Type:        entityType,
		Description: fmt.Sprintf("%s %s", kind, name),
		FilePath:    p.sourcePath,
		LineNo:      lineNo + 1,
	})
	p.exports = append(p.exports, name)

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

	if methodName == "" || methodName == p.className {
		return
	}

	p.methodName = methodName
	p.methodLine = lineNo
	p.methodBody = ""

	desc := fmt.Sprintf("%s.%s()", p.className, methodName)
	if returnType != "" {
		desc = fmt.Sprintf("%s.%s() -> RETURNS -> %s", p.className, methodName, returnType)
	}

	p.addEntity(Entity{
		Name:        methodName,
		Type:        EntityMethod,
		Description: desc,
		FilePath:    p.sourcePath,
		LineNo:      lineNo + 1,
	})

	p.addRelation(Relation{
		Source:      p.className,
		Target:      methodName,
		Description: fmt.Sprintf("%s.%s()", p.className, methodName),
		Keywords:    "method, defines",
		Weight:      1.0,
	})

	relMethodName := fmt.Sprintf("%s.%s", p.className, methodName)
	p.exports = append(p.exports, relMethodName)

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

	if propName == "" || p.className == "" {
		return
	}

	desc := fmt.Sprintf("%s.%s", p.className, propName)
	if propType != "" {
		desc = fmt.Sprintf("%s.%s -> RETURNS -> %s", p.className, propName, propType)
	}

	p.addEntity(Entity{
		Name:        propName,
		Type:        EntityProperty,
		Description: desc,
		FilePath:    p.sourcePath,
		LineNo:      lineNo + 1,
	})

	p.addRelation(Relation{
		Source:      p.className,
		Target:      propName,
		Description: fmt.Sprintf("%s.%s", p.className, propName),
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

	if tm := thisCallRegex.FindStringSubmatch(line); tm != nil {
		callee := tm[1]
		p.calls = append(p.calls, CallSite{
			CallerFile:   p.sourcePath,
			CallerMethod: p.methodName,
			CalleeName:   callee,
			CalleeClass:  p.className,
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

		p.calls = append(p.calls, CallSite{
			CallerFile:   p.sourcePath,
			CallerMethod: p.methodName,
			CalleeName:   callee,
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
			if p.inClass && p.braceDepth == p.classDepth {
				p.inClass = false
				p.className = ""
				p.baseType = ""
				p.interfaces = nil
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
