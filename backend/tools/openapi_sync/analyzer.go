package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const (
	apiImportPath    = "incidenthub/backend/internal/api"
	modelsImportPath = "incidenthub/backend/internal/models"
)

var pathParamPattern = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

type routeSpec struct {
	Method         string
	Path           string
	HandlerName    string
	Secured        bool
	RequiresTenant bool
}

type groupState struct {
	Prefix         string
	RequiresTenant bool
}

type moduleLoader struct {
	rootDir    string
	modulePath string
	packages   map[string]*parsedPackage
}

type parsedPackage struct {
	Path    string
	Dir     string
	Files   []*parsedFile
	Types   map[string]*typeDecl
	Funcs   map[string]*funcDecl
	Methods map[string]map[string]*funcDecl
}

type parsedFile struct {
	Pkg     *parsedPackage
	Path    string
	AST     *ast.File
	Imports map[string]string
}

type typeDecl struct {
	Name string
	Spec *ast.TypeSpec
	File *parsedFile
}

type funcDecl struct {
	Name     string
	Receiver string
	Decl     *ast.FuncDecl
	File     *parsedFile
}

type typeRef struct {
	PkgPath string
	File    *parsedFile
	Expr    ast.Expr
}

type namedTypeKey struct {
	PkgPath string
	Name    string
}

type schemaBuilder struct {
	loader     *moduleLoader
	components map[string]any
	names      map[namedTypeKey]string
	usedNames  map[string]namedTypeKey
	building   map[namedTypeKey]bool
}

type operationAnalysis struct {
	QueryParams map[string]map[string]any
	RequestBody map[string]any
	Responses   map[string]map[string]any
}

type responseAccumulator struct {
	ContentType string
	Schemas     []map[string]any
	Description string
}

type valueBinding struct {
	Type      *typeRef
	Schema    map[string]any
	Context   bool
	Request   bool
	Multipart bool
}

type scope struct {
	parent   *scope
	bindings map[string]valueBinding
}

type funcCollector struct {
	gen             *openAPIGenerator
	fn              *funcDecl
	scope           *scope
	queryParams     map[string]map[string]any
	errorCodes      map[string]struct{}
	responses       map[string]*responseAccumulator
	requestBody     map[string]any
	formFields      map[string]map[string]any
	fileFields      map[string]bool
	multiFileFields map[string]bool
	hasMultipart    bool
	helperVisits    map[string]struct{}
}

type openAPIGenerator struct {
	loader        *moduleLoader
	apiPkg        *parsedPackage
	schemaBuilder *schemaBuilder
	handlerFields map[string]typeRef
}

func newModuleLoader(rootDir, modulePath string) *moduleLoader {
	return &moduleLoader{
		rootDir:    rootDir,
		modulePath: modulePath,
		packages:   map[string]*parsedPackage{},
	}
}

func (l *moduleLoader) loadPackage(importPath string) (*parsedPackage, error) {
	if pkg, ok := l.packages[importPath]; ok {
		return pkg, nil
	}
	if importPath != l.modulePath && !strings.HasPrefix(importPath, l.modulePath+"/") {
		return nil, fmt.Errorf("package %s is outside module", importPath)
	}
	rel := strings.TrimPrefix(importPath, l.modulePath)
	rel = strings.TrimPrefix(rel, "/")
	dir := filepath.Join(l.rootDir, rel)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	astPkg, ok := pkgs[filepath.Base(dir)]
	if !ok {
		for _, candidate := range pkgs {
			astPkg = candidate
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("package %s not found in %s", importPath, dir)
	}

	pkg := &parsedPackage{
		Path:    importPath,
		Dir:     dir,
		Files:   make([]*parsedFile, 0, len(astPkg.Files)),
		Types:   map[string]*typeDecl{},
		Funcs:   map[string]*funcDecl{},
		Methods: map[string]map[string]*funcDecl{},
	}

	for filePath, fileAST := range astPkg.Files {
		parsed := &parsedFile{
			Pkg:     pkg,
			Path:    filePath,
			AST:     fileAST,
			Imports: map[string]string{},
		}
		for _, imp := range fileAST.Imports {
			pathValue, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			alias := defaultImportAlias(pathValue)
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if alias == "_" || alias == "." || alias == "" {
				continue
			}
			parsed.Imports[alias] = pathValue
		}
		for _, decl := range fileAST.Decls {
			switch typed := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range typed.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					pkg.Types[typeSpec.Name.Name] = &typeDecl{
						Name: typeSpec.Name.Name,
						Spec: typeSpec,
						File: parsed,
					}
				}
			case *ast.FuncDecl:
				entry := &funcDecl{
					Name: typed.Name.Name,
					Decl: typed,
					File: parsed,
				}
				if typed.Recv != nil && len(typed.Recv.List) > 0 {
					entry.Receiver = receiverName(typed.Recv.List[0].Type)
					if _, ok := pkg.Methods[entry.Receiver]; !ok {
						pkg.Methods[entry.Receiver] = map[string]*funcDecl{}
					}
					pkg.Methods[entry.Receiver][entry.Name] = entry
				} else {
					pkg.Funcs[entry.Name] = entry
				}
			}
		}
		pkg.Files = append(pkg.Files, parsed)
	}

	l.packages[importPath] = pkg
	return pkg, nil
}

func defaultImportAlias(pathValue string) string {
	base := filepath.Base(pathValue)
	if len(base) > 1 && base[0] == 'v' {
		allDigits := true
		for _, r := range base[1:] {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			dir := filepath.Dir(pathValue)
			return filepath.Base(dir)
		}
	}
	return base
}

func receiverName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.IndexExpr:
		return receiverName(typed.X)
	case *ast.IndexListExpr:
		return receiverName(typed.X)
	default:
		return ""
	}
}

func discoverRoutes(apiPkg *parsedPackage) ([]routeSpec, error) {
	methods := apiPkg.Methods["Handler"]
	if methods == nil {
		return nil, fmt.Errorf("Handler methods not found")
	}
	registerFn := methods["Register"]
	if registerFn == nil || registerFn.Decl.Body == nil {
		return nil, fmt.Errorf("Handler.Register not found")
	}

	groups := map[string]groupState{
		registerFn.Decl.Type.Params.List[0].Names[0].Name: {Prefix: ""},
	}
	routes := make([]routeSpec, 0, 128)
	collectRouteStatements(registerFn.Decl.Body.List, groups, &routes)
	slices.SortFunc(routes, func(a, b routeSpec) int {
		if cmp.Compare(a.Path, b.Path) != 0 {
			return cmp.Compare(a.Path, b.Path)
		}
		return cmp.Compare(a.Method, b.Method)
	})
	return routes, nil
}

func collectRouteStatements(stmts []ast.Stmt, groups map[string]groupState, routes *[]routeSpec) {
	for _, stmt := range stmts {
		switch typed := stmt.(type) {
		case *ast.AssignStmt:
			if len(typed.Lhs) != 1 || len(typed.Rhs) != 1 {
				continue
			}
			name, ok := typed.Lhs[0].(*ast.Ident)
			if !ok {
				continue
			}
			call, ok := typed.Rhs[0].(*ast.CallExpr)
			if !ok {
				continue
			}
			group, ok := parseGroupCall(call, groups)
			if ok {
				groups[name.Name] = group
			}
		case *ast.ExprStmt:
			call, ok := typed.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			if route, ok := parseRouteCall(call, groups); ok {
				*routes = append(*routes, route)
			}
		case *ast.IfStmt:
			collectRouteStatements(typed.Body.List, cloneGroups(groups), routes)
			if typed.Else != nil {
				switch elseStmt := typed.Else.(type) {
				case *ast.BlockStmt:
					collectRouteStatements(elseStmt.List, cloneGroups(groups), routes)
				case *ast.IfStmt:
					collectRouteStatements([]ast.Stmt{elseStmt}, cloneGroups(groups), routes)
				}
			}
		case *ast.BlockStmt:
			collectRouteStatements(typed.List, cloneGroups(groups), routes)
		}
	}
}

func cloneGroups(groups map[string]groupState) map[string]groupState {
	out := make(map[string]groupState, len(groups))
	for key, value := range groups {
		out[key] = value
	}
	return out
}

func parseGroupCall(call *ast.CallExpr, groups map[string]groupState) (groupState, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Group" {
		return groupState{}, false
	}
	base, ok := selector.X.(*ast.Ident)
	if !ok {
		return groupState{}, false
	}
	baseGroup, ok := groups[base.Name]
	if !ok {
		return groupState{}, false
	}
	pathValue := ""
	if len(call.Args) > 0 {
		pathValue, _ = stringLiteral(call.Args[0])
	}
	return groupState{
		Prefix:         joinPaths(baseGroup.Prefix, pathValue),
		RequiresTenant: baseGroup.RequiresTenant || callHasResolveTenant(call.Args[1:]),
	}, true
}

func parseRouteCall(call *ast.CallExpr, groups map[string]groupState) (routeSpec, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return routeSpec{}, false
	}
	method := strings.ToUpper(strings.TrimSpace(selector.Sel.Name))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
	default:
		return routeSpec{}, false
	}
	base, ok := selector.X.(*ast.Ident)
	if !ok {
		return routeSpec{}, false
	}
	group, ok := groups[base.Name]
	if !ok || len(call.Args) < 2 {
		return routeSpec{}, false
	}
	pathValue, ok := stringLiteral(call.Args[0])
	if !ok {
		return routeSpec{}, false
	}
	handlerName := ""
	if handlerSelector, ok := call.Args[1].(*ast.SelectorExpr); ok {
		handlerName = handlerSelector.Sel.Name
	}
	if handlerName == "" {
		return routeSpec{}, false
	}
	fullPath := normalizePath(joinPaths(group.Prefix, pathValue))
	return routeSpec{
		Method:         method,
		Path:           fullPath,
		HandlerName:    handlerName,
		Secured:        isSecuredRoute(strings.ReplaceAll(fullPath, "{", ":")),
		RequiresTenant: group.RequiresTenant || callHasResolveTenant(call.Args[2:]),
	}, true
}

func callHasResolveTenant(args []ast.Expr) bool {
	for _, arg := range args {
		call, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		base, ok := selector.X.(*ast.Ident)
		if ok && base.Name == "middleware" && selector.Sel.Name == "ResolveTenant" {
			return true
		}
	}
	return false
}

func joinPaths(prefix, path string) string {
	switch {
	case prefix == "" && path == "":
		return "/"
	case prefix == "":
		return path
	case path == "":
		return prefix
	default:
		return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/")
	}
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	return pathParamPattern.ReplaceAllString(path, `{$1}`)
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func newSchemaBuilder(loader *moduleLoader) *schemaBuilder {
	return &schemaBuilder{
		loader:     loader,
		components: map[string]any{},
		names:      map[namedTypeKey]string{},
		usedNames:  map[string]namedTypeKey{},
		building:   map[namedTypeKey]bool{},
	}
}

func newOpenAPIGenerator(loader *moduleLoader) (*openAPIGenerator, error) {
	apiPkg, err := loader.loadPackage(apiImportPath)
	if err != nil {
		return nil, err
	}
	fields, err := handlerFieldTypes(apiPkg)
	if err != nil {
		return nil, err
	}
	return &openAPIGenerator{
		loader:        loader,
		apiPkg:        apiPkg,
		schemaBuilder: newSchemaBuilder(loader),
		handlerFields: fields,
	}, nil
}

func handlerFieldTypes(apiPkg *parsedPackage) (map[string]typeRef, error) {
	decl := apiPkg.Types["Handler"]
	if decl == nil {
		return nil, fmt.Errorf("Handler type not found")
	}
	structType, ok := decl.Spec.Type.(*ast.StructType)
	if !ok {
		return nil, fmt.Errorf("Handler is not a struct")
	}
	fields := map[string]typeRef{}
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		ref := typeRef{
			PkgPath: apiPkg.Path,
			File:    decl.File,
			Expr:    field.Type,
		}
		for _, name := range field.Names {
			fields[name.Name] = ref
		}
	}
	return fields, nil
}

func (g *openAPIGenerator) BuildSpec(base map[string]any) (map[string]any, error) {
	routes, err := discoverRoutes(g.apiPkg)
	if err != nil {
		return nil, err
	}
	paths := map[string]any{}
	tagNames := map[string]struct{}{}
	for _, route := range routes {
		op, err := g.buildOperation(route)
		if err != nil {
			return nil, err
		}
		tag := deriveTag(route.Path)
		tagNames[tag] = struct{}{}
		op["tags"] = []string{tag}
		methodKey := strings.ToLower(route.Method)
		entry, _ := paths[route.Path].(map[string]any)
		if entry == nil {
			entry = map[string]any{}
		}
		entry[methodKey] = op
		paths[route.Path] = entry
	}

	tags := make([]map[string]any, 0, len(tagNames))
	for tag := range tagNames {
		tags = append(tags, map[string]any{"name": tag})
	}
	slices.SortFunc(tags, func(a, b map[string]any) int {
		return cmp.Compare(a["name"].(string), b["name"].(string))
	})

	spec := cloneJSONMap(base)
	if spec["openapi"] == nil {
		spec["openapi"] = "3.0.3"
	}
	if spec["info"] == nil {
		spec["info"] = map[string]any{
			"title":       "IncidentHub API",
			"version":     "0.1.0",
			"description": "REST API for multi-tenant IncidentHub SOC platform.",
		}
	}
	spec["servers"] = []map[string]any{{"url": "/"}}
	spec["paths"] = orderedMap(paths)
	spec["tags"] = tags
	g.schemaBuilder.ensureNamedSchema(namedTypeKey{PkgPath: apiImportPath, Name: "errorResponse"})
	spec["components"] = map[string]any{
		"schemas": orderedMap(g.schemaBuilder.components),
		"securitySchemes": map[string]any{
			"BearerAuth": map[string]any{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		},
	}
	return spec, nil
}

func (g *openAPIGenerator) buildOperation(route routeSpec) (map[string]any, error) {
	handler := g.apiPkg.Methods["Handler"][route.HandlerName]
	if handler == nil {
		return nil, fmt.Errorf("handler %s not found", route.HandlerName)
	}
	analysis := g.analyzeHandler(handler)
	parameters := make([]map[string]any, 0)
	for _, param := range pathParameters(route.Path) {
		parameters = append(parameters, param)
	}
	if route.RequiresTenant {
		parameters = append(parameters, tenantHeaderParameter())
	}
	queryNames := make([]string, 0, len(analysis.QueryParams))
	for name := range analysis.QueryParams {
		queryNames = append(queryNames, name)
	}
	slices.Sort(queryNames)
	for _, name := range queryNames {
		parameters = append(parameters, analysis.QueryParams[name])
	}

	op := map[string]any{
		"operationId": operationID(route.Method, route.Path),
		"summary":     splitCamelCase(route.HandlerName),
		"responses":   mapOfMapsToAny(analysis.Responses),
	}
	if len(parameters) > 0 {
		op["parameters"] = parameters
	}
	if route.Secured {
		op["security"] = []map[string][]string{{"BearerAuth": {}}}
	}
	if analysis.RequestBody != nil {
		op["requestBody"] = analysis.RequestBody
	}
	return op, nil
}

func (g *openAPIGenerator) analyzeHandler(fn *funcDecl) operationAnalysis {
	collector := &funcCollector{
		gen:             g,
		fn:              fn,
		scope:           newScope(nil),
		queryParams:     map[string]map[string]any{},
		errorCodes:      map[string]struct{}{},
		responses:       map[string]*responseAccumulator{},
		formFields:      map[string]map[string]any{},
		fileFields:      map[string]bool{},
		multiFileFields: map[string]bool{},
		helperVisits:    map[string]struct{}{},
	}
	if fn.Decl.Recv != nil && len(fn.Decl.Recv.List) > 0 && len(fn.Decl.Recv.List[0].Names) > 0 {
		name := fn.Decl.Recv.List[0].Names[0].Name
		ref := typeRef{PkgPath: g.apiPkg.Path, File: fn.File, Expr: fn.Decl.Recv.List[0].Type}
		collector.scope.set(name, valueBinding{Type: &ref})
	}
	if fn.Decl.Type.Params != nil {
		for _, field := range fn.Decl.Type.Params.List {
			for _, name := range field.Names {
				binding := valueBinding{
					Type: &typeRef{PkgPath: g.apiPkg.Path, File: fn.File, Expr: field.Type},
				}
				if isEchoContextType(binding.Type) {
					binding.Context = true
				}
				collector.scope.set(name.Name, binding)
			}
		}
	}
	if fn.Decl.Body != nil {
		collector.walkBlock(fn.Decl.Body.List, collector.scope, fn)
	}
	collector.finalizeRequestBody()

	responses := map[string]map[string]any{}
	for code, response := range collector.responses {
		responses[code] = response.toOpenAPI()
	}
	for code := range collector.errorCodes {
		if _, ok := responses[code]; ok {
			continue
		}
		responses[code] = errorResponseSpec(code)
	}
	if len(responses) == 0 {
		responses[defaultResponseCode("GET", "")] = map[string]any{"description": "Success"}
	}
	return operationAnalysis{
		QueryParams: collector.queryParams,
		RequestBody: collector.requestBody,
		Responses:   responses,
	}
}

func newScope(parent *scope) *scope {
	return &scope{
		parent:   parent,
		bindings: map[string]valueBinding{},
	}
}

func (s *scope) child() *scope {
	return newScope(s)
}

func (s *scope) set(name string, binding valueBinding) {
	if name == "" || name == "_" {
		return
	}
	s.bindings[name] = binding
}

func (s *scope) get(name string) (valueBinding, bool) {
	for current := s; current != nil; current = current.parent {
		value, ok := current.bindings[name]
		if ok {
			return value, true
		}
	}
	return valueBinding{}, false
}

func (c *funcCollector) walkBlock(stmts []ast.Stmt, sc *scope, currentFn *funcDecl) {
	for _, stmt := range stmts {
		switch typed := stmt.(type) {
		case *ast.DeclStmt:
			c.handleDeclStmt(typed, sc, currentFn)
		case *ast.AssignStmt:
			c.handleAssignStmt(typed, sc, currentFn)
		case *ast.IfStmt:
			if typed.Init != nil {
				c.walkBlock([]ast.Stmt{typed.Init}, sc.child(), currentFn)
			}
			c.inspectExpr(typed.Cond, sc, currentFn)
			c.walkBlock(typed.Body.List, sc.child(), currentFn)
			if typed.Else != nil {
				switch elseStmt := typed.Else.(type) {
				case *ast.BlockStmt:
					c.walkBlock(elseStmt.List, sc.child(), currentFn)
				case *ast.IfStmt:
					c.walkBlock([]ast.Stmt{elseStmt}, sc.child(), currentFn)
				}
			}
		case *ast.RangeStmt:
			c.inspectExpr(typed.X, sc, currentFn)
			c.walkBlock(typed.Body.List, sc.child(), currentFn)
		case *ast.ForStmt:
			if typed.Init != nil {
				c.walkBlock([]ast.Stmt{typed.Init}, sc.child(), currentFn)
			}
			if typed.Cond != nil {
				c.inspectExpr(typed.Cond, sc, currentFn)
			}
			if typed.Post != nil {
				c.inspectStmtExpressions(typed.Post, sc, currentFn)
			}
			c.walkBlock(typed.Body.List, sc.child(), currentFn)
		case *ast.SwitchStmt:
			if typed.Init != nil {
				c.walkBlock([]ast.Stmt{typed.Init}, sc.child(), currentFn)
			}
			if typed.Tag != nil {
				c.inspectExpr(typed.Tag, sc, currentFn)
			}
			for _, stmt := range typed.Body.List {
				caseClause, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range caseClause.List {
					c.inspectExpr(expr, sc, currentFn)
				}
				c.walkBlock(caseClause.Body, sc.child(), currentFn)
			}
		case *ast.TypeSwitchStmt:
			if typed.Init != nil {
				c.walkBlock([]ast.Stmt{typed.Init}, sc.child(), currentFn)
			}
			c.inspectStmtExpressions(typed.Assign, sc, currentFn)
			for _, stmt := range typed.Body.List {
				caseClause, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				c.walkBlock(caseClause.Body, sc.child(), currentFn)
			}
		case *ast.ExprStmt:
			c.inspectExpr(typed.X, sc, currentFn)
		case *ast.ReturnStmt:
			for _, result := range typed.Results {
				c.inspectExpr(result, sc, currentFn)
			}
			c.handleReturnStmt(typed, sc, currentFn)
		default:
			c.inspectStmtExpressions(stmt, sc, currentFn)
		}
	}
}

func (c *funcCollector) inspectStmtExpressions(stmt ast.Stmt, sc *scope, currentFn *funcDecl) {
	ast.Inspect(stmt, func(node ast.Node) bool {
		expr, ok := node.(ast.Expr)
		if !ok {
			return true
		}
		c.inspectExpr(expr, sc, currentFn)
		return true
	})
}

func (c *funcCollector) handleDeclStmt(stmt *ast.DeclStmt, sc *scope, currentFn *funcDecl) {
	genDecl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.VAR {
		return
	}
	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		if valueSpec.Type != nil {
			ref := typeRef{PkgPath: currentFn.File.Pkg.Path, File: currentFn.File, Expr: valueSpec.Type}
			for _, name := range valueSpec.Names {
				sc.set(name.Name, valueBinding{Type: &ref})
			}
		}
		if len(valueSpec.Values) == 1 && len(valueSpec.Names) > 1 {
			results := c.resolveCallResults(valueSpec.Values[0], sc, currentFn)
			for idx, name := range valueSpec.Names {
				if idx < len(results) {
					ref := results[idx]
					sc.set(name.Name, valueBinding{Type: &ref})
				}
			}
		}
		for idx, value := range valueSpec.Values {
			c.inspectExpr(value, sc, currentFn)
			if idx >= len(valueSpec.Names) {
				continue
			}
			name := valueSpec.Names[idx]
			if valueSpec.Type == nil {
				c.bindValue(sc, name.Name, value, currentFn)
			}
		}
	}
}

func (c *funcCollector) handleAssignStmt(stmt *ast.AssignStmt, sc *scope, currentFn *funcDecl) {
	if len(stmt.Rhs) == 1 && len(stmt.Lhs) > 1 {
		results := c.resolveCallResults(stmt.Rhs[0], sc, currentFn)
		if len(results) > 0 {
			for idx, lhs := range stmt.Lhs {
				name, ok := lhs.(*ast.Ident)
				if !ok || idx >= len(results) {
					continue
				}
				ref := results[idx]
				sc.set(name.Name, valueBinding{Type: &ref})
			}
		}
	}
	for _, expr := range stmt.Rhs {
		c.inspectExpr(expr, sc, currentFn)
	}
	for idx, lhs := range stmt.Lhs {
		name, ok := lhs.(*ast.Ident)
		if !ok || idx >= len(stmt.Rhs) {
			continue
		}
		c.bindValue(sc, name.Name, stmt.Rhs[idx], currentFn)
	}
}

func (c *funcCollector) bindValue(sc *scope, name string, expr ast.Expr, currentFn *funcDecl) {
	if name == "" || name == "_" {
		return
	}
	if call, ok := expr.(*ast.CallExpr); ok && isContextRequestCall(call, sc) {
		sc.set(name, valueBinding{Request: true})
		return
	}
	if schema := c.schemaForExpr(expr, sc, currentFn); schema != nil {
		sc.set(name, valueBinding{Schema: schema})
	}
	if ref, ok := c.inferType(expr, sc, currentFn); ok {
		sc.set(name, valueBinding{Type: &ref})
	}
}

func isContextRequestCall(call *ast.CallExpr, sc *scope) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Request" || len(call.Args) != 0 {
		return false
	}
	base, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	binding, ok := sc.get(base.Name)
	return ok && binding.Context
}

func (c *funcCollector) inspectExpr(expr ast.Expr, sc *scope, currentFn *funcDecl) {
	ast.Inspect(expr, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			c.recordCallExpr(typed, sc, currentFn)
		case *ast.IndexExpr:
			c.recordIndexExpr(typed, sc)
		}
		return true
	})
}

func (c *funcCollector) recordCallExpr(call *ast.CallExpr, sc *scope, currentFn *funcDecl) {
	selector, hasSelector := call.Fun.(*ast.SelectorExpr)
	if hasSelector {
		if base, ok := selector.X.(*ast.Ident); ok {
			binding, known := sc.get(base.Name)
			if known && binding.Context {
				switch selector.Sel.Name {
				case "QueryParam":
					if name, ok := stringLiteralArg(call, 0); ok {
						c.queryParams[name] = queryParameter(name)
					}
				case "Bind":
					if len(call.Args) > 0 {
						c.captureJSONRequestBody(call.Args[0], sc, currentFn)
					}
				case "FormFile":
					c.hasMultipart = true
					if name, ok := stringLiteralArg(call, 0); ok {
						c.fileFields[name] = true
					}
				}
			}
			if known && binding.Request {
				switch selector.Sel.Name {
				case "ParseMultipartForm":
					c.hasMultipart = true
				case "FormValue":
					if name, ok := stringLiteralArg(call, 0); ok {
						c.hasMultipart = true
						c.formFields[name] = map[string]any{"type": "string"}
					}
				}
			}
		}
	}

	if code, ok := extractHTTPErrorCode(call); ok {
		c.errorCodes[code] = struct{}{}
	}

	if fn, ok := c.resolveLocalFunctionCall(call, sc, currentFn); ok {
		c.collectNestedInputs(fn, call, sc)
	}
}

func (c *funcCollector) recordIndexExpr(expr *ast.IndexExpr, sc *scope) {
	name, ok := multipartFileFieldName(expr, sc)
	if !ok {
		return
	}
	c.hasMultipart = true
	c.multiFileFields[name] = true
}

func multipartFileFieldName(expr *ast.IndexExpr, sc *scope) (string, bool) {
	indexName, ok := stringLiteral(expr.Index)
	if !ok {
		return "", false
	}
	selector, ok := expr.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "File" {
		return "", false
	}
	multipartSelector, ok := selector.X.(*ast.SelectorExpr)
	if !ok || multipartSelector.Sel.Name != "MultipartForm" {
		return "", false
	}
	base, ok := multipartSelector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	binding, ok := sc.get(base.Name)
	return indexName, ok && binding.Request
}

func stringLiteralArg(call *ast.CallExpr, index int) (string, bool) {
	if index >= len(call.Args) {
		return "", false
	}
	return stringLiteral(call.Args[index])
}

func (c *funcCollector) captureJSONRequestBody(arg ast.Expr, sc *scope, currentFn *funcDecl) {
	target, ok := unwrapPointerIdent(arg)
	if !ok {
		return
	}
	binding, ok := sc.get(target)
	if !ok || binding.Type == nil {
		return
	}
	c.requestBody = map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": c.gen.schemaBuilder.schemaForType(*binding.Type),
			},
		},
	}
}

func unwrapPointerIdent(expr ast.Expr) (string, bool) {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return "", false
	}
	ident, ok := unary.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	return ident.Name, true
}

func (c *funcCollector) collectNestedInputs(fn *funcDecl, call *ast.CallExpr, sc *scope) {
	key := fn.File.Pkg.Path + ":" + fn.Name
	if _, seen := c.helperVisits[key]; seen {
		return
	}
	ctxParams := map[string]valueBinding{}
	reqParams := map[string]valueBinding{}
	if fn.Decl.Type.Params != nil {
		argIndex := 0
		for _, field := range fn.Decl.Type.Params.List {
			for _, name := range field.Names {
				if argIndex >= len(call.Args) {
					break
				}
				arg := call.Args[argIndex]
				if ident, ok := arg.(*ast.Ident); ok {
					if binding, ok := sc.get(ident.Name); ok {
						if binding.Context {
							ctxParams[name.Name] = valueBinding{Context: true}
						}
						if binding.Request {
							reqParams[name.Name] = valueBinding{Request: true}
						}
					}
				}
				argIndex++
			}
		}
	}
	if len(ctxParams) == 0 && len(reqParams) == 0 {
		return
	}
	c.helperVisits[key] = struct{}{}
	helperScope := newScope(nil)
	for name, binding := range ctxParams {
		helperScope.set(name, binding)
	}
	for name, binding := range reqParams {
		helperScope.set(name, binding)
	}
	helperCollector := &funcCollector{
		gen:             c.gen,
		fn:              fn,
		scope:           helperScope,
		queryParams:     c.queryParams,
		errorCodes:      c.errorCodes,
		responses:       map[string]*responseAccumulator{},
		formFields:      c.formFields,
		fileFields:      c.fileFields,
		multiFileFields: c.multiFileFields,
		hasMultipart:    c.hasMultipart,
		helperVisits:    c.helperVisits,
	}
	if fn.Decl.Body != nil {
		helperCollector.walkBlock(fn.Decl.Body.List, helperScope, fn)
	}
	c.hasMultipart = c.hasMultipart || helperCollector.hasMultipart
}

func (c *funcCollector) handleReturnStmt(stmt *ast.ReturnStmt, sc *scope, currentFn *funcDecl) {
	if len(stmt.Results) != 1 {
		return
	}
	call, ok := stmt.Results[0].(*ast.CallExpr)
	if !ok {
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		if code, ok := extractHTTPErrorCode(call); ok {
			c.errorCodes[code] = struct{}{}
		}
		return
	}
	base, ok := selector.X.(*ast.Ident)
	if !ok {
		return
	}
	binding, ok := sc.get(base.Name)
	if !ok || !binding.Context {
		return
	}

	switch selector.Sel.Name {
	case "JSON":
		if len(call.Args) < 2 {
			return
		}
		code := statusCodeString(call.Args[0])
		schema := c.schemaForExpr(call.Args[1], sc, currentFn)
		c.addResponse(code, "application/json", schema)
	case "NoContent":
		if len(call.Args) < 1 {
			return
		}
		code := statusCodeString(call.Args[0])
		c.addResponse(code, "", nil)
	case "Blob":
		if len(call.Args) < 2 {
			return
		}
		code := statusCodeString(call.Args[0])
		contentType, _ := stringLiteral(call.Args[1])
		var schema map[string]any
		if contentType == "application/json" {
			schema = map[string]any{"type": "object", "additionalProperties": true}
		} else {
			schema = map[string]any{"type": "string", "format": "binary"}
		}
		c.addResponse(code, contentType, schema)
	case "HTML":
		if len(call.Args) < 1 {
			return
		}
		code := statusCodeString(call.Args[0])
		c.addResponse(code, "text/html", map[string]any{"type": "string"})
	case "String":
		if len(call.Args) < 1 {
			return
		}
		code := statusCodeString(call.Args[0])
		c.addResponse(code, "text/plain", map[string]any{"type": "string"})
	}
}

func (c *funcCollector) addResponse(code, contentType string, schema map[string]any) {
	if code == "" {
		return
	}
	entry, ok := c.responses[code]
	if !ok {
		entry = &responseAccumulator{
			ContentType: contentType,
			Schemas:     make([]map[string]any, 0, 1),
			Description: responseDescription(code),
		}
		c.responses[code] = entry
	}
	if schema != nil {
		entry.Schemas = appendSchemaUnique(entry.Schemas, schema)
	}
	if entry.ContentType == "" {
		entry.ContentType = contentType
	}
}

func appendSchemaUnique(items []map[string]any, schema map[string]any) []map[string]any {
	serialized, _ := json.Marshal(schema)
	for _, existing := range items {
		current, _ := json.Marshal(existing)
		if string(current) == string(serialized) {
			return items
		}
	}
	return append(items, schema)
}

func (c *funcCollector) finalizeRequestBody() {
	if c.requestBody != nil || !c.hasMultipart {
		return
	}
	properties := map[string]any{}
	required := make([]string, 0)
	keys := make([]string, 0, len(c.formFields)+len(c.fileFields)+len(c.multiFileFields))
	for name := range c.formFields {
		keys = append(keys, name)
	}
	for name := range c.fileFields {
		keys = append(keys, name)
	}
	for name := range c.multiFileFields {
		if _, ok := c.fileFields[name]; !ok {
			keys = append(keys, name)
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	for _, name := range keys {
		switch {
		case c.multiFileFields[name]:
			properties[name] = map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":   "string",
					"format": "binary",
				},
			}
		case c.fileFields[name]:
			properties[name] = map[string]any{
				"type":   "string",
				"format": "binary",
			}
		default:
			properties[name] = c.formFields[name]
		}
	}
	if len(properties) == 0 {
		return
	}
	c.requestBody = map[string]any{
		"required": false,
		"content": map[string]any{
			"multipart/form-data": map[string]any{
				"schema": objectSchema(properties, required, false),
			},
		},
	}
}

func (c *funcCollector) resolveCallResults(expr ast.Expr, sc *scope, currentFn *funcDecl) []typeRef {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "make" && len(call.Args) > 0 {
		return []typeRef{{PkgPath: currentFn.File.Pkg.Path, File: currentFn.File, Expr: call.Args[0]}}
	}
	if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "append" && len(call.Args) > 0 {
		if ref, ok := c.inferType(call.Args[0], sc, currentFn); ok {
			return []typeRef{ref}
		}
	}
	fn, ok := c.resolveFunctionSignature(call, sc, currentFn)
	if !ok || fn.Decl.Type.Results == nil {
		return nil
	}
	results := make([]typeRef, 0, len(fn.Decl.Type.Results.List))
	for _, field := range fn.Decl.Type.Results.List {
		count := 1
		if len(field.Names) > 0 {
			count = len(field.Names)
		}
		for i := 0; i < count; i++ {
			results = append(results, typeRef{
				PkgPath: fn.File.Pkg.Path,
				File:    fn.File,
				Expr:    field.Type,
			})
		}
	}
	return results
}

func (c *funcCollector) resolveLocalFunctionCall(call *ast.CallExpr, sc *scope, currentFn *funcDecl) (*funcDecl, bool) {
	fn, ok := c.resolveFunctionSignature(call, sc, currentFn)
	if !ok {
		return nil, false
	}
	if !strings.HasPrefix(fn.File.Pkg.Path, c.gen.loader.modulePath) {
		return nil, false
	}
	return fn, true
}

func (c *funcCollector) resolveFunctionSignature(call *ast.CallExpr, sc *scope, currentFn *funcDecl) (*funcDecl, bool) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		fn := currentFn.File.Pkg.Funcs[fun.Name]
		return fn, fn != nil
	case *ast.SelectorExpr:
		if pkgIdent, ok := fun.X.(*ast.Ident); ok {
			if importPath, ok := currentFn.File.Imports[pkgIdent.Name]; ok {
				pkg, err := c.gen.loader.loadPackage(importPath)
				if err == nil {
					fn := pkg.Funcs[fun.Sel.Name]
					return fn, fn != nil
				}
			}
		}
		receiverType, ok := c.inferType(fun.X, sc, currentFn)
		if !ok {
			return nil, false
		}
		key, ok := c.gen.schemaBuilder.namedTypeKey(receiverType)
		if !ok {
			return nil, false
		}
		pkg, err := c.gen.loader.loadPackage(key.PkgPath)
		if err != nil {
			return nil, false
		}
		methods := pkg.Methods[key.Name]
		if methods == nil {
			return nil, false
		}
		fn := methods[fun.Sel.Name]
		return fn, fn != nil
	default:
		return nil, false
	}
}

func (c *funcCollector) inferType(expr ast.Expr, sc *scope, currentFn *funcDecl) (typeRef, bool) {
	switch typed := expr.(type) {
	case *ast.Ident:
		if binding, ok := sc.get(typed.Name); ok && binding.Type != nil {
			return *binding.Type, true
		}
		if currentFn.File.Pkg.Types[typed.Name] != nil {
			return typeRef{PkgPath: currentFn.File.Pkg.Path, File: currentFn.File, Expr: typed}, true
		}
	case *ast.ParenExpr:
		return c.inferType(typed.X, sc, currentFn)
	case *ast.UnaryExpr:
		if typed.Op == token.AND {
			if inner, ok := c.inferType(typed.X, sc, currentFn); ok {
				return typeRef{
					PkgPath: inner.PkgPath,
					File:    inner.File,
					Expr:    &ast.StarExpr{X: inner.Expr},
				}, true
			}
		}
	case *ast.CompositeLit:
		return typeRef{PkgPath: currentFn.File.Pkg.Path, File: currentFn.File, Expr: typed.Type}, true
	case *ast.CallExpr:
		results := c.resolveCallResults(typed, sc, currentFn)
		if len(results) > 0 {
			return results[0], true
		}
	case *ast.SelectorExpr:
		if baseIdent, ok := typed.X.(*ast.Ident); ok {
			if importPath, ok := currentFn.File.Imports[baseIdent.Name]; ok {
				return typeRef{PkgPath: importPath, File: currentFn.File, Expr: typed}, true
			}
		}
		parentType, ok := c.inferType(typed.X, sc, currentFn)
		if !ok {
			return typeRef{}, false
		}
		fieldType, ok := c.gen.lookupStructField(parentType, typed.Sel.Name)
		if !ok {
			return typeRef{}, false
		}
		return fieldType, true
	}
	return typeRef{}, false
}

func (c *funcCollector) schemaForExpr(expr ast.Expr, sc *scope, currentFn *funcDecl) map[string]any {
	switch typed := expr.(type) {
	case *ast.ParenExpr:
		return c.schemaForExpr(typed.X, sc, currentFn)
	case *ast.Ident:
		if binding, ok := sc.get(typed.Name); ok {
			if binding.Schema != nil {
				return cloneJSONMap(binding.Schema)
			}
			if binding.Type != nil {
				return c.gen.schemaBuilder.schemaForType(*binding.Type)
			}
		}
		switch typed.Name {
		case "true", "false":
			return map[string]any{"type": "boolean"}
		case "nil":
			return nil
		}
	case *ast.BasicLit:
		return literalSchema(typed)
	case *ast.UnaryExpr:
		return c.schemaForExpr(typed.X, sc, currentFn)
	case *ast.CompositeLit:
		if mapSchema := c.schemaForMapLiteral(typed, sc, currentFn); mapSchema != nil {
			return mapSchema
		}
		ref := typeRef{PkgPath: currentFn.File.Pkg.Path, File: currentFn.File, Expr: typed.Type}
		return c.gen.schemaBuilder.schemaForType(ref)
	case *ast.CallExpr:
		if schema := c.specialSchemaForCall(typed, sc, currentFn); schema != nil {
			return schema
		}
		if ref, ok := c.inferType(typed, sc, currentFn); ok {
			return c.gen.schemaBuilder.schemaForType(ref)
		}
	case *ast.SelectorExpr:
		if ref, ok := c.inferType(typed, sc, currentFn); ok {
			return c.gen.schemaBuilder.schemaForType(ref)
		}
	}
	return map[string]any{"type": "object", "additionalProperties": true}
}

func (c *funcCollector) schemaForMapLiteral(lit *ast.CompositeLit, sc *scope, currentFn *funcDecl) map[string]any {
	mapType, ok := lit.Type.(*ast.MapType)
	if !ok {
		return nil
	}
	keyIdent, ok := mapType.Key.(*ast.Ident)
	if !ok || keyIdent.Name != "string" {
		return nil
	}
	properties := map[string]any{}
	dynamic := false
	for _, item := range lit.Elts {
		keyValue, ok := item.(*ast.KeyValueExpr)
		if !ok {
			dynamic = true
			continue
		}
		name, ok := stringLiteral(keyValue.Key)
		if !ok {
			dynamic = true
			continue
		}
		properties[name] = c.schemaForExpr(keyValue.Value, sc, currentFn)
	}
	return objectSchema(properties, nil, dynamic)
}

func (c *funcCollector) specialSchemaForCall(call *ast.CallExpr, sc *scope, currentFn *funcDecl) map[string]any {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		switch fun.Name {
		case "pagedListPayload":
			if len(call.Args) == 0 {
				return nil
			}
			return objectSchema(map[string]any{
				"items":       c.schemaForExpr(call.Args[0], sc, currentFn),
				"page":        map[string]any{"type": "integer"},
				"page_size":   map[string]any{"type": "integer"},
				"total":       map[string]any{"type": "integer"},
				"total_pages": map[string]any{"type": "integer"},
			}, nil, false)
		case "mapCatalogItems":
			return map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			}
		case "catalogItemToPayload", "workflowVaultSecretItemToPayload", "notificationBotPayload", "notificationSettingsPayload", "defaultNotificationSettingsPayload", "adminNotificationSettingsPayload":
			return map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			}
		}
	}
	return nil
}

func literalSchema(lit *ast.BasicLit) map[string]any {
	switch lit.Kind {
	case token.STRING:
		return map[string]any{"type": "string"}
	case token.INT:
		return map[string]any{"type": "integer"}
	case token.FLOAT:
		return map[string]any{"type": "number"}
	default:
		return map[string]any{"type": "string"}
	}
}

func (g *openAPIGenerator) lookupStructField(ref typeRef, fieldName string) (typeRef, bool) {
	key, ok := g.schemaBuilder.namedTypeKey(ref)
	if !ok {
		return typeRef{}, false
	}
	pkg, err := g.loader.loadPackage(key.PkgPath)
	if err != nil {
		return typeRef{}, false
	}
	decl := pkg.Types[key.Name]
	if decl == nil {
		return typeRef{}, false
	}
	structType, ok := decl.Spec.Type.(*ast.StructType)
	if !ok {
		return typeRef{}, false
	}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			if name.Name == fieldName {
				return typeRef{
					PkgPath: pkg.Path,
					File:    decl.File,
					Expr:    field.Type,
				}, true
			}
		}
	}
	return typeRef{}, false
}

func (b *schemaBuilder) namedTypeKey(ref typeRef) (namedTypeKey, bool) {
	switch typed := ref.Expr.(type) {
	case *ast.Ident:
		return namedTypeKey{PkgPath: ref.PkgPath, Name: typed.Name}, true
	case *ast.StarExpr:
		return b.namedTypeKey(typeRef{PkgPath: ref.PkgPath, File: ref.File, Expr: typed.X})
	case *ast.SelectorExpr:
		if pkgIdent, ok := typed.X.(*ast.Ident); ok && ref.File != nil {
			importPath := ref.File.Imports[pkgIdent.Name]
			if importPath != "" {
				return namedTypeKey{PkgPath: importPath, Name: typed.Sel.Name}, true
			}
		}
	}
	return namedTypeKey{}, false
}

func (b *schemaBuilder) schemaForType(ref typeRef) map[string]any {
	switch typed := ref.Expr.(type) {
	case *ast.ParenExpr:
		return b.schemaForType(typeRef{PkgPath: ref.PkgPath, File: ref.File, Expr: typed.X})
	case *ast.StarExpr:
		return nullableSchema(b.schemaForType(typeRef{PkgPath: ref.PkgPath, File: ref.File, Expr: typed.X}))
	case *ast.ArrayType:
		if eltIdent, ok := typed.Elt.(*ast.Ident); ok && eltIdent.Name == "byte" {
			return map[string]any{"type": "string", "format": "byte"}
		}
		return map[string]any{
			"type":  "array",
			"items": b.schemaForType(typeRef{PkgPath: ref.PkgPath, File: ref.File, Expr: typed.Elt}),
		}
	case *ast.MapType:
		schema := map[string]any{
			"type": "object",
		}
		if valueIdent, ok := typed.Value.(*ast.Ident); ok && (valueIdent.Name == "any" || valueIdent.Name == "interface{}") {
			schema["additionalProperties"] = true
			return schema
		}
		schema["additionalProperties"] = b.schemaForType(typeRef{PkgPath: ref.PkgPath, File: ref.File, Expr: typed.Value})
		return schema
	case *ast.StructType:
		return b.inlineStructSchema(ref.PkgPath, ref.File, typed)
	case *ast.InterfaceType:
		return map[string]any{"type": "object", "additionalProperties": true}
	case *ast.Ident:
		if primitive := primitiveSchema(typed.Name); primitive != nil {
			return primitive
		}
		key := namedTypeKey{PkgPath: ref.PkgPath, Name: typed.Name}
		return map[string]any{"$ref": "#/components/schemas/" + b.ensureNamedSchema(key)}
	case *ast.SelectorExpr:
		if pkgIdent, ok := typed.X.(*ast.Ident); ok && ref.File != nil {
			importPath := ref.File.Imports[pkgIdent.Name]
			if external := externalSchema(importPath, typed.Sel.Name); external != nil {
				return external
			}
			key := namedTypeKey{PkgPath: importPath, Name: typed.Sel.Name}
			return map[string]any{"$ref": "#/components/schemas/" + b.ensureNamedSchema(key)}
		}
	}
	return map[string]any{"type": "object", "additionalProperties": true}
}

func (b *schemaBuilder) ensureNamedSchema(key namedTypeKey) string {
	if name, ok := b.names[key]; ok {
		return name
	}
	name := preferredSchemaName(key)
	if existingKey, collision := b.usedNames[name]; collision && existingKey != key {
		name = exportedName(filepath.Base(strings.ReplaceAll(key.PkgPath, "/", "_"))) + name
	}
	b.names[key] = name
	b.usedNames[name] = key
	if b.building[key] {
		return name
	}
	b.building[key] = true
	defer delete(b.building, key)

	if schema := externalSchema(key.PkgPath, key.Name); schema != nil {
		b.components[name] = schema
		return name
	}

	pkg, err := b.loader.loadPackage(key.PkgPath)
	if err != nil {
		b.components[name] = map[string]any{"type": "object", "additionalProperties": true}
		return name
	}
	decl := pkg.Types[key.Name]
	if decl == nil {
		b.components[name] = map[string]any{"type": "object", "additionalProperties": true}
		return name
	}
	b.components[name] = b.schemaForType(typeRef{
		PkgPath: pkg.Path,
		File:    decl.File,
		Expr:    decl.Spec.Type,
	})
	return name
}

func preferredSchemaName(key namedTypeKey) string {
	base := exportedName(key.Name)
	if key.PkgPath == modelsImportPath {
		return base
	}
	return base
}

func exportedName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func primitiveSchema(name string) map[string]any {
	switch name {
	case "string":
		return map[string]any{"type": "string"}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return map[string]any{"type": "integer"}
	case "float32", "float64":
		return map[string]any{"type": "number"}
	case "byte":
		return map[string]any{"type": "integer"}
	case "any":
		return map[string]any{"type": "object", "additionalProperties": true}
	}
	return nil
}

func externalSchema(importPath, name string) map[string]any {
	switch {
	case importPath == "time" && name == "Time":
		return map[string]any{"type": "string", "format": "date-time"}
	case importPath == "github.com/google/uuid" && name == "UUID":
		return map[string]any{"type": "string", "format": "uuid"}
	case importPath == "encoding/json" && name == "RawMessage":
		return map[string]any{"type": "object", "additionalProperties": true}
	case importPath == "" && name == "Time":
		return map[string]any{"type": "string", "format": "date-time"}
	}
	return nil
}

func (b *schemaBuilder) inlineStructSchema(pkgPath string, file *parsedFile, structType *ast.StructType) map[string]any {
	properties := map[string]any{}
	required := make([]string, 0)
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		jsonName, omitEmpty, skip := jsonFieldName(field)
		if skip {
			continue
		}
		if jsonName == "" {
			jsonName = field.Names[0].Name
		}
		ref := typeRef{PkgPath: pkgPath, File: file, Expr: field.Type}
		properties[jsonName] = b.schemaForType(ref)
		if !omitEmpty && !isOptionalField(field.Type) {
			required = append(required, jsonName)
		}
	}
	return objectSchema(properties, required, false)
}

func jsonFieldName(field *ast.Field) (name string, omitEmpty bool, skip bool) {
	if field.Tag == nil {
		if len(field.Names) == 0 {
			return "", false, true
		}
		return field.Names[0].Name, false, false
	}
	tagValue, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false
	}
	jsonTag := reflect.StructTag(tagValue).Get("json")
	if jsonTag == "-" {
		return "", false, true
	}
	if jsonTag == "" {
		if len(field.Names) == 0 {
			return "", false, true
		}
		return field.Names[0].Name, false, false
	}
	parts := strings.Split(jsonTag, ",")
	name = parts[0]
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func isOptionalField(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return true
	case *ast.MapType:
		return true
	case *ast.ArrayType:
		return true
	case *ast.Ident:
		return typed.Name == "any"
	case *ast.InterfaceType:
		return true
	default:
		return false
	}
}

func objectSchema(properties map[string]any, required []string, dynamic bool) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": orderedMap(properties),
	}
	if len(required) > 0 {
		slices.Sort(required)
		required = slices.Compact(required)
		schema["required"] = required
	}
	if dynamic {
		schema["additionalProperties"] = true
	}
	return schema
}

func nullableSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	if _, ok := schema["$ref"]; ok {
		return map[string]any{
			"allOf":    []map[string]any{schema},
			"nullable": true,
		}
	}
	out := cloneJSONMap(schema)
	out["nullable"] = true
	return out
}

func cloneJSONMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func orderedMap(input map[string]any) map[string]any {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		out[key] = input[key]
	}
	return out
}

func mapOfMapsToAny(input map[string]map[string]any) map[string]any {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		out[key] = input[key]
	}
	return out
}

func pathParameters(path string) []map[string]any {
	parts := strings.Split(path, "/")
	params := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	for _, part := range parts {
		if len(part) < 3 || part[0] != '{' || part[len(part)-1] != '}' {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		schema := map[string]any{"type": "string"}
		if strings.HasSuffix(strings.ToLower(name), "id") {
			schema["format"] = "uuid"
		}
		params = append(params, map[string]any{
			"name":     name,
			"in":       "path",
			"required": true,
			"schema":   schema,
		})
	}
	return params
}

func tenantHeaderParameter() map[string]any {
	return map[string]any{
		"name":        "X-Tenant-ID",
		"in":          "header",
		"required":    false,
		"description": "Tenant context for tenant-scoped routes.",
		"schema": map[string]any{
			"type":   "string",
			"format": "uuid",
		},
	}
}

func queryParameter(name string) map[string]any {
	schema := map[string]any{"type": "string"}
	switch name {
	case "limit", "offset", "page", "page_size", "overdue_minutes", "year":
		schema["type"] = "integer"
	case "include_global":
		schema["type"] = "boolean"
	}
	if strings.HasSuffix(strings.ToLower(name), "_id") {
		schema["format"] = "uuid"
	}
	return map[string]any{
		"name":     name,
		"in":       "query",
		"required": false,
		"schema":   schema,
	}
}

func operationID(method, path string) string {
	sanitized := strings.NewReplacer("/", "_", "{", "", "}", "", "-", "_", ":", "_").Replace(path)
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		sanitized = "root"
	}
	return strings.ToLower(method) + "_" + sanitized
}

func splitCamelCase(input string) string {
	var out []rune
	for idx, r := range input {
		if idx > 0 && unicode.IsUpper(r) && (unicode.IsLower(rune(input[idx-1])) || (idx+1 < len(input) && unicode.IsLower(rune(input[idx+1])))) {
			out = append(out, ' ')
		}
		out = append(out, r)
	}
	return strings.TrimSpace(string(out))
}

func deriveTag(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" || !strings.HasPrefix(path, "/api/") {
		switch path {
		case "/healthz", "/metrics":
			return "System"
		default:
			return "Documentation"
		}
	}
	segment := trimmed
	if idx := strings.Index(segment, "/"); idx >= 0 {
		segment = segment[:idx]
	}
	switch segment {
	case "auth":
		return "Auth"
	case "me":
		return "Profile"
	case "users", "tenant-users":
		return "Users"
	case "tenants":
		return "Tenants"
	case "alerts":
		return "Alerts"
	case "dashboard":
		return "Dashboard"
	case "duty":
		return "Duty"
	case "activity":
		return "Activity"
	case "system":
		return "System"
	case "cases":
		return "Cases"
	case "case-statuses":
		return "Case Statuses"
	case "tasks":
		return "Tasks"
	case "connectors":
		return "Connectors"
	case "workflows":
		return "Workflows"
	case "forum":
		return "Forum"
	case "communications":
		return "Communications"
	case "case-comments":
		return "Case Comments"
	case "catalog":
		return "Catalog"
	case "search":
		return "Search"
	case "notification-bots":
		return "Notifications"
	case "admin":
		return "Administration"
	case "ai":
		return "AI"
	default:
		return splitCamelCase(exportedName(strings.ReplaceAll(segment, "-", " ")))
	}
}

func statusCodeString(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		return strings.TrimSpace(typed.Value)
	case *ast.SelectorExpr:
		if base, ok := typed.X.(*ast.Ident); ok && base.Name == "http" {
			return httpStatusNameToCode(typed.Sel.Name)
		}
	case *ast.Ident:
		return strings.TrimSpace(typed.Name)
	}
	return ""
}

func httpStatusNameToCode(name string) string {
	switch name {
	case "StatusOK":
		return "200"
	case "StatusCreated":
		return "201"
	case "StatusAccepted":
		return "202"
	case "StatusNoContent":
		return "204"
	case "StatusBadRequest":
		return "400"
	case "StatusUnauthorized":
		return "401"
	case "StatusForbidden":
		return "403"
	case "StatusNotFound":
		return "404"
	case "StatusConflict":
		return "409"
	case "StatusGone":
		return "410"
	case "StatusLocked":
		return "423"
	case "StatusTooManyRequests":
		return "429"
	case "StatusRequestEntityTooLarge":
		return "413"
	case "StatusBadGateway":
		return "502"
	case "StatusServiceUnavailable":
		return "503"
	case "StatusInternalServerError":
		return "500"
	default:
		return strings.TrimPrefix(name, "Status")
	}
}

func extractHTTPErrorCode(call *ast.CallExpr) (string, bool) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if base, ok := fun.X.(*ast.Ident); ok && base.Name == "echo" && fun.Sel.Name == "NewHTTPError" && len(call.Args) > 0 {
			code := statusCodeString(call.Args[0])
			return code, code != ""
		}
	}
	return "", false
}

func responseDescription(code string) string {
	switch code {
	case "200":
		return "Success"
	case "201":
		return "Created"
	case "202":
		return "Accepted"
	case "204":
		return "No Content"
	case "400":
		return "Bad Request"
	case "401":
		return "Unauthorized"
	case "403":
		return "Forbidden"
	case "404":
		return "Not Found"
	case "409":
		return "Conflict"
	case "413":
		return "Payload Too Large"
	case "423":
		return "Locked"
	case "500":
		return "Internal Server Error"
	case "502":
		return "Bad Gateway"
	case "503":
		return "Service Unavailable"
	default:
		return "Response"
	}
}

func errorResponseSpec(code string) map[string]any {
	return map[string]any{
		"description": responseDescription(code),
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"},
			},
		},
	}
}

func (r *responseAccumulator) toOpenAPI() map[string]any {
	response := map[string]any{
		"description": r.Description,
	}
	if r.ContentType == "" || len(r.Schemas) == 0 {
		return response
	}
	schema := r.Schemas[0]
	if len(r.Schemas) > 1 {
		schema = map[string]any{"oneOf": r.Schemas}
	}
	response["content"] = map[string]any{
		r.ContentType: map[string]any{
			"schema": schema,
		},
	}
	return response
}

func isEchoContextType(ref *typeRef) bool {
	if ref == nil {
		return false
	}
	switch typed := ref.Expr.(type) {
	case *ast.StarExpr:
		return isEchoContextType(&typeRef{PkgPath: ref.PkgPath, File: ref.File, Expr: typed.X})
	case *ast.SelectorExpr:
		base, ok := typed.X.(*ast.Ident)
		if !ok || typed.Sel.Name != "Context" {
			return false
		}
		if ref.File == nil {
			return base.Name == "echo"
		}
		return ref.File.Imports[base.Name] == "github.com/labstack/echo/v5"
	default:
		return false
	}
}

func isSecuredRoute(path string) bool {
	trimmed := strings.TrimSpace(path)
	switch trimmed {
	case "/healthz", "/metrics", "/dev/swagger", "/dev/swagger/openapi.json", "/swagger", "/swagger/openapi.json", "/api/v1/auth/login", "/api/v1/auth/refresh":
		return false
	default:
		return strings.HasPrefix(trimmed, "/api/")
	}
}

func defaultResponseCode(method, path string) string {
	if method == "DELETE" {
		return "200"
	}
	if method == "POST" && strings.HasSuffix(path, "/auth/logout") {
		return "204"
	}
	if method == "POST" {
		return "201"
	}
	return "200"
}
