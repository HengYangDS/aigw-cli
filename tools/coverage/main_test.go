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
	branchReport   string
	branchErr      error
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
			module := strings.Split(packageName, "/")[0]
			if _, err := fmt.Fprintf(stdout, "%s\t%s\n", packageName, module); err != nil {
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

func (r *recordingRunner) RunInput(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if r.branchErr != nil {
		return r.branchErr
	}
	if _, err := io.Copy(io.Discard, stdin); err != nil {
		return err
	}
	report := r.branchReport
	if report == "" {
		packages := r.listedPackages
		if len(packages) == 0 {
			packages = profilePackages(r.profile)
		}
		seen := map[string]bool{}
		var body strings.Builder
		body.WriteString(`<coverage version="1">`)
		for _, packageName := range packages {
			if seen[packageName] {
				continue
			}
			seen[packageName] = true
			relative := strings.TrimPrefix(packageName, strings.Split(packageName, "/")[0])
			relative = strings.TrimPrefix(relative, "/")
			if relative != "" {
				relative += "/"
			}
			fmt.Fprintf(&body, `<file path="%sa.go"><lineToCover lineNumber="1" covered="true" branchesToCover="100" coveredBranches="100"/></file>`, relative)
		}
		body.WriteString(`</coverage>`)
		report = body.String()
	}
	_, err := io.WriteString(stdout, report)
	return err
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
minimum_branch_percent = 95.0
comparison = "greater-than"
threshold_scopes = ["aggregate"]
package_observation = "required"
covermode = "atomic"
packages = ["./..."]
branch_analyzer = "go-bcov"
owner = "product-toolchain"
source = "Go statement profile with go-bcov branch analysis"
risk_model = "uncovered control-flow can corrupt credentials or projections"
measurement = "exact aggregate statement and branch counts plus package observation diagnostics"
false_positive_cost = "aggregate evidence can hide a local blind spot unless package execution and ratios remain visible"
remediation = "test behavior, remove unreachable code, or simplify the owner"
review_condition = "reassess after repeated denominator-only blocks"
`

func TestRealMainPassesAggregatePolicyAndReportsPackageEvidence(t *testing.T) {
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
		t.Errorf("arguments %q do not attribute full-suite execution to source packages", runner.args)
	}
	if got := runner.args[len(runner.args)-1]; got != "./..." {
		t.Fatalf("last argument = %q, want ./...", got)
	}
	if !strings.Contains(stdout.String(), "package example statement coverage: 96.00%") || !strings.Contains(stdout.String(), "statement coverage: 96.00%") || !strings.Contains(stdout.String(), "branch coverage: 100.00%") || !strings.Contains(stdout.String(), "required > 95.00%") {
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

func TestRealMainRejectsExactAggregateFloor(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 95 1\nexample/a.go:3.1,4.1 5 0\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "coverage 95.00% does not exceed 95.00%") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainReportsLowPackageRatioWhenAggregatePasses(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/low/a.go:1.1,2.1 94 1\nexample/low/a.go:3.1,4.1 6 0\nexample/high/b.go:1.1,2.1 400 1\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 0 {
		t.Fatalf("realMain code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "package example/low statement coverage: 94.00%") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunBranchCoverageReportsExactAndLowPackageRatiosWhenAggregatePasses(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(profile, []byte("mode: atomic\nexample/low/a.go:1.1,2.1 1 1\nexample/high/b.go:1.1,2.1 1 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := loadPolicy(writePolicy(t, validPolicy))
	if err != nil {
		t.Fatal(err)
	}
	packages := []packageInfo{
		{ImportPath: "example/low", ModulePath: "example"},
		{ImportPath: "example/high", ModulePath: "example"},
	}
	for _, test := range []struct {
		name    string
		covered int
	}{
		{name: "exact floor", covered: 95},
		{name: "below floor", covered: 94},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := fmt.Sprintf(`<coverage><file path="low/a.go"><lineToCover branchesToCover="100" coveredBranches="%d"/></file><file path="high/b.go"><lineToCover branchesToCover="400" coveredBranches="400"/></file></coverage>`, test.covered)
			var stdout, stderr bytes.Buffer
			err := runBranchCoverage(profile, packages, policy, &stdout, &stderr, &recordingRunner{branchReport: report})
			if err != nil || !strings.Contains(stdout.String(), "package example/low branch coverage") {
				t.Fatalf("error=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRealMainRejectsWhollyUnexecutedPackage(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/idle/a.go:1.1,2.1 4 0\nexample/live/b.go:1.1,2.1 100 1\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "package example/idle has no executed statements") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRealMainRejectsPackageWithoutMeasuredStatements(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/empty/a.go:1.1,2.1 0 0\nexample/live/b.go:1.1,2.1 100 1\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("realMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "package example/empty has no measured statements") {
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

type noInputRunner struct{}

func (runner *noInputRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
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
		{name: "wrong comparison", body: strings.Replace(validPolicy, "greater-than", "at-least", 1), want: "comparison", code: 1},
		{name: "wrong threshold scope", body: strings.Replace(validPolicy, `["aggregate"]`, `["package"]`, 1), want: "threshold_scopes", code: 1},
		{name: "missing package observation", body: strings.Replace(validPolicy, "required", "optional", 1), want: "package_observation", code: 1},
		{name: "invalid floor", body: strings.Replace(validPolicy, "95.0", "101.0", 1), want: "minimum_statement_percent", code: 1},
		{name: "invalid branch floor", body: strings.Replace(validPolicy, "minimum_branch_percent = 95.0", "minimum_branch_percent = 0", 1), want: "minimum_branch_percent", code: 1},
		{name: "wrong mode", body: strings.Replace(validPolicy, "atomic", "set", 1), want: "covermode", code: 1},
		{name: "no packages", body: strings.Replace(validPolicy, "[\"./...\"]", "[]", 1), want: "packages", code: 1},
		{name: "wrong branch analyzer", body: strings.Replace(validPolicy, "go-bcov", "gobco", 1), want: "branch_analyzer", code: 1},
		{name: "missing owner", body: strings.Replace(validPolicy, "product-toolchain", "", 1), want: "owner and source", code: 1},
		{name: "missing risk model", body: strings.Replace(validPolicy, "uncovered control-flow can corrupt credentials or projections", "", 1), want: "risk rationale", code: 1},
		{name: "missing measurement", body: strings.Replace(validPolicy, "exact aggregate statement and branch counts plus package observation diagnostics", "", 1), want: "risk rationale", code: 1},
		{name: "missing false-positive cost", body: strings.Replace(validPolicy, "aggregate evidence can hide a local blind spot unless package execution and ratios remain visible", "", 1), want: "risk rationale", code: 1},
		{name: "missing remediation", body: strings.Replace(validPolicy, "test behavior, remove unreachable code, or simplify the owner", "", 1), want: "risk rationale", code: 1},
		{name: "missing review condition", body: strings.Replace(validPolicy, "reassess after repeated denominator-only blocks", "", 1), want: "risk rationale", code: 1},
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

func TestParsePackageList(t *testing.T) {
	packages, err := parsePackageList("example/internal/a\texample\nexample\texample\nexample/internal/a\texample\n")
	if err != nil || len(packages) != 2 || packages[0].ImportPath != "example" || packages[1].ImportPath != "example/internal/a" {
		t.Fatalf("packages=%+v err=%v", packages, err)
	}
	for _, body := range []string{"broken\n", "foreign/pkg\texample\n"} {
		if _, err := parsePackageList(body); err == nil {
			t.Fatalf("invalid package list accepted: %q", body)
		}
	}
}

func TestParseBranchReportCountsEveryPackage(t *testing.T) {
	packages := []packageInfo{{ImportPath: "example", ModulePath: "example"}, {ImportPath: "example/internal/a", ModulePath: "example"}}
	report := `<coverage version="1"><file path="main.go"><lineToCover lineNumber="1" covered="true"/></file><file path="internal/a/a.go"><lineToCover lineNumber="2" covered="true" branchesToCover="4" coveredBranches="3"/></file></coverage>`
	counts, err := parseBranchReport([]byte(report), packages)
	if err != nil || counts["example"].Total != 0 || counts["example/internal/a"] != (coverageCount{Covered: 3, Total: 4}) {
		t.Fatalf("counts=%+v err=%v", counts, err)
	}
}

func TestParseBranchReportRejectsInvalidOrIncompleteEvidence(t *testing.T) {
	packages := []packageInfo{{ImportPath: "example/internal/a", ModulePath: "example"}}
	reports := []string{
		`<coverage version="1"></coverage>`,
		`<coverage version="1"><file path="foreign/a.go"/></coverage>`,
		`<coverage version="1"><file path="../internal/a/a.go"/></coverage>`,
		`<coverage version="1"><file path="internal/a/a.go"/><file path="internal/a/a.go"/></coverage>`,
		`<coverage version="1"><file path="internal/a/a.go"><lineToCover branchesToCover="1" coveredBranches="2"/></file></coverage>`,
		`not xml`,
	}
	for _, report := range reports {
		if _, err := parseBranchReport([]byte(report), packages); err == nil {
			t.Fatalf("invalid branch report accepted: %s", report)
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

func TestRunBranchCoverageRejectsMissingInputCapabilityAndProfile(t *testing.T) {
	policy, err := loadPolicy(writePolicy(t, validPolicy))
	if err != nil {
		t.Fatal(err)
	}
	packages := []packageInfo{{ImportPath: "example/a", ModulePath: "example"}}
	if err := runBranchCoverage("missing", packages, policy, io.Discard, io.Discard, &noInputRunner{}); err == nil || !strings.Contains(err.Error(), "standard input") {
		t.Fatalf("missing input capability error = %v", err)
	}
	if err := runBranchCoverage(filepath.Join(t.TempDir(), "missing.out"), packages, policy, io.Discard, io.Discard, &recordingRunner{}); err == nil || !strings.Contains(err.Error(), "open coverage profile") {
		t.Fatalf("missing profile error = %v", err)
	}
}

func TestRunBranchCoverageRejectsAnalyzerAndPolicyFailures(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(profile, []byte("mode: atomic\nexample/a.go:1.1,2.1 1 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := loadPolicy(writePolicy(t, validPolicy))
	if err != nil {
		t.Fatal(err)
	}
	packages := []packageInfo{{ImportPath: "example/a", ModulePath: "example"}}
	for _, test := range []struct {
		name   string
		runner *recordingRunner
		want   string
	}{
		{name: "execution", runner: &recordingRunner{branchErr: errors.New("unavailable")}, want: "analyzer execution"},
		{name: "report", runner: &recordingRunner{branchReport: "not xml"}, want: "decode branch report"},
		{name: "package unobserved", runner: &recordingRunner{branchReport: `<coverage><file path="a/a.go"><lineToCover branchesToCover="100" coveredBranches="0"/></file></coverage>`}, want: "policy is not satisfied"},
		{name: "aggregate empty", runner: &recordingRunner{branchReport: `<coverage><file path="a/a.go"><lineToCover/></file></coverage>`}, want: "policy is not satisfied"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runBranchCoverage(profile, packages, policy, &stdout, &stderr, test.runner)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestSystemRunnerExecutesInputCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := (systemRunner{}).RunInput("go", []string{"env", "GOVERSION"}, strings.NewReader("ignored"), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "go") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
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
