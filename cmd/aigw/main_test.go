package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const mainHelperEnvironment = "AIGW_TEST_MAIN_HELPER"

func TestMainProcess(t *testing.T) {
	if os.Getenv(mainHelperEnvironment) == "1" {
		os.Args = []string{"AIGW.EXE", "--version"}
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
	if code := run("/opt/aigw/AIGW.EXE", []string{"--version"}, &stdout, &stderr); code != 0 {
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
	if code := run("aigw", []string{"not-a-command"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), `unknown command "not-a-command"`) {
		t.Fatalf("run() stdout = %q, want unknown-command diagnostic", stdout.String())
	}
}

func TestRunAIGWRoutesClaudeInvocation(t *testing.T) {
	setAIGWTestEnvironment(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run("/usr/local/bin/CLAUDE", []string{"--version"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "Claude adapter is not enabled") {
		t.Fatalf("run() stdout = %q, want adapter diagnostic", stdout.String())
	}
}

func TestRunAIGWReportsInitializationFailure(t *testing.T) {
	setAIGWTestEnvironment(t)
	t.Setenv("AIGW_SECRET_BACKEND", "invalid-test-backend")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run("aigw", []string{"--version"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "aigw:") || !strings.Contains(stderr.String(), "invalid-test-backend") {
		t.Fatalf("run() stderr = %q, want initialization diagnostic", stderr.String())
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
