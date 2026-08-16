package main

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIgnoreHelpers(t *testing.T) {
	p := mustPolicy(t)
	if shouldIgnoreDirName("vendor", p) != true {
		t.Fatal("vendor")
	}
	if shouldIgnoreDirName(".", p) {
		t.Fatal("dot")
	}
	if shouldIgnoreDirName("..", p) {
		t.Fatal("dotdot")
	}
	if shouldIgnoreDirName(".hidden", p) {
		t.Fatal("hidden not ignored unless listed")
	}
	if !shouldIgnoreRelPath("vendor/pkg/a.go", p) {
		t.Fatal("ignore root")
	}
	if !shouldIgnoreRelPath("internal/runtime/x.go", p) {
		t.Fatal("ignore dir name")
	}
	if shouldIgnoreRelPath("internal/pkg/a.go", p) {
		t.Fatal("normal path")
	}
	if shouldIgnoreRelPath("", p) {
		t.Fatal("empty")
	}
}

func TestRelativePolicyFromRoot(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".config", "checks", "architecture")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "policy.toml"), []byte(validPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", defaultPolicyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if report.Policy != defaultPolicyPath {
		t.Fatalf("policy=%q", report.Policy)
	}
}

func TestCollectIgnoresNonGoAndRuntime(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "note.txt"), "x")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "runtime", "hidden.go"), "package runtime\n")
	writeFile(t, filepath.Join(root, "records", "internal", "x.go"), "package x\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%s", code, stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if !report.OK {
		t.Fatalf("ignored paths produced findings: %+v", report.Findings)
	}
}

func TestTypeAliasUnexportedAndDefined(t *testing.T) {
	fset := token.NewFileSet()
	src := `package p
type Exported = int
type unexported = int
type Defined int
`
	parsed, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	report := newReport("p", ".")
	checkExportedTypeAliases(fset, parsed, "p.go", &report)
	if len(report.Findings) != 1 || report.Findings[0].Name != "Exported" {
		t.Fatalf("%+v", report.Findings)
	}
}

func TestCallArgsAndFieldIdents(t *testing.T) {
	if callArgsMatchParams(&ast.CallExpr{}, nil) != true {
		t.Fatal("empty")
	}
	if callArgsMatchParams(&ast.CallExpr{Args: []ast.Expr{&ast.Ident{Name: "x"}}}, nil) {
		t.Fatal("nil params with args")
	}
	if callArgsMatchParams(&ast.CallExpr{Args: []ast.Expr{&ast.BasicLit{}}}, []string{"x"}) {
		t.Fatal("non ident arg")
	}
	fields := &ast.FieldList{List: []*ast.Field{{Type: &ast.Ident{Name: "int"}}}}
	if fieldIdents(fields) != nil {
		t.Fatal("anonymous")
	}
	if fieldIdents(nil) != nil {
		t.Fatal("nil fields")
	}
	named := &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{{Name: "x"}}, Type: &ast.Ident{Name: "int"}}}}
	if got := fieldIdents(named); len(got) != 1 || got[0] != "x" {
		t.Fatalf("%v", got)
	}
	if isImportedSelectorCall(&ast.Ident{Name: "x"}, map[string]struct{}{"fmt": {}}) {
		t.Fatal("not selector")
	}
	if isImportedSelectorCall(&ast.SelectorExpr{X: &ast.SelectorExpr{}, Sel: &ast.Ident{Name: "Y"}}, map[string]struct{}{"fmt": {}}) {
		t.Fatal("nested selector")
	}
}

func TestIsFunctionAliasDefaultFalse(t *testing.T) {
	if isFunctionAliasExpr(&ast.BasicLit{}, "X", true) {
		t.Fatal("lit")
	}
	if isFunctionAliasExpr(&ast.Ident{Name: "x"}, "X", false) {
		t.Fatal("ident without explicit")
	}
	if !isFunctionAliasExpr(&ast.SelectorExpr{X: &ast.Ident{Name: "pkg"}, Sel: &ast.Ident{Name: "Other"}}, "X", true) {
		t.Fatal("explicit selector")
	}
}

func TestCheckGoASTReadError(t *testing.T) {
	report := newReport("p", ".")
	err := checkGoAST(t.TempDir(), []goFileInfo{{relPath: "missing.go", name: "missing.go", dir: ".", isTest: false}}, mustPolicy(t), &report)
	if err == nil {
		t.Fatal("expected read error")
	}
}

func TestCheckGoASTSkipsTestFiles(t *testing.T) {
	report := newReport("p", ".")
	err := checkGoAST(
		t.TempDir(),
		[]goFileInfo{{relPath: "missing_test.go", name: "missing_test.go", dir: ".", isTest: true}},
		mustPolicy(t),
		&report,
	)
	if err != nil || !report.OK {
		t.Fatalf("test-only source entered the production AST plane: report=%+v err=%v", report, err)
	}
}

func TestAbsolutePolicyOutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	policyPath := writePolicy(t, other, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	report := decodeReport(t, stdout.String())
	if report.Policy == "" {
		t.Fatal("empty policy")
	}
}

func TestExprStmtNonCallNotWrapper(t *testing.T) {
	src := `package p
func Do() { x = 1 }
var x int
`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if isTrivialWrapper(fn, nil) {
			t.Fatal("assignment should not wrap")
		}
	}
}

func mustPolicy(t *testing.T) policy {
	t.Helper()
	path := writePolicy(t, t.TempDir(), validPolicy)
	p, err := loadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFunctionVarMultiNameAndBlank(t *testing.T) {
	fset := token.NewFileSet()
	src := `package p
import "fmt"
var _, Keep func(a ...any) (int, error) = fmt.Println, fmt.Println
var OnlyOne, _ = fmt.Sprint, fmt.Sprint
`
	// First line: multi names with explicit func type.
	// Note: Go requires same type for multi var with type; values aligned.
	src = `package p
import "fmt"
var F1, F2 func(a ...any) (int, error) = fmt.Println, fmt.Sprint
var _ func() = local
var Sprint = fmt.Sprint
func local() {}
`
	parsed, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	report := newReport("p", ".")
	checkFunctionVarAliases(fset, parsed, "p.go", &report)
	names := map[string]bool{}
	for _, f := range report.Findings {
		names[f.Name] = true
	}
	if !names["F1"] || !names["F2"] || !names["Sprint"] {
		t.Fatalf("%v", names)
	}
	if names["_"] {
		t.Fatal("blank should not be reported")
	}
}

func TestAnalyzeRepositoryDirect(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	report, err := analyzeRepository(root, mustPolicy(t), "policy.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("%+v", report.Findings)
	}
}

func TestAnalyzeRepositoryReportsStageFailures(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name string
		root string
		set  func(*policy)
	}{
		{name: "module identity", root: t.TempDir(), set: func(p *policy) { p.CheckModuleIdentity = true }},
		{name: "portability", root: missing, set: func(p *policy) { p.CheckPortability = true }},
		{name: "semantic names", root: missing, set: func(p *policy) { p.CheckSemanticNames = true }},
		{name: "text layout", root: missing},
		{name: "package children", root: t.TempDir(), set: func(p *policy) {
			p.PackageChildren = map[string][]string{"invalid\x00root": {"child"}}
		}},
		{name: "Go roots", root: t.TempDir(), set: func(p *policy) { p.GoRoots = []string{"invalid\x00root"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := policy{}
			if test.set != nil {
				test.set(&p)
			}
			if _, err := analyzeRepository(test.root, p, "policy.toml"); err == nil {
				t.Fatal("stage failure was accepted")
			}
		})
	}
}

func TestAnalyzeRepositoryPropagatesSemanticStageFailures(t *testing.T) {
	original := repositoryAnalysis
	t.Cleanup(func() { repositoryAnalysis = original })
	want := errors.New("stage failed")
	for name, configure := range map[string]func(){
		"decision records": func() {
			repositoryAnalysis.decisionRecords = func(string, *Report) error { return want }
		},
		"peer imports": func() {
			repositoryAnalysis.peerImports = func(string, []goFileInfo, policy, *Report) error { return want }
		},
		"import edges": func() {
			repositoryAnalysis.importEdges = func(string, []goFileInfo, policy, *Report) error { return want }
		},
		"Go AST": func() {
			repositoryAnalysis.goAST = func(string, []goFileInfo, policy, *Report) error { return want }
		},
	} {
		t.Run(name, func(t *testing.T) {
			repositoryAnalysis = original
			configure()
			p := policy{CheckDecisionRecords: name == "decision records"}
			if _, err := analyzeRepository(t.TempDir(), p, "policy.toml"); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestCollectGoFilesRejectsInvalidRoot(t *testing.T) {
	if _, err := collectGoFiles(t.TempDir(), policy{GoRoots: []string{"invalid\x00root"}}); err == nil {
		t.Fatal("invalid Go root was accepted")
	}
}

func TestCollectGoFilesIgnoresConfiguredFilePath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "ignored.go"), "package internal\n")
	files, err := collectGoFiles(root, policy{GoRoots: []string{"internal"}, IgnoreRoots: []string{"internal"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("ignored path collected: files=%+v", files)
	}
}

func TestIsFuncTypeNil(t *testing.T) {
	if isFuncTypeExpr(nil) {
		t.Fatal("nil")
	}
}

func TestMainCoversEntry(t *testing.T) {
	originalArgs := os.Args
	originalExit := exitFunc
	defer func() {
		os.Args = originalArgs
		exitFunc = originalExit
	}()
	var code int
	exitFunc = func(c int) { code = c }
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	os.Args = []string{"architecture", "-root", root, "-policy", policyPath}
	main()
	if code != 0 {
		t.Fatalf("main exit=%d", code)
	}
}

func TestRunReportsAnalysisFailure(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing-go-source"), filepath.Join(root, "internal", "broken.go")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "analyze repository") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestFunctionVarValueCountMismatch(t *testing.T) {
	// Multi-name without enough values is invalid Go, so build AST manually.
	report := newReport("p", ".")
	fset := token.NewFileSet()
	parsed := &ast.File{
		Name: ast.NewIdent("p"),
		Decls: []ast.Decl{
			&ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names:  []*ast.Ident{ast.NewIdent("A"), ast.NewIdent("B")},
						Type:   &ast.FuncType{Params: &ast.FieldList{}},
						Values: []ast.Expr{&ast.Ident{Name: "local"}},
					},
					&ast.ValueSpec{
						Names: []*ast.Ident{ast.NewIdent("C")},
						// no values
					},
					&ast.ValueSpec{
						Names:  []*ast.Ident{nil, ast.NewIdent("D")},
						Type:   &ast.FuncType{Params: &ast.FieldList{}},
						Values: []ast.Expr{&ast.Ident{Name: "local"}, &ast.Ident{Name: "local"}},
					},
				},
			},
			&ast.GenDecl{Tok: token.CONST},
		},
	}
	_ = fset
	checkFunctionVarAliases(token.NewFileSet(), parsed, "p.go", &report)
	// A has value+func type; B lacks value; nil name skipped; D flagged
	names := map[string]bool{}
	for _, f := range report.Findings {
		names[f.Name] = true
	}
	if !names["A"] || !names["D"] || names["B"] || names["C"] {
		t.Fatalf("%v %+v", names, report.Findings)
	}
}

func TestFieldIdentsEmptyName(t *testing.T) {
	fields := &ast.FieldList{List: []*ast.Field{
		{Names: []*ast.Ident{{Name: ""}}, Type: &ast.Ident{Name: "int"}},
	}}
	if fieldIdents(fields) != nil {
		t.Fatal("empty name")
	}
}

func TestImportedPackageNilPath(t *testing.T) {
	parsed := &ast.File{Imports: []*ast.ImportSpec{{Path: nil}}}
	if got := importedPackageNames(parsed); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

func TestShouldIgnoreRelPathEmptyParts(t *testing.T) {
	p := mustPolicy(t)
	// strings.Split("", "/") yields []string{""}; first part "" not in ignore roots
	if shouldIgnoreRelPath("", p) {
		t.Fatal("empty")
	}
}

func TestFinalizeNameAndMessageOrder(t *testing.T) {
	report := newReport("p", ".")
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "x", Name: "b", Message: "m2"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "x", Name: "a", Message: "m1"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "x", Name: "a", Message: "m0"})
	report.finalize()
	if report.Findings[0].Name != "a" || report.Findings[0].Message != "m0" {
		t.Fatalf("%+v", report.Findings)
	}
}

func TestSelectorSameNameNilSel(t *testing.T) {
	// Sel nil shouldn't panic; treat as non-alias without explicit func
	expr := &ast.SelectorExpr{X: ast.NewIdent("pkg"), Sel: nil}
	if isFunctionAliasExpr(expr, "Foo", false) {
		t.Fatal("nil sel")
	}
}
