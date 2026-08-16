package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
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
