package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

func checkGoAST(root string, files []goFileInfo, p policy, report *Report) error {
	fset := token.NewFileSet()
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
		_, err = parser.ParseFile(
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
	}
	return nil
}
