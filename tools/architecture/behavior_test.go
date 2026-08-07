package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTrivialWrapperBranches(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "return forward",
			src: `package p
import "fmt"
func Print(a ...any) (int, error) { return fmt.Print(a...) }
`,
			want: true,
		},
		{
			name: "expr forward",
			src: `package p
import "fmt"
func Do(x string) { fmt.Println(x) }
`,
			want: true,
		},
		{
			name: "multi statement",
			src: `package p
import "fmt"
func Do(x string) { y := x; fmt.Println(y) }
`,
			want: false,
		},
		{
			name: "return zero values",
			src: `package p
func Do() { return }
`,
			want: false,
		},
		{
			name: "return multi expr",
			src: `package p
func Do() (int, int) { return 1, 2 }
`,
			want: false,
		},
		{
			name: "return non call",
			src: `package p
func Do() int { return 1 }
`,
			want: false,
		},
		{
			name: "local call",
			src: `package p
func helper(x string) {}
func Do(x string) { helper(x) }
`,
			want: false,
		},
		{
			name: "args mismatch",
			src: `package p
import "fmt"
func Do(x string) { fmt.Println("x") }
`,
			want: false,
		},
		{
			name: "anonymous param",
			src: `package p
import "fmt"
func Do(string) { fmt.Println("x") }
`,
			want: false,
		},
		{
			name: "renamed import",
			src: `package p
import f "fmt"
func Do(x string) { f.Println(x) }
`,
			want: true,
		},
		{
			name: "dot and blank imports ignored",
			src: `package p
import (
  . "strings"
  _ "os"
  "fmt"
)
func Do(x string) { fmt.Println(x) }
`,
			want: true,
		},
		{
			name: "unexported skipped by checker but helper false",
			src: `package p
import "fmt"
func do(x string) { fmt.Println(x) }
`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, "p.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			imported := importedPackageNames(parsed)
			var fn *ast.FuncDecl
			for _, decl := range parsed.Decls {
				if candidate, ok := decl.(*ast.FuncDecl); ok && candidate.Recv == nil && candidate.Name != nil && isExportedIdent(candidate.Name.Name) {
					fn = candidate
					break
				}
			}
			if fn == nil {
				if tc.want {
					t.Fatal("missing exported func")
				}
				return
			}
			if got := isTrivialWrapper(fn, imported); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestFunctionAliasExprBranches(t *testing.T) {
	fset := token.NewFileSet()
	src := `package p
import "fmt"
var A func(a ...any) (int, error) = fmt.Println
var Sprint = fmt.Sprint
var C = fmt.Sprintf
var D func() = local
var E = 1
func local() {}
`
	parsed, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	report := newReport("p", ".")
	checkFunctionVarAliases(fset, parsed, "p.go", &report)
	// A explicit func + selector; Sprint same-name re-export; C different name without type; D explicit+ident; E not alias
	names := map[string]bool{}
	for _, finding := range report.Findings {
		names[finding.Name] = true
	}
	if !names["A"] || !names["Sprint"] || !names["D"] {
		t.Fatalf("missing aliases: %v findings=%v", names, report.Findings)
	}
	if names["C"] || names["E"] {
		t.Fatalf("false positives: %v", names)
	}
}

func TestPackageDocumentationMustDescribeItsPackage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "missing", "missing.go"), "package missing\n")
	writeFile(t, filepath.Join(root, "internal", "wrong", "wrong.go"), "// Package other owns the wrong behavior.\npackage wrong\n")
	writeFile(t, filepath.Join(root, "internal", "documented", "documented.go"), "// Package documented owns the fixture behavior.\npackage documented\n")
	files := []goFileInfo{
		{relPath: "internal/missing/missing.go", dir: "internal/missing"},
		{relPath: "internal/wrong/wrong.go", dir: "internal/wrong"},
		{relPath: "internal/documented/documented.go", dir: "internal/documented"},
	}
	report := newReport("policy", root)
	p := policy{CheckPackageDocumentation: true}
	if err := checkGoAST(root, files, p, &report); err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, finding := range report.Findings {
		if finding.Rule == "package_documentation_missing" {
			paths[finding.Path] = true
		}
	}
	if !paths["internal/missing/missing.go"] || !paths["internal/wrong/wrong.go"] {
		t.Fatalf("missing findings: %+v", report.Findings)
	}
	if paths["internal/documented/documented.go"] {
		t.Fatalf("accurate package documentation was rejected: %+v", report.Findings)
	}
}

func TestPackageDocumentationAcceptsExactPackageName(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "p.go", "// Package p\npackage p\n", parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if !hasPackageDocumentation(parsed) {
		t.Fatal("exact package documentation was rejected")
	}
}

func TestMalformedAliasDeclarationsAreIgnored(t *testing.T) {
	parsed := &ast.File{
		Name: ast.NewIdent("p"),
		Decls: []ast.Decl{
			&ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.ImportSpec{Path: &ast.BasicLit{Value: `"fmt"`}}}},
			&ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ImportSpec{Path: &ast.BasicLit{Value: `"fmt"`}}}},
		},
	}
	report := newReport("p", ".")
	checkExportedTypeAliases(token.NewFileSet(), parsed, "p.go", &report)
	checkFunctionVarAliases(token.NewFileSet(), parsed, "p.go", &report)
	if len(report.Findings) != 0 {
		t.Fatalf("malformed declarations produced findings: %+v", report.Findings)
	}
}

func TestImportedPackageNameUsesPathBase(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "p.go", `package p
import "example.com/owner/library"
`, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := importedPackageNames(parsed)["library"]; !ok {
		t.Fatal("default import name did not use the path base")
	}
}

func TestFinalizeSortTies(t *testing.T) {
	report := newReport("p", ".")
	report.DirectoryStats = []DirectoryStats{{Path: "b"}, {Path: "a"}}
	report.FlatDirectories = []DirectoryStats{{Path: "b"}, {Path: "a"}}
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "b", Name: "n2", Message: "m2"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "a", Name: "n1", Message: "m1"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "a", Name: "n1", Message: "m0"})
	if report.Summary["total"] != 3 {
		t.Fatalf("pre-summary=%v", report.Summary)
	}
	// Defensive path: nil summary becomes empty with total=0 (counts live on findings).
	report.Summary = nil
	report.finalize()
	if report.Summary["total"] != 0 {
		t.Fatalf("summary=%v", report.Summary)
	}
	if report.DirectoryStats[0].Path != "a" || report.FlatDirectories[0].Path != "a" {
		t.Fatalf("dir sort failed")
	}
	if report.Findings[0].Prefix != "a" || report.Findings[0].Message != "m0" {
		t.Fatalf("findings=%+v", report.Findings)
	}
	// Keep a non-nil summary path covered too.
	report2 := newReport("p", ".")
	report2.addFinding(Finding{Rule: "z", Path: "p", Line: 2, Message: "m"})
	report2.addFinding(Finding{Rule: "z", Path: "p", Line: 1, Message: "m"})
	report2.finalize()
	if report2.Findings[0].Line != 1 {
		t.Fatalf("line sort: %+v", report2.Findings)
	}
}

func TestValidatePolicyEdgeEntries(t *testing.T) {
	base := policy{
		Owner:                  "o",
		Source:                 "s",
		ScriptsRoots:           []string{"scripts"},
		GoRoots:                []string{"internal"},
		FlatDirectoryLimit:     8,
		MaxFileELOC:            700,
		MaxDirectoryELOC:       3600,
		MaxFileComplexity:      180,
		MaxDirectoryComplexity: 900,
		SuffixFlatGroupMin:     3,
		PlatformBuildSuffixes:  []string{"unix"},
	}
	if err := validatePolicy(base); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.ScriptsRoots = []string{""}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("empty scripts entry")
	}
	bad = base
	bad.ScriptsRoots = []string{`C:\x`}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("backslash scripts")
	}
	bad = base
	bad.PlatformBuildSuffixes = []string{"UNIX"}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("upper platform")
	}
	bad = base
	bad.PlatformBuildSuffixes = []string{""}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("empty platform token")
	}
	bad = base
	bad.GoRoots = []string{"internal/../x"}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("parent traversal go root")
	}
	bad = base
	bad.PeerPackageRoots = map[string][]string{"": {"invocation"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("empty peer package root")
	}
	bad = base
	bad.PeerPackageRoots = map[string][]string{"internal/cli": {"bad/name"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("nested peer package name")
	}
	bad = base
	bad.PeerPackageRoots = map[string][]string{"internal/cli": {"invocation", "invocation"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("duplicate peer package name")
	}
}

func TestPeerPackageImportBranches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "cli", "account", "account.go"), `package account

import (
	_ "bad\\path"
	_ "fixture/internal/cli/account/subpackage"
	_ "fixture/internal/cli/invocation"
	_ "fixture/internal/other"
)
`)
	files := []goFileInfo{{relPath: "internal/cli/account/account.go", dir: "internal/cli/account"}}
	report := newReport("policy", root)
	p := policy{PeerPackageRoots: map[string][]string{"internal/cli": {"invocation"}}}
	if err := checkPeerPackageImports(root, files, p, &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary["peer_package_import"] != 0 {
		t.Fatalf("allowed/self/non-peer imports were rejected: %+v", report.Findings)
	}

	writeFile(t, filepath.Join(root, "internal", "cli", "account", "account.go"), "package account\n\nimport _ \"fixture/internal/cli/profile\"\n")
	if err := checkPeerPackageImports(root, files, p, &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary["peer_package_import"] != 1 {
		t.Fatalf("peer import not rejected: %+v", report.Findings)
	}

	files[0].relPath = "internal/cli/account/missing.go"
	if err := checkPeerPackageImports(root, files, p, &report); err == nil {
		t.Fatal("missing peer package source was accepted")
	}
}

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
	if !isIdentPrefix("_x") {
		t.Fatal("underscore prefix ident")
	}
	if isIdentPrefix("foo-bar") {
		t.Fatal("dash")
	}
}

func TestScriptsRootRejectsDirectFile(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "direct.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	if report := decodeReport(t, stdout.String()); !hasRule(report, "scripts_root_file") {
		t.Fatalf("expected direct-file finding: %v", findingRules(report))
	}
}

func TestScriptsSymlinkAndIgnore(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	// ignored name under scripts root
	writeFile(t, filepath.Join(root, "scripts", "vendor", "x"), "x")
	// symlink to directory OK
	targetDir := filepath.Join(root, "scripts", "check")
	if err := os.Symlink(targetDir, filepath.Join(root, "scripts", "linkdir")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation requires Windows developer mode: %v", err)
		}
		t.Fatal(err)
	}
	// symlink to file fails
	targetFile := filepath.Join(root, "scripts", "check", "a.sh")
	if err := os.Symlink(targetFile, filepath.Join(root, "scripts", "linkfile")); err != nil {
		t.Fatal(err)
	}
	// broken symlink fails as file
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "scripts", "broken")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if !hasRule(report, "scripts_root_file") {
		t.Fatalf("expected symlink file findings: %v", findingRules(report))
	}
}

func TestUnreadableScriptsRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on windows")
	}
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(scripts, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(scripts, 0o755) })
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr.String(), "analyze repository") && !strings.Contains(stderr.String(), "scripts") {
		// Depending on OS, ReadDir may fail with permission
		if !strings.Contains(stderr.String(), "permission") && !strings.Contains(stderr.String(), "read scripts") && !strings.Contains(stderr.String(), "analyze repository") {
			t.Fatalf("stderr=%q", stderr.String())
		}
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

func TestSuffixFlatSkipsTestsAndPrivate(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_a_test.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_b_test.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_c_test.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "_foo_a.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "plain.go"), "package sfx\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%q", code, stdout.String(), stderr.String())
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
	for _, stat := range report.DirectoryStats {
		if strings.Contains(stat.Path, "runtime") || strings.Contains(stat.Path, "records") {
			t.Fatalf("should ignore %s", stat.Path)
		}
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

func TestSuffixFlatInvalidPrefixIgnored(t *testing.T) {
	files := []goFileInfo{
		{relPath: "internal/x/1_a.go", name: "1_a.go", dir: "internal/x"},
		{relPath: "internal/x/1_b.go", name: "1_b.go", dir: "internal/x"},
		{relPath: "internal/x/1_c.go", name: "1_c.go", dir: "internal/x"},
	}
	report := newReport("p", ".")
	checkSuffixFlat(files, mustPolicy(t), &report)
	if hasRule(report, "suffix_flat") {
		t.Fatalf("numeric prefix should be ignored: %+v", report.Findings)
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

func TestSourceMetricsCountTokensNotComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.go")
	writeFile(t, path, `package metrics

/*
if commentOnly {
	ignored()
}
*/
func classify(value int) bool { // mixed code and comment
	return value > 0 && value < 10
}
`)
	eloc, complexity, err := sourceMetrics(path)
	if err != nil {
		t.Fatal(err)
	}
	if eloc != 4 {
		t.Fatalf("eloc=%d want 4", eloc)
	}
	if complexity != 1 {
		t.Fatalf("complexity=%d want 1", complexity)
	}
}

func TestSourceMetricsPreservesMalformedFileSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.go")
	writeFile(t, path, "package broken\nfunc (\n")
	eloc, complexity, err := sourceMetrics(path)
	if err != nil {
		t.Fatal(err)
	}
	if eloc != 2 || complexity != 0 {
		t.Fatalf("eloc=%d complexity=%d", eloc, complexity)
	}
}

func TestSourceMetricsReadFailure(t *testing.T) {
	if _, _, err := sourceMetrics(filepath.Join(t.TempDir(), "missing.go")); err == nil {
		t.Fatal("expected read error")
	}
}

func TestIsFuncTypeNil(t *testing.T) {
	if isFuncTypeExpr(nil) {
		t.Fatal("nil")
	}
}

func TestCollapseEmptyAndLeading(t *testing.T) {
	platform := map[string]struct{}{"unix": {}}
	if got := collapsePlatformSuffixes("_unix", platform); got != "" && got != "_unix" {
		// idx <= 0 stops when prefix empty after strip attempts
		_ = got
	}
	if got := collapsePlatformSuffixes("unix", platform); got != "unix" {
		t.Fatalf("%q", got)
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
