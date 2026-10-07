package projection

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestNativePackageProjectionUsesOnePortableReleaseInputOwner(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, ".gitlab-ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	type nativeJob struct {
		Before []string `yaml:"before_script"`
		Script []string `yaml:"script"`
		After  []string `yaml:"after_script"`
		Rules  []struct {
			If string `yaml:"if"`
		} `yaml:"rules"`
	}
	var pipeline struct {
		Darwin  nativeJob `yaml:"native-darwin"`
		Linux   nativeJob `yaml:"native-linux"`
		Windows nativeJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		t.Fatal(err)
	}
	for platform, job := range map[string]nativeJob{"darwin": pipeline.Darwin, "linux": pipeline.Linux, "windows": pipeline.Windows} {
		commands := strings.Join(append(slices.Clone(job.Before), job.Script...), "\n")
		for _, required := range []string{"--input-package", "--input-sha256", "--candidate-source", "AIGW_NATIVE_INPUT_PACKAGE"} {
			if !strings.Contains(commands, required) {
				t.Errorf("%s omitted the release-owned native package input %s", platform, required)
			}
		}
		if len(job.After) == 0 {
			t.Errorf("%s has no exact cleanup after a failed native journey", platform)
		}
	}
}

func checkProjectedPerformanceArguments(t *testing.T, script, platform string) {
	t.Helper()
	root := t.TempDir()
	checkout := filepath.Join(root, "native checkout")
	if err := os.Mkdir(checkout, 0o700); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(checkout)
	if err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(root, "arguments")
	for name, body := range map[string]string{
		"dbus-run-session": "[ \"$1\" = -- ] || exit 91\nshift\nexec \"$@\"\n",
		"gdbus":            "printf '%s\\n' /org/freedesktop/secrets/collection/session\n",
		"mise":             "printf '%s\\000' \"${AIGW_VERIFY_SYSTEM_KEYRING:-0}\" \"$@\" > \"$AIGW_ARGUMENT_WITNESS\"\nexit \"$AIGW_NATIVE_COMMAND_EXIT\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ name, repository, exit string }{
		{"success", "https://forge.invalid/native checkout", "0"},
		{"failure with empty argument", "", "23"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "sh", "-c", script)
			command.Dir = checkout
			command.WaitDelay = time.Second
			command.Env = append(os.Environ(), "PATH="+root+":/usr/bin:/bin", "CI_PIPELINE_SOURCE=api", "CI_PROJECT_DIR=manager-relative-build",
				"CI_PROJECT_URL="+test.repository, "AIGW_NATIVE_PERFORMANCE=true", "AIGW_VERIFY_SYSTEM_KEYRING=0", "AIGW_FULL_NATIVE_QUALITY=false",
				"AIGW_BASELINE_TAG=v1.2.3", "AIGW_BASELINE_ARTIFACTS=", "AIGW_CANDIDATE_TAG=", "AIGW_CANDIDATE_ARTIFACTS=candidate bytes",
				"AIGW_CANDIDATE_SOURCE=selected-source", "AIGW_NATIVE_INPUT_PACKAGE=", "AIGW_NATIVE_PERFORMANCE_ATTRIBUTION=false",
				"AIGW_NATIVE_DIAGNOSTIC_CLIENT=", "AIGW_NATIVE_CLIENTS=false", "AIGW_ARGUMENT_WITNESS="+witness, "AIGW_NATIVE_COMMAND_EXIT="+test.exit)
			output, runErr := command.CombinedOutput()
			var exit *exec.ExitError
			if test.exit == "0" && runErr != nil || test.exit != "0" && (!errors.As(runErr, &exit) || exit.ExitCode() != 23) {
				t.Fatalf("native command exit was not preserved: %v, %s", runErr, output)
			}
			arguments, err := os.ReadFile(witness)
			if err != nil {
				t.Fatal(err)
			}
			keyring := "0"
			if platform == "linux" {
				keyring = "1"
			}
			want := []string{keyring, "exec", "--locked", "--", "go", "run", "./tools/ci", "native", "--platform", platform, "--full-quality=false", "--",
				"--peer", "gitlab", "--repository", test.repository, "--baseline-tag", "v1.2.3", "--artifacts", "candidate bytes", "--candidate",
				"--candidate-source", "selected-source", "--performance", filepath.Join(physical, "build", "verification", "performance")}
			if got := strings.Split(strings.TrimSuffix(string(arguments), "\x00"), "\x00"); !slices.Equal(got, want) {
				t.Fatalf("native command argument identity = %q, want %q", got, want)
			}
		})
	}
}

func TestLinuxPerformanceUsesPrivateSecretServiceOnBothPeers(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		Linux gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	var github struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				If   string `yaml:"if"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	prepared := false
	performance := ""
	for _, step := range github.Jobs["native-linux"].Steps {
		if step.Name == "Prepare native Secret Service" {
			prepared = strings.Contains(step.If, "inputs.performance") && strings.Contains(step.Run, "dbus-x11 gnome-keyring libglib2.0-bin")
		}
		if step.Name == "Measure historical release performance" {
			performance = step.Run
		}
	}
	if !prepared {
		t.Error("artifact-only GitHub performance lacks its native-store prerequisites")
	}
	commands := map[string]string{"github": performance, "gitlab": strings.Join(gitlab.Linux.Script, "\n")}
	for peer, command := range commands {
		bus := strings.Index(command, "dbus-run-session")
		alias := strings.Index(command, "SetAlias default /org/freedesktop/secrets/collection/session")
		worker := strings.Index(command, "AIGW_VERIFY_SYSTEM_KEYRING=1")
		if bus < 0 || alias <= bus || worker <= alias {
			t.Errorf("%s performance starts outside an unlocked private Secret Service session", peer)
			continue
		}
		if !strings.Contains(command, "--performance") || !strings.Contains(command[worker:], "AIGW_SECRET_SERVICE") {
			t.Errorf("%s private native-store session does not enclose the selected performance command", peer)
		}
		if peer == "gitlab" && !strings.Contains(command[worker:], "./tools/ci native --platform linux --full-quality=\"${AIGW_FULL_NATIVE_QUALITY:-false}\" -- \"$@\"") {
			t.Error("GitLab private session does not receive the release owner's selected argument vector")
		}
		if !strings.Contains(command[bus:worker], "bash -s -euo pipefail -- \"$@\"") {
			t.Errorf("%s private session loses the native command's exact argument vector", peer)
		}
		if strings.Contains(command, "TestNativeProductJourney/system_credential_store") {
			t.Errorf("%s performance duplicates standalone lifecycle qualification", peer)
		}
	}
}

func checkLinuxNativeSupplySelection(t *testing.T, before []string) {
	t.Helper()
	root := t.TempDir()
	witness := filepath.Join(root, "packages")
	for name, body := range map[string]string{
		"timeout": "[ \"$1\" = --verbose ] && [ \"$2\" = --kill-after=5s ] && [ \"$3\" = 240s ] || exit 91\nshift 3\nexec \"$@\"\n",
		"apt-get": "printf '%s\\n' \"$@\" >> \"$AIGW_PACKAGE_WITNESS\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	options := []string{"-o", "Acquire::Retries=1", "-o", "Acquire::http::Timeout=30", "-o", "Acquire::https::Timeout=30", "install", "--no-install-recommends", "-y"}
	for _, test := range []struct {
		name, pack, full, refresh, performance string
		compiler, store                        bool
	}{
		{"source", "", "false", "false", "false", true, true},
		{"prebuilt lifecycle", "fixture", "false", "false", "false", false, false},
		{"prebuilt performance", "fixture", "false", "false", "true", false, true},
		{"full source", "fixture", "true", "false", "false", true, true},
		{"lock refresh", "fixture", "false", "true", "false", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(witness, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "sh", "-c", strings.Join(before[1:3], "\n")+"\nprintf '%s' \"$CGO_ENABLED\"")
			command.Env = append(os.Environ(), "PATH="+root+":/usr/bin:/bin", "AIGW_PACKAGE_WITNESS="+witness, "AIGW_NATIVE_INPUT_PACKAGE="+test.pack,
				"AIGW_CANDIDATE_ARTIFACTS=", "AIGW_CANDIDATE_TAG=", "AIGW_FULL_NATIVE_QUALITY="+test.full,
				"AIGW_REFRESH_LOCKS="+test.refresh, "AIGW_NATIVE_PERFORMANCE="+test.performance, "CGO_ENABLED=1")
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("selected Linux prerequisites: %v, %s", err, output)
			}
			wantCGO := "0"
			if test.compiler {
				wantCGO = "1"
			}
			if got := string(output); got != wantCGO {
				t.Fatalf("selected compiler environment = %q, want %q", got, wantCGO)
			}
			data, err := os.ReadFile(witness)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if test.compiler {
				want = append(want, options...)
				want = append(want, "gcc", "libc6-dev")
			}
			if test.store {
				want = append(want, options...)
				want = append(want, "dbus-x11", "gnome-keyring", "libglib2.0-bin")
			}
			if got := strings.Fields(string(data)); !slices.Equal(got, want) {
				t.Fatalf("selected native prerequisites = %q, want %q", got, want)
			}
		})
	}
}
