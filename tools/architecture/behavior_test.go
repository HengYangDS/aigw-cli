package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecisionRecordReadFailureIsReported(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "decisions")
	writeFile(t, filepath.Join(directory, decisionRegister), "# Decisions\n")
	if err := os.Symlink(filepath.Join(root, "missing-record"), filepath.Join(directory, "dr-0001-missing.md")); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err == nil || !strings.Contains(err.Error(), "read docs/decisions/dr-0001-missing.md") {
		t.Fatalf("decision record read error = %v", err)
	}
}

func TestDecisionRecordDirectoryReadFailureIsReported(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "decisions")
	writeFile(t, filepath.Join(directory, decisionRegister), "# Decisions\n")
	want := errors.New("directory read failed")
	report := newReport("policy", root)
	err := checkDecisionRecordsWithReadDir(root, &report, func(string) ([]os.DirEntry, error) {
		return nil, want
	})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "read Decision Records") {
		t.Fatalf("directory read error = %v", err)
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
		Owner:             "o",
		Source:            "s",
		RiskModel:         "risk",
		Measurement:       "measurement",
		FalsePositiveCost: "cost",
		Remediation:       "remediation",
		ReviewCondition:   "review",
		GoRoots:           []string{"internal"},
		TrackedCarrierOwners: map[string]map[string]string{
			".": {"internal": "product"},
		},
	}
	if err := validatePolicy(base); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad = base
	bad.TrackedCarrierOwners = nil
	if err := validatePolicy(bad); err == nil {
		t.Fatal("missing tracked carrier owners")
	}
	bad = base
	bad.TrackedCarrierOwners = map[string]map[string]string{"internal": {"file.go": "product"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("missing repository root owner map")
	}
	bad = base
	bad.TrackedCarrierOwners = map[string]map[string]string{
		".":           {"internal": "product"},
		"../internal": {"file.go": "product"},
	}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("non-portable tracked carrier parent")
	}
	bad = base
	bad.TrackedCarrierOwners = map[string]map[string]string{".": {}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("empty tracked carrier child map")
	}
	bad = base
	bad.TrackedCarrierOwners = map[string]map[string]string{".": {"bad/name": "product"}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("nested tracked carrier name")
	}
	bad = base
	bad.TrackedCarrierOwners = map[string]map[string]string{".": {"internal": ""}}
	if err := validatePolicy(bad); err == nil {
		t.Fatal("empty tracked carrier responsibility")
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

func TestTrackedCarrierOwnersFormAClosedPositiveTopology(t *testing.T) {
	files := []string{
		".config/checks/architecture/policy.toml",
		".config/ci/pipeline.cue",
		"records/local.json",
		"README.md",
		"cmd/aigw/main.go",
		"docs/README.md",
	}
	p := policy{
		IgnoreRoots: []string{"records"},
		TrackedCarrierOwners: map[string]map[string]string{
			".": {
				".config":   "repository policy",
				"README.md": "product entry point",
				"cmd":       "command composition",
				"docs":      "documentation",
			},
			".config": {
				"checks": "quality policy",
				"ci":     "continuous integration model",
			},
		},
	}
	report := newReport("policy", ".")
	checkTrackedCarrierOwners(files, p, &report)
	if got := countRule(report, "tracked_carrier_owner"); got != 0 {
		t.Fatalf("tracked carrier findings = %d, want none: %+v", got, report.Findings)
	}

	files = append(files, ".config/release/goreleaser.yaml", "orphan.toml")
	report = newReport("policy", ".")
	checkTrackedCarrierOwners(files, p, &report)
	if got := countRule(report, "tracked_carrier_owner"); got != 2 {
		t.Fatalf("tracked carrier findings = %d, want two undeclared children: %+v", got, report.Findings)
	}
	paths := map[string]bool{}
	for _, finding := range report.Findings {
		paths[finding.Path] = true
	}
	for _, want := range []string{".config/release", "orphan.toml"} {
		if !paths[want] {
			t.Fatalf("tracked carrier findings = %+v, missing %q", report.Findings, want)
		}
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
	if err := os.Mkdir(filepath.Join(root, "tools"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	p := policy{PackageChildren: map[string][]string{"tools": {"release"}}}
	readFailure := errors.New("deterministic read failure")
	if err := checkPackageChildrenWithReadDir(root, p, &report, func(string) ([]fs.DirEntry, error) {
		return nil, readFailure
	}); !errors.Is(err, readFailure) {
		t.Fatalf("directory read error = %v", err)
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
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	writeFile(t, filepath.Join(root, "tools", "release", "test.go"), "package main\n")
	writeFile(t, filepath.Join(root, "tools", "release", "broken.go"), "package main\nimport (\n")
	writeFile(t, filepath.Join(root, "tools", "release", "allowed.go"), "package main\nimport (\n _ \"example.com/aigw/tools/release\"\n _ \"example.com/aigw/tools/repository\"\n)\n")
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
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	files := []goFileInfo{{relPath: "tools/release/missing.go", dir: "tools/release"}}
	report := newReport("policy", root)
	p := policy{AllowedImportEdges: map[string][]string{"tools/release": {}}}
	if err := checkImportEdges(root, files, p, &report); err == nil {
		t.Fatal("missing managed source was accepted")
	}
}

func TestImportEdgesRequireModuleIdentity(t *testing.T) {
	root := t.TempDir()
	report := newReport("policy", root)
	p := policy{AllowedImportEdges: map[string][]string{"tools/release": {}}}
	if err := checkImportEdges(root, nil, p, &report); err == nil || !strings.Contains(err.Error(), "read go.mod") {
		t.Fatalf("module identity error = %v", err)
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
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	path := filepath.Join(root, "tools", "release", "main.go")
	writeFile(t, path, "package main\n\nimport _ \"example.com/aigw/internal/upgrade\"\n")
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
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
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
