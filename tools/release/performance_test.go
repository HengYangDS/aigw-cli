//go:build performance_acceptance

package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/native"
	"aigw-cli/internal/upgrade/artifact"
	"aigw-cli/tools/release/performance"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestNativePerformance(t *testing.T) {
	attribution := os.Getenv("AIGW_PERFORMANCE_ATTRIBUTION") == "1"
	if !attribution {
		TestNativePeakMemory(t)
	}
	output, candidateRoot := os.Getenv("AIGW_PERFORMANCE_OUTPUT"), os.Getenv("AIGW_ACCEPTANCE_RELEASE")
	if !filepath.IsAbs(output) || !filepath.IsAbs(candidateRoot) || os.Getenv("AIGW_ACCEPTANCE_BASELINE") == "" {
		t.Fatal("performance acceptance requires an absolute output, published candidate and explicit baseline")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatalf("performance output must be new: %v", err)
	}
	hyperfine, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	programs := nativePerformancePrograms(t)
	backends := []string{"env"}
	if runtime.GOOS == "linux" {
		backends = append(backends, "file")
	}
	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") == "1" {
		backends = append(backends, "keyring")
	}
	var measurements []performance.Measurement
	var memory []performance.Memory
	defer func() {
		writeNativePerformanceSummary(t, output, hyperfine, programs, measurements, memory, attribution)
	}()
	for block, order := range [][]int{{0, 1}, {1, 0}} {
		for _, index := range order {
			program := programs[index]
			for _, backend := range backends {
				complete := false
				t.Run(fmt.Sprintf("block-%d/%s/%s", block+1, program.Variant, backend), func(t *testing.T) {
					journey := nativePerformanceJourney(t, program.Path, programs[1].Path, backend)
					rows, err := journey.measurePerformance(hyperfine, output, program.Variant, backend, block+1)
					measurements = append(measurements, rows...)
					if err != nil {
						t.Error(err)
						return
					}
					before, sidecar := readFile(t, journey.settings), readFile(t, journey.settings+".aigw-state.json")
					journey.run("sync")
					if !bytes.Equal(before, readFile(t, journey.settings)) || !bytes.Equal(sidecar, readFile(t, journey.settings+".aigw-state.json")) {
						t.Fatal("performance journey changed a converged projection")
					}
					if backend == "env" && !attribution {
						row, err := journey.measureMemory(program.Variant, block+1)
						memory = append(memory, row)
						if err != nil {
							t.Error(err)
							return
						}
					}
					complete = true
				})
				if !complete {
					return
				}
			}
		}
	}
	if attribution {
		return
	}
}

func writeNativePerformanceSummary(t *testing.T, output, hyperfine string, programs []performance.Program, measurements []performance.Measurement, memory []performance.Memory, attribution bool) []performance.Measurement {
	t.Helper()
	identity, err := exec.CommandContext(t.Context(), hyperfine, "--version").Output()
	if err != nil {
		t.Error(err)
	}
	tool := strings.TrimSpace(string(identity))
	pooled, err := performance.Pooled(measurements)
	if err != nil {
		t.Error(err)
		pooled = []performance.Measurement{}
	}
	for _, row := range append(slices.Clone(measurements), pooled...) {
		t.Logf("%s/%s/%s block=%d p95=%.3fms budget=%.0fms", row.Variant, row.Backend, row.Case, row.Block, row.P95*1000, row.Budget*1000)
	}
	summary := performance.Summary{
		Scope: "full-performance", Controllers: make(map[string]performance.Identity), ControllerExecution: make(map[string][]performance.Execution),
		OS: runtime.GOOS, Arch: runtime.GOARCH, Tool: tool,
		IdentityScope: "Immutable selected files; Windows controller and descendant image, machine, creation time and parent ownership observed through untimed native create events and retained handles",
		MemoryScope:   "Configured status: per-child wait4 on macOS, GNU time on Linux, retained-handle peak working set on Windows; bytes, no periodic sampling; calibrated against a large parent",
		ClientScope:   "controlled client discovery; native projected helper; no Provider inference",
		Programs:      programs, Blocks: measurements, Pooled: pooled, Memory: memory,
	}
	current, err := performance.ObserveCurrent()
	if err != nil {
		t.Error(err)
	}
	summary.ControllerExecution["verifier"] = current
	for _, row := range measurements {
		if len(row.ControllerExecution) != 0 {
			summary.ControllerExecution["hyperfine"] = row.ControllerExecution
			break
		}
	}
	verifier, err := os.Executable()
	if err != nil {
		t.Error(err)
	}
	for name, path := range map[string]string{"hyperfine": hyperfine, "verifier": verifier} {
		selected, err := performance.Identify(path)
		if err != nil {
			t.Error(err)
			continue
		}
		summary.Controllers[name] = selected
	}

	if attribution {
		summary.Scope, summary.MemoryScope = "component-attribution", "not measured; diagnostic-only scope"
	} else if err := summary.Review(); err != nil {
		t.Error(err)
	}
	summary.Qualification = !attribution && !t.Failed()

	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "summary.json"), append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return pooled
}

func nativePerformancePrograms(t *testing.T) []performance.Program {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	verified, err := target.ReadProgram(archive, checksums, version)
	if err != nil || !bytes.Equal(verified, readFile(t, candidate)) {
		t.Fatalf("candidate program differs from verified archive: %v", err)
	}
	baseline := requireNativeLifecycleBaseline(t, func() string { return "" })
	programs := []performance.Program{{Variant: "baseline", Path: baseline}, {Variant: "candidate", Path: candidate}}
	for index := range programs {
		data := readFile(t, programs[index].Path)
		programs[index].SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		programs[index].Bytes = len(data)
	}
	if programs[0].SHA256 == programs[1].SHA256 {
		t.Fatal("candidate and predecessor must be distinct published programs")
	}
	return programs
}

func nativePerformanceJourney(t *testing.T, program, credentialWorker, backend string) *journeyFixture {
	t.Helper()
	const account, token = "native-system-keyring-probe", "synthetic-performance-token"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)
	j := newNativeJourney(t, program, server.URL, true)
	manifest := []byte(nativeCurrentSchemaManifest(server.URL) + `
[models.claude-second]
label = "Claude Second"

[routes.performance-second]
label = "Performance Second"
account = "native-system-keyring-probe"
model = "claude-second"
upstream_model = "claude-second"
interfaces = { anthropic = ["text"] }
`)
	if err := os.WriteFile(j.manifest, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	j.preparePerformanceCredentials(backend, account, credentialWorker)
	t.Cleanup(j.uninstallAndRequireInstallationRemoved)
	args := []string{"setup", "--from", j.manifest, "--account", account}
	if backend == "env" {
		j.setEnvironment(secrets.EnvironmentKey(account), token)
		j.run(args...)
	} else {
		j.runWithInput(j.binary, token+"\n", append(args, "--token-stdin")...)
	}
	j.requireClaudeCredential(token)
	var catalog struct {
		Accounts map[string]any `toml:"accounts"`
		Routes   map[string]any `toml:"routes"`
	}
	if err := toml.Unmarshal(readFile(t, j.config), &catalog); err != nil {
		t.Fatal(err)
	}
	if actual := slices.Sorted(maps.Keys(catalog.Accounts)); !slices.Equal(actual, []string{account}) {
		t.Fatalf("performance Account inputs differ: %v", actual)
	}
	if actual := slices.Sorted(maps.Keys(catalog.Routes)); !slices.Equal(actual, []string{"native-system-keyring-probe-claude", "performance-second"}) {
		t.Fatalf("performance model-configuration inputs differ: %v", actual)
	}
	return j
}

func (j *journeyFixture) preparePerformanceCredentials(backend, account, credentialWorker string) {
	j.testing.Helper()
	if backend != "keyring" {
		j.setEnvironment("AIGW_SECRET_BACKEND", backend)
		return
	}
	j.enableSystemCredentialStore()
	// Use one verified candidate for fixture observation and cleanup;
	// measurements remain bound to each variant's own source and reader.
	store, err := secrets.Select(secrets.Selection{Backend: "keyring", Executable: credentialWorker})
	if err != nil {
		j.testing.Fatal(err)
	}
	requireUnoccupiedNativeCredentialSlots(j.testing, store, account)
	if runtime.GOOS == "darwin" {
		requireUnoccupiedLegacyKeychainSlot(j.testing, account, "")
	}
}

func (j *journeyFixture) measurePerformance(hyperfine, output, variant, backend string, block int) ([]performance.Measurement, error) {
	j.testing.Helper()
	preparer, err := os.Executable()
	if err != nil {
		return nil, err
	}
	j.setEnvironment("AIGW_TEST_PERFORMANCE_CONFIG_ROOT", filepath.Dir(j.config))
	j.setEnvironment("AIGW_TEST_PERFORMANCE_SETTINGS_ROOT", filepath.Dir(j.settings))
	var settings struct {
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil {
		return nil, fmt.Errorf("read projected helper: %w", err)
	}
	if settings.APIKeyHelper == "" {
		return nil, errors.New("projected helper is absent")
	}
	return j.measureSelectedPerformance(hyperfine, output, variant, backend, preparer, settings.APIKeyHelper, block)
}

func (j *journeyFixture) measureSelectedPerformance(hyperfine, output, variant, backend, preparer, helperCommand string, block int) ([]performance.Measurement, error) {
	config, err := configuration.NewStore(j.config).Load()
	if err != nil {
		return nil, err
	}
	resolved, err := config.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		return nil, err
	}
	scope := resolved.CredentialProjectionFingerprint(configuration.ClientClaude)
	reader, err := credential.ExecutableFromCommand(helperCommand, configuration.ClientClaude, scope, runtime.GOOS)
	if err != nil {
		return nil, err
	}
	shell := "/bin/sh"
	helper := []string{shell, "-c", helperCommand}
	if runtime.GOOS == "windows" {
		shell = os.Getenv("ComSpec")
		if err := os.WriteFile(filepath.Join(j.root, "credential.cmd"), []byte("@echo off\r\n"+helperCommand+"\r\n"), 0o600); err != nil {
			return nil, err
		}
		helper = []string{shell, "/d", "/c", "credential.cmd"}
	}
	attribution := os.Getenv("AIGW_PERFORMANCE_ATTRIBUTION") == "1"
	cases := j.performanceCases(helper, backend, preparer)
	selected := map[string]string{"credential": shell}
	if attribution {
		cases = j.attributionCases(helper, shell, reader, scope)
		selected = map[string]string{"credential": shell, "shell-startup": shell, "source-startup": j.source, "reader-startup": reader, "credential-direct": reader}
	}
	var measurements []performance.Measurement
	for _, test := range cases {
		if attribution && test.Name == "projection" && backend != "env" {
			continue
		}
		name := fmt.Sprintf("%s-%s-%s-%d", variant, backend, test.Name, block)
		row := performance.Measurement{Variant: variant, Backend: backend, Case: test.Name, Block: block, Budget: test.Budget}
		path := selected[test.Name]
		if path == "" {
			path = j.binary
		}
		identity, err := performance.Identify(path)
		if err != nil {
			return append(measurements, row), err
		}
		row.Executable = &identity
		if test.Name == "credential" {
			readerIdentity, err := performance.Identify(reader)
			if err != nil {
				return append(measurements, row), err
			}
			row.Reader = &readerIdentity
		}
		if attribution && (test.Name == "projection" || test.Name == "sync") {
			row, err = j.measureWorkloadProcesses(j.testing.Context(), filepath.Join(output, name+".json"), row, test)
		} else {
			row, err = j.measureNativeCommand(hyperfine, filepath.Join(output, name+".json"), row, test)
		}
		measurements = append(measurements, row)
		if err != nil {
			return measurements, err
		}
		if row.Diagnostics && !attribution {
			j.testing.Errorf("Hyperfine %s: native diagnostics prevent qualification; raw samples and redacted streams retained", name)
		}
	}
	if attribution && backend == "keyring" {
		rows, err := j.measureNativeCredentialOperations(output, variant, backend, block)
		return append(measurements, rows...), err
	}
	return measurements, nil
}

func (j *journeyFixture) measureNativeCommand(hyperfine, output string, row performance.Measurement, workload performance.Workload) (performance.Measurement, error) {
	row.Command = slices.Clone(workload.Command)
	if os.Getenv("AIGW_PERFORMANCE_ATTRIBUTION") != "1" {
		if err := workload.Review(row.Executable); err != nil {
			return row, err
		}
	}
	command := performance.Command{Tool: hyperfine, Directory: j.root, Output: output,
		Arguments: workload.Arguments(output), Environment: j.environment, Sensitive: j.sensitiveInputs, Measurement: row}
	var err error
	row.Execution, row.ControllerExecution, err = performance.ObserveCommand(j.testing.Context(), command, workload)
	if err != nil {
		return row, err
	}
	command.Measurement = row
	return performance.Measure(j.testing.Context(), command)
}

func (j *journeyFixture) performanceCases(helper []string, backend, preparer string) []performance.Workload {
	selectArgs := []string{j.binary, "use", "--for", "claude", "performance-second"}
	resetArgs := []string{j.binary, "use", "--for", "claude", "native-system-keyring-probe-claude"}
	cases := []performance.Workload{
		{Name: "credential", Command: helper, Budget: performance.Budget("credential")},
		{Name: "projection", Command: selectArgs, Prepare: resetArgs, Budget: performance.Budget("projection")},
		{Name: "setup", Command: []string{j.binary, "setup", "--from", j.manifest, "--account", "native-system-keyring-probe"}, Prepare: []string{preparer, "prepare-performance-setup", filepath.Dir(j.config), filepath.Dir(j.settings), j.config, j.settings}, Budget: performance.Budget("setup")},
		{Name: "sync", Command: []string{j.binary, "sync"}, Prepare: resetArgs, Budget: performance.Budget("sync")},
	}
	if backend == "env" {
		for _, args := range [][]string{{"version", "--version"}, {"help", "--help"}, {"status", "status", "--json"}, {"export", "config", "export"}} {
			cases = append(cases, performance.Workload{Name: args[0], Command: append([]string{j.binary}, args[1:]...), Budget: performance.Budget(args[0])})
		}
	}
	return cases
}

func (j *journeyFixture) attributionCases(helper []string, shell, reader, scope string) []performance.Workload {
	startup := []string{shell, "-c", "exit 0"}
	if runtime.GOOS == "windows" {
		startup = []string{shell, "/d", "/c", "exit", "0"}
	}
	return []performance.Workload{
		{Name: "credential", Command: helper, Budget: 0.1},
		{Name: "shell-startup", Command: startup},
		{Name: "source-startup", Command: []string{j.source, "--version"}},
		{Name: "reader-startup", Command: []string{reader, "--version"}},
		{Name: "credential-direct", Command: []string{reader, "credential", configuration.ClientClaude, scope}},
		j.performanceCases(helper, "env", "")[1],
		j.performanceCases(helper, "env", "")[3],
	}
}

func (j *journeyFixture) measureNativeCredentialOperations(output, variant, backend string, block int) ([]performance.Measurement, error) {
	j.testing.Helper()
	for key, value := range environmentValues(j.environment) {
		if os.Getenv(key) != value {
			j.testing.Setenv(key, value)
		}
	}
	identity, err := performance.Identify(j.source)
	if err != nil {
		return nil, err
	}
	operations := []struct {
		name string
		run  func() error
	}{
		{"native-read-api", func() error {
			value, err := native.Read(j.source, secrets.Service, "native-system-keyring-probe")
			if err != nil {
				return err
			}
			if value != "synthetic-performance-token" {
				return errors.New("synthetic native credential read differs")
			}
			return nil
		}},
		{"native-exists-api", func() error {
			exists, err := native.Exists(j.source, secrets.Service, "native-system-keyring-probe")
			if err != nil {
				return err
			}
			if !exists {
				return errors.New("synthetic native credential is absent")
			}
			return nil
		}},
	}
	var rows []performance.Measurement
	for _, operation := range operations {
		row := performance.Measurement{Variant: variant, Backend: backend, Case: operation.name, Block: block, Executable: &identity}
		path := filepath.Join(output, fmt.Sprintf("%s-%s-%s-%d.json", variant, backend, operation.name, block))
		ctx, cancel := context.WithTimeout(j.testing.Context(), time.Minute)
		row, err := performance.MeasureOperation(ctx, path, row, operation.run)
		cancel()
		rows = append(rows, row)
		if err != nil {
			return rows, err
		}
	}
	return rows, nil
}

// Hyperfine shell=none uses shell_words on every OS, including Windows.
func TestNativePerformanceCases(t *testing.T) {
	journey := &journeyFixture{root: "owned root", binary: "installed program", config: filepath.Join("owned config directory", "config.toml"), settings: filepath.Join("owned settings directory", "settings.json"), manifest: "team manifest"}
	reset := []string{journey.binary, "use", "--for", "claude", "native-system-keyring-probe-claude"}
	for _, backend := range []string{"env", "file", "keyring"} {
		t.Run(backend, func(t *testing.T) {
			cases := journey.performanceCases([]string{"projected helper"}, backend, "test preparer")
			want := []performance.Workload{
				{Name: "credential", Command: []string{"projected helper"}, Budget: 0.1},
				{Name: "projection", Command: []string{journey.binary, "use", "--for", "claude", "performance-second"}, Prepare: reset, Budget: 0.25},
				{Name: "setup", Command: []string{journey.binary, "setup", "--from", journey.manifest, "--account", "native-system-keyring-probe"}, Prepare: []string{"test preparer", "prepare-performance-setup", "owned config directory", "owned settings directory", journey.config, journey.settings}, Budget: 0.25},
				{Name: "sync", Command: []string{journey.binary, "sync"}, Prepare: reset, Budget: 0.25},
			}
			if backend == "env" {
				for _, args := range [][]string{{"version", "--version"}, {"help", "--help"}, {"status", "status", "--json"}, {"export", "config", "export"}} {
					want = append(want, performance.Workload{Name: args[0], Command: append([]string{journey.binary}, args[1:]...), Budget: 0.1})
				}
			}
			if !reflect.DeepEqual(cases, want) {
				t.Fatalf("performance cases omit or change a measured boundary: got=%#v want=%#v", cases, want)
			}
		})
	}
}

func TestNativeAttributionKeepsTheProjectedReaderBoundary(t *testing.T) {
	journey := &journeyFixture{source: "verified source", binary: "installed path"}
	got := journey.attributionCases([]string{"original shell helper"}, "selected shell", "copied reader", "exact-scope")
	want := []performance.Workload{
		{Name: "credential", Command: []string{"original shell helper"}, Budget: 0.1},
		{Name: "shell-startup", Command: []string{"selected shell", "/d", "/c", "exit", "0"}},
		{Name: "source-startup", Command: []string{journey.source, "--version"}},
		{Name: "reader-startup", Command: []string{"copied reader", "--version"}},
		{Name: "credential-direct", Command: []string{"copied reader", "credential", configuration.ClientClaude, "exact-scope"}},
		journey.performanceCases([]string{"original shell helper"}, "env", "")[1],
		journey.performanceCases([]string{"original shell helper"}, "env", "")[3],
	}
	if runtime.GOOS != "windows" {
		want[1].Command = []string{"selected shell", "-c", "exit 0"}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attribution changed the helper executable or qualification scope: %#v", got)
	}
}
