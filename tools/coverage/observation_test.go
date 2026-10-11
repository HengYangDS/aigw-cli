package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestRealMainJSONPreservesTheSingleNativeAttempt(t *testing.T) {
	events := "{\"Action\":\"start\",\"Package\":\"example\"}\n" +
		"{\"Action\":\"run\",\"Package\":\"example\",\"Test\":\"TestValue\"}\n" +
		"{\"Action\":\"pass\",\"Package\":\"example\",\"Test\":\"TestValue\"}\n"
	for _, test := range []struct {
		name, profile, diagnostic string
		failure                   error
		code                      int
	}{
		{"success", "mode: atomic\nexample/a.go:1.1,2.1 100 1\n", "statement coverage: 100.00%", nil, 0},
		{"native failure", "mode: atomic\nexample/a.go:1.1,2.1 100 1\n", "native test refused", errors.New("native test refused"), 1},
		{"coverage failure", "mode: atomic\nexample/a.go:1.1,2.1 95 1\nexample/a.go:3.1,4.1 5 0\n", "does not exceed 95.00%", nil, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &recordingRunner{profile: test.profile, output: events, err: test.failure}
			retained := filepath.Join(t.TempDir(), "coverage.out")
			var stdout, stderr bytes.Buffer
			arguments := []string{"--policy", writePolicy(t, validPolicy), "--json", "--race", "--tags=native_resource_acceptance", "--profile-output", retained}
			if code := realMain(arguments, &stdout, &stderr, runner); code != test.code {
				t.Fatalf("JSON attempt code=%d, want %d: %s", code, test.code, &stderr)
			}
			if stdout.String() != events {
				t.Fatalf("native JSON bytes changed: %q", &stdout)
			}
			if !strings.Contains(stderr.String(), test.diagnostic) {
				t.Fatalf("coverage diagnostic missing from stderr: %s", &stderr)
			}
			if len(runner.calls) != 2 || runner.calls[1][0] != "test" {
				t.Fatalf("want one inventory and one native test attempt: %q", runner.calls)
			}
			for _, argument := range []string{"-json", "-count=1", "-race", "-tags=native_resource_acceptance", "-covermode=atomic", "-coverpkg=example", "./..."} {
				if !slices.Contains(runner.args, argument) {
					t.Fatalf("native attempt lost selected argument %s: %q", argument, runner.args)
				}
			}
			profile, err := os.ReadFile(retained)
			if err != nil || string(profile) != test.profile {
				t.Fatalf("same-attempt counters were not retained: %q, %v", profile, err)
			}
			if _, err := os.Stat(runner.path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temporary native profile remains: %v", err)
			}
		})
	}
}

func TestReadCoverageMergesRepeatedCrossPackageRanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverage.out")
	body := "mode: atomic\nexample/a.go:1.1,2.1 96 0\nexample/a.go:3.1,4.1 4 0\nexample/a.go:1.1,2.1 96 1\nexample/a.go:3.1,4.1 4 0\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := readCoverage(path, "atomic")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 100 || result.Covered != 96 {
		t.Fatalf("coverage = %d/%d, want 96/100", result.Covered, result.Total)
	}
}

func TestRealMainForwardsBuildTagsToTheWholeGoScope(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "declarations.go"), []byte("package declarations\ntype Value string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{
		profile:        "mode: atomic\nexample/a/a.go:1.1,2.1 100 1\n",
		listedPackages: []string{"example/a", "example/declarations"},
		metadata:       fmt.Sprintf(`{"Dir":%q,"ImportPath":"example/declarations","GoFiles":["declarations.go"]}`, directory),
	}
	var stdout, stderr bytes.Buffer
	arguments := []string{"--policy", writePolicy(t, validPolicy), "--tags=native_resource_acceptance"}
	if code := realMain(arguments, &stdout, &stderr, runner); code != 0 {
		t.Fatalf("tagged coverage code=%d stderr=%s", code, &stderr)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("Go scope calls=%q, want inventory, tests and declaration inspection", runner.calls)
	}
	for _, arguments := range runner.calls {
		if !slices.Contains(arguments, "-tags=native_resource_acceptance") {
			t.Fatalf("Go scope lost selected build tags: %q", arguments)
		}
	}
	if !strings.Contains(stdout.String(), "statement coverage: 100.00%") || !strings.Contains(stdout.String(), "declarations only") {
		t.Fatalf("tagged coverage lost aggregate or declaration evidence: %s", &stdout)
	}
}

func TestReadCoverageRejectsConflictingRepeatedRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverage.out")
	body := "mode: atomic\nexample/a.go:1.1,2.1 1 0\nexample/a.go:1.1,2.1 2 1\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCoverage(path, "atomic"); err == nil || !strings.Contains(err.Error(), "conflicts with repeated source range") {
		t.Fatalf("error = %v", err)
	}
}

func TestPackageObservationPreservesEveryMissingPackageAndSourceFailure(t *testing.T) {
	failure := errors.New("package metadata unavailable")
	runner := &recordingRunner{metadataErr: failure}
	expected := []string{"example/first", "example/second"}
	err := (coverageResult{}).requireCoverage(expected, 95, runner, &bytes.Buffer{})
	if !errors.Is(err, failure) {
		t.Fatalf("source inspection cause was lost: %v", err)
	}
	for _, pkg := range expected {
		if !strings.Contains(err.Error(), "package "+pkg+" is absent from the coverage profile") {
			t.Fatalf("missing package %s was not reported: %v", pkg, err)
		}
	}
}

func TestPackageObservationRejectsLostDeclarationEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data.go"), []byte("package data\ntype Value string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{metadata: fmt.Sprintf(`{"ImportPath":"example/data","Dir":%q,"GoFiles":["data.go"]}`, root)}
	err := (coverageResult{}).requireCoverage([]string{"example/data"}, 95, runner, rejectingWriter{})
	if err == nil || !strings.Contains(err.Error(), "write package observation") {
		t.Fatalf("lost declaration-only observation was accepted: %v", err)
	}
}

func TestCoverageObservesDeclarationOnlyPackagesWithNativeGo(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":              "module coverage-fixture\nrequire tool.fixture v0.0.0\nreplace tool.fixture => ./.toolchain\n",
		"live/value.go":       "package live\nimport \"tool.fixture\"\nfunc Value() int { return tool.Value() }\n",
		"live/value_test.go":  "package live\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"value\") } }\n",
		"data/value.go":       "package data\ntype Value struct { Name string }\nconst Default = 1\n",
		"data/unselected.go":  "//go:build coverage_unselected\n\npackage data\nfunc Unselected() int { return 2 }\n",
		"empty/value.go":      "package empty\nfunc Value() {}\n",
		".toolchain/go.mod":   "module tool.fixture\n",
		".toolchain/value.go": "package tool\nfunc Value() int { return 1 }\nfunc Unused() int { return 0 }\n",
	}
	for name, content := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	policy := writePolicy(t, validPolicy)
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	if code := realMain([]string{"--policy", policy, "--json"}, &stdout, &stderr, systemRunner{}); code != 0 {
		t.Fatalf("native declaration-only observation: code=%d\n%s\n%s", code, &stdout, &stderr)
	}
	decoder, passed := json.NewDecoder(&stdout), false
	for decoder.More() {
		var event struct {
			Action string `json:"Action"`
			Test   string `json:"Test"`
		}
		if err := decoder.Decode(&event); err != nil {
			t.Fatalf("native event stream contains non-JSON output: %v", err)
		}
		passed = passed || event.Action == "pass" && event.Test == "TestValue"
	}
	if !passed {
		t.Fatal("native event stream lost the executed test")
	}
	for _, report := range []string{
		"package coverage-fixture/data statement coverage: not applicable (declarations only)",
		"package coverage-fixture/live statement coverage: 100.00% (1/1 statements)",
		"package coverage-fixture/empty statement coverage: not applicable (0 measured statements)",
	} {
		if !strings.Contains(stderr.String(), report) {
			t.Fatalf("native package coverage disappeared from stderr: %s", &stderr)
		}
	}
	if strings.Contains(stderr.String(), "package tool.fixture statement coverage:") {
		t.Fatalf("dependency counters entered product evidence: %s", &stderr)
	}
	if output, err := exec.Command("go", "test", "-cover", "./data").CombinedOutput(); err != nil || !bytes.Contains(output, []byte("[no test files]")) {
		t.Fatalf("native denominator premise: %v\n%s", err, output)
	}
}

func TestDeclarationOnlyObservationUsesSelectedSource(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name, source string
		cgo, want    bool
	}{
		{"types and constants", "package fixture\ntype Value struct { Name string }; const Limit = 2", false, true},
		{"external function declaration", "package fixture\nfunc External()", false, true},
		{"function body", "package fixture\nfunc Value() int { return 1 }", false, false},
		{"empty body", "package fixture\nfunc Value() {}", false, false},
		{"function literal", "package fixture\nvar Value = func() int { return 1 }", false, false},
		{"cgo function body", "package fixture\nfunc Value() int { return 1 }", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := filepath.Join(root, "source.go")
			if err := os.WriteFile(file, []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			field := "GoFiles"
			if test.cgo {
				field = "CgoFiles"
			}
			metadata, err := json.Marshal(map[string]any{"ImportPath": "fixture", "Dir": root, field: []string{"source.go"}})
			if err != nil {
				t.Fatal(err)
			}
			got, err := declarationOnlyPackage("fixture", &recordingRunner{metadata: string(metadata)})
			if err != nil || got != test.want {
				t.Fatalf("declaration-only=%t, want %t: %v", got, test.want, err)
			}
		})
	}
}

func TestDeclarationOnlyObservationRequiresReadableMatchingEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invalid.go"), []byte("package fixture\nfunc ("), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata := func(name, file string) string {
		return fmt.Sprintf(`{"ImportPath":%q,"Dir":%q,"GoFiles":[%q]}`, name, root, file)
	}
	for _, runner := range []*recordingRunner{
		{metadataErr: errors.New("list failed")},
		{metadata: "invalid"},
		{metadata: `{}`},
		{metadata: metadata("other", "invalid.go")},
		{metadata: metadata("fixture", "invalid.go")},
		{metadata: metadata("fixture", "missing.go")},
	} {
		if got, err := declarationOnlyPackage("fixture", runner); err == nil || got {
			t.Fatalf("invalid observation accepted: %t %v", got, err)
		}
	}
}

func TestReadCoverageRejectsMalformedProfiles(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty"},
		{name: "wrong mode", body: "mode: set\nexample/a.go:1.1,2.1 1 1\n"},
		{name: "malformed row", body: "mode: atomic\nmalformed\n"},
		{name: "missing source range", body: "mode: atomic\nexample/a.go 1 1\n"},
		{name: "invalid statements", body: "mode: atomic\nexample/a.go:1.1,2.1 nope 1\n"},
		{name: "invalid count", body: "mode: atomic\nexample/a.go:1.1,2.1 1 nope\n"},
		{name: "negative statements", body: "mode: atomic\nexample/a.go:1.1,2.1 -1 1\n"},
		{name: "negative count", body: "mode: atomic\nexample/a.go:1.1,2.1 1 -1\n"},
		{name: "zero statements", body: "mode: atomic\nexample/a.go:1.1,2.1 0 1\n"},
		{name: "overflow", body: "mode: atomic\nexample/a.go:1.1,2.1 " + strconv.FormatInt(math.MaxInt64, 10) + " 1\nexample/a.go:3.1,4.1 1 1\n"},
		{name: "scanner error", body: "mode: atomic\n" + strings.Repeat("x", 70_000)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "coverage.out")
			if err := os.WriteFile(path, []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := readCoverage(path, "atomic"); err == nil {
				t.Fatal("readCoverage accepted malformed profile")
			}
		})
	}
}

func TestReadCoverageRejectsMissingFile(t *testing.T) {
	if _, err := readCoverage(filepath.Join(t.TempDir(), "missing.out"), "atomic"); err == nil {
		t.Fatal("readCoverage accepted a missing file")
	}
}

func TestParsePackageList(t *testing.T) {
	packages, err := parsePackageList("example/internal/a\texample\nexample\texample\nexample/internal/a\texample\n")
	if err != nil || !slices.Equal(packages, []string{"example", "example/internal/a"}) {
		t.Fatalf("packages=%+v err=%v", packages, err)
	}
	for _, body := range []string{"broken\n", "foreign/pkg\texample\n"} {
		if _, err := parsePackageList(body); err == nil {
			t.Fatalf("invalid package list accepted: %q", body)
		}
	}
}

func TestRetainCoverageProfile(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.out")
	if err := os.WriteFile(source, []byte("mode: atomic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := retainCoverageProfile(source, ""); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "nested", "coverage.out")
	if err := retainCoverageProfile(source, target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "mode: atomic\n" {
		t.Fatalf("retained profile = %q, %v", data, err)
	}
	if err := retainCoverageProfile(filepath.Join(t.TempDir(), "missing"), target); err == nil {
		t.Fatal("missing source was accepted")
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := retainCoverageProfile(source, filepath.Join(blocked, "coverage.out")); err == nil {
		t.Fatal("invalid target parent was accepted")
	}
	runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 100 1\n", err: errors.New("native test refused")}
	var stdout, stderr bytes.Buffer
	if code := realMain([]string{"--policy", writePolicy(t, validPolicy), "--profile-output", filepath.Join(blocked, "coverage.out")}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("combined test/retention failure code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "native test refused") || !strings.Contains(stderr.String(), blocked) || stdout.Len() != 0 {
		t.Fatalf("combined native failures lost evidence: stdout=%q stderr=%q", &stdout, &stderr)
	}
	if _, err := os.Stat(runner.path); !os.IsNotExist(err) {
		t.Fatalf("combined native failures left their temporary profile: %v", err)
	}
}
