package main

import (
	"aigw-cli/internal/secrets/native"
	"aigw-cli/internal/upgrade"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const mainHelperEnvironment = "AIGW_TEST_MAIN_HELPER"

// An extracted successor that preprojects clients while its predecessor is
// still installed must not migrate the predecessor's Keychain items.
func TestCredentialRoleRequiresTheInstalledProgram(t *testing.T) {
	root := t.TempDir()
	installed := filepath.Join(root, "bin", "aigw")
	extracted := filepath.Join(root, "release", "aigw")
	reader := filepath.Join(root, "credential", strings.Repeat("ab", 32), "aigw")
	for _, path := range []string{installed, upgrade.RollbackPath(installed), extracted, reader} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("program"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name       string
		executable string
		err        error
		want       native.Role
	}{
		{name: "installed program", executable: installed, want: native.InstalledProgram},
		{name: "extracted release", executable: extracted, want: native.OtherProgram},
		{name: "versioned reader", executable: reader, want: native.OtherProgram},
		{name: "unknown executable", err: errors.New("executable unavailable"), want: native.OtherProgram},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := credentialRole(test.executable, test.err); got != test.want {
				t.Fatalf("credential role = %d, want %d", got, test.want)
			}
		})
	}
}

func TestMainProcess(t *testing.T) {
	if os.Getenv(mainHelperEnvironment) == "1" {
		os.Args = []string{"CLAUDE", "--version"}
		main()
		return
	}

	setAIGWTestEnvironment(t)
	command := exec.Command(os.Args[0], "-test.run=^TestMainProcess$")
	command.Env = append(os.Environ(), mainHelperEnvironment+"=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("main subprocess: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "0.1.0-dev") {
		t.Fatalf("main subprocess output = %q, want version", output)
	}
}

func TestRunAIGWVersion(t *testing.T) {
	setAIGWTestEnvironment(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "0.1.0-dev") {
		t.Fatalf("run() stdout = %q, want version", stdout.String())
	}
}

func TestRunAIGWRendersCommandError(t *testing.T) {
	setAIGWTestEnvironment(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"not-a-command"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), `unknown command "not-a-command"`) {
		t.Fatalf("run() stdout = %q, want unknown-command diagnostic", stdout.String())
	}
}

func TestRunAIGWReportsInitializationFailure(t *testing.T) {
	setAIGWTestEnvironment(t)
	const canary = "private-token-fragment"
	t.Setenv("AIGW_SECRET_BACKEND", canary)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if strings.Contains(stderr.String(), canary) || !strings.Contains(stderr.String(), "AIGW") {
		t.Fatalf("run() stderr = %q, want safe initialization diagnostic", stderr.String())
	}
}

func TestRunCredentialFailureKeepsStandardOutputEmpty(t *testing.T) {
	setAIGWTestEnvironment(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"credential", "unsupported"}, &stdout, &stderr); code != 1 {
		t.Fatalf("credential exit code=%d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("credential failure contaminated token output: %s", &stdout)
	}
	if !strings.Contains(stderr.String(), "credential") || !strings.Contains(stderr.String(), "aigw sync") {
		t.Fatalf("credential failure lacks recovery guidance: %q", stderr.String())
	}
}

func TestRunCredentialInitializationFailureRedactsInput(t *testing.T) {
	setAIGWTestEnvironment(t)
	const canary = "private-backend-input"
	t.Setenv("AIGW_SECRET_BACKEND", canary)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"credential", "codex", "projection"}, &stdout, &stderr); code != 1 {
		t.Fatalf("credential initialization exit code=%d", code)
	}
	if stdout.Len() != 0 || strings.Contains(stderr.String(), canary) || !strings.Contains(stderr.String(), "backend") {
		t.Fatalf("unsafe initialization result: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestNativeCredentialSubprocessRejectsForeignServiceBeforeInitialization(t *testing.T) {
	setAIGWTestEnvironment(t)
	t.Setenv("AIGW_SECRET_BACKEND", "must-not-be-initialized")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"__aigw-native-credential-read", "foreign-service", "team"}, &stdout, &stderr); code == 0 {
		t.Fatal("invalid credential subprocess invocation succeeded")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("credential subprocess leaked application or credential diagnostics")
	}
}

func setAIGWTestEnvironment(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("XDG_DATA_HOME", home)
	t.Setenv("APPDATA", home)
	t.Setenv("LOCALAPPDATA", home)
	t.Setenv("AIGW_SECRET_BACKEND", "env")
	t.Setenv("NO_COLOR", "1")
}
