package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryContractsRejectNonPortableModuleIdentity(t *testing.T) {
	for name, module := range map[string]string{
		"forge":      "gitlab.example.local/group/aigw-cli",
		"personal":   "github.com/example-user/aigw-cli",
		"url":        "https://example.test/aigw-cli",
		"filesystem": "/opt/team/aigw-cli",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "go.mod"), "module "+module+"\n\ngo 1.26.5\n")
			writeFile(t, filepath.Join(root, "cmd", "aigw", "main.go"), "package main\n")
			report := newReport("policy", root)
			if err := checkModuleIdentity(root, &report); err != nil {
				t.Fatal(err)
			}
			if !hasRule(report, "module_identity") {
				t.Fatalf("module %q accepted: %+v", module, report.Findings)
			}
		})
	}
}

func TestRepositoryContractsRejectMissingModuleAndMissingRoot(t *testing.T) {
	root := t.TempDir()
	report := newReport("policy", root)
	if err := checkModuleIdentity(root, &report); err == nil {
		t.Fatal("missing go.mod accepted")
	}
	writeFile(t, filepath.Join(root, "go.mod"), "go 1.26.5\n")
	if err := checkModuleIdentity(root, &report); err == nil {
		t.Fatal("missing module declaration accepted")
	}
	if err := checkPortability(filepath.Join(root, "missing"), &report); err == nil {
		t.Fatal("missing portability root accepted")
	}
}

func TestDecisionRecordReadFailureIsReported(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "decisions")
	writeFile(t, filepath.Join(directory, "README.md"), "# Decisions\n")
	if err := os.Symlink(filepath.Join(root, "missing-record"), filepath.Join(directory, "dr-0001-missing.md")); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err == nil || !strings.Contains(err.Error(), "read docs/decisions/dr-0001-missing.md") {
		t.Fatalf("decision record read error = %v", err)
	}
}

func TestRepositoryContractsRejectPublicPackageAndForeignInternalImport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module aigw-cli\n\ngo 1.26.5\n")
	writeFile(t, filepath.Join(root, "client", "client.go"), "package client\n")
	writeFile(t, filepath.Join(root, "cmd", "aigw", "main.go"), "package main\n\nimport _ \"gitlab.example.local/group/aigw-cli/internal/core\"\n")
	report := newReport("policy", root)
	if err := checkModuleIdentity(root, &report); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"public_go_package", "foreign_internal_import"} {
		if !hasRule(report, rule) {
			t.Fatalf("missing %s: %+v", rule, report.Findings)
		}
	}
}

func TestRepositoryContractsRejectPortableSourceLeaks(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "scripts", "release", "publish.sh"), "AIGW_GITLAB_AUTHOR_EMAIL=${AIGW_GITLAB_AUTHOR_EMAIL:-maintainer@example.test}\n")
	writeFile(t, filepath.Join(root, ".config", "release", "team.allowed-signers"), "actor ssh-ed25519 fixture\n")
	writeFile(t, filepath.Join(root, ".github", "workflows", "verify.yml"), "runs-on: [self-hosted, macos]\n")
	runGit(t, root, "add", ".")
	report := newReport("policy", root)
	if err := checkPortability(root, &report); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"implicit_publication_identity", "tracked_trust_anchor", "fixed_runner_inventory"} {
		if !hasRule(report, rule) {
			t.Fatalf("missing %s: %+v", rule, report.Findings)
		}
	}
}

func TestRepositoryContractsRejectShellOwnedAutomation(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "scripts", "release", "publish.sh"), "#!/bin/sh\nprintf 'publish\\n'\n")
	writeFile(t, filepath.Join(root, "tools", "quality"), "#!/usr/bin/env bash\nprintf 'quality\\n'\n")
	writeFile(t, filepath.Join(root, ".githooks", "pre-commit"), "#!/bin/sh\nethos hook admit pre-tool\n")
	runGit(t, root, "add", ".")

	report := newReport("policy", root)
	if err := checkPortability(root, &report); err != nil {
		t.Fatal(err)
	}
	if got := countRule(report, "shell_owned_automation"); got != 3 {
		t.Fatalf("shell-owned findings = %d, want 3: %+v", got, report.Findings)
	}
}

func countRule(report Report, rule string) int {
	count := 0
	for _, finding := range report.Findings {
		if finding.Rule == rule {
			count++
		}
	}
	return count
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
	report.addFinding(Finding{Rule: "a", Path: "z", Message: "path-z"})
	report.addFinding(Finding{Rule: "a", Path: "a", Message: "path-a"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "b", Name: "n2", Message: "m2"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "a", Name: "n1", Message: "m1"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "a", Name: "n1", Message: "m0"})
	if report.Summary["total"] != 5 {
		t.Fatalf("pre-summary=%v", report.Summary)
	}
	// Defensive path: nil summary becomes empty with total=0 (counts live on findings).
	report.Summary = nil
	report.finalize()
	if report.Summary["total"] != 0 {
		t.Fatalf("summary=%v", report.Summary)
	}
	if report.Findings[0].Path != "a" || report.Findings[1].Prefix != "a" || report.Findings[1].Message != "m0" {
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
		Owner:   "o",
		Source:  "s",
		GoRoots: []string{"internal"},
	}
	if err := validatePolicy(base); err != nil {
		t.Fatal(err)
	}
	bad := base
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
	bad = base
	bad.PackageChildren = map[string][]string{"tools": {}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("empty package children")
	}
	bad = base
	bad.PackageChildren = map[string][]string{"../tools": {"release"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("invalid package root")
	}
	bad = base
	bad.PackageChildren = map[string][]string{"tools": {"release", "release"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("duplicate package child")
	}
	bad = base
	bad.PackageChildren = map[string][]string{"tools": {"release/legacy"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("nested package child")
	}
	bad = base
	bad.AllowedImportEdges = map[string][]string{"tools/release": {"internal/upgrade", "internal/upgrade"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("duplicate import edge")
	}
	bad = base
	bad.AllowedImportEdges = map[string][]string{"../tools/release": {}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("invalid import edge source")
	}
	bad = base
	bad.AllowedImportEdges = map[string][]string{"tools/release": {"../internal/upgrade"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("invalid import edge target")
	}
}

func TestPackageChildrenEnforcePositiveTopology(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tools", "release", "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "tools", "legacy", "main.go"), "package main\n")
	report := newReport("policy", root)
	p := policy{PackageChildren: map[string][]string{"tools": {"release", "coverage"}}}
	if err := checkPackageChildren(root, p, &report); err != nil {
		t.Fatal(err)
	}
	if got := report.Summary["package_child"]; got != 1 {
		t.Fatalf("package child findings = %d, want unexpected child only: %+v", got, report.Findings)
	}
	if got := report.Findings[0].Path; got != "tools/legacy" {
		t.Fatalf("package child path = %q, want tools/legacy", got)
	}

	report = newReport("policy", root)
	if err := checkPackageChildren(filepath.Join(root, "missing"), p, &report); err != nil {
		t.Fatalf("absent managed roots must be inert: %v", err)
	}
	if got := report.Summary["package_child"]; got != 0 {
		t.Fatalf("absent managed roots produced findings: %+v", report.Findings)
	}
}

func TestPackageChildrenIgnoreNonDirectoriesAndRejectInvalidRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tools"), "not a directory\n")
	report := newReport("policy", root)
	p := policy{PackageChildren: map[string][]string{"tools": {"release"}}}
	if err := checkPackageChildren(root, p, &report); err != nil {
		t.Fatalf("non-directory managed root must be inert: %v", err)
	}
	p.PackageChildren = map[string][]string{"invalid\x00root": {"release"}}
	if err := checkPackageChildren(root, p, &report); err == nil {
		t.Fatal("invalid managed root was accepted")
	}
}

func TestPackageChildrenReportsUnreadableManagedRoot(t *testing.T) {
	root := t.TempDir()
	managed := filepath.Join(root, "tools")
	if err := os.Mkdir(managed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(managed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(managed, 0o700) })
	report := newReport("policy", root)
	p := policy{PackageChildren: map[string][]string{"tools": {"release"}}}
	if err := checkPackageChildren(root, p, &report); err == nil {
		t.Fatal("unreadable managed root was accepted")
	}
}

func TestPackageChildrenIgnoreHiddenDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tools", ".cache", "main.go"), "package main\n")
	report := newReport("policy", root)
	p := policy{PackageChildren: map[string][]string{"tools": {"release"}}}
	if err := checkPackageChildren(root, p, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("hidden directory produced findings: %+v", report.Findings)
	}
}

func TestImportEdgesSkipTestsMalformedImportsAndAllowedDependencies(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tools", "release", "test.go"), "package main\n")
	writeFile(t, filepath.Join(root, "tools", "release", "broken.go"), "package main\nimport (\n")
	writeFile(t, filepath.Join(root, "tools", "release", "allowed.go"), "package main\nimport (\n _ \"aigw-cli/tools/release\"\n _ \"aigw-cli/tools/repository\"\n)\n")
	files := []goFileInfo{
		{relPath: "tools/release/test.go", dir: "tools/release", isTest: true},
		{relPath: "tools/release/broken.go", dir: "tools/release"},
		{relPath: "tools/release/allowed.go", dir: "tools/release"},
	}
	report := newReport("policy", root)
	p := policy{AllowedImportEdges: map[string][]string{"tools/release": {"tools/repository"}}}
	if err := checkImportEdges(root, files, p, &report); err != nil {
		t.Fatal(err)
	}
	if got := report.Summary["import_edge"]; got != 0 {
		t.Fatalf("allowed or inert imports produced findings: %+v", report.Findings)
	}
}

func TestImportEdgesReportUnavailableManagedSource(t *testing.T) {
	root := t.TempDir()
	files := []goFileInfo{{relPath: "tools/release/missing.go", dir: "tools/release"}}
	report := newReport("policy", root)
	p := policy{AllowedImportEdges: map[string][]string{"tools/release": {}}}
	if err := checkImportEdges(root, files, p, &report); err == nil {
		t.Fatal("missing managed source was accepted")
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

func TestPeerPackageImportsSkipMalformedSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "cli", "account", "account.go"), "package account\nimport (\n")
	files := []goFileInfo{{relPath: "internal/cli/account/account.go", dir: "internal/cli/account"}}
	report := newReport("policy", root)
	p := policy{PeerPackageRoots: map[string][]string{"internal/cli": {}}}
	if err := checkPeerPackageImports(root, files, p, &report); err != nil {
		t.Fatal(err)
	}
}

func TestImportEdgesRejectToolToProductRuntimeDependency(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tools", "release", "main.go")
	writeFile(t, path, "package main\n\nimport _ \"aigw-cli/internal/upgrade\"\n")
	files := []goFileInfo{{relPath: "tools/release/main.go", dir: "tools/release"}}
	report := newReport("policy", root)
	policy := policy{AllowedImportEdges: map[string][]string{"tools/release": {}}}
	if err := checkImportEdges(root, files, policy, &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary["import_edge"] != 1 {
		t.Fatalf("tool-to-runtime import not rejected: %+v", report.Findings)
	}

	writeFile(t, path, "package main\n\nimport _ \"github.com/example/library\"\n")
	report = newReport("policy", root)
	if err := checkImportEdges(root, files, policy, &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary["import_edge"] != 0 {
		t.Fatalf("third-party import rejected: %+v", report.Findings)
	}
}

func TestImportEdgesRequireEveryProductionPackageOwner(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "managed", "managed.go"), "package managed\n")
	writeFile(t, filepath.Join(root, "internal", "unmanaged", "unmanaged.go"), "package unmanaged\n")
	p := policy{
		GoRoots:             []string{"internal"},
		AllowedImportEdges:  map[string][]string{"internal/managed": {}},
		RequireImportOwners: true,
	}
	report, err := analyzeRepository(root, p, "policy.toml")
	if err != nil {
		t.Fatal(err)
	}
	if countRule(report, "unmanaged_import_owner") != 1 {
		t.Fatalf("findings = %+v", report.Findings)
	}
}
