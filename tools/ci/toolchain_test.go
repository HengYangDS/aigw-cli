package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

type miseToolProbe struct {
	arguments []string
	version   string
}

var miseToolProbes = map[string]miseToolProbe{
	"go":                           {[]string{"go", "version"}, `^go version go(\S+)`},
	"node":                         {[]string{"node", "--version"}, `^v(\S+)`},
	"npm":                          {[]string{"npm", "--version"}, `^(\S+)`},
	"cue":                          {[]string{"cue", "version"}, `^cue version v(\S+)`},
	"gh":                           {[]string{"gh", "--version"}, `^gh version (\S+)`},
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
	"shellcheck":                                       {[]string{"shellcheck", "--version"}, `(?m)^version: (\S+)`},
	"typos":                                            {[]string{"typos", "--version"}, `^typos-cli (\S+)`},
	"github:indygreg/apple-platform-rs[version_prefix=apple-codesign/]": {[]string{"rcodesign", "--version"}, `^apple-codesign (\S+)`},
}

func miseToolEnabled(name string) bool {
	enabled := strings.TrimSpace(os.Getenv("MISE_ENABLE_TOOLS"))
	if enabled == "" {
		return true
	}
	name, _, _ = strings.Cut(name, "[")
	return slices.Contains(strings.Split(enabled, ","), name)
}

func miseCommandEnabled(name string) bool {
	for tool, probe := range miseToolProbes {
		if probe.arguments[0] == name {
			return miseToolEnabled(tool)
		}
	}
	return true
}

func requireMiseTool(t *testing.T, name string) {
	t.Helper()
	if !miseToolEnabled(name) {
		t.Skipf("%s is outside the active platform toolchain", name)
	}
}

func TestMiseToolSelectionHonorsTheActiveToolchain(t *testing.T) {
	t.Setenv("MISE_ENABLE_TOOLS", "go,github:golangci/golangci-lint")
	for name, want := range map[string]bool{
		"go":                            true,
		"github:golangci/golangci-lint": true,
		"github:indygreg/apple-platform-rs[version_prefix=x/]": false,
		"github:lycheeverse/lychee":                            false,
	} {
		if got := miseToolEnabled(name); got != want {
			t.Errorf("mise tool %q enabled = %t, want %t", name, got, want)
		}
	}
}

func TestMiseToolExecutablesMatchDeclaredVersions(t *testing.T) {
	// Version observation uses test-owned CLI configuration.
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GLAB_CONFIG_DIR", t.TempDir())
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration miseConfiguration
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	for name, declared := range configuration.Tools {
		if !miseToolEnabled(name) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			probe, present := miseToolProbes[name]
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
	for name := range miseToolProbes {
		if _, declared := configuration.Tools[name]; !declared {
			t.Errorf("executable probe %s has no declared consumer", name)
		}
	}
}
