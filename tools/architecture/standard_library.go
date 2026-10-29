package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// checkStandardLibraryOnly holds each package that standard_library_only lists
// to standard-library imports: no other package of the module and no
// third-party module. Test files are fixtures and may import more. A listed
// package without production Go files is a stale entry.
func checkStandardLibraryOnly(root string, files []goFileInfo, p policy, report *Report) error {
	if len(p.StandardLibraryOnly) == 0 {
		return nil
	}
	module, err := readModuleIdentity(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	listed := make(map[string]bool, len(p.StandardLibraryOnly))
	for _, dir := range p.StandardLibraryOnly {
		listed[dir] = true
	}
	present := map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range files {
		if file.isTest || !listed[file.dir] {
			continue
		}
		present[file.dir] = true
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.relPath)))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.relPath, err)
		}
		parsed, err := parser.ParseFile(fset, file.relPath, data, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse imports %s: %w", file.relPath, err)
		}
		for _, imported := range parsed.Imports {
			importPath, _ := strconv.Unquote(imported.Path.Value)
			if isStandardLibrary(importPath, module) {
				continue
			}
			report.addFinding(Finding{
				Rule:    "standard_library_only",
				Path:    file.relPath,
				Line:    fset.Position(imported.Pos()).Line,
				Package: file.dir,
				Message: fmt.Sprintf("package %q may import only the standard library, not %q", file.dir, importPath),
			})
		}
	}
	for _, dir := range p.StandardLibraryOnly {
		if present[dir] {
			continue
		}
		report.addFinding(Finding{
			Rule:    "standard_library_only_entry",
			Path:    dir,
			Package: dir,
			Message: fmt.Sprintf("standard_library_only lists %q, which has no production Go files; delete the stale entry", dir),
		})
	}
	return nil
}

// isStandardLibrary reports whether an import path names a standard-library
// package: it lies outside the module, its first element has no dot, and it is
// not cgo's pseudo-package.
func isStandardLibrary(importPath, module string) bool {
	if importPath == "C" || importPath == module || strings.HasPrefix(importPath, module+"/") {
		return false
	}
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}
