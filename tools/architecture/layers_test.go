package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// layerFixture writes a module with one production file per package, which
// imports the given module packages, and returns the module root and its files.
func layerFixture(t *testing.T, imports map[string][]string) (string, []goFileInfo) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	files := make([]goFileInfo, 0, len(imports))
	for _, dir := range slices.Sorted(maps.Keys(imports)) {
		var body strings.Builder
		body.WriteString("package " + filepath.Base(dir) + "\n")
		for _, target := range imports[dir] {
			fmt.Fprintf(&body, "\nimport _ %q\n", "example.com/aigw/"+target)
		}
		relative := dir + "/source.go"
		writeFile(t, filepath.Join(root, filepath.FromSlash(relative)), body.String())
		files = append(files, goFileInfo{relPath: relative, name: "source.go", dir: dir})
	}
	return root, files
}

func layerPolicy() policy {
	return policy{
		Layers: [][]string{
			{"tools/ci"}, {"tools"}, {"cmd/aigw"}, {"internal/cli"}, {"internal/app"}, {"internal/app/renaming"},
			{"internal/client"}, {"internal/secret", "internal/store"}, {"internal/domain"},
		},
		LayerSeams: map[string][]string{
			"internal/cli":    {"internal/cli/invocation"},
			"internal/client": {"internal/client"},
		},
		LayerFloors: map[string]string{"tools": "internal/client"},
	}
}

func layerReport(t *testing.T, p policy, imports map[string][]string) Report {
	t.Helper()
	root, files := layerFixture(t, imports)
	report := newReport("policy", root)
	if err := checkLayers(root, files, p, &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func refusedImports(report Report) []string {
	var refused []string
	for _, finding := range report.Findings {
		if finding.Rule == "layer_import" {
			refused = append(refused, strings.TrimSuffix(finding.Path, "/source.go")+" -> "+finding.Package)
		}
	}
	slices.Sort(refused)
	return refused
}

func TestLayersAdmitLowerRowsDescendantsAndSeams(t *testing.T) {
	report := layerReport(t, layerPolicy(), map[string][]string{
		"cmd/aigw":                   {"internal/cli", "internal/domain"},
		"internal/cli":               {"internal/cli/account", "internal/app"},
		"internal/cli/account":       {"internal/cli/invocation", "internal/app/renaming"},
		"internal/cli/invocation":    {"internal/domain"},
		"internal/app":               {"internal/app/renaming"},
		"internal/app/renaming":      {"internal/store"},
		"internal/client":            {"internal/domain"},
		"internal/client/claude":     {"internal/client", "internal/secret"},
		"internal/secret":            {"internal/domain"},
		"internal/store":             {"internal/domain"},
		"internal/domain":            nil,
		"tools/ci":                   {"tools/release/construction", "internal/client/claude"},
		"tools/release/construction": {"internal/store"},
	})
	if !report.OK {
		t.Fatalf("an admitted import was refused: %+v", report.Findings)
	}
}

func TestLayersRefuseTheirOwnRowAndHigherRows(t *testing.T) {
	report := layerReport(t, layerPolicy(), map[string][]string{
		"internal/secret":        {"internal/store"},
		"internal/client/claude": {"internal/client/codex"},
		"internal/client/codex":  nil,
		"internal/domain":        {"internal/app", "internal/unplaced"},
		"internal/app":           nil,
		"internal/store":         nil,
	})
	want := []string{
		"internal/client/claude -> internal/client/codex",
		"internal/domain -> internal/app",
		"internal/domain -> internal/unplaced",
		"internal/secret -> internal/store",
	}
	if got := refusedImports(report); !slices.Equal(got, want) {
		t.Fatalf("refused = %v, want %v", got, want)
	}
	for target, phrase := range map[string]string{
		"internal/app":      `"internal/domain" in row 9 of the layer table may not import "internal/app" in row 5`,
		"internal/unplaced": `may not import "internal/unplaced" which no row places`,
	} {
		for _, finding := range report.Findings {
			if finding.Rule == "layer_import" && finding.Package == target && !strings.Contains(finding.Message, phrase) {
				t.Fatalf("finding does not name both rows: %s", finding.Message)
			}
		}
	}
}

func TestLayerSeamsServeOnlyTheirOwnSubtree(t *testing.T) {
	report := layerReport(t, layerPolicy(), map[string][]string{
		"internal/app":    {"internal/cli/invocation"},
		"internal/store":  {"internal/client"},
		"internal/client": nil,
	})
	want := []string{"internal/app -> internal/cli/invocation", "internal/store -> internal/client"}
	if got := refusedImports(report); !slices.Equal(got, want) {
		t.Fatalf("refused = %v, want %v", got, want)
	}
}

func TestLayersCountOnlyImportsThroughASeamAsItsUse(t *testing.T) {
	p := policy{
		Layers:     [][]string{{"internal/client"}},
		LayerSeams: map[string][]string{"internal/client": {"internal/client/discovery"}},
	}
	report := layerReport(t, p, map[string][]string{
		"internal/client":           {"internal/client/discovery"},
		"internal/client/discovery": nil,
	})
	if got := countRule(report, "unused_layer_seam"); got != 1 {
		t.Fatalf("an import of a descendant counted as a use of the seam: %+v", report.Findings)
	}
}

func TestLayersPlaceAPackageInTheRowOfItsLongestPrefix(t *testing.T) {
	report := layerReport(t, layerPolicy(), map[string][]string{
		"internal/app/catalog":  {"internal/app/renaming"},
		"internal/app/renaming": {"internal/app/catalog"},
	})
	if got, want := refusedImports(report), []string{"internal/app/renaming -> internal/app/catalog"}; !slices.Equal(got, want) {
		t.Fatalf("refused = %v, want %v", got, want)
	}
}

func TestLayerFloorsKeepToolsOffTheCommandTree(t *testing.T) {
	report := layerReport(t, layerPolicy(), map[string][]string{
		"tools/release": {"internal/cli", "internal/client", "tools/ci"},
		"tools/ci":      {"tools/release"},
	})
	want := []string{"tools/release -> internal/cli", "tools/release -> tools/ci"}
	if got := refusedImports(report); !slices.Equal(got, want) {
		t.Fatalf("refused = %v, want %v", got, want)
	}
	for _, finding := range report.Findings {
		if finding.Package == "internal/cli" && !strings.Contains(finding.Message, `packages of "tools" import outside it only from the row of "internal/client" down`) {
			t.Fatalf("a floor's refusal does not name the floor: %s", finding.Message)
		}
	}
}

func TestLayersReportUnplacedPackagesAndUnusedEntries(t *testing.T) {
	p := policy{
		Layers:      [][]string{{"internal/domain"}, {"internal/retired"}},
		LayerSeams:  map[string][]string{"internal/domain": {"internal/domain/unused"}},
		LayerFloors: map[string]string{"internal/domain/typo": "internal/retired"},
	}
	root, files := layerFixture(t, map[string][]string{"internal/domain": nil, "internal/other": nil})
	writeFile(t, filepath.Join(root, "internal", "other", "second.go"), "package other\n")
	files = append(files, goFileInfo{relPath: "internal/other/second.go", name: "second.go", dir: "internal/other"})
	report := newReport("policy", root)
	if err := checkLayers(root, files, p, &report); err != nil {
		t.Fatal(err)
	}
	for rule, want := range map[string]int{"layer_unplaced": 1, "unused_layer_prefix": 1, "unused_layer_seam": 1, "unused_layer_floor": 1, "layer_import": 0} {
		if got := countRule(report, rule); got != want {
			t.Fatalf("%s findings = %d, want %d: %+v", rule, got, want, report.Findings)
		}
	}
}

func TestLayersSkipTestFilesAndOtherModules(t *testing.T) {
	root, files := layerFixture(t, map[string][]string{"internal/domain": nil, "internal/app": nil})
	writeFile(t, filepath.Join(root, "internal", "domain", "source_test.go"), "package domain\n\nimport _ \"example.com/aigw/internal/app\"\n")
	writeFile(t, filepath.Join(root, "internal", "domain", "source.go"), "package domain\n\nimport (\n\t_ \"fmt\"\n\t_ \"github.com/example/library\"\n)\n")
	files = append(files, goFileInfo{relPath: "internal/domain/source_test.go", name: "source_test.go", dir: "internal/domain", isTest: true})
	report := newReport("policy", root)
	if err := checkLayers(root, files, policy{Layers: [][]string{{"internal/app"}, {"internal/domain"}}}, &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("a test file or another module's import was checked: %+v", report.Findings)
	}
}

func TestLayersRequireModuleIdentityAndReadableSources(t *testing.T) {
	p := policy{Layers: [][]string{{"internal"}}}
	empty := t.TempDir()
	report := newReport("policy", empty)
	if err := checkLayers(empty, nil, p, &report); err == nil || !strings.Contains(err.Error(), "read go.mod") {
		t.Fatalf("module identity error = %v", err)
	}
	if err := checkLayers(t.TempDir(), nil, policy{}, &report); err != nil {
		t.Fatalf("a policy without layers read the module: %v", err)
	}
	root, files := layerFixture(t, map[string][]string{"internal/cli": nil})
	files[0].relPath = "internal/cli/missing.go"
	if err := checkLayers(root, files, p, &report); err == nil || !strings.Contains(err.Error(), "internal/cli/missing.go") {
		t.Fatalf("missing source error = %v", err)
	}
	writeFile(t, filepath.Join(root, "internal", "cli", "source.go"), "package cli\nimport (\n")
	files[0].relPath = "internal/cli/source.go"
	if err := checkLayers(root, files, p, &report); err == nil || !strings.Contains(err.Error(), "internal/cli/source.go") {
		t.Fatalf("unreadable import syntax error = %v", err)
	}
}

func TestValidateLayersRefusesEachInvalidTable(t *testing.T) {
	thirteenRows := make([][]string, maxLayerRows+1)
	for i := range thirteenRows {
		thirteenRows[i] = []string{"internal/row" + string(rune('a'+i))}
	}
	twelve := policy{
		Layers:      thirteenRows[:maxLayerRows],
		LayerSeams:  map[string][]string{"internal/rowa": {"internal/rowa/seam"}},
		LayerFloors: map[string]string{"internal/rowa": "internal/rowb"},
	}
	if err := validateLayers(twelve); err != nil {
		t.Fatalf("a table of %d rows with a seam and a floor was refused: %v", maxLayerRows, err)
	}
	for name, test := range map[string]struct {
		p    policy
		want string
	}{
		"too many rows":              {policy{Layers: thirteenRows}, "at most 12 rows"},
		"empty row":                  {policy{Layers: [][]string{{}}}, "rows must be non-empty"},
		"repeated prefix":            {policy{Layers: [][]string{{"internal"}, {"internal"}}}, "invalid or repeated prefix"},
		"invalid prefix":             {policy{Layers: [][]string{{"../internal"}}}, "invalid or repeated prefix"},
		"seams without layers":       {policy{LayerSeams: map[string][]string{"internal/cli": {"internal/cli/invocation"}}}, "which is empty"},
		"floors without layers":      {policy{LayerFloors: map[string]string{"tools": "internal"}}, "which is empty"},
		"unplaced seam subtree":      {policy{Layers: [][]string{{"internal/app"}}, LayerSeams: map[string][]string{"internal/cli": {"internal/cli/invocation"}}}, "has no row"},
		"seam outside its subtree":   {policy{Layers: [][]string{{"internal/cli"}, {"internal/app"}}, LayerSeams: map[string][]string{"internal/cli": {"internal/app"}}}, "its own subtree's packages"},
		"seam above its subtree":     {policy{Layers: [][]string{{"internal/client/x"}, {"internal/client"}}, LayerSeams: map[string][]string{"internal/client": {"internal/client/x"}}}, "at or below its row"},
		"invalid floor subtree":      {policy{Layers: [][]string{{"internal"}}, LayerFloors: map[string]string{"../tools": "internal"}}, "has no row"},
		"unplaced floor subtree":     {policy{Layers: [][]string{{"internal/client"}}, LayerFloors: map[string]string{"tools": "internal/client"}}, "has no row"},
		"floor outside the layers":   {policy{Layers: [][]string{{"internal"}, {"tools"}}, LayerFloors: map[string]string{"tools": "internal/client"}}, "must name a prefix"},
		"floor above its subtree":    {policy{Layers: [][]string{{"internal/cli"}, {"tools"}}, LayerFloors: map[string]string{"tools": "internal/cli"}}, "row below the subtree"},
		"floor on its subtree's row": {policy{Layers: [][]string{{"internal/client", "tools"}}, LayerFloors: map[string]string{"tools": "internal/client"}}, "row below the subtree"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateLayers(test.p); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
