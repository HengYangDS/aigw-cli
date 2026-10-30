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
	thirteenRows := make([][]string, maxLayerRows+1)
	for i := range thirteenRows {
		thirteenRows[i] = []string{"internal/row" + string(rune('a'+i))}
	}
	if err := validatePackagePolicy(policy{GoRoots: base.GoRoots, Layers: thirteenRows[:maxLayerRows], LayerSeams: map[string][]string{"internal/rowa": {"internal/rowa/seam"}}, LayerFloors: map[string]string{"internal/rowa": "internal/rowb"}}); err != nil {
		t.Fatalf("a table of %d rows with a seam and a floor was refused: %v", maxLayerRows, err)
	}
	for name, p := range map[string]policy{
		"parent traversal go root": {GoRoots: []string{"internal/../x"}},
		"empty package children":   {PackageChildren: map[string][]string{"tools": {}}},
		"invalid package root":     {PackageChildren: map[string][]string{"../tools": {"release"}}},
		"duplicate package child":  {PackageChildren: map[string][]string{"tools": {"release", "release"}}},
		"nested package child":     {PackageChildren: map[string][]string{"tools": {"release/legacy"}}},
		"too many layer rows":      {Layers: thirteenRows},
		"empty layer row":          {Layers: [][]string{{}}},
		"repeated layer prefix":    {Layers: [][]string{{"internal"}, {"internal"}}},
		"invalid layer prefix":     {Layers: [][]string{{"../internal"}}},
		"empty layer seams":        {LayerSeams: map[string][]string{"internal/cli": {}}},
		"duplicate layer seam":     {LayerSeams: map[string][]string{"internal/cli": {"internal/cli/invocation", "internal/cli/invocation"}}},
		"seam outside its subtree": {LayerSeams: map[string][]string{"internal/cli": {"internal/app"}}},
		"floor outside the layers": {Layers: [][]string{{"internal"}}, LayerFloors: map[string]string{"tools": "internal/client"}},
		"invalid floor subtree":    {Layers: [][]string{{"internal"}}, LayerFloors: map[string]string{"../tools": "internal"}},
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
				"layer_seams": func(p *policy) {
					p.LayerSeams = map[string][]string{"internal": {name}}
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

func TestPackageMembershipOrdersDiagnosticsByRoot(t *testing.T) {
	p := policy{
		GoRoots:              []string{"internal"},
		PackageChildren:      map[string][]string{"internal": {"client", "credential"}},
		CompositionRootFiles: map[string][]string{"internal/cli": {"app.go"}},
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
