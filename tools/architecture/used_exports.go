package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
// count. A used identifier also uses what its exported API names: the types in
// its signature or declared type, the exported fields and methods of a used
// type, including those promoted from an embedded unexported type, and the
// constants declared with a used type. Methods and fields are not checked
// themselves. Go syntax stays with the Go toolchain: a file with a syntax
// error contributes what parses before the error. A listed package without
// production Go files is a stale entry.
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
		// A syntax error is the Go toolchain's to report; the partial tree keeps
		// what parsed before it, and a file that yields no tree contributes nothing.
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
	api []ast.Node
}

// exportedSurface holds a package's exported top-level declarations and its
// unexported types, which an exported type can embed.
type exportedSurface struct {
	production   bool
	declarations map[string]surfaceEntry
	// constants maps a type name to the exported constants declared with it.
	constants map[string][]string
}

func collectExportedSurface(files []goFileInfo, dir string, fset *token.FileSet, parse func(goFileInfo) (*ast.File, error)) (exportedSurface, error) {
	surface := exportedSurface{declarations: map[string]surfaceEntry{}, constants: map[string][]string{}}
	methods := map[string][]ast.Node{}
	for _, file := range files {
		if file.dir != dir || file.isTest {
			continue
		}
		surface.production = true
		tree, err := parse(file)
		if err != nil {
			return exportedSurface{}, err
		}
		if tree == nil {
			continue
		}
		// declare records one declaration; a name declared in several files,
		// such as one per platform, keeps its first position and every file's API.
		declare := func(name *ast.Ident, api ...ast.Node) {
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
						declare(decl.Name, decl.Type)
					}
				case len(decl.Recv.List) > 0 && decl.Name.IsExported():
					owner := receiverTypeName(decl.Recv.List[0].Type)
					methods[owner] = append(methods[owner], decl.Type)
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
func collectGenDecl(decl *ast.GenDecl, declare func(*ast.Ident, ...ast.Node), constants map[string][]string) {
	var repeated ast.Expr
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			declare(spec.Name, exportedShape(spec)...)
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
					declare(name, declared)
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

// exportedShape returns what a caller outside the package reaches through a
// type: its type parameters, and its embedded and exported fields and methods,
// or else its whole underlying type.
func exportedShape(spec *ast.TypeSpec) []ast.Node {
	var nodes []ast.Node
	if spec.TypeParams != nil {
		nodes = append(nodes, spec.TypeParams)
	}
	var members *ast.FieldList
	switch shape := spec.Type.(type) {
	case *ast.StructType:
		members = shape.Fields
	case *ast.InterfaceType:
		members = shape.Methods
	default:
		return append(nodes, spec.Type)
	}
	for _, member := range members.List {
		if len(member.Names) == 0 || slices.ContainsFunc(member.Names, (*ast.Ident).IsExported) {
			nodes = append(nodes, member.Type)
		}
	}
	return nodes
}

func receiverTypeName(expr ast.Expr) string {
	switch receiver := expr.(type) {
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
	for len(queue) > 0 {
		name := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		declaration, ok := s.declarations[name]
		if !ok || reached[name] {
			continue
		}
		reached[name] = true
		queue = append(queue, s.constants[name]...)
		for _, node := range declaration.api {
			ast.Inspect(node, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.SelectorExpr:
					// Another package's identifier.
					return false
				case *ast.Ident:
					queue = append(queue, node.Name)
				}
				return true
			})
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
// dir select through their import of it. A dot import makes every identifier
// that its file leaves unresolved a possible selection.
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
		if tree == nil {
			continue
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
			for _, name := range tree.Unresolved {
				used[name.Name] = true
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
