package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		{"workflow-lint", "go"},
	} {
		if !slices.ContainsFunc(repositoryQualityGraph.Gates, func(candidate qualityGate) bool {
			return candidate.ID == gate.id && candidate.Command.Name == gate.name
		}) {
			t.Fatalf("quality graph lacks %s through %s", gate.id, gate.name)
		}
	}
}

func TestRepositoryQualityGraphIncludesNativePerformanceCases(t *testing.T) {
	index := slices.IndexFunc(repositoryQualityGraph.Gates, func(gate qualityGate) bool {
		return gate.ID == "performance-acceptance"
	})
	if index < 0 {
		t.Fatal("source admission lacks its native performance owner")
	}
	args := repositoryQualityGraph.Gates[index].Command.Args
	index = slices.Index(args, "-run")
	if index < 0 || index+1 >= len(args) {
		t.Fatalf("performance acceptance lacks its bounded test selection: %v", args)
	}
	selection, err := regexp.Compile(args[index+1])
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{
		"TestNativePeakMemoryBudget":                               true,
		"TestNativePerformanceCases":                               true,
		"TestNativeAttributionKeepsTheProjectedReaderBoundary":     true,
		"TestNativeAttributionSummaryRetainsItsNonqualifyingScope": true,
		"TestNativeMemoryRetainsInterruptedObservations":           true,
		"TestNativePerformance":                                    false,
	} {
		if got := selection.MatchString(name); got != want {
			t.Fatalf("performance source admission selects %s = %t, want %t", name, got, want)
		}
	}
}

func TestRepositoryQualityGraphOwnsTheArchitectureEditionProvider(t *testing.T) {
	gateIndex := slices.IndexFunc(repositoryQualityGraph.Gates, func(gate qualityGate) bool {
		return gate.ID == "node-source"
	})
	if gateIndex < 0 {
		t.Fatal("quality graph lacks the source-owned Architecture Edition Provider gate")
	}
	gate := repositoryQualityGraph.Gates[gateIndex]
	if gate.Command.Name != "node" || !slices.Equal(gate.Command.Args, []string{
		"--test",
		"architecture/edition-provider/test/source.test.mjs",
		"tools/ci/test/text.test.mjs",
	}) {
		t.Fatalf("architecture Edition Provider gate = %#v", gate.Command)
	}
	carrierIndex := slices.IndexFunc(repositoryQualityGraph.Carriers, func(carrier carrierQuality) bool {
		return carrier.Class == "architecture-edition-provider"
	})
	if carrierIndex < 0 || !slices.Contains(
		repositoryQualityGraph.Carriers[carrierIndex].Gates,
		"node-source",
	) {
		t.Fatal("architecture Edition Provider carrier is not bound to its source gate")
	}
}

func TestSpellingUsesCurrentCheckoutAndNativePolicy(t *testing.T) {
	repository := repositoryRoot(t)
	root := newGitRepository(t)
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
	call := exec.CommandContext(t.Context(), "cue", "export", ".config/ci/pipeline.cue", ".ethos/workspace.toml", "--expression", "toolchainTools.native", "--out", "json")
	call.Dir = repositoryRoot(t)
	output, err := call.Output()
	if err != nil {
		t.Fatal(err)
	}
	var tools []string
	if err := json.Unmarshal(output, &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) == 0 {
		t.Fatal("native toolchain must declare its portable tools")
	}
	for _, tool := range tools {
		for _, platform := range want {
			matches := 0
			for _, entry := range lock.Tools[tool] {
				_, native := entry[platform]
				packageLock, _ := entry["aube"].(map[string]any)
				path, _ := packageLock["path"].(string)
				digest, _ := packageLock["digest"].(string)
				packageLocked := entry["backend"] == "npm:npm" && path != "" && strings.HasPrefix(digest, "sha256:")
				if native || packageLocked {
					matches++
				}
			}
			if matches != 1 {
				t.Errorf("%s %s locks = %d, want one native or platform-independent package lock", tool, platform, matches)
			}
		}
	}
}

func TestWorkflowGateResourceBoundaryIsNativeAndExact(t *testing.T) {
	root := newGitRepository(t)
	file := filepath.Join(root, ".github", "workflows", "probe.yml")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	valid := "name: probe\non: push\ndefaults:\n  run:\n    shell: bash\njobs:\n  probe:\n    runs-on: ubuntu-latest\n    steps:\n      - run: printf native\n"
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(valid)
	problem := errors.New("native workflow validator failed")
	if err := checkWorkflows(root, func(command) error { return problem }); !errors.Is(err, problem) {
		t.Fatalf("native validator error was not propagated: %v", err)
	}
	write(valid)
	block := filepath.Join(root, "build")
	if err := os.WriteFile(block, []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkWorkflows(root, func(command) error { return nil }); err == nil {
		t.Fatal("unavailable evidence parent accepted")
	}
	if err := os.Remove(block); err != nil {
		t.Fatal(err)
	}
	write("jobs: [\n")
	if err := checkWorkflows(root, func(command) error { return nil }); err == nil {
		t.Fatal("malformed native YAML accepted")
	}
	write(strings.ReplaceAll(valid, "shell: bash", "shell: sh"))
	if err := checkWorkflows(root, systemRunner); err != nil {
		t.Fatalf("native sh workflow rejected: %v", err)
	}
}
