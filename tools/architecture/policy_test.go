package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func countRule(report Report, rule string) int {
	count := 0
	for _, finding := range report.Findings {
		if finding.Rule == rule {
			count++
		}
	}
	return count
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
		TrackedCarrierClasses: map[string]carrierClass{
			"go": {
				Responsibility: "Go source",
				Prefixes:       []string{"internal"},
				Suffixes:       []string{".go"},
			},
		},
	}
	if err := validatePolicy(base); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.TrackedCarrierClasses = nil
	if err := validatePolicy(bad); err == nil {
		t.Fatal("missing tracked carrier classes")
	}
	for name, class := range map[string]carrierClass{
		"empty responsibility": {Prefixes: []string{"internal"}},
		"missing selector":     {Responsibility: "Go source"},
		"invalid path":         {Responsibility: "Go source", Prefixes: []string{"../internal"}},
		"invalid suffix":       {Responsibility: "Go source", Suffixes: []string{"go"}},
	} {
		t.Run(name, func(t *testing.T) {
			bad := base
			bad.TrackedCarrierClasses = map[string]carrierClass{"go": class}
			if err := validatePolicy(bad); err == nil {
				t.Fatal("invalid carrier responsibility accepted")
			}
		})
	}
	for name, p := range map[string]policy{
		"parent traversal go root":    {GoRoots: []string{"internal/../x"}},
		"empty peer package root":     {PeerPackageRoots: map[string][]string{"": {"invocation"}}},
		"nested peer package name":    {PeerPackageRoots: map[string][]string{"internal/cli": {"bad/name"}}},
		"duplicate peer package name": {PeerPackageRoots: map[string][]string{"internal/cli": {"invocation", "invocation"}}},
		"empty package children":      {PackageChildren: map[string][]string{"tools": {}}},
		"invalid package root":        {PackageChildren: map[string][]string{"../tools": {"release"}}},
		"duplicate package child":     {PackageChildren: map[string][]string{"tools": {"release", "release"}}},
		"nested package child":        {PackageChildren: map[string][]string{"tools": {"release/legacy"}}},
		"duplicate import edge":       {AllowedImportEdges: map[string][]string{"tools/release": {"internal/upgrade", "internal/upgrade"}}},
		"invalid import source":       {AllowedImportEdges: map[string][]string{"../tools/release": {}}},
		"invalid import target":       {AllowedImportEdges: map[string][]string{"tools/release": {"../internal/upgrade"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if p.GoRoots == nil {
				p.GoRoots = base.GoRoots
			}
			if err := validatePackagePolicy(p); err == nil {
				t.Fatal("invalid package policy accepted")
			}
		})
	}
}

func TestPackageMembershipUsesOnePortableNameContract(t *testing.T) {
	for _, name := range []string{".", "..", " cli", "cli ", "C:cli", `parent\cli`} {
		t.Run(name, func(t *testing.T) {
			for field, configure := range map[string]func(*policy){
				"package_children": func(p *policy) {
					p.PackageChildren = map[string][]string{"internal": {name}}
				},
				"peer_package_roots": func(p *policy) {
					p.PeerPackageRoots = map[string][]string{"internal": {name}}
				},
			} {
				p := policy{GoRoots: []string{"internal"}}
				configure(&p)
				if err := validatePackagePolicy(p); err == nil || !strings.Contains(err.Error(), field) {
					t.Fatalf("%s admitted nonportable package member %q: %v", field, name, err)
				}
			}
		})
	}
}

func TestCompositionMembershipUsesPortableGoBaseNames(t *testing.T) {
	for _, name := range []string{" app.go", `parent\app.go`, "C:app.go"} {
		p := policy{
			GoRoots:              []string{"internal"},
			CompositionRootFiles: map[string][]string{"internal": {name}},
		}
		if err := validatePackagePolicy(p); err == nil || !strings.Contains(err.Error(), "composition_root_files") {
			t.Fatalf("admitted nonportable composition file %q: %v", name, err)
		}
	}
}

func TestPackageMembershipPreservesEmptyAllowancesAndRootOrder(t *testing.T) {
	p := policy{
		GoRoots:              []string{"internal"},
		PackageChildren:      map[string][]string{"internal": {"client", "credential"}},
		CompositionRootFiles: map[string][]string{"internal/cli": {"app.go"}},
		PeerPackageRoots:     map[string][]string{"internal/client": {}},
		AllowedImportEdges:   map[string][]string{"internal/client": {}, "tools/ci": {"tools/ci/projection"}},
	}
	if err := validatePackagePolicy(p); err != nil {
		t.Fatal(err)
	}
	p.PackageChildren = map[string][]string{"z-last": {".."}, "a-first": {"."}}
	for range 20 {
		if err := validatePackagePolicy(p); err == nil || !strings.Contains(err.Error(), `"a-first"`) {
			t.Fatalf("membership diagnostics are not ordered by root: %v", err)
		}
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
}

func TestTrackedCarriersResolveToExactlyOneResponsibility(t *testing.T) {
	files := []string{
		"cmd/aigw/main.go",
		"docs/README.md",
		"openspec/changes/archive/2026-08-01-example/tasks.md",
	}
	p := policy{TrackedCarrierClasses: map[string]carrierClass{
		"go": {
			Responsibility: "Go source",
			Prefixes:       []string{"cmd", "internal", "tools"},
			Suffixes:       []string{".go"},
		},
		"documentation": {
			Responsibility: "current documentation",
			Prefixes:       []string{"docs"},
			Suffixes:       []string{".md"},
		},
		"openspec-archive": {
			Responsibility: "immutable OpenSpec history",
			Prefixes:       []string{"openspec/changes/archive"},
		},
	}}
	report := newReport("policy", ".")
	checkTrackedCarrierClasses(files, p, &report)
	if got := countRule(report, "tracked_carrier_responsibility"); got != 0 {
		t.Fatalf("tracked carrier findings = %d, want none: %+v", got, report.Findings)
	}

	files = append(files, "orphan.ini")
	report = newReport("policy", ".")
	checkTrackedCarrierClasses(files, p, &report)
	if got := countRule(report, "tracked_carrier_responsibility"); got != 1 {
		t.Fatalf("unclassified carrier findings = %d, want one: %+v", got, report.Findings)
	}

	p.TrackedCarrierClasses["go-entrypoint"] = carrierClass{
		Responsibility: "duplicate Go source owner",
		ExactPaths:     []string{"cmd/aigw/main.go"},
	}
	report = newReport("policy", ".")
	checkTrackedCarrierClasses(files[:3], p, &report)
	if got := countRule(report, "tracked_carrier_responsibility"); got != 1 {
		t.Fatalf("multiply classified carrier findings = %d, want one: %+v", got, report.Findings)
	}
}

func TestPackageChildrenRequireDeclaredRoots(t *testing.T) {
	for _, state := range []string{"missing", "file", "invalid"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			managedRoot := "tools"
			switch state {
			case "file":
				writeFile(t, filepath.Join(root, managedRoot), "not a directory\n")
			case "invalid":
				managedRoot = "invalid\x00root"
			}
			report := newReport("policy", root)
			p := policy{PackageChildren: map[string][]string{managedRoot: {"release"}}}
			if err := checkPackageChildren(root, p, &report); err == nil {
				t.Fatalf("%s managed root was accepted", state)
			}
		})
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

func TestImportEdgesAcceptAllowedDependenciesAndExcludeTestEdges(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	writeFile(t, filepath.Join(root, "tools", "release", "test.go"), "package main\n")
	writeFile(t, filepath.Join(root, "tools", "release", "allowed.go"), "package main\nimport (\n _ \"example.com/aigw/tools/release\"\n _ \"example.com/aigw/tools/repository\"\n)\n")
	files := []goFileInfo{
		{relPath: "tools/release/test.go", dir: "tools/release", isTest: true},
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

func TestImportAnalysisRequiresReadableSyntax(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	source := "internal/cli/account/account.go"
	writeFile(t, filepath.Join(root, filepath.FromSlash(source)), "package account\nimport (\n")
	files := []goFileInfo{{relPath: source, dir: "internal/cli/account"}}
	p := policy{
		AllowedImportEdges: map[string][]string{"internal/cli/account": {}},
		PeerPackageRoots:   map[string][]string{"internal/cli": {}},
	}
	for name, check := range map[string]func(string, []goFileInfo, policy, *Report) error{
		"import edges": checkImportEdges,
		"peer imports": checkPeerPackageImports,
	} {
		t.Run(name, func(t *testing.T) {
			report := newReport("policy", root)
			err := check(root, files, p, &report)
			if err == nil || !strings.Contains(err.Error(), source) {
				t.Fatalf("unreadable import syntax must fail with its source path: %v", err)
			}
		})
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
