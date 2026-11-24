package main

import (
	"go/ast"
	"go/build/constraint"
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
		return gate.ID == "selection-matrix"
	})
	if index < 0 {
		t.Fatal("the quality graph has no selection-matrix gate")
	}
	args := repositoryQualityGraph.Gates[index].Command.Args
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
// package, split by whether their file carries the selection_matrix tag.
func acceptanceTests(t *testing.T) (tagged, untagged []string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repositoryRoot(t), "internal", "cli", "acceptance", "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		matrix := false
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Pos() > file.Package || !constraint.IsGoBuild(comment.Text) {
					continue
				}
				expression, err := constraint.Parse(comment.Text)
				if err != nil {
					t.Fatal(err)
				}
				with := expression.Eval(func(tag string) bool { return tag == "selection_matrix" })
				without := expression.Eval(func(string) bool { return false })
				matrix = with && !without
			}
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !strings.HasPrefix(function.Name.Name, "Test") {
				continue
			}
			if matrix {
				tagged = append(tagged, function.Name.Name)
			} else {
				untagged = append(untagged, function.Name.Name)
			}
		}
	}
	return tagged, untagged
}
