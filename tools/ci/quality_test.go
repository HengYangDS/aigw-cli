package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestRepositoryQualityGraphCoversTrackedCarrierAuthority(t *testing.T) {
	root := repositoryRoot(t)
	classes, err := loadTrackedCarrierClasses(filepath.Join(root, ".config", "checks", "architecture", "policy.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateQualityGraph(repositoryQualityGraph, classes); err != nil {
		t.Fatal(err)
	}
}

func TestQualityGraphRejectsIncompleteOrParallelCoverage(t *testing.T) {
	valid := repositoryQualityGraph
	valid.Carriers = []carrierQuality{{Class: "source", Required: []qualityConcern{qualityFormat, qualityTest}, Gates: []string{"format", "coverage"}}}
	tests := []struct {
		name    string
		graph   qualityGraph
		classes []string
		want    string
	}{
		{"missing carrier", valid, []string{"source", "documentation"}, "missing carrier quality coverage"},
		{"unknown carrier", valid, nil, "unknown carrier quality coverage"},
		{"unknown gate", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: []carrierQuality{{Class: "source", Required: []qualityConcern{qualityFormat}, Gates: []string{"missing"}}}}, []string{"source"}, "unknown quality gate"},
		{"missing concern", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: []carrierQuality{{Class: "source", Required: []qualityConcern{qualityType}, Gates: []string{"format"}}}}, []string{"source"}, "missing required quality concern"},
		{"unused gate", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: []carrierQuality{{Class: "source", Required: []qualityConcern{qualityFormat}, Gates: []string{"format"}}}}, []string{"source"}, "quality gate has no carrier consumer"},
		{"duplicate gate", qualityGraph{Gates: append(slices.Clone(valid.Gates), valid.Gates[0]), CommonGates: valid.CommonGates, Carriers: valid.Carriers}, []string{"source"}, "duplicate quality gate"},
		{"duplicate carrier", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: append(slices.Clone(valid.Carriers), valid.Carriers[0])}, []string{"source"}, "duplicate carrier quality coverage"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateQualityGraph(test.graph, test.classes); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("quality graph error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestConfiguredQualityUsesTheArchitectureCarrierAuthority(t *testing.T) {
	root := t.TempDir()
	policy := filepath.Join(root, ".config", "checks", "architecture", "policy.toml")
	write := func(content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(policy, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("[tracked_carrier_classes.unknown]\nresponsibility = 'fixture'\nprefixes = ['fixture']\n")
	t.Chdir(root)
	t.Setenv("AIGW_COMMIT_BASE", "")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	if _, err := configuredQualityCommands(root); err == nil || !strings.Contains(err.Error(), "unknown carrier quality coverage") {
		t.Fatalf("quality coverage error = %v", err)
	}
}

func TestRepositoryQualityGraphIncludesStableGenericChecks(t *testing.T) {
	for _, gate := range []struct {
		id   string
		name string
	}{
		{"spelling", "go"},
		{"workflow-lint", "actionlint"},
	} {
		if !slices.ContainsFunc(repositoryQualityGraph.Gates, func(candidate qualityGate) bool {
			return candidate.ID == gate.id && candidate.Command.Name == gate.name
		}) {
			t.Fatalf("quality graph lacks %s through %s", gate.id, gate.name)
		}
	}
}

func TestRepositoryQualityGraphOwnsTheArchitecturePublisherEdition(t *testing.T) {
	gateIndex := slices.IndexFunc(repositoryQualityGraph.Gates, func(gate qualityGate) bool {
		return gate.ID == "architecture-edition-source"
	})
	if gateIndex < 0 {
		t.Fatal("quality graph lacks the source-owned Architecture Publisher Edition gate")
	}
	gate := repositoryQualityGraph.Gates[gateIndex]
	if gate.Command.Name != "node" || !slices.Equal(gate.Command.Args, []string{
		"--test",
		"integrations/architecture-publisher/test/source-ownership.test.mjs",
	}) {
		t.Fatalf("architecture Edition gate = %#v", gate.Command)
	}
	carrierIndex := slices.IndexFunc(repositoryQualityGraph.Carriers, func(carrier carrierQuality) bool {
		return carrier.Class == "architecture-publisher-integration"
	})
	if carrierIndex < 0 || !slices.Contains(
		repositoryQualityGraph.Carriers[carrierIndex].Gates,
		"architecture-edition-source",
	) {
		t.Fatal("architecture Edition carrier is not bound to its source gate")
	}
}

func TestSpellingUsesCurrentCheckoutAndNativePolicy(t *testing.T) {
	repository := repositoryRoot(t)
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".config/checks/spelling/policy.toml", string(readFile(t, filepath.Join(repository, ".config", "checks", "spelling", "policy.toml"))))
	write(".gitignore", "ignored.md\n")
	misspelling := string([]byte{'t', 'e', 'h'})
	write("README.md", "This sentence contains "+misspelling+" defect.\n")
	write(".config/current.toml", "description = '"+misspelling+" hidden defect'\n")
	write("ignored.md", "This file contains "+misspelling+" ignored defect.\n")
	write("openspec/changes/archive/2026-01-01-history/tasks.md", string([]byte{'b', 'a'})+"\n")
	if output, err := exec.Command("git", "-C", root, "add", ".config", ".gitignore", "README.md", "openspec").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	var diagnostics []byte
	err := checkSpelling(root, func(call command) error {
		var err error
		diagnostics, err = systemOutputRunner(call)
		return err
	})
	if err == nil || !strings.Contains(string(diagnostics), "README.md") || !strings.Contains(string(diagnostics), ".config/current.toml") {
		t.Fatalf("spelling error = %v\n%s", err, diagnostics)
	}
	write("README.md", "This sentence is correct.\n")
	write(".config/current.toml", "description = 'current hidden configuration'\n")
	if err := checkSpelling(root, systemRunner); err != nil {
		t.Fatalf("spelling rejected current clean sources: %v", err)
	}
}

func TestWorkflowLintUsesShellCheck(t *testing.T) {
	workflow := filepath.Join(t.TempDir(), "workflow.yml")
	content := "name: shellcheck-probe\non: push\njobs:\n  probe:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo $UNQUOTED\n"
	if err := os.WriteFile(workflow, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("actionlint", "-oneline", workflow).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "[shellcheck]") {
		t.Fatalf("actionlint did not execute ShellCheck: %v\n%s", err, output)
	}
}

func TestPortableQualityToolsHaveCompletePlatformLocks(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "mise.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Tools map[string][]map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(content, &lock); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"platforms.linux-arm64",
		"platforms.linux-x64",
		"platforms.macos-arm64",
		"platforms.macos-x64",
		"platforms.windows-arm64",
		"platforms.windows-x64",
	}
	for _, tool := range []string{"shellcheck", "typos"} {
		entries := lock.Tools[tool]
		if len(entries) != 1 {
			t.Fatalf("%s lock entries = %d, want one", tool, len(entries))
		}
		var got []string
		for key := range entries[0] {
			if strings.HasPrefix(key, "platforms.") {
				got = append(got, key)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("%s platform locks = %q, want %q", tool, got, want)
		}
	}
}
