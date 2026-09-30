package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestMiseOSVScannerUsesOfficialRelease(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration miseConfiguration
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	const scanner = "github:google/osv-scanner"
	if version := configuredMiseToolVersion(t, configuration.Tools, scanner); version == "" {
		t.Fatal("official OSV Scanner release is not pinned")
	}
	options, ok := configuration.Tools[scanner].(map[string]any)
	if !ok || options["slsa_signer_identity"] != "https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@refs/tags/v2.1.0" ||
		options["slsa_signer_issuer"] != "https://token.actions.githubusercontent.com" {
		t.Fatalf("OSV Scanner lacks its verified release signer: %#v", options)
	}
	if _, present := configuration.Tools["go:github.com/google/osv-scanner/v2/cmd/osv-scanner"]; present {
		t.Fatal("source-built OSV Scanner remains a parallel installation")
	}
}

type miseTask struct {
	Name string   `json:"name"`
	Run  []string `json:"run"`
}

func TestMisePerformanceToolIsTaskScoped(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration struct {
		Tools map[string]any `toml:"tools"`
		Tasks struct {
			Performance struct {
				Tools map[string]string `toml:"tools"`
				Run   string            `toml:"run"`
			} `toml:"performance"`
		} `toml:"tasks"`
	}
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	task := configuration.Tasks.Performance
	if task.Tools["github:sharkdp/hyperfine"] == "" || task.Run != "go run ./tools/release accept-native" {
		t.Fatalf("performance must bind Hyperfine to the existing native acceptance owner: %#v", task)
	}
	if _, present := configuration.Tools["github:sharkdp/hyperfine"]; present {
		t.Fatal("a task-specific measurement tool became a mandatory general tool")
	}
}

type miseConfiguration struct {
	Tools    map[string]any `toml:"tools"`
	Settings struct {
		LegacyVersionFile      *bool `toml:"legacy_version_file"`
		NotFoundSystemFallback *bool `toml:"not_found_system_fallback"`
		UseVersionsHost        *bool `toml:"use_versions_host"`
	} `toml:"settings"`
}

func configuredMiseToolVersion(t *testing.T, tools map[string]any, name string) string {
	t.Helper()
	switch declaration := tools[name].(type) {
	case nil:
		return ""
	case string:
		return declaration
	case map[string]any:
		version, ok := declaration["version"].(string)
		if !ok || version == "" {
			t.Fatalf("mise tool %q lacks a version: %#v", name, declaration)
		}
		return version
	default:
		t.Fatalf("mise tool %q has an unsupported declaration: %T", name, declaration)
		return ""
	}
}

func TestMiseGoEnvironmentIsBoundToThisRepository(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration miseConfiguration
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	userEnvironment := filepath.Join(t.TempDir(), "go-env")
	const userSettings = "GOFLAGS=-modfile=foreign.mod\n"
	if err := os.WriteFile(userEnvironment, []byte(userSettings), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", userEnvironment)
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", filepath.Join(t.TempDir(), "foreign.work"))
	t.Setenv("GOTOOLCHAIN", "auto")
	command := exec.Command("mise", "-C", root, "exec", "--locked", "--", "go", "env", "-json", "GOENV", "GOWORK", "GOTOOLCHAIN", "GOFLAGS", "GOMOD", "GOVERSION")
	// The assertion exercises repository [env]; Mise safe mode deliberately
	// suppresses that declaration in an outer proof runner.
	command.Env = append(os.Environ(), "MISE_SAFE=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("observe repository Go environment: %v\n%s", err, output)
	}
	var observed map[string]string
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		// go env reports the resolved file path, which is empty when disabled.
		"GOENV":       "",
		"GOWORK":      "off",
		"GOTOOLCHAIN": "local",
		"GOFLAGS":     "",
		"GOMOD":       filepath.Join(root, "go.mod"),
		"GOVERSION":   "go" + configuredMiseToolVersion(t, configuration.Tools, "go"),
	} {
		if got := observed[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if content, err := os.ReadFile(userEnvironment); err != nil || string(content) != userSettings {
		t.Fatalf("repository tooling modified user settings: %q, %v", content, err)
	}
	if _, err := os.Stat(os.Getenv("GOWORK")); !os.IsNotExist(err) {
		t.Fatalf("repository tooling materialized a foreign workspace: %v", err)
	}
}

type ethosProfile struct {
	Proof struct {
		Gates []struct {
			ID            string   `toml:"id"`
			Command       []string `toml:"command"`
			NetworkPolicy string   `toml:"network_policy"`
			WritesFiles   bool     `toml:"writes_files"`
		} `toml:"gates"`
	} `toml:"proof"`
}

func TestMiseConfigurationKeepsToolResolutionRepositoryBound(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration miseConfiguration
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	for name, setting := range map[string]*bool{
		"legacy_version_file":       configuration.Settings.LegacyVersionFile,
		"not_found_system_fallback": configuration.Settings.NotFoundSystemFallback,
		"use_versions_host":         configuration.Settings.UseVersionsHost,
	} {
		if setting == nil || *setting {
			t.Errorf("mise setting %s must be explicitly false", name)
		}
	}
}

func TestMiseTasksDelegateToCanonicalOwners(t *testing.T) {
	root := repositoryRoot(t)
	command := exec.Command("mise", "-C", root, "tasks", "ls", "--json")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list mise tasks: %v", err)
	}
	var listed []miseTask
	if err := json.Unmarshal(output, &listed); err != nil {
		t.Fatalf("decode mise tasks: %v", err)
	}
	got := make(map[string][]string, len(listed))
	for _, task := range listed {
		got[task.Name] = task.Run
	}
	want := map[string][]string{
		"bootstrap": {
			"mise install --locked",
			"mise exec --locked -- go mod tidy -diff",
			"mise exec --locked -- npm ci --include=dev --ignore-scripts",
		},
		"check":   {"go run ./tools/ci source"},
		"native":  {"go run ./tools/ci native"},
		"release": {"go run ./tools/release build dist"},
		"dependencies:resolve": {
			"git diff --exit-code HEAD -- mise.toml mise.lock .mise/locks",
			"mise lock --platform {{ os() }}-{{ arch() }}",
			"git diff --exit-code HEAD -- mise.toml mise.lock .mise/locks",
			"mise lock --platform {{ os() }}-{{ arch() }}",
			"git diff --exit-code HEAD -- mise.toml mise.lock .mise/locks",
		},
	}
	for name, commands := range want {
		if !reflect.DeepEqual(got[name], commands) {
			t.Errorf("mise task %s = %#v, want %#v", name, got[name], commands)
		}
	}
}

func TestBootstrapRejectsAmbientGoWithEmptyToolCache(t *testing.T) {
	root := repositoryRoot(t)
	isolated := t.TempDir()
	t.Setenv("MISE_DATA_DIR", filepath.Join(isolated, "mise-data"))
	t.Setenv("MISE_CACHE_DIR", filepath.Join(isolated, "mise-cache"))
	t.Setenv("MISE_CONFIG_DIR", filepath.Join(isolated, "mise-config"))
	t.Setenv("MISE_OFFLINE", "true")
	t.Setenv("MISE_AUTO_INSTALL", "false")
	t.Setenv("MISE_TASK_RUN_AUTO_INSTALL", "true")
	marker := filepath.Join(isolated, "ambient-go-called")
	t.Setenv("AIGW_AMBIENT_GO_MARKER", marker)
	shims := filepath.Join(isolated, "shims")
	if err := os.Mkdir(shims, 0o700); err != nil {
		t.Fatal(err)
	}
	name, content := "go", "#!/bin/sh\nprintf 'called\\n' >> \"$AIGW_AMBIENT_GO_MARKER\"\nexit 99\n"
	if runtime.GOOS == "windows" {
		name = "go.cmd"
		content = "@echo off\r\necho called>>\"%AIGW_AMBIENT_GO_MARKER%\"\r\nexit /b 99\r\n"
	}
	if err := os.WriteFile(filepath.Join(shims, name), []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shims+string(os.PathListSeparator)+os.Getenv("PATH"))
	if output, err := exec.Command("mise", "-C", root, "which", "go").CombinedOutput(); err == nil {
		t.Fatalf("isolated mise unexpectedly resolved Go: %s", output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "mise", "-C", root, "run", "--locked", "bootstrap").CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("isolated bootstrap exceeded deadline: %v\n%s", ctx.Err(), output)
	}
	if err == nil {
		t.Fatalf("offline bootstrap unexpectedly succeeded with an empty tool cache: %s", output)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("bootstrap executed ambient Go instead of rejecting missing locked tools: %s", output)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestOfflineDependencyTasksNeverPullAnImage(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	type dependencyTask struct {
		Run     string `toml:"run"`
		Timeout string `toml:"timeout"`
	}
	var configuration struct {
		Tasks struct {
			Check   dependencyTask `toml:"dependencies:check"`
			Inspect dependencyTask `toml:"dependencies:inspect"`
		} `toml:"tasks"`
	}
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		task dependencyTask
	}{
		{name: "dependencies:check", task: configuration.Tasks.Check},
		{name: "dependencies:inspect", task: configuration.Tasks.Inspect},
	} {
		if !strings.Contains(test.task.Run, "docker run --pull=never --rm --read-only --network none") || test.task.Timeout != "2m" {
			t.Errorf("%s must use a cached image without network or an unbounded wait: %#v", test.name, test.task)
		}
	}
}

func TestEthosProofDelegatesStaticAnalysisToQualityOwner(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, ".ethos", "profile.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var profile ethosProfile
	if err := toml.Unmarshal(content, &profile); err != nil {
		t.Fatal(err)
	}

	want := []string{"mise", "exec", "--locked", "--", "go", "run", "./tools/ci", "quality"}
	found := false
	for _, gate := range profile.Proof.Gates {
		if (gate.ID == "go-behavior" || gate.ID == "go-static-analysis") && !gate.WritesFiles {
			t.Errorf("gate %q omits its verification-output and temporary-file writes", gate.ID)
		}
		if gate.ID == "go-static-analysis" {
			found = true
			if !reflect.DeepEqual(gate.Command, want) {
				t.Fatalf("go-static-analysis command = %#v, want %#v", gate.Command, want)
			}
			if gate.NetworkPolicy != "required" {
				t.Errorf("quality includes npm signature and OSV checks but declares network policy %q", gate.NetworkPolicy)
			}
		}
	}
	if !found {
		t.Fatal("go-static-analysis proof gate is missing")
	}
}

func TestNotarizationUsesBoundedNativeTask(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration struct {
		Tasks struct {
			Notary struct {
				Run     string `toml:"run"`
				Timeout string `toml:"timeout"`
				Raw     bool   `toml:"raw"`
			} `toml:"release:notary"`
		} `toml:"tasks"`
	}
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	task := configuration.Tasks.Notary
	if task.Run != "xcrun notarytool" || task.Timeout != "5m" || !task.Raw {
		t.Fatalf("notarization must delegate to Apple's command with bounded native execution: %#v", task)
	}
}
