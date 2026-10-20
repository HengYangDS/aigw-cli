package projection

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestNativePackageProjectionUsesOnePortableReleaseInputOwner(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, ".gitlab-ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Darwin  gitLabJob `yaml:"native-darwin"`
		Linux   gitLabJob `yaml:"native-linux"`
		Windows gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		t.Fatal(err)
	}
	for platform, job := range map[string]gitLabJob{"darwin": pipeline.Darwin, "linux": pipeline.Linux, "windows": pipeline.Windows} {
		native := "mise exec --locked -- go run ./tools/ci native --platform " + platform + " --from-env"
		if !slices.Contains(job.Script, native) {
			t.Errorf("%s does not derive its release inputs in the native command", platform)
		}
		// The native command alone maps the declared inputs to release flags.
		if commands := strings.Join(append(slices.Clone(job.BeforeScript), job.Script...), "\n"); strings.Contains(commands, "--input-package") || strings.Contains(commands, "--candidate-source") {
			t.Errorf("%s assembles release inputs outside the native command", platform)
		}
		if len(job.AfterScript) == 0 {
			t.Errorf("%s has no exact cleanup after a failed native journey", platform)
		}
	}
}

func TestLinuxNativeReleaseUsesPrivateSecretServiceOnBothPeers(t *testing.T) {
	projections := canonicalProjections(t)
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
	performance, lifecycle := "", ""
	for _, step := range github.Jobs["native-linux"].Steps {
		if step.Name == "Prepare native Secret Service" {
			prepared = strings.Contains(step.If, "inputs.baseline_tag") && strings.Contains(step.If, "inputs.performance") && strings.Contains(step.Run, "dbus-x11 gnome-keyring libglib2.0-bin")
		}
		if step.Name == "Measure historical release performance" {
			performance = step.Run
		}
		if step.Name == "Run historical release acceptance" {
			lifecycle = step.Run
		}
	}
	if !prepared {
		t.Error("GitHub native release acceptance lacks its native-store prerequisites")
	}
	for peer, command := range map[string]string{"github performance": performance, "github lifecycle": lifecycle} {
		bus := strings.Index(command, "dbus-run-session")
		alias := strings.Index(command, "SetAlias default /org/freedesktop/secrets/collection/session")
		worker := strings.Index(command, "AIGW_VERIFY_SYSTEM_KEYRING=1")
		if bus < 0 || alias <= bus || worker <= alias {
			t.Errorf("%s acceptance starts outside an unlocked private Secret Service session", peer)
			continue
		}
		if !strings.Contains(command[worker:], "AIGW_SECRET_SERVICE") || peer == "github lifecycle" && !strings.Contains(command, "accept-native") || peer != "github lifecycle" && !strings.Contains(command, "--performance") {
			t.Errorf("%s private native-store session does not enclose the selected release command", peer)
		}
		if !strings.Contains(command[bus:worker], "bash -s -euo pipefail -- \"$@\"") {
			t.Errorf("%s private session loses the native command's exact argument vector", peer)
		}
		if strings.Contains(command, "TestNativeProductJourney/system_credential_store") {
			t.Errorf("%s release acceptance duplicates standalone lifecycle qualification", peer)
		}
	}
	// GitLab's native command starts the same private session around itself
	// when release inputs reach the system credential store.
	script := strings.Join(gitlab.Linux.Script, "\n")
	if !strings.Contains(script, "./tools/ci native --platform linux --from-env") || strings.Contains(script, "dbus-run-session") {
		t.Errorf("GitLab Linux release acceptance does not leave its private session to the native command: %q", gitlab.Linux.Script)
	}
	if !slices.ContainsFunc(gitlab.Linux.BeforeScript, func(step string) bool { return strings.Contains(step, "dbus-x11 gnome-keyring libglib2.0-bin") }) {
		t.Error("GitLab native release acceptance lacks its native-store prerequisites")
	}
}

func checkLinuxNativeSupplySelection(t *testing.T, packages string) {
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
	for name, selected := range map[string]string{"source": "gcc libc6-dev", "artifact": ""} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(witness, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "sh", "-c", packages)
			command.Env = append(os.Environ(), "PATH="+root+":/usr/bin:/bin", "AIGW_PACKAGE_WITNESS="+witness, "AIGW_LINUX_COMPILER_PACKAGES="+selected)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("selected Linux prerequisites: %v, %s", err, output)
			}
			data, err := os.ReadFile(witness)
			if err != nil {
				t.Fatal(err)
			}
			want := append(append(slices.Clone(options), strings.Fields(selected)...), "dbus-x11", "gnome-keyring", "libglib2.0-bin")
			if got := strings.Fields(string(data)); !slices.Equal(got, want) {
				t.Fatalf("selected native prerequisites = %q, want %q", got, want)
			}
		})
	}
}
