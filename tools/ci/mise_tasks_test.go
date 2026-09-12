package main

import (
	"bytes"
	"debug/buildinfo"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
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

type miseConfiguration struct {
	Tools    map[string]string `toml:"tools"`
	Settings struct {
		LegacyVersionFile      *bool `toml:"legacy_version_file"`
		NotFoundSystemFallback *bool `toml:"not_found_system_fallback"`
	} `toml:"settings"`
}

func TestMiseToolExecutablesMatchDeclaredVersions(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration miseConfiguration
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	probes := map[string]struct {
		arguments []string
		version   string
	}{
		"go":                           {[]string{"go", "version"}, `^go version go(\S+)`},
		"node":                         {[]string{"node", "--version"}, `^v(\S+)`},
		"cue":                          {[]string{"cue", "version"}, `^cue version v(\S+)`},
		"glab":                         {[]string{"glab", "--version"}, `^glab (\S+)`},
		"github:goreleaser/goreleaser": {[]string{"goreleaser", "--version"}, `(?m)^GitVersion:\s+(\S+)`},
		"github:anchore/syft":          {[]string{"syft", "version"}, `(?m)^Version:\s+(\S+)`},
		"go:github.com/google/osv-scanner/v2/cmd/osv-scanner": {[]string{"osv-scanner", "--version"}, `^osv-scanner version: (\S+)`},
		"taplo":             {[]string{"taplo", "--version"}, `^taplo (\S+)`},
		"github:boyter/scc": {[]string{"scc", "--version"}, `^scc version (\S+)`},
		"github:editorconfig-checker/editorconfig-checker": {[]string{"editorconfig-checker", "--version"}, `^v(\S+)`},
		"github:gitleaks/gitleaks":                         {[]string{"gitleaks", "version"}, `^(\S+)`},
		"github:golangci/golangci-lint":                    {[]string{"golangci-lint", "version"}, `^golangci-lint has version (\S+)`},
		"github:rhysd/actionlint":                          {[]string{"actionlint", "--version"}, `^(\S+)`},
		"github:lycheeverse/lychee":                        {[]string{"lychee", "--version"}, `^lychee (\S+)`},
	}
	for name, declared := range configuration.Tools {
		t.Run(name, func(t *testing.T) {
			probe, present := probes[name]
			if !present {
				t.Fatalf("declared tool %s has no executable version probe", name)
			}
			command := exec.Command("mise", append([]string{"-C", root, "exec", "--locked", "--"}, probe.arguments...)...)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("execute locked tool %s: %v\n%s", name, err, output)
			}
			observed := regexp.MustCompile(probe.version).FindStringSubmatch(strings.TrimSpace(string(output)))
			want := strings.TrimPrefix(strings.TrimPrefix(declared, "lychee-"), "v")
			if len(observed) != 2 || observed[1] != want {
				t.Fatalf("tool %s version does not match %s: %s", name, declared, output)
			}
		})
	}
	for name := range probes {
		if _, declared := configuration.Tools[name]; !declared {
			t.Errorf("executable probe %s has no declared consumer", name)
		}
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
			"git diff --exit-code HEAD -- mise.toml mise.lock",
			"mise lock --platform {{ os() }}-{{ arch() }}",
			"git diff --exit-code HEAD -- mise.toml mise.lock",
			"mise lock --platform {{ os() }}-{{ arch() }}",
			"git diff --exit-code HEAD -- mise.toml mise.lock",
		},
	}
	for name, commands := range want {
		if !reflect.DeepEqual(got[name], commands) {
			t.Errorf("mise task %s = %#v, want %#v", name, got[name], commands)
		}
	}
}

// isolatedDevelopmentCheckout snapshots current source and lock inputs, not another lane's environment.
func isolatedDevelopmentCheckout(t *testing.T, repository string) (string, map[string][]byte) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	inputs := map[string][]byte{}
	for _, name := range []string{".config/miserc.toml", "mise.toml", "mise.lock", "go.mod", "go.sum", "package.json", "package-lock.json"} {
		content, err := os.ReadFile(filepath.Join(repository, name))
		if err != nil {
			t.Fatal(err)
		}
		inputs[name] = content
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := currentRepositoryFiles(repository, "Go source", "*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		relative, err := filepath.Rel(repository, file)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, inputs
}

func TestMiseBootstrapReconstructsCheckoutLocalPackages(t *testing.T) {
	repository := repositoryRoot(t)
	root, inputs := isolatedDevelopmentCheckout(t, repository)
	var configuration miseConfiguration
	if err := toml.Unmarshal(inputs["mise.toml"], &configuration); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Dependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(inputs["package.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Dependencies) == 0 {
		t.Fatal("repository bootstrap has no declared package consumers")
	}
	cacheCommand := exec.Command("mise", "-C", repository, "exec", "--locked", "--", "npm", "config", "get", "cache")
	cacheOutput, err := cacheCommand.Output()
	if err != nil {
		t.Fatalf("resolve populated npm cache: %v", err)
	}
	cache := strings.TrimSpace(string(cacheOutput))
	if !filepath.IsAbs(cache) {
		t.Fatalf("npm cache must resolve to an absolute directory: %q", cache)
	}
	for name, value := range map[string]string{
		"NPM_CONFIG_CACHE":           cache,
		"NPM_CONFIG_OFFLINE":         "true",
		"NPM_CONFIG_AUDIT":           "false",
		"NPM_CONFIG_FUND":            "false",
		"NPM_CONFIG_UPDATE_NOTIFIER": "false",
		"MISE_TRUSTED_CONFIG_PATHS":  root,
		"MISE_CONFIG_DIR":            t.TempDir(),
		"MISE_GLOBAL_CONFIG_FILE":    filepath.Join(t.TempDir(), "config.toml"),
		"MISE_SYSTEM_CONFIG_DIR":     t.TempDir(),
		"GOPROXY":                    "off",
		"GOSUMDB":                    "off",
	} {
		t.Setenv(name, value)
	}
	for _, variable := range []string{"NPM_CONFIG_USERCONFIG", "NPM_CONFIG_GLOBALCONFIG"} {
		file := filepath.Join(t.TempDir(), "npmrc")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(variable, file)
	}
	residue := filepath.Join(root, "node_modules", ".bootstrap-residue")
	for pass, environment := range []struct{ nodeEnv, omit string }{
		{"production", ""},
		{"", "dev"},
	} {
		t.Setenv("NODE_ENV", environment.nodeEnv)
		t.Setenv("NPM_CONFIG_OMIT", environment.omit)
		for _, arguments := range [][]string{
			{"exec", "--locked", "--", "go", "mod", "tidy"},
			{"exec", "--locked", "--", "npm", "install", "--package-lock-only", "--ignore-scripts"},
			{"run", "bootstrap"},
		} {
			command := exec.Command("mise", append([]string{"-C", root}, arguments...)...)
			command.Dir = root
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("resolution/bootstrap pass %d, %v: %v\n%s", pass+1, arguments, err, output)
			}
		}
		if _, err := os.Stat(residue); !os.IsNotExist(err) {
			t.Fatalf("bootstrap retained previous installation residue: %v", err)
		}
		for name, want := range manifest.Dependencies {
			content := readFile(t, filepath.Join(root, "node_modules", filepath.FromSlash(name), "package.json"))
			var installed struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(content, &installed); err != nil || installed.Version != want {
				t.Fatalf("installed %s = %q, want %q: %v", name, installed.Version, want, err)
			}
		}
		for name, probe := range map[string]struct {
			arguments []string
			version   string
			pattern   string
		}{
			"node": {[]string{"node", "--version"}, configuration.Tools["node"], `^v(\S+)$`},
			"@fission-ai/openspec": {
				[]string{"node", filepath.Join(root, "node_modules", "@fission-ai", "openspec", "bin", "openspec.js"), "--version"},
				manifest.Dependencies["@fission-ai/openspec"], `^(\S+)$`,
			},
			"markdownlint-cli2": {
				[]string{"node", filepath.Join(root, "node_modules", "markdownlint-cli2", "markdownlint-cli2-bin.mjs"), "-"},
				manifest.Dependencies["markdownlint-cli2"], `^markdownlint-cli2 v(\S+)`,
			},
			"prettier": {
				[]string{"node", filepath.Join(root, "node_modules", "prettier", "bin", "prettier.cjs"), "--version"},
				manifest.Dependencies["prettier"], `^(\S+)$`,
			},
		} {
			if probe.version == "" {
				t.Fatalf("executable version probe has no declared dependency: %s", name)
			}
			command := exec.Command("mise", append([]string{"-C", root, "exec", "--locked", "--"}, probe.arguments...)...)
			command.Stdin = strings.NewReader("# Bootstrap\n")
			output, err := command.CombinedOutput()
			identity := regexp.MustCompile(probe.pattern).FindStringSubmatch(strings.TrimSpace(string(output)))
			if err != nil || len(identity) != 2 || identity[1] != probe.version {
				t.Fatalf("executable %s version mismatch, want %s: %v\n%s", name, probe.version, err, output)
			}
		}
		for name, want := range inputs {
			if got := readFile(t, filepath.Join(root, name)); !bytes.Equal(got, want) {
				t.Fatalf("resolution, bootstrap or tool execution changed %s", name)
			}
		}
		if pass == 0 {
			if err := os.WriteFile(residue, []byte("previous installation"), 0o600); err != nil {
				t.Fatal(err)
			}
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
