package main

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

type ethosProfile struct {
	Proof struct {
		CodeCorrectnessGates []string          `toml:"code_correctness_gates"`
		CodeCorrectnessMap   map[string]string `toml:"code_correctness_map"`
		Gates                []struct {
			ID                    string   `toml:"id"`
			Command               []string `toml:"command"`
			Providers             []string `toml:"providers"`
			VerificationProviders []string `toml:"verification_providers"`
			DependsOn             []string `toml:"depends_on"`
			NetworkPolicy         string   `toml:"network_policy"`
			WritesFiles           bool     `toml:"writes_files"`
			NativeOutputs         []struct {
				Path     string   `toml:"path"`
				Format   string   `toml:"format"`
				Sessions []string `toml:"sessions"`
			} `toml:"native_outputs"`
		} `toml:"gates"`
	} `toml:"proof"`
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

// lockedExecution is how every proof gate reaches the repository's locked
// toolchain.
var lockedExecution = []string{"mise", "exec", "--locked", "--"}

const behaviorVerifier = "ethos.adapters.gates.code_quality:behavior_report"

func readEthosProfile(t *testing.T) ethosProfile {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".ethos", "profile.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var profile ethosProfile
	if err := toml.Unmarshal(content, &profile); err != nil {
		t.Fatal(err)
	}
	return profile
}

func qualityGraphCommand(t *testing.T, id string) command {
	t.Helper()
	index := slices.IndexFunc(repositoryQualityGraph.Gates, func(gate qualityGate) bool { return gate.ID == id })
	if index < 0 {
		t.Fatalf("quality graph has no %q gate", id)
	}
	return repositoryQualityGraph.Gates[index].Command
}

// testFiles returns a command's operands, the arguments that are not flags.
func testFiles(arguments []string) []string {
	var files []string
	for _, argument := range arguments {
		if !strings.HasPrefix(argument, "-") {
			files = append(files, argument)
		}
	}
	slices.Sort(files)
	return files
}

func TestEthosProofJudgesEachNativeTestAttemptOnce(t *testing.T) {
	profile := readEthosProfile(t)
	if profile.Proof.CodeCorrectnessMap["behavior"] != "go-behavior" {
		t.Error("behavior must be mapped to the Go coverage attempt")
	}
	reports := map[string][]string{}
	for _, gate := range profile.Proof.Gates {
		// A behaviour provider without the gate's own reports runs the tests again.
		if slices.Contains(gate.Providers, behaviorVerifier) {
			t.Errorf("gate %q asks ETHOS to run the tests itself", gate.ID)
		}
		if !slices.Contains(gate.VerificationProviders, behaviorVerifier) {
			continue
		}
		if !slices.Contains(profile.Proof.CodeCorrectnessGates, gate.ID) || len(gate.VerificationProviders) != 1 ||
			len(gate.Providers) != 0 || len(gate.DependsOn) != 0 || !gate.WritesFiles ||
			!slices.Equal(gate.Command[:min(len(lockedExecution), len(gate.Command))], lockedExecution) {
			t.Errorf("gate %q must be one locked native attempt that ETHOS judges: %#v", gate.ID, gate)
		}
		formats := map[string]bool{}
		for _, output := range gate.NativeOutputs {
			formats[output.Format] = true
			written := "{native_output}/" + output.Path
			if !slices.ContainsFunc(gate.Command, func(argument string) bool { return strings.HasSuffix(argument, written) }) ||
				len(output.Sessions) != 1 {
				t.Errorf("gate %q declares %s output %q that its attempt does not write", gate.ID, output.Format, output.Path)
			}
		}
		if len(gate.NativeOutputs) != 2 || !formats["junit-xml"] || !formats["lcov"] {
			t.Errorf("gate %q must declare one JUnit and one LCOV report: %#v", gate.ID, gate.NativeOutputs)
		}
		reports[gate.ID] = gate.Command[min(len(lockedExecution), len(gate.Command)):]
	}
	requireGoAttempt(t, reports["go-behavior"])
	requireNodeAttempt(t, reports["node-behavior"])
	if len(reports) != 2 {
		t.Fatalf("behaviour gates judged from their reports: %v", slices.Sorted(maps.Keys(reports)))
	}
}

// requireGoAttempt requires the Go behaviour gate to be the quality graph's
// coverage command plus its two report flags.
func requireGoAttempt(t *testing.T, attempt []string) {
	t.Helper()
	coverage := qualityGraphCommand(t, "coverage")
	base := append([]string{coverage.Name}, coverage.Args...)
	if len(attempt) != len(base)+4 || !slices.Equal(attempt[:len(base)], base) ||
		attempt[len(base)] != "--junit-output" || attempt[len(base)+2] != "--lcov-output" {
		t.Fatalf("go-behavior must run the coverage gate's command with both reports: %q", attempt)
	}
}

// requireNodeAttempt requires the Node behaviour gate to run exactly the Node
// test files that the quality graph runs, and those to be every tracked one.
func requireNodeAttempt(t *testing.T, attempt []string) {
	t.Helper()
	source := qualityGraphCommand(t, "node-source")
	tracked, err := currentRepositoryFiles(repositoryRoot(t), "Node test", "*.test.mjs")
	if err != nil {
		t.Fatal(err)
	}
	if len(attempt) == 0 || attempt[0] != source.Name || !slices.Contains(attempt, "--test") {
		t.Fatalf("node-behavior must run Node's test runner: %q", attempt)
	}
	if behavior, graph := testFiles(attempt[1:]), testFiles(source.Args); !reflect.DeepEqual(behavior, graph) ||
		!reflect.DeepEqual(graph, tracked) {
		t.Fatalf("Node test files: node-behavior %q, node-source %q, tracked %q", behavior, graph, tracked)
	}
}
