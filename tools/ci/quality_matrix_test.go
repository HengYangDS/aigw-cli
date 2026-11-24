package main

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The selection-matrix gate runs the exhaustive selection tests of the
// selection_matrix build tag. Its pattern selects those tests and no test of
// the acceptance package's untagged files, which go-behavior already runs: a
// test that both gates select runs twice in every proof.
func TestSelectionMatrixGateSelectsOnlyItsOwnTag(t *testing.T) {
	index := slices.IndexFunc(repositoryQualityGraph.Gates, func(gate qualityGate) bool {
		return gate.ID == selectionMatrixGate
	})
	if index < 0 {
		t.Fatal("the quality graph has no selection-matrix gate")
	}
	args := repositoryQualityGraph.Gates[index].Command.Args
	// A fresh run of the package whose tests are read below: a cached pass
	// never stands in for the matrix.
	if !slices.Contains(args, "-count=1") || !slices.Contains(args, "./"+selectionMatrixPackage) {
		t.Fatalf("the selection-matrix gate does not run %s afresh: %v", selectionMatrixPackage, args)
	}
	at := slices.Index(args, "-run")
	if at < 0 || at+1 >= len(args) || !slices.Contains(args, "-tags=selection_matrix") {
		t.Fatalf("the selection-matrix gate selects no tagged tests by name: %v", args)
	}
	selection, err := regexp.Compile(args[at+1])
	if err != nil {
		t.Fatal(err)
	}
	tagged, untagged := acceptanceTests(t)
	if len(tagged) == 0 {
		t.Fatal("the acceptance package has no test of the selection_matrix tag")
	}
	for _, name := range tagged {
		if !selection.MatchString(name) {
			t.Errorf("the selection-matrix gate misses %s, a test of its own tag", name)
		}
	}
	for _, name := range untagged {
		if selection.MatchString(name) {
			t.Errorf("the selection-matrix gate also runs %s, which go-behavior runs", name)
		}
	}
}

// acceptanceTests returns the top-level test functions of the acceptance
// package that this platform builds, split by whether their file is built only
// with the selection_matrix tag. Go's own build context decides, so a file's
// other constraints (its platform, a Go version) count as Go counts them.
func acceptanceTests(t *testing.T) (tagged, untagged []string) {
	t.Helper()
	folder := filepath.Join(repositoryRoot(t), filepath.FromSlash(selectionMatrixPackage))
	files, err := filepath.Glob(filepath.Join(folder, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	withTag, without := build.Default, build.Default
	withTag.BuildTags, without.BuildTags = []string{"selection_matrix"}, nil
	fileSet := token.NewFileSet()
	for _, path := range files {
		inMatrix, err := withTag.MatchFile(folder, filepath.Base(path))
		if err != nil {
			t.Fatal(err)
		}
		inBehavior, err := without.MatchFile(folder, filepath.Base(path))
		if err != nil {
			t.Fatal(err)
		}
		if !inMatrix && !inBehavior {
			continue
		}
		file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !strings.HasPrefix(function.Name.Name, "Test") {
				continue
			}
			if inBehavior {
				untagged = append(untagged, function.Name.Name)
			} else {
				tagged = append(tagged, function.Name.Name)
			}
		}
	}
	return tagged, untagged
}
