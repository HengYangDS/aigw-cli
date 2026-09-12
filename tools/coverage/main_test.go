package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
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
	metadata       string
	metadataErr    error
}

type rejectingWriter struct{ prefix string }

func (writer rejectingWriter) Write(data []byte) (int, error) {
	if writer.prefix != "" && !bytes.HasPrefix(data, []byte(writer.prefix)) {
		return len(data), nil
	}
	return 0, errors.New("write rejected")
}

func (r *recordingRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	if slices.Equal(args[:min(2, len(args))], []string{"list", "-json"}) {
		if _, err := io.WriteString(stdout, r.metadata); err != nil {
			return err
		}
		return r.metadataErr
	}
	if len(args) > 0 && args[0] == "list" {
		packages := r.listedPackages
		if len(packages) == 0 {
			packages = profilePackages(r.profile)
		}
		if len(packages) == 0 {
			packages = []string{"example/a"}
		}
		for _, packageName := range packages {
			module, _, _ := strings.Cut(packageName, "/")
			if _, err := fmt.Fprintf(stdout, "%s\t%s\n", packageName, module); err != nil {
				return err
			}
		}
		return r.listErr
	}
	r.name = name
	r.args = append([]string(nil), args...)
	for _, arg := range args {
		if after, ok := strings.CutPrefix(arg, "-coverprofile="); ok {
			r.path = after
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
	for line := range strings.SplitSeq(profile, "\n") {
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

func TestLoadPolicyAcceptsTheGoCoverageAuthorityWithoutASecondAnalyzer(t *testing.T) {
	policy := `minimum_statement_percent = 95.0
comparison = "greater-than"
threshold_scopes = ["aggregate"]
package_observation = "required"
covermode = "atomic"
packages = ["./..."]
owner = "product-toolchain"
source = "Go statement coverage profile"
risk_model = "unobserved behavior in any semantic owner can corrupt credentials routes client projections or publication behavior without an observable regression"
measurement = "one complete Go profile with exact aggregate statement counts plus canonical-package observation diagnostics"
false_positive_cost = "aggregate evidence can hide a local blind spot unless every package remains present executed and visible"
remediation = "test meaningful behavior remove unreachable code or simplify the responsible semantic owner; wholly unexecuted packages and exclusions are not accepted"
review_condition = "reassess when escaped defects show the aggregate boundary or mandatory package observation is insufficient"
`

	if _, err := loadPolicy(writePolicy(t, policy)); err != nil {
		t.Fatalf("statement-only policy rejected: %v", err)
	}
}

const validPolicy = `minimum_statement_percent = 95.0
comparison = "greater-than"
threshold_scopes = ["aggregate"]
package_observation = "required"
covermode = "atomic"
packages = ["./..."]
owner = "product-toolchain"
source = "Go statement coverage profile"
risk_model = "unobserved behavior can corrupt credentials or projections"
measurement = "exact aggregate statement counts plus package observation diagnostics"
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
	if !contains(runner.args, "-coverpkg=example") {
		t.Errorf("arguments %q do not attribute full-suite execution to source packages", runner.args)
	}
	if got := runner.args[len(runner.args)-1]; got != "./..." {
		t.Fatalf("last argument = %q, want ./...", got)
	}
	if !strings.Contains(stdout.String(), "package example statement coverage: 96.00%") || !strings.Contains(stdout.String(), "statement coverage: 96.00%") || !strings.Contains(stdout.String(), "required > 95.00%") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(runner.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary profile remains: %v", err)
	}
}

func TestRealMainRejectsResultOutputFailure(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	for _, test := range []struct {
		name, additionalProfile, rejectedPrefix, diagnostic string
	}{
		{"measured package", "", "package example ", "write package observation"},
		{"zero-statement package", "example/empty/data.go:1.1,2.1 0 0\n", "package example/empty ", "write package observation"},
		{"aggregate result", "", "statement coverage:", "write coverage result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 96 1\nexample/a.go:3.1,4.1 4 0\n" + test.additionalProfile}
			var stderr bytes.Buffer
			if code := realMain([]string{"--policy", policyPath}, rejectingWriter{prefix: test.rejectedPrefix}, &stderr, runner); code != 1 {
				t.Fatalf("realMain code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), test.diagnostic) {
				t.Fatalf("stderr = %q", stderr.String())
			}
			if _, err := os.Stat(runner.path); !os.IsNotExist(err) {
				t.Fatalf("failed report retained temporary coverage profile: %v", err)
			}
		})
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

func TestRealMainReportsPackageWithoutMeasuredStatements(t *testing.T) {
	policyPath := writePolicy(t, validPolicy)
	runner := &recordingRunner{profile: "mode: atomic\nexample/empty/a.go:1.1,2.1 0 0\nexample/live/b.go:1.1,2.1 100 1\n"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := realMain([]string{"--policy", policyPath}, &stdout, &stderr, runner); code != 0 {
		t.Fatalf("realMain code = %d, stderr=%s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), "package example/empty statement coverage: not applicable (0 measured statements)") {
		t.Fatalf("stdout = %q", stdout.String())
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

func TestRealMainBindsCoverageToSelectedPackages(t *testing.T) {
	for _, foreign := range []string{"", "strings", "exampleish", "example/undeclared"} {
		t.Run(foreign, func(t *testing.T) {
			profile := "mode: atomic\nexample/a.go:1.1,2.1 100 1\nexample/second/b.go:1.1,2.1 100 1\n"
			if foreign != "" {
				profile += foreign + "/foreign.go:1.1,2.1 1000 1\n"
			}
			runner := &recordingRunner{profile: profile, listedPackages: []string{"example/second", "example"}}
			var stdout, stderr bytes.Buffer
			code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, runner)
			if !contains(runner.args, "-coverpkg=example,example/second") {
				t.Fatalf("coverage selection differs from package observation: %q", runner.args)
			}
			if foreign == "" {
				if code != 0 || !strings.Contains(stdout.String(), "(200/200 statements)") {
					t.Fatalf("selected scope rejected: code=%d, %s, %s", code, &stdout, &stderr)
				}
				return
			}
			if code != 1 || !strings.Contains(stderr.String(), "package "+foreign+" is outside the selected coverage scope") {
				t.Fatalf("foreign counters changed the denominator: code=%d, %s, %s", code, &stdout, &stderr)
			}
		})
	}
}

func TestRealMainRejectsNativeCommandFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		runner recordingRunner
		want   string
	}{
		{"package enumeration", recordingRunner{listErr: errors.New("list failed")}, "go list failed"},
		{"test execution", recordingRunner{err: errors.New("test failure")}, "go test failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, &test.runner); code != 1 {
				t.Fatalf("realMain code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
		})
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

func TestCoverageCleanupFailurePreventsSuccessfulExit(t *testing.T) {
	for _, failure := range []error{nil, errors.New("test process failed")} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			directory := t.TempDir()
			remove := removeCoverageProfile
			t.Cleanup(func() { removeCoverageProfile = remove })
			var removed string
			removeCoverageProfile = func(path string) error {
				removed = path
				return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
			}
			var profilePath string
			restore := stubCoverageProfile(func() (*os.File, error) {
				profile, err := os.CreateTemp(directory, "coverage-")
				if err != nil {
					return nil, err
				}
				profilePath = profile.Name()
				return profile, nil
			})
			defer restore()
			var stdout, stderr bytes.Buffer
			runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 100 1\n", err: failure}
			code := realMain([]string{"--policy", writePolicy(t, validPolicy)}, &stdout, &stderr, runner)
			if code != 1 || removed != profilePath || !strings.Contains(stderr.String(), "remove coverage profile") || !strings.Contains(stderr.String(), profilePath) {
				t.Fatalf("cleanup failed without a failing result: code=%d, %s", code, stderr.String())
			}
			if failure != nil && !strings.Contains(stderr.String(), failure.Error()) {
				t.Fatalf("cleanup hid the test failure: %s", stderr.String())
			}
		})
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
		{name: "wrong comparison", body: strings.Replace(validPolicy, "greater-than", "at-least", 1), want: "comparison", code: 1},
		{name: "wrong threshold scope", body: strings.Replace(validPolicy, `["aggregate"]`, `["package"]`, 1), want: "threshold_scopes", code: 1},
		{name: "missing package observation", body: strings.Replace(validPolicy, "required", "optional", 1), want: "package_observation", code: 1},
		{name: "invalid floor", body: strings.Replace(validPolicy, "95.0", "101.0", 1), want: "minimum_statement_percent", code: 1},
		{name: "wrong mode", body: strings.Replace(validPolicy, "atomic", "set", 1), want: "covermode", code: 1},
		{name: "no packages", body: strings.Replace(validPolicy, "[\"./...\"]", "[]", 1), want: "packages", code: 1},
		{name: "missing owner", body: strings.Replace(validPolicy, "product-toolchain", "", 1), want: "owner and source", code: 1},
		{name: "missing risk model", body: strings.Replace(validPolicy, "unobserved behavior can corrupt credentials or projections", "", 1), want: "risk rationale", code: 1},
		{name: "missing measurement", body: strings.Replace(validPolicy, "exact aggregate statement counts plus package observation diagnostics", "", 1), want: "risk rationale", code: 1},
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

func contains(values []string, wanted string) bool {
	return slices.Contains(values, wanted)
}
