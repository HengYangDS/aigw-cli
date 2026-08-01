package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type recordingRunner struct {
	profile        string
	err            error
	name           string
	args           []string
	path           string
	listedPackages []string
	listErr        error
}

type rejectingWriter struct{}

func (rejectingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write rejected")
}

func (r *recordingRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "list" {
		packages := r.listedPackages
		if len(packages) == 0 {
			packages = profilePackages(r.profile)
		}
		if len(packages) == 0 {
			packages = []string{"example/a"}
		}
		for _, packageName := range packages {
			if _, err := fmt.Fprintln(stdout, packageName); err != nil {
				return err
			}
		}
		return r.listErr
	}
	r.name = name
	r.args = append([]string(nil), args...)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-coverprofile=") {
			r.path = strings.TrimPrefix(arg, "-coverprofile=")
			if r.profile != "" {
				if err := os.WriteFile(r.path, []byte(r.profile), 0o600); err != nil {
					return err
				}
			}
		}
	}
	return r.err
}

func profilePackages(profile string) []string {
	seen := map[string]bool{}
	var packages []string
	for _, line := range strings.Split(profile, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		separator := strings.LastIndex(fields[0], ":")
		if separator <= 0 {
			continue
		}
		packageName := path.Dir(fields[0][:separator])
		if !seen[packageName] {
			seen[packageName] = true
			packages = append(packages, packageName)
		}
	}
	return packages
}

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validPolicy = `minimum_statement_percent = 95.0
comparison = "strictly-greater-than"
covermode = "atomic"
packages = ["./..."]
owner = "product-toolchain"
source = "go test coverage profile"
`

func TestRealMainPassesOnlyStrictlyAbovePolicy(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 96 1\nexample/a.go:3.1,4.1 4 0\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := realMain([]string{"--policy", policyPath, "--race"}, &stdout, &stderr, runner); code != 0 {
		t.Fatalf("realMain code = %d, stderr = %q", code, stderr.String())
	}
	if runner.name != "go" {
		t.Fatalf("command = %q, want go", runner.name)
	}
	if !contains(runner.args, "./...") {
		t.Fatalf("test arguments %q do not include the complete package set", runner.args)
	}
	wantArgs := []string{"test", "-count=1", "-race", "-covermode=atomic"}
	for _, want := range wantArgs {
		if !contains(runner.args, want) {
			t.Errorf("arguments %q lack %q", runner.args, want)
		}
	}
	if !contains(runner.args, "-coverpkg=./...") {
		t.Errorf("arguments %q lack cross-package instrumentation", runner.args)
	}
	if got := runner.args[len(runner.args)-1]; got != "./..." {
		t.Fatalf("last argument = %q, want ./...", got)
	}
	if !strings.Contains(stdout.String(), "96.00%") || !strings.Contains(stdout.String(), "required > 95.00%") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(runner.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary profile remains: %v", err)
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

func TestRealMainRejectsResultOutputFailure(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 96 1\nexample/a.go:3.1,4.1 4 0\n"}
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, rejectingWriter{}, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "write coverage result") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsExactFloor(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 95 1\nexample/a.go:3.1,4.1 5 0\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "95.00% does not exceed 95.00%") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsPackageBelowFloorDespitePassingAggregate(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/low/a.go:1.1,2.1 94 1\nexample/low/a.go:3.1,4.1 6 0\nexample/high/b.go:1.1,2.1 400 1\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "package example/low coverage 94.00% does not exceed 95.00%") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsListedPackageMissingFromProfile(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{
		profile:        "mode: atomic\nexample/covered/a.go:1.1,2.1 100 1\n",
		listedPackages: []string{"example/covered", "example/missing"},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "package example/missing is absent from the coverage profile") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsPackageEnumerationFailure(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{listErr: errors.New("list failed")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "go list failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsTestFailure(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{err: errors.New("test failure")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "go test failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestSystemRunnerExecutesCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := (systemRunner{}).Run("go", []string{"version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout.String(), "go version ") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRealMainRejectsPositionalArgumentAndInvalidProfile(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	tests := []struct {
		name    string
		args    []string
		profile string
		want    string
		code    int
	}{
		{name: "positional argument", args: []string{"--policy", policyPath, "./internal/..."}, want: "does not accept positional", code: 2},
		{name: "empty profile", args: []string{"--policy", policyPath}, want: "read coverage profile", code: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := realMain(test.args, &stdout, &stderr, &recordingRunner{profile: test.profile}); code != test.code {
				t.Fatalf("realMain code = %d, want %d", code, test.code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestRealMainRejectsUnavailableTemporaryDirectory(t *testing.T) {
	// TMPDIR is not honored by os.CreateTemp on Windows (it uses
	// TMP/TEMP/USERPROFILE instead), so the temporary-directory failure is
	// forced directly through createCoverageProfile instead of relying on
	// a platform-specific environment variable.
	missing := filepath.Join(t.TempDir(), "missing")
	restore := stubCoverageProfile(func() (*os.File, error) {
		return os.CreateTemp(missing, "aigw-coverage-*.out")
	})
	defer restore()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, &recordingRunner{}); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "create coverage profile") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsCoverageProfileCloseFailure(t *testing.T) {
	restore := stubCoverageProfile(func() (*os.File, error) {
		file, err := os.CreateTemp(t.TempDir(), "aigw-coverage-*.out")
		if err != nil {
			return nil, err
		}
		// Close it early so the Close call inside realMain observes the
		// already-closed error deterministically on every platform.
		if err := file.Close(); err != nil {
			return nil, err
		}
		return file, nil
	})
	defer restore()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, &recordingRunner{}); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "close coverage profile") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func stubCoverageProfile(factory func() (*os.File, error)) func() {
	original := createCoverageProfile
	createCoverageProfile = factory
	return func() { createCoverageProfile = original }
}

type emptyListRunner struct{}

func (emptyListRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	return nil
}

func TestRealMainRejectsEmptyPackageList(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, emptyListRunner{}); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "go list returned no packages") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainDeduplicatesRepeatedListedPackages(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{
		profile:        "mode: atomic\nexample/dup/a.go:1.1,2.1 100 1\n",
		listedPackages: []string{"example/dup", "example/dup"},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 0 {
		t.Fatalf("realMain code = %d, stderr = %q", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "is absent from the coverage profile") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsInvalidArgumentsAndPolicy(t *testing.T) {
	tests := []struct {
		name string
		args []string
		body string
		want string
		code int
	}{
		{name: "unknown flag", args: []string{"--unknown"}, want: "flag provided but not defined", code: 2},
		{name: "missing policy", args: []string{"--policy", filepath.Join(t.TempDir(), "missing.toml")}, want: "load coverage policy", code: 1},
		{name: "unknown field", body: validPolicy + "exclude = [\"tools\"]\n", want: "load coverage policy", code: 1},
		{name: "wrong comparison", body: strings.Replace(validPolicy, "strictly-greater-than", "greater-than-or-equal", 1), want: "comparison", code: 1},
		{name: "invalid floor", body: strings.Replace(validPolicy, "95.0", "101.0", 1), want: "minimum_statement_percent", code: 1},
		{name: "wrong mode", body: strings.Replace(validPolicy, "atomic", "set", 1), want: "covermode", code: 1},
		{name: "no packages", body: strings.Replace(validPolicy, "[\"./...\"]", "[]", 1), want: "packages", code: 1},
		{name: "missing owner", body: strings.Replace(validPolicy, "product-toolchain", "", 1), want: "owner and source", code: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := test.args
			if test.body != "" {
				args = []string{"--policy", writePolicy(t, test.body)}
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := realMain(args, &stdout, &stderr, &recordingRunner{}); code != test.code {
				t.Fatalf("realMain code = %d, want %d", code, test.code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
		})
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

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
