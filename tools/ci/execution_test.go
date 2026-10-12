package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func TestSourceCommandsKeepSuccessfulOutputQuietWithoutSuppressingWarnings(t *testing.T) {
	t.Setenv("AIGW_COMMIT_BASE", "")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	commands, err := configuredSourceCommands(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []command{
		{Name: "go", Args: []string{"run", "./tools/release", "scan-dependencies", ".", "build/verification"}},
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
	index := slices.IndexFunc(qualityCommands, func(call command) bool { return call.Name == "go" && slices.Contains(call.Args, "scan-dependencies") })
	if index < 0 {
		t.Fatal("source gate has no dependency scan")
	}
	// CI consumes the construction owner; this native fixture checks its selected
	// lockfile policy independently of online advisory and license availability.
	call := command{Name: "osv-scanner", Dir: root,
		Args: []string{"scan", "source", "--config", policy, "--lockfile", "go.mod", "--lockfile", "package-lock.json", "--no-ignore", "--offline", "--no-call-analysis=go", "--format=json", "--all-packages", "--verbosity", "warn"},
		Env:  []string{"OSV_SCALIBR_LOCAL_DB_CACHE_DIRECTORY=" + filepath.Join(root, "cache")},
	}
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

func TestWorkflowGateQualifiesNativeLargeShellInputAndRefusesShellDefects(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	gate := slices.IndexFunc(qualityCommands, func(call command) bool { return slices.Contains(call.Args, "check-workflows") })
	if gate < 0 {
		t.Fatal("workflow gate lacks complete native shell validation")
	}
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("workflow repository: %v\n%s", err, output)
	}
	path := filepath.Join(root, ".github", "workflows", "verify.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, script string
		invalid      bool
	}{
		{"large finite input", strings.Repeat("# native finite input\n", 600) + "printf '%s\\n' '${{ github.ref }}'\n", false},
		{"native shell defect", "echo $unquoted\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			workflow := "name: Probe\non: workflow_dispatch\ndefaults:\n  run:\n    shell: bash\njobs:\n  probe:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n"
			for line := range strings.SplitSeq(strings.TrimSuffix(test.script, "\n"), "\n") {
				workflow += "          " + line + "\n"
			}
			workflow += "  windows:\n    runs-on: windows-latest\n    defaults:\n      run:\n        shell: pwsh\n    steps:\n      - run: Write-Output 'native Windows'\n"
			if err := os.WriteFile(path, []byte(workflow), 0o600); err != nil {
				t.Fatal(err)
			}
			err := checkWorkflows(root, systemRunner)
			if (err != nil) != test.invalid {
				t.Fatalf("workflow native shell admission: invalid=%v error=%v", test.invalid, err)
			}
			ledgers, err := filepath.Glob(filepath.Join(root, "build", "verification", "workflows", "check-*", "scripts.json"))
			if err != nil || len(ledgers) == 0 {
				t.Fatalf("workflow failure lost the complete input identity ledger: %v %v", ledgers, err)
			}
		})
	}
}

func TestWorkflowScriptSelectionPreservesSourceAndShellInheritance(t *testing.T) {
	file := filepath.Join(t.TempDir(), "workflow.yml")
	const source = `name: Native
on: workflow_dispatch
defaults:
  run:
    shell: bash
jobs:
  unix:
    runs-on: ubuntu-latest
    steps:
      - run: >
          printf '%s' '${{ github.ref }}'
          retained
      - run: Write-Output native
        shell: pwsh
  windows:
    runs-on: windows-latest
    defaults:
      run:
        shell: pwsh
    steps:
      - run: Write-Output native
`
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	scripts, err := workflowScripts(file)
	if err != nil || len(scripts) != 3 {
		t.Fatalf("native YAML scripts: %+v %v", scripts, err)
	}
	if scripts[0].Shell != "bash" || scripts[1].Shell != "pwsh" || scripts[2].Shell != "pwsh" || scripts[0].Bytes != len("printf '%s' '${{ github.ref }}' retained\n") || strings.Contains(scripts[0].Input, "${{") {
		t.Fatalf("source/shell projection changed: %+v", scripts)
	}
	for _, shell := range []string{"", "${{ inputs.shell }}", "unknown-shell"} {
		invalid := strings.ReplaceAll(source, "shell: bash", "shell: "+shell)
		if err := os.WriteFile(file, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := workflowScripts(file); err == nil {
			t.Fatalf("unadmitted shell %q silently skipped", shell)
		}
	}
	if _, err := workflowShellInput("echo '${{ unterminated'", "bash"); err == nil {
		t.Fatal("unterminated workflow expression accepted")
	}
}
