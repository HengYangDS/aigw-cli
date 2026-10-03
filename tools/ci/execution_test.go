package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestGateSequenceStopsBeforeExecutionWhenProgressCannotBeWritten(t *testing.T) {
	output, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err = runCommands([]command{{Name: "first"}, {Name: "second"}}, output, func(command) error {
		calls++
		return nil
	})
	if !errors.Is(err, os.ErrClosed) || calls != 0 {
		t.Fatalf("progress output error=%v executed gates=%d", err, calls)
	}
}

func TestSystemRunnerPropagatesSetupAndCommandFailures(t *testing.T) {
	root := t.TempDir()
	if err := systemRunner(command{Name: "go", Args: []string{"version"}, Dir: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("systemRunner accepted an unavailable working directory")
	}
	if err := systemRunner(command{Name: "definitely-not-an-aigw-command", Dir: root}); err == nil {
		t.Fatal("systemRunner hid command failure")
	}
}

func TestSystemRunnerPreservesSuccessfulToolDiagnostics(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	diagnosticsPath := filepath.Join(root, "diagnostics.txt")
	diagnostics, err := os.Create(diagnosticsPath)
	if err != nil {
		t.Fatal(err)
	}
	previousStderr := os.Stderr
	os.Stderr = diagnostics
	err = systemRunner(command{
		Name: os.Args[0],
		Args: []string{"-test.run=^TestSystemRunnerDiagnosticHelper$"},
		Env:  []string{"AIGW_CI_TEST_DIAGNOSTIC=tool progress"},
	})
	os.Stderr = previousStderr
	if closeErr := diagnostics.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatalf("successful tool progress = %v", err)
	}
	got, err := os.ReadFile(diagnosticsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "tool progress" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestSystemRunnerRejectsSuccessfulNativeWarnings(t *testing.T) {
	for _, test := range []struct {
		name, diagnostic string
		invalid          bool
	}{
		{"ordinary progress", "tool progress", false},
		{"zero finding counts", "warning_count=0 error_count=0", false},
		{"warning", "WARN native gate: incomplete analysis", true},
		{"colored warning", "\x1b[33mwarning\x1b[0m: partial analysis", true},
		{"error", "ERROR native gate: incomplete analysis", true},
		{"deprecation", "DeprecationWarning: obsolete configuration", true},
		{"truncated evidence", strings.Repeat("x", commandDiagnosticLimit+1), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			diagnostics, err := os.Create(filepath.Join(root, "diagnostics.txt"))
			if err != nil {
				t.Fatal(err)
			}
			environment := "AIGW_CI_TEST_DIAGNOSTIC=" + test.diagnostic
			if len(test.diagnostic) > commandDiagnosticLimit {
				environment = fmt.Sprintf("AIGW_CI_TEST_DIAGNOSTIC_SIZE=%d", len(test.diagnostic))
			}
			previous := os.Stderr
			os.Stderr = diagnostics
			err = systemRunner(command{
				Name: os.Args[0],
				Args: []string{"-test.run=^TestSystemRunnerDiagnosticHelper$"},
				Env:  []string{environment},
				Dir:  root,
			})
			os.Stderr = previous
			if closeErr := diagnostics.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if (err != nil) != test.invalid {
				t.Fatalf("native stderr admission: invalid=%t error=%v", test.invalid, err)
			}
			if got := readFile(t, diagnostics.Name()); string(got) != test.diagnostic {
				t.Fatal("native stderr evidence was lost")
			}
		})
	}
}

func TestSystemRunnerBoundsFailedCommandDiagnostics(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	diagnostics, err := os.Create(filepath.Join(root, "diagnostics.txt"))
	if err != nil {
		t.Fatal(err)
	}
	previousStderr := os.Stderr
	os.Stderr = diagnostics
	err = systemRunner(command{
		Name: os.Args[0],
		Args: []string{"-test.run=^TestSystemRunnerDiagnosticHelper$"},
		Env: []string{
			fmt.Sprintf("AIGW_CI_TEST_DIAGNOSTIC_SIZE=%d", commandDiagnosticLimit+1),
			"AIGW_CI_TEST_FAIL=1",
		},
	})
	os.Stderr = previousStderr
	if closeErr := diagnostics.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err == nil || !strings.Contains(err.Error(), "[diagnostics truncated]") {
		t.Fatalf("failure diagnostics = %v", err)
	}
}

func TestSystemRunnerDiagnosticHelper(t *testing.T) {
	diagnostic, hasDiagnostic := os.LookupEnv("AIGW_CI_TEST_DIAGNOSTIC")
	sizeText, hasSize := os.LookupEnv("AIGW_CI_TEST_DIAGNOSTIC_SIZE")
	if !hasDiagnostic && !hasSize {
		return
	}
	if hasSize {
		size, err := strconv.Atoi(sizeText)
		if err != nil {
			t.Fatal(err)
		}
		diagnostic = strings.Repeat("x", size)
	}
	if _, err := fmt.Fprint(os.Stderr, diagnostic); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("AIGW_CI_TEST_FAIL") == "1" {
		os.Exit(1)
	}
}

func TestSystemOutputRunnerPropagatesSetupFailuresWithoutResidue(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, root)
	}
	for _, call := range []command{
		{Name: "node", Args: []string{"--version"}, Dir: filepath.Join(root, "missing")},
		{Name: "definitely-not-an-aigw-command", Dir: root},
	} {
		output, err := systemOutputRunner(call)
		if err == nil || len(output) != 0 {
			t.Fatalf("command setup: error=%v output=%q", err, output)
		}
		entries, readErr := os.ReadDir(root)
		if readErr != nil || len(entries) != 0 {
			t.Fatalf("setup failure residue: %v error=%v", entries, readErr)
		}
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, filepath.Join(root, "missing"))
	}
	output, err := systemOutputRunner(command{Name: "node", Args: []string{"--version"}})
	if err == nil || !strings.Contains(err.Error(), "create command output") || len(output) != 0 {
		t.Fatalf("unavailable capture directory: error=%v output=%q", err, output)
	}
}

func TestSystemOutputRunnerPreservesNativeImmediateExitEvidence(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, root)
	}
	foreign := filepath.Join(root, "foreign.txt")
	if err := os.WriteFile(foreign, []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("x", 8192) + " complete-native-evidence\n"
	for _, test := range []struct {
		name, stdout, stderr string
		exit                 int
	}{
		{"successful warning", "", payload, 0},
		{"successful result", payload, "", 0},
		{"failed mixed output", "result\n", payload, 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := systemOutputRunner(command{
				Name: "node",
				Args: []string{"-e", "process.stdout.write(process.env.AIGW_CI_TEST_STDOUT); process.stderr.write(process.env.AIGW_CI_TEST_STDERR); process.exit(Number(process.env.AIGW_CI_TEST_EXIT));"},
				Env: []string{
					"AIGW_CI_TEST_STDOUT=" + test.stdout,
					"AIGW_CI_TEST_STDERR=" + test.stderr,
					"AIGW_CI_TEST_EXIT=" + strconv.Itoa(test.exit),
				},
				Dir: root,
			})
			if string(output) != test.stdout+test.stderr || (err != nil) != (test.exit != 0) {
				t.Fatalf("native evidence: exit=%d error=%v bytes=%d want=%d", test.exit, err, len(output), len(test.stdout)+len(test.stderr))
			}
			entries, readErr := os.ReadDir(root)
			if readErr != nil || len(entries) != 1 || entries[0].Name() != "foreign.txt" {
				t.Fatalf("output capture residue: %v error=%v", entries, readErr)
			}
		})
	}
	if content, err := os.ReadFile(foreign); err != nil || string(content) != "retained" {
		t.Fatalf("foreign content changed: %q error=%v", content, err)
	}
}

func TestSourceCommandsKeepSuccessfulOutputQuietWithoutSuppressingWarnings(t *testing.T) {
	t.Setenv("AIGW_COMMIT_BASE", "")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	commands, err := configuredSourceCommands(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []command{
		{Name: "osv-scanner", Args: []string{"scan", "source", "--config", ".config/checks/dependencies/policy.toml", "--lockfile", "go.mod", "--lockfile", "package-lock.json", "--format", "table", "--verbosity", "warn", "."}},
	}
	for _, expected := range want {
		if !slices.ContainsFunc(commands, func(call command) bool { return reflect.DeepEqual(call, expected) }) {
			t.Fatalf("source commands lack quiet-success contract %#v: %#v", expected, commands)
		}
	}
}

func TestDependencyScanUsesOwnedPolicyWithoutChangingCallerFilters(t *testing.T) {
	policy := ".config/checks/dependencies/policy.toml"
	content := readFile(t, filepath.Join(repositoryRoot(t), filepath.FromSlash(policy)))
	root := t.TempDir()
	filter := "[[PackageOverrides]]\nignore = true\n"
	inputs := map[string][]byte{
		policy:              content,
		"go.mod":            []byte("module example.com/fixture\ngo 1.23.0\nrequire example.com/dependency v1.0.0\n"),
		"package-lock.json": []byte(`{"name":"fixture","version":"1.0.0","lockfileVersion":3,"packages":{"node_modules/dependency":{"version":"1.0.0"}}}`),
		"osv-scanner.toml":  []byte(filter),
	}
	// Native offline databases make this configuration test independent of the network.
	var database bytes.Buffer
	if err := zip.NewWriter(&database).Close(); err != nil {
		t.Fatal(err)
	}
	for _, ecosystem := range []string{"Go", "npm"} {
		inputs[filepath.Join("cache", "osv-scalibr", ecosystem, "all.zip")] = database.Bytes()
	}
	for name, data := range inputs {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	index := slices.IndexFunc(qualityCommands, func(call command) bool { return call.Name == "osv-scanner" })
	if index < 0 {
		t.Fatal("source gate has no dependency scan")
	}
	call := qualityCommands[index]
	call.Dir = root
	call.Env = []string{"OSV_SCALIBR_LOCAL_DB_CACHE_DIRECTORY=" + filepath.Join(root, "cache")}
	// The private test checkout lives beneath the repository's ignored build tree.
	call.Args = append(slices.Clone(call.Args), "--no-ignore", "--offline", "--no-call-analysis=go", "--format=json", "--all-packages")
	output, err := systemOutputRunner(call)
	if err != nil {
		t.Fatalf("native dependency scan failed: %v\n%s", err, output)
	}
	var report struct {
		Results []struct {
			Packages []struct {
				Package struct {
					Name string `json:"name"`
				} `json:"package"`
			} `json:"packages"`
		} `json:"results"`
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode native dependency scan: %v\n%s", err, output)
	}
	var names []string
	for _, result := range report.Results {
		for _, item := range result.Packages {
			names = append(names, item.Package.Name)
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"dependency", "example.com/dependency"}) {
		t.Fatalf("dependency inventory = %v, want both unfiltered lockfiles", names)
	}
	for name, expected := range inputs {
		if actual := readFile(t, filepath.Join(root, filepath.FromSlash(name))); !bytes.Equal(actual, expected) {
			t.Fatalf("dependency scan changed %s", name)
		}
	}
}

func TestCommandRunnersHonorExecutionContextWithoutCreatingArtifacts(t *testing.T) {
	root := t.TempDir()
	caller := t.TempDir()
	t.Chdir(caller)
	if err := os.WriteFile("build", []byte("unowned caller content"), 0o600); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "environment.go")
	if err := os.WriteFile(helper, []byte("package main\nimport \"os\"\nfunc main(){ if os.Getenv(\"AIGW_CI_TEST\") != \"isolated\" { os.Exit(1) } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, runner := range map[string]commandRunner{
		"stream": systemRunner,
		"capture": func(call command) error {
			_, err := systemOutputRunner(call)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := runner(command{Name: "go", Args: []string{"run", "environment.go"}, Env: []string{"AIGW_CI_TEST=isolated"}, Dir: root}); err != nil {
				t.Fatalf("execution context: %v", err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "environment.go" {
				t.Fatalf("unexpected target artifacts: %v error=%v", entries, err)
			}
		})
	}
	if content, err := os.ReadFile("build"); err != nil || string(content) != "unowned caller content" {
		t.Fatalf("caller content changed: %q error=%v", content, err)
	}
}
