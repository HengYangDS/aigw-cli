package main

import (
	"debug/buildinfo"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestMiseOSVScannerUsesRepositoryCompiler(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration miseConfiguration
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("mise", "-C", root, "which", "osv-scanner").CombinedOutput()
	if err != nil {
		t.Fatalf("resolve locked scanner: %v\n%s", err, output)
	}
	info, err := buildinfo.ReadFile(strings.TrimSpace(string(output)))
	if err != nil {
		t.Fatalf("read scanner compiler identity: %v", err)
	}
	if want := "go" + configuration.Tools["go"]; info.GoVersion != want {
		t.Fatalf("scanner compiler = %s, want %s; rebuild with mise install --force go:github.com/google/osv-scanner/v2/cmd/osv-scanner", info.GoVersion, want)
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
		Tools map[string]string `toml:"tools"`
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
	if configuration.Tools["github:sharkdp/hyperfine"] != "" {
		t.Fatal("a task-specific measurement tool became a mandatory general tool")
	}
}

type miseConfiguration struct {
	Tools    map[string]string `toml:"tools"`
	Settings struct {
		LegacyVersionFile      *bool `toml:"legacy_version_file"`
		NotFoundSystemFallback *bool `toml:"not_found_system_fallback"`
	} `toml:"settings"`
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
		"GOVERSION":   "go" + configuration.Tools["go"],
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

func TestMiseConfigurationRejectsAmbientToolFallbacks(t *testing.T) {
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
		"bootstrap": {"go mod tidy -diff", "npm ci --include=dev --ignore-scripts"},
		"check":     {"go run ./tools/ci source"},
		"native":    {"go run ./tools/ci native"},
		"release":   {"go run ./tools/release build dist"},
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
