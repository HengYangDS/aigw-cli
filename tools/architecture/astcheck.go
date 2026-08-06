package main

import (
	"fmt"
	"go/ast"
	"go/parser"
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
