//go:build performance_acceptance

package main

import (
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/upgrade/artifact"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type performanceSamples struct {
	Times       []float64 `json:"times"`
	ExitCodes   []int     `json:"exit_codes"`
	MemoryBytes []uint64  `json:"memory_usage_byte"`
}

func (s performanceSamples) percentile() (float64, error) {
	if len(s.Times) < 40 || len(s.ExitCodes) != len(s.Times) {
		return 0, errors.New("performance evidence requires at least forty completed samples")
	}
	for index, value := range s.Times {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) || s.ExitCodes[index] != 0 {
			return 0, errors.New("performance samples require finite positive durations and successful commands")
		}
	}
	ordered := slices.Clone(s.Times)
	slices.Sort(ordered)
	return ordered[int(math.Ceil(0.95*float64(len(ordered))))-1], nil
}

type performanceMeasurement struct {
	Variant string             `json:"variant"`
	Backend string             `json:"backend"`
	Case    string             `json:"case"`
	Block   int                `json:"block"`
	P95     float64            `json:"p95_seconds"`
	Budget  float64            `json:"budget_seconds"`
	Raw     string             `json:"raw"`
	Samples performanceSamples `json:"-"`
}

type performanceProgram struct {
	Variant string `json:"variant"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Bytes   int    `json:"bytes"`
}

func TestNativePerformance(t *testing.T) {
	if !t.Run("native memory calibration", TestNativePeakMemory) {
		t.Fatal("native memory accounting failed its independent allocation calibration")
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
	identity, err := exec.CommandContext(t.Context(), hyperfine, "--version").Output()
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
	var measurements []performanceMeasurement
	var memory []memoryMeasurement
	for block, order := range [][]int{{0, 1}, {1, 0}} {
		for _, index := range order {
			program := programs[index]
			for _, backend := range backends {
				t.Run(fmt.Sprintf("block-%d/%s/%s", block+1, program.Variant, backend), func(t *testing.T) {
					journey := nativePerformanceJourney(t, program.Path, backend)
					rows := journey.measurePerformance(hyperfine, output, program.Variant, backend, block+1)
					measurements = append(measurements, rows...)
					if backend == "env" {
						memory = append(memory, journey.measureMemory(program.Variant, block+1))
					}
				})
			}
		}
	}
	pooled := pooledPerformance(t, measurements)
	summary := struct {
		OS          string                   `json:"os"`
		Arch        string                   `json:"arch"`
		Tool        string                   `json:"tool"`
		MemoryScope string                   `json:"memory_scope"`
		ClientScope string                   `json:"client_scope"`
		Programs    []performanceProgram     `json:"programs"`
		Blocks      []performanceMeasurement `json:"blocks"`
		Pooled      []performanceMeasurement `json:"pooled"`
		Memory      []memoryMeasurement      `json:"memory"`
	}{runtime.GOOS, runtime.GOARCH, strings.TrimSpace(string(identity)),
		"Configured status: per-child wait4 on macOS, GNU time on Linux, retained-handle peak working set on Windows; bytes, no periodic sampling; calibrated against a large parent",
		"controlled client discovery; native projected helper; no Provider inference",
		programs, measurements, pooled, memory}
	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "summary.json"), append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, row := range append(slices.Clone(measurements), pooled...) {
		t.Logf("%s/%s/%s block=%d p95=%.3fms budget=%.0fms", row.Variant, row.Backend, row.Case, row.Block, row.P95*1000, row.Budget*1000)
		if row.Variant == "candidate" && row.P95 > row.Budget {
			t.Errorf("candidate %s/%s block=%d exceeds declared budget", row.Backend, row.Case, row.Block)
		}
	}
	baseline, candidate := programs[0].Bytes, programs[1].Bytes
	if candidate-baseline > 1<<20 && float64(candidate) > float64(baseline)*1.1 {
		t.Error("executable size growth requires review: exceeds both 10% and 1 MiB")
	}
	if err := reviewPeakMemory(memory); err != nil {
		t.Error(err)
	}
}

func nativePerformancePrograms(t *testing.T) []performanceProgram {
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
	programs := []performanceProgram{{Variant: "baseline", Path: baseline}, {Variant: "candidate", Path: candidate}}
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

func nativePerformanceJourney(t *testing.T, program, backend string) *journeyFixture {
	t.Helper()
	const account, token = "native-system-keyring-probe", "synthetic-performance-token"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)
	j := newNativeJourney(t, program, server.URL, true)
	j.preparePerformanceCredentials(backend, account)
	t.Cleanup(j.uninstallAndRequireOwnedFilesAbsent)
	args := []string{"setup", "--from", j.manifest, "--account", account}
	if backend == "env" {
		j.setEnvironment(secrets.EnvironmentKey(account), token)
		j.run(args...)
	} else {
		j.runWithInput(j.binary, token+"\n", append(args, "--token-stdin")...)
	}
	j.run("profile", "add", "performance-second", "--account", account, "--for", "claude", "--model", "claude-second")
	j.requireClaudeCredential(token)
	return j
}

func (j *journeyFixture) preparePerformanceCredentials(backend, account string) {
	j.testing.Helper()
	if backend != "keyring" {
		j.setEnvironment("AIGW_SECRET_BACKEND", backend)
		return
	}
	j.enableSystemCredentialStore()
	store, err := secrets.Select(secrets.Selection{Backend: "keyring"})
	if err != nil {
		j.testing.Fatal(err)
	}
	if exists, err := store.Exists(account); err != nil || exists {
		j.testing.Fatalf("native performance requires unoccupied credential slot: exists=%t error=%v", exists, err)
	}
	j.testing.Cleanup(func() {
		if err := store.Delete(account); err != nil {
			j.testing.Errorf("remove owned performance credential: %v", err)
		}
	})
}

func (j *journeyFixture) measurePerformance(hyperfine, output, variant, backend string, block int) []performanceMeasurement {
	j.testing.Helper()
	var settings struct {
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil || settings.APIKeyHelper == "" {
		j.testing.Fatalf("read projected helper: %v", err)
	}
	helper := performanceCommand("/bin/sh", "-c", settings.APIKeyHelper)
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(filepath.Join(j.root, "credential.cmd"), []byte("@echo off\r\n"+settings.APIKeyHelper+"\r\n"), 0o600); err != nil {
			j.testing.Fatal(err)
		}
		helper = performanceCommand(os.Getenv("ComSpec"), "/d", "/c", "credential.cmd")
	}
	cases := []struct {
		name, command, prepare string
		budget                 float64
	}{
		{"credential", helper, "", 0.1},
		{"projection", performanceCommand(j.binary, "use", "performance-second"), performanceCommand(j.binary, "use", "native-system-keyring-probe-claude"), 0.25},
	}
	if backend == "env" {
		for _, args := range [][]string{{"version", "--version"}, {"help", "--help"}, {"status", "status", "--json"}, {"export", "config", "export"}} {
			cases = append(cases, struct {
				name, command, prepare string
				budget                 float64
			}{args[0], performanceCommand(append([]string{j.binary}, args[1:]...)...), "", 0.1})
		}
	}
	var measurements []performanceMeasurement
	for _, test := range cases {
		name := fmt.Sprintf("%s-%s-%s-%d", variant, backend, test.name, block)
		raw := filepath.Join(output, name+".json")
		args := []string{"--shell=none", "--warmup", "5", "--runs", "40", "--output=pipe", "--style", "basic", "--export-json", raw}
		if test.prepare != "" {
			args = append(args, "--prepare", test.prepare)
		}
		command := exec.CommandContext(j.testing.Context(), hyperfine, append(args, test.command)...)
		command.Env, command.Dir = j.environment, j.root
		log, runErr := command.CombinedOutput()
		if err := os.WriteFile(filepath.Join(output, name+".log"), log, 0o600); err != nil {
			j.testing.Fatal(err)
		}
		if runErr != nil {
			j.testing.Fatalf("Hyperfine %s: %v\n%s", name, runErr, log)
		}
		var report struct {
			Results []performanceSamples `json:"results"`
		}
		if err := json.Unmarshal(readFile(j.testing, raw), &report); err != nil || len(report.Results) != 1 {
			j.testing.Fatalf("Hyperfine must produce one nonempty result: %v", err)
		}
		samples := report.Results[0]
		p95, err := samples.percentile()
		if err != nil {
			j.testing.Fatal(err)
		}
		measurements = append(measurements, performanceMeasurement{variant, backend, test.name, block, p95, test.budget, filepath.Base(raw), samples})
	}
	before, sidecar := readFile(j.testing, j.settings), readFile(j.testing, j.settings+".aigw-state.json")
	j.run("sync")
	if !bytes.Equal(before, readFile(j.testing, j.settings)) || !bytes.Equal(sidecar, readFile(j.testing, j.settings+".aigw-state.json")) {
		j.testing.Fatal("performance journey changed a converged projection")
	}
	return measurements
}

// Hyperfine shell=none uses shell_words on every OS, including Windows.
func performanceCommand(args ...string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
}

func pooledPerformance(t *testing.T, measurements []performanceMeasurement) []performanceMeasurement {
	t.Helper()
	groups := make(map[string]performanceMeasurement)
	for _, row := range measurements {
		key := row.Variant + "/" + row.Backend + "/" + row.Case
		group, exists := groups[key]
		if !exists {
			group = row
			group.Block, group.Raw, group.Samples = 0, "", performanceSamples{}
		}
		group.Samples.Times = append(group.Samples.Times, row.Samples.Times...)
		group.Samples.ExitCodes = append(group.Samples.ExitCodes, row.Samples.ExitCodes...)
		groups[key] = group
	}
	var pooled []performanceMeasurement
	for key, row := range groups {
		if len(row.Samples.Times) != 80 {
			t.Errorf("%s lacks two complete forty-sample blocks", key)
			continue
		}
		var err error
		row.P95, err = row.Samples.percentile()
		if err != nil {
			t.Fatal(err)
		}
		pooled = append(pooled, row)
	}
	slices.SortFunc(pooled, func(a, b performanceMeasurement) int {
		return strings.Compare(a.Variant+a.Backend+a.Case, b.Variant+b.Backend+b.Case)
	})
	return pooled
}

func TestNativePerformanceSamples(t *testing.T) {
	times := make([]float64, 40)
	for index := range times {
		times[index] = float64(index+1) / 1000
	}
	result := performanceSamples{Times: times, ExitCodes: make([]int, 40)}
	before := slices.Clone(times)
	p95, err := result.percentile()
	if err != nil || p95 != 0.038 || !slices.Equal(before, times) {
		t.Fatalf("p95=%v error=%v or raw samples changed", p95, err)
	}
	result.ExitCodes[0] = 1
	if _, err := result.percentile(); err == nil {
		t.Fatal("a failed command qualified as performance evidence")
	}
	if _, err := (performanceSamples{}).percentile(); err == nil {
		t.Fatal("an empty result qualified as performance evidence")
	}
	result.ExitCodes[0] = 0
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		result.Times[0] = value
		if _, err := result.percentile(); err == nil {
			t.Fatalf("invalid duration qualified: %v", value)
		}
	}
}

func TestNativePerformanceCommand(t *testing.T) {
	if got := performanceCommand(`C:\program files\aigw.exe`, "a'b", ""); got != `'C:\program files\aigw.exe' 'a'\''b' ''` {
		t.Fatalf("Hyperfine argv quoting = %s", got)
	}
}

func TestNativePerformancePooledSamples(t *testing.T) {
	var rows []performanceMeasurement
	for block := range 2 {
		times := make([]float64, 40)
		for index := range times {
			times[index] = float64(block+1) / 100
		}
		rows = append(rows, performanceMeasurement{Variant: "candidate", Backend: "env", Case: "status", Block: block + 1, Samples: performanceSamples{Times: times, ExitCodes: make([]int, 40)}})
	}
	pooled := pooledPerformance(t, rows)
	if len(pooled) != 1 || pooled[0].P95 != 0.02 || len(rows[0].Samples.Times) != 40 {
		t.Fatalf("pooled result lost a block or changed its source: %#v", pooled)
	}
}
