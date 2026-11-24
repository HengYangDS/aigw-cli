package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// Every top-level test of this package calls t.Parallel first, which keeps
// the package's run short under the race detector, unless it changes the
// state of the whole process (t.Setenv, t.Chdir), which Go refuses in a
// parallel test. Go runs those first and alone, so they never overlap a
// parallel one. The files of every build tag are read.
func TestEveryTestRunsInParallelUnlessItChangesTheProcess(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !isTopLevelTest(function) || changesTheProcess(function.Body) {
				continue
			}
			if !callsParallelFirst(function.Body) {
				t.Errorf("%s: %s does not call t.Parallel first", fileSet.Position(function.Pos()), function.Name.Name)
			}
		}
	}
}

// isTopLevelTest reports whether function is a test that `go test` runs:
// Test… with one *testing.T parameter.
func isTopLevelTest(function *ast.FuncDecl) bool {
	if function.Recv != nil || function.Body == nil || len(function.Name.Name) < 4 || function.Name.Name[:4] != "Test" {
		return false
	}
	params := function.Type.Params.List
	if len(params) != 1 {
		return false
	}
	pointer, ok := params[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "T"
}

// changesTheProcess reports whether body, or a subtest in it, calls
// t.Setenv or t.Chdir.
func changesTheProcess(body *ast.BlockStmt) bool {
	changes := false
	ast.Inspect(body, func(node ast.Node) bool {
		if name, ok := methodOfT(node); ok && (name == "Setenv" || name == "Chdir") {
			changes = true
		}
		return !changes
	})
	return changes
}

// callsParallelFirst reports whether the first statement of body is
// t.Parallel().
func callsParallelFirst(body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return false
	}
	statement, ok := body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	name, ok := methodOfT(statement.X)
	return ok && name == "Parallel"
}

// methodOfT returns the method that node calls on t, if node is such a call.
func methodOfT(node ast.Node) (string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok || receiver.Name != "t" {
		return "", false
	}
	return selector.Sel.Name, true
}
