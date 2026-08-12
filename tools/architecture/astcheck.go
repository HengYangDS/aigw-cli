package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

func checkGoAST(root string, files []goFileInfo, p policy, report *Report) error {
	fset := token.NewFileSet()
	type packageDocumentation struct {
		name       string
		path       string
		documented bool
	}
	packageDocs := map[string]packageDocumentation{}
	for _, file := range files {
		if file.isTest {
			// Production surface only; tests may use local aliases freely.
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(file.relPath))
		src, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("read %s: %w", file.relPath, err)
		}
		parsed, err := parser.ParseFile(
			fset,
			file.relPath,
			src,
			parser.ParseComments|parser.SkipObjectResolution,
		)
		if err != nil {
			// Skip unparseable files with a precise finding rather than aborting the gate.
			report.addFinding(Finding{
				Rule:    "go_parse_error",
				Path:    file.relPath,
				Message: fmt.Sprintf("failed to parse Go file: %v", err),
			})
			continue
		}
		if p.CheckPackageDocumentation && parsed.Name != nil && parsed.Name.Name != "main" {
			state := packageDocs[file.dir]
			if state.path == "" {
				state.name = parsed.Name.Name
				state.path = file.relPath
			}
			state.documented = state.documented || hasPackageDocumentation(parsed)
			packageDocs[file.dir] = state
		}
		if p.CheckExportedTypeAlias {
			checkExportedTypeAliases(fset, parsed, file.relPath, report)
		}
		if p.CheckFunctionVarAlias {
			checkFunctionVarAliases(fset, parsed, file.relPath, report)
		}
		if p.CheckTrivialWrappers {
			checkTrivialWrappers(fset, parsed, file.relPath, report)
		}
	}
	for _, state := range packageDocs {
		if state.documented {
			continue
		}
		report.addFinding(Finding{
			Rule:    "package_documentation_missing",
			Path:    state.path,
			Line:    1,
			Package: state.name,
			Message: fmt.Sprintf("package %q requires an accurate Package %s documentation comment", state.name, state.name),
		})
	}
	return nil
}

func hasPackageDocumentation(parsed *ast.File) bool {
	if parsed.Name == nil || parsed.Doc == nil {
		return false
	}
	prefix := "Package " + parsed.Name.Name
	documentation := strings.TrimSpace(parsed.Doc.Text())
	if !strings.HasPrefix(documentation, prefix) {
		return false
	}
	if len(documentation) == len(prefix) {
		return true
	}
	next := documentation[len(prefix)]
	return next == ' ' || next == '.' || next == ':'
}

func checkExportedTypeAliases(fset *token.FileSet, parsed *ast.File, relPath string, report *Report) {
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Assign == token.NoPos {
				continue
			}
			if !isExportedIdent(typeSpec.Name.Name) {
				continue
			}
			pos := fset.Position(typeSpec.Pos())
			report.addFinding(Finding{
				Rule:    "exported_type_alias",
				Path:    relPath,
				Line:    pos.Line,
				Name:    typeSpec.Name.Name,
				Message: fmt.Sprintf("exported type alias %s is forbidden; define a named type or import the concrete type at use sites", typeSpec.Name.Name),
			})
		}
	}
}

func checkFunctionVarAliases(fset *token.FileSet, parsed *ast.File, relPath string, report *Report) {
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if len(valueSpec.Values) == 0 {
				continue
			}
			explicitFunc := isFuncTypeExpr(valueSpec.Type)
			for i, name := range valueSpec.Names {
				if name == nil || name.Name == "_" || !isExportedIdent(name.Name) {
					continue
				}
				if i >= len(valueSpec.Values) {
					continue
				}
				if !isFunctionAliasExpr(valueSpec.Values[i], name.Name, explicitFunc) {
					continue
				}
				pos := fset.Position(name.Pos())
				report.addFinding(Finding{
					Rule:    "function_var_alias",
					Path:    relPath,
					Line:    pos.Line,
					Name:    name.Name,
					Message: fmt.Sprintf("exported function variable alias %s is forbidden; call the defining symbol directly", name.Name),
				})
			}
		}
	}
}

func isFuncTypeExpr(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	_, ok := expr.(*ast.FuncType)
	return ok
}

// isFunctionAliasExpr reports high-confidence function value re-exports.
// With an explicit func type, any ident/selector RHS counts. Without a type,
// only same-name selector re-exports (var Foo = pkg.Foo) are flagged.
func isFunctionAliasExpr(expr ast.Expr, exportedName string, explicitFunc bool) bool {
	switch value := expr.(type) {
	case *ast.Ident:
		return explicitFunc
	case *ast.SelectorExpr:
		if explicitFunc {
			return true
		}
		if value.Sel != nil && value.Sel.Name == exportedName {
			return true
		}
		return false
	default:
		return false
	}
}

func checkTrivialWrappers(fset *token.FileSet, parsed *ast.File, relPath string, report *Report) {
	importedSelectors := importedPackageNames(parsed)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv != nil {
			// Methods are out of scope for the low-false-positive wrapper rule.
			continue
		}
		if fn.Name == nil || !isExportedIdent(fn.Name.Name) {
			continue
		}
		if !isTrivialWrapper(fn, importedSelectors) {
			continue
		}
		pos := fset.Position(fn.Pos())
		report.addFinding(Finding{
			Rule:    "trivial_wrapper",
			Path:    relPath,
			Line:    pos.Line,
			Name:    fn.Name.Name,
			Message: fmt.Sprintf("exported function %s is a trivial pass-through wrapper; call the imported function directly", fn.Name.Name),
		})
	}
}

func importedPackageNames(parsed *ast.File) map[string]struct{} {
	names := map[string]struct{}{}
	for _, imp := range parsed.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			switch imp.Name.Name {
			case ".", "_":
				continue
			default:
				names[imp.Name.Name] = struct{}{}
				continue
			}
		}
		// Default alias is the last path element.
		base := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			base = path[idx+1:]
		}
		// Strip major-version suffix v2 etc. for matching? Keep as-is; selector uses package name from code.
		names[base] = struct{}{}
	}
	return names
}

func isTrivialWrapper(fn *ast.FuncDecl, imported map[string]struct{}) bool {
	body := fn.Body.List
	// Allow a single optional empty decl block? No — exactly one statement.
	if len(body) != 1 {
		return false
	}
	params := fieldIdents(fn.Type.Params)
	var call *ast.CallExpr
	switch stmt := body[0].(type) {
	case *ast.ReturnStmt:
		if len(stmt.Results) != 1 {
			// Multi-return trivial forward: return a, b where both from one call is rare;
			// only accept a single call expression result.
			if len(stmt.Results) == 0 {
				return false
			}
			// return x, y is not a single imported call.
			return false
		}
		call, _ = stmt.Results[0].(*ast.CallExpr)
	case *ast.ExprStmt:
		call, _ = stmt.X.(*ast.CallExpr)
	default:
		return false
	}
	if call == nil {
		return false
	}
	if !isImportedSelectorCall(call.Fun, imported) {
		return false
	}
	return callArgsMatchParams(call, params)
}

func isImportedSelectorCall(fun ast.Expr, imported map[string]struct{}) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = imported[pkgIdent.Name]
	return ok
}

func fieldIdents(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var names []string
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			// Anonymous parameter — cannot reliably match; treat as non-wrapper.
			return nil
		}
		for _, name := range field.Names {
			if name.Name == "" {
				return nil
			}
			names = append(names, name.Name)
		}
	}
	return names
}

func callArgsMatchParams(call *ast.CallExpr, params []string) bool {
	if params == nil {
		// nil means anonymous params present — not a clean forwarder.
		// Empty slice means zero params.
		params = []string{}
	}
	if len(call.Args) != len(params) {
		return false
	}
	for i, arg := range call.Args {
		ident, ok := arg.(*ast.Ident)
		if !ok || ident.Name != params[i] {
			return false
		}
	}
	return true
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

type functionMetric struct {
	Name       string
	Line       int
	ELOC       int
	Complexity int
	Nesting    int
}

func functionMetrics(filePath string) ([]functionMetric, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filePath, nil, parser.SkipObjectResolution)
	if err != nil {
		var parseError scanner.ErrorList
		if errors.As(err, &parseError) {
			return nil, nil
		}
		return nil, err
	}
	var metrics []functionMetric
	ast.Inspect(parsed, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		metric := functionMetric{Name: fn.Name.Name, Line: fset.Position(fn.Pos()).Line}
		metric.ELOC = effectiveLines(fset, fn.Body)
		metric.Complexity, metric.Nesting = decisionMetrics(fn.Body)
		metrics = append(metrics, metric)
		return false
	})
	return metrics, nil
}

func effectiveLines(fset *token.FileSet, node ast.Node) int {
	lines := map[int]struct{}{}
	ast.Inspect(node, func(current ast.Node) bool {
		if current == nil {
			return true
		}
		start := fset.Position(current.Pos()).Line
		end := fset.Position(current.End()).Line
		for line := start; line <= end; line++ {
			if line > 0 {
				lines[line] = struct{}{}
			}
		}
		return true
	})
	return len(lines)
}

func decisionMetrics(node ast.Node) (int, int) {
	complexity := 0
	maximumNesting := 0
	var visit func(ast.Node, int)
	visit = func(current ast.Node, nesting int) {
		if current == nil {
			return
		}
		if statement, ok := current.(*ast.IfStmt); ok {
			complexity++
			branchNesting := nesting + 1
			maximumNesting = max(maximumNesting, branchNesting)
			visit(statement.Init, branchNesting)
			visit(statement.Cond, branchNesting)
			visit(statement.Body, branchNesting)
			if alternate, ok := statement.Else.(*ast.IfStmt); ok {
				// An else-if chain is one decision level, not progressively
				// deeper nesting. Counting it as nested rewards mechanical
				// rewrites that do not reduce cognitive load.
				visit(alternate, nesting)
			} else {
				visit(statement.Else, branchNesting)
			}
			return
		}
		childNesting := nesting
		switch typed := current.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			complexity++
			childNesting++
		case *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				complexity++
			}
		}
		maximumNesting = max(maximumNesting, childNesting)
		for _, child := range childNodes(current) {
			visit(child, childNesting)
		}
	}
	visit(node, 0)
	return complexity, maximumNesting
}

func childNodes(node ast.Node) []ast.Node {
	var children []ast.Node
	ast.Inspect(node, func(child ast.Node) bool {
		if child == nil || child == node {
			return true
		}
		children = append(children, child)
		return false
	})
	return children
}

func sourceMetrics(path string) (int, int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}

	// The Go scanner omits comments by default. Counting the distinct source
	// lines occupied by real tokens therefore handles block comments, mixed
	// code/comment lines, and raw strings without a text-level approximation.
	lineSet := map[int]struct{}{}
	fset := token.NewFileSet()
	file := fset.AddFile(path, fset.Base(), len(src))
	var lexical scanner.Scanner
	lexical.Init(file, src, nil, 0)
	for {
		pos, tok, _ := lexical.Scan()
		if tok == token.EOF {
			break
		}
		lineSet[fset.Position(pos).Line] = struct{}{}
	}

	parsed, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		// Syntax errors are reported as path-addressable go_parse_error findings
		// by checkGoAST. Preserve the source-size measurement here so malformed
		// files do not disappear from directory totals.
		return len(lineSet), 0, nil
	}
	complexity := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return len(lineSet), complexity, nil
}
