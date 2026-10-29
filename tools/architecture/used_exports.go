package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
)

// checkUsedExportsOnly holds each package that used_exports_only lists to the
// exported identifiers that something outside the package uses. It checks the
// exported top-level functions, types, variables and constants of the
// package's production files. A Go file in another directory uses one when it
// selects it through an import of the package; the package's own tests do not
// count. A used identifier also uses the types that its exported API names:
// those in its signature or declared type, the exported fields and methods of a
// used type, also of an unexported type or anonymous struct or interface that
// such an API names, and the constants declared with a used type. Parameter,
// result and type parameter names are not the package's types. Methods and
// fields are not checked themselves. Go syntax stays with
// the Go toolchain: a file with a syntax error contributes what the parser
// recovers. A listed package without production Go files is a stale entry.
func checkUsedExportsOnly(root string, files []goFileInfo, p policy, report *Report) error {
	if len(p.UsedExportsOnly) == 0 {
		return nil
	}
	module, err := readModuleIdentity(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	parse := func(file goFileInfo) (*ast.File, error) {
		if tree, ok := parsed[file.relPath]; ok {
			return tree, nil
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.relPath)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.relPath, err)
		}
		// A syntax error is the Go toolchain's to report; the parser still returns
		// the tree that it recovers.
		tree, _ := parser.ParseFile(fset, file.relPath, data, 0)
		parsed[file.relPath] = tree
		return tree, nil
	}
	for _, dir := range p.UsedExportsOnly {
		surface, err := collectExportedSurface(files, dir, fset, parse)
		if err != nil {
			return err
		}
		if !surface.production {
			report.addFinding(Finding{
				Rule:    "used_exports_only_entry",
				Path:    dir,
				Package: dir,
				Message: fmt.Sprintf("used_exports_only lists %q, which has no production Go files; delete the stale entry", dir),
			})
			continue
		}
		used, err := externalSelections(files, module+"/"+dir, dir, parse)
		if err != nil {
			return err
		}
		reached := surface.reach(used)
		for _, declaration := range surface.sorted() {
			if !declaration.exported || reached[declaration.name] {
				continue
			}
			report.addFinding(Finding{
				Rule:    "unused_export",
				Path:    declaration.path,
				Line:    declaration.line,
				Package: dir,
				Message: fmt.Sprintf("package %q exports %s, which nothing outside the package uses; unexport or delete it", dir, declaration.name),
			})
		}
	}
	return nil
}

type surfaceEntry struct {
	name     string
	exported bool
	path     string
	line     int
	// api holds the parts of the declaration that a caller outside the package
	// reaches, gathered from every file that declares the name.
	api []apiNode
}

// apiNode is one part of a declaration's API with the type parameters in scope
// there, whose names do not refer to the package's types.
type apiNode struct {
	node   ast.Node
	params map[string]bool
}

// exportedSurface holds a package's exported top-level declarations and its
// unexported types, which a used API can name.
type exportedSurface struct {
	production   bool
	declarations map[string]surfaceEntry
	// constants maps a type name to the exported constants declared with it.
	constants map[string][]string
}

func collectExportedSurface(files []goFileInfo, dir string, fset *token.FileSet, parse func(goFileInfo) (*ast.File, error)) (exportedSurface, error) {
	surface := exportedSurface{declarations: map[string]surfaceEntry{}, constants: map[string][]string{}}
	methods := map[string][]apiNode{}
	for _, file := range files {
		if file.dir != dir || file.isTest {
			continue
		}
		surface.production = true
		tree, err := parse(file)
		if err != nil {
			return exportedSurface{}, err
		}
		// declare records one declaration; a name declared in several files,
		// such as one per platform, keeps its first position and every file's API.
		declare := func(name *ast.Ident, api ...apiNode) {
			declaration, ok := surface.declarations[name.Name]
			if !ok {
				declaration = surfaceEntry{
					name: name.Name, exported: name.IsExported(), path: file.relPath, line: fset.Position(name.Pos()).Line,
				}
			}
			declaration.api = append(declaration.api, api...)
			surface.declarations[name.Name] = declaration
		}
		for _, decl := range tree.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				switch {
				case decl.Recv == nil:
					if decl.Name.IsExported() {
						declare(decl.Name, apiNode{decl.Type, fieldNames(decl.Type.TypeParams)})
					}
				case len(decl.Recv.List) > 0 && decl.Name.IsExported():
					owner := receiverTypeName(decl.Recv.List[0].Type)
					// A generic method adds its own type parameters to its receiver's.
					params := receiverParams(decl.Recv.List[0].Type)
					maps.Copy(params, fieldNames(decl.Type.TypeParams))
					methods[owner] = append(methods[owner], apiNode{decl.Type, params})
				}
			case *ast.GenDecl:
				collectGenDecl(decl, declare, surface.constants)
			}
		}
	}
	for owner, signatures := range methods {
		if declaration, ok := surface.declarations[owner]; ok {
			declaration.api = append(declaration.api, signatures...)
			surface.declarations[owner] = declaration
		}
	}
	return surface, nil
}

// collectGenDecl declares the types, exported variables and exported
// constants of one declaration. A constant without its own type or value
// repeats the type of the one before it, as Go's implicit repetition does.
func collectGenDecl(decl *ast.GenDecl, declare func(*ast.Ident, ...apiNode), constants map[string][]string) {
	var repeated ast.Expr
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			params := fieldNames(spec.TypeParams)
			var api []apiNode
			for _, node := range exportedShape(spec) {
				api = append(api, apiNode{node, params})
			}
			declare(spec.Name, api...)
		case *ast.ValueSpec:
			declared := spec.Type
			if decl.Tok == token.CONST {
				switch {
				case spec.Type != nil:
					repeated = spec.Type
				case len(spec.Values) > 0:
					repeated = nil
				}
				declared = repeated
			}
			for _, name := range spec.Names {
				if !name.IsExported() {
					continue
				}
				if declared != nil {
					declare(name, apiNode{node: declared})
				} else {
					declare(name)
				}
				if typeName, ok := declared.(*ast.Ident); ok && decl.Tok == token.CONST {
					constants[typeName.Name] = append(constants[typeName.Name], name.Name)
				}
			}
		}
	}
}

// exportedShape returns the parts of a type declaration that reach follows:
// its type parameters and its type, whose unexported members reach skips.
func exportedShape(spec *ast.TypeSpec) []ast.Node {
	if spec.TypeParams != nil {
		return []ast.Node{spec.TypeParams, spec.Type}
	}
	return []ast.Node{spec.Type}
}

// fieldNames returns the names that a field list declares, such as type
// parameters.
func fieldNames(fields *ast.FieldList) map[string]bool {
	names := map[string]bool{}
	if fields != nil {
		for _, field := range fields.List {
			for _, name := range field.Names {
				names[name.Name] = true
			}
		}
	}
	return names
}

// receiverParams returns the type parameter names that a method's receiver
// declares, as in func (Box[T]) Get() T.
func receiverParams(expr ast.Expr) map[string]bool {
	names := map[string]bool{}
	var indices []ast.Expr
	switch receiver := expr.(type) {
	case *ast.ParenExpr:
		return receiverParams(receiver.X)
	case *ast.StarExpr:
		return receiverParams(receiver.X)
	case *ast.IndexExpr:
		indices = []ast.Expr{receiver.Index}
	case *ast.IndexListExpr:
		indices = receiver.Indices
	}
	for _, index := range indices {
		if name, ok := index.(*ast.Ident); ok {
			names[name.Name] = true
		}
	}
	return names
}

func receiverTypeName(expr ast.Expr) string {
	switch receiver := expr.(type) {
	case *ast.ParenExpr:
		return receiverTypeName(receiver.X)
	case *ast.StarExpr:
		return receiverTypeName(receiver.X)
	case *ast.IndexExpr:
		return receiverTypeName(receiver.X)
	case *ast.IndexListExpr:
		return receiverTypeName(receiver.X)
	case *ast.Ident:
		return receiver.Name
	}
	return ""
}

// reach closes the used identifiers over the exported API that each one names.
func (s exportedSurface) reach(used map[string]bool) map[string]bool {
	reached := map[string]bool{}
	queue := make([]string, 0, len(used))
	for name := range used {
		queue = append(queue, name)
	}
	var params map[string]bool
	var visit func(ast.Node) bool
	members := func(fields *ast.FieldList) {
		for _, field := range fields.List {
			if len(field.Names) == 0 || slices.ContainsFunc(field.Names, (*ast.Ident).IsExported) {
				ast.Inspect(field.Type, visit)
			}
		}
	}
	visit = func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.SelectorExpr:
			// Another package's identifier.
			return false
		case *ast.StructType:
			members(node.Fields)
			return false
		case *ast.InterfaceType:
			members(node.Methods)
			return false
		case *ast.Field:
			// A parameter, result or type parameter name is not a type.
			if node.Type != nil {
				ast.Inspect(node.Type, visit)
			}
			return false
		case *ast.Ident:
			if !params[node.Name] {
				queue = append(queue, node.Name)
			}
		}
		return true
	}
	for len(queue) > 0 {
		name := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		declaration, ok := s.declarations[name]
		if !ok || reached[name] {
			continue
		}
		reached[name] = true
		queue = append(queue, s.constants[name]...)
		for _, part := range declaration.api {
			params = part.params
			ast.Inspect(part.node, visit)
		}
	}
	return reached
}

func (s exportedSurface) sorted() []surfaceEntry {
	declarations := make([]surfaceEntry, 0, len(s.declarations))
	for _, declaration := range s.declarations {
		declarations = append(declarations, declaration)
	}
	sort.Slice(declarations, func(i, j int) bool {
		if declarations[i].path != declarations[j].path {
			return declarations[i].path < declarations[j].path
		}
		return declarations[i].line < declarations[j].line
	})
	return declarations
}

// externalSelections names the identifiers of importPath that Go files outside
// dir select through their import of it. A dot import makes every exported
// identifier that its file leaves unresolved a possible selection.
func externalSelections(files []goFileInfo, importPath, dir string, parse func(goFileInfo) (*ast.File, error)) (map[string]bool, error) {
	used := map[string]bool{}
	for _, file := range files {
		if file.dir == dir {
			continue
		}
		tree, err := parse(file)
		if err != nil {
			return nil, err
		}
		local := ""
		for _, spec := range tree.Imports {
			if value, _ := strconv.Unquote(spec.Path.Value); value == importPath {
				local = path.Base(importPath)
				if spec.Name != nil {
					local = spec.Name.Name
				}
			}
		}
		switch local {
		case "", "_":
			continue
		case ".":
			// Another package can select only exported names.
			for _, name := range tree.Unresolved {
				if name.IsExported() {
					used[name.Name] = true
				}
			}
			continue
		}
		ast.Inspect(tree, func(node ast.Node) bool {
			if selection, ok := node.(*ast.SelectorExpr); ok {
				if qualifier, ok := selection.X.(*ast.Ident); ok && qualifier.Obj == nil && qualifier.Name == local {
					used[selection.Sel.Name] = true
				}
			}
			return true
		})
	}
	return used, nil
}
