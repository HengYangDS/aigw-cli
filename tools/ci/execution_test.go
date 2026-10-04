package main

import (
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

func TestSystemRunnerPreservesNativeImmediateExitDiagnostics(t *testing.T) {
	root := t.TempDir()
	outputRoot := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, root)
	}
	for _, test := range []struct {
		name, tail string
		size, exit int
		invalid    bool
	}{
		{"ordinary progress", " tool progress\n", 8192, 0, false},
		{"warning", " warning: incomplete analysis\n", 8192, 0, true},
		{"warning beyond pipe capacity", " warning: incomplete analysis\n", commandDiagnosticLimit - 1, 0, true},
		{"failed large diagnostics", " failure detail\n", commandDiagnosticLimit - 1, 7, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics, err := os.Create(filepath.Join(outputRoot, test.name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			previous := os.Stderr
			os.Stderr = diagnostics
			err = systemRunner(command{
				Name: "node",
				Args: []string{"-e", "process.stderr.write('x'.repeat(Number(process.env.AIGW_CI_TEST_SIZE)) + process.env.AIGW_CI_TEST_TAIL); process.exit(Number(process.env.AIGW_CI_TEST_EXIT));"},
				Env: []string{
					"AIGW_CI_TEST_SIZE=" + strconv.Itoa(test.size),
					"AIGW_CI_TEST_TAIL=" + test.tail,
					"AIGW_CI_TEST_EXIT=" + strconv.Itoa(test.exit),
				},
				Dir: root,
			})
			os.Stderr = previous
			if closeErr := diagnostics.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if (err != nil) != test.invalid {
				t.Fatalf("immediate native exit: invalid=%t error=%v", test.invalid, err)
			}
			want := strings.Repeat("x", test.size) + test.tail
			if got := readFile(t, diagnostics.Name()); string(got) != want {
				t.Fatalf("native diagnostics: received=%d expected=%d", len(got), len(want))
			}
			if entries, readErr := os.ReadDir(root); readErr != nil || len(entries) != 0 {
				t.Fatalf("diagnostic capture residue: %v error=%v", entries, readErr)
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

func TestCommandRunnersPropagateSetupFailuresWithoutResidue(t *testing.T) {
	for name, runner := range map[string]outputRunner{
		"stream":  func(call command) ([]byte, error) { return nil, systemRunner(call) },
		"capture": systemOutputRunner,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, root)
			}
			for _, call := range []command{
				{Name: "node", Args: []string{"--version"}, Dir: filepath.Join(root, "missing")},
				{Name: "definitely-not-an-aigw-command", Dir: root},
			} {
				output, err := runner(call)
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
			output, err := runner(command{Name: "node", Args: []string{"--version"}})
			if err == nil || !strings.Contains(err.Error(), "create command") || len(output) != 0 {
				t.Fatalf("unavailable capture directory: error=%v output=%q", err, output)
			}
		})
	}
}

func TestSystemRunnerRejectsFailedDiagnosticReplayWithoutResidue(t *testing.T) {
	root := t.TempDir()
	output, err := os.Create(filepath.Join(t.TempDir(), "closed-stderr"))
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, root)
	}
	previous := os.Stderr
	os.Stderr = output
	err = systemRunner(command{
		Name: os.Args[0],
		Args: []string{"-test.run=^TestSystemRunnerDiagnosticHelper$"},
		Env:  []string{"AIGW_CI_TEST_DIAGNOSTIC=tool progress"},
		Dir:  root,
	})
	os.Stderr = previous
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("diagnostic replay failure = %v", err)
	}
	if entries, readErr := os.ReadDir(root); readErr != nil || len(entries) != 0 {
		t.Fatalf("failed replay residue: %v error=%v", entries, readErr)
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
		{Name: "go", Args: []string{"run", "./tools/release", "scan-dependencies", ".", "build/verification/dependencies"}},
	}
	for _, expected := range want {
		if !slices.ContainsFunc(commands, func(call command) bool { return reflect.DeepEqual(call, expected) }) {
			t.Fatalf("source commands lack quiet-success contract %#v: %#v", expected, commands)
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
