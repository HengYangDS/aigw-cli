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
	err := (coverageResult{}).requirePackages(expected, runner, &bytes.Buffer{})
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
	err := (coverageResult{}).requirePackages([]string{"example/data"}, runner, rejectingWriter{})
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
	if code := realMain([]string{"--policy", policy}, &stdout, &stderr, systemRunner{}); code != 0 {
		t.Fatalf("native declaration-only observation: code=%d\n%s\n%s", code, &stdout, &stderr)
	}
	if !strings.Contains(stdout.String(), "package coverage-fixture/data statement coverage: not applicable (declarations only)") {
		t.Fatalf("declaration-only package disappeared: %s", &stdout)
	}
	if !strings.Contains(stdout.String(), "package coverage-fixture/live statement coverage: 100.00% (1/1 statements)") {
		t.Fatalf("executable package evidence changed: %s", &stdout)
	}
	if !strings.Contains(stdout.String(), "package coverage-fixture/empty statement coverage: not applicable (0 measured statements)") {
		t.Fatalf("native zero denominator was not reported: %s", &stdout)
	}
	if strings.Contains(stdout.String(), "package tool.fixture statement coverage:") {
		t.Fatalf("dependency counters entered product evidence: %s", &stdout)
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

func TestCoveragePercentHandlesEmptyTotal(t *testing.T) {
	if coveragePercent(1, 0) != 0 {
		t.Fatal("empty total must not produce coverage")
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
}
