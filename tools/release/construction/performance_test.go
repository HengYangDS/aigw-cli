package construction

import (
	"aigw-cli/tools/release/performance"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestNativePerformanceOwnsResultsAndCleanup(t *testing.T) {
	for _, test := range []struct {
		name    string
		failAt  int
		emit    bool
		clients bool
		summary string
		accept  bool
	}{
		{"success", 0, true, false, "valid", true},
		{"preflight failure", 1, false, false, "", false},
		{"performance failure", 2, false, false, "", false},
		{"missing results", 0, false, false, "", false},
		{"explicit clients and performance", 0, true, true, "valid", true},
		{"unqualified results", 0, true, false, `{"qualification":false,"scope":"full-performance"}`, false},
		{"diagnostic results cannot qualify", 0, true, false, `{"qualification":false,"scope":"component-attribution"}`, false},
		{"missing qualification", 0, true, false, `{"blocks":[{}]}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			source := t.TempDir()
			writeNativeArchive(t, source, "1.2.3")
			output := filepath.Join(t.TempDir(), "measurements")
			want := [][]string{
				{"test", "./tools/release/performance", "-run", "^TestMeasureRetainsSeparateDiagnosticStreams$", "-count=1", "-timeout=60s"},
				{"test", "-tags=performance_acceptance", "./tools/release", "-run", "^TestNativePerformance$", "-count=1", "-v"},
			}
			if test.clients {
				want = append([][]string{{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClientJourney$", "-count=1", "-v"}}, want...)
			}
			var stage string
			calls := 0
			err := acceptNative(request, source, os.Getenv("AIGW_ACCEPTANCE_BASELINE"), NativeAcceptance{Clients: test.clients, Performance: output}, func(call toolCall) error {
				stage = strings.TrimPrefix(call.Env[0], "AIGW_ACCEPTANCE_RELEASE=")
				if call.Name != "go" || call.Directory != request.Root || stage == source {
					t.Fatalf("performance escaped native stage: %#v", call)
				}
				if calls >= len(want) || !slices.Equal(call.Args, want[calls]) {
					t.Fatalf("performance did not qualify its native preparation consumer first: %#v", call)
				}
				calls++
				if calls == test.failAt {
					return errors.New(test.name)
				}
				if calls != len(want) {
					return nil
				}
				if !slices.Contains(call.Env, "AIGW_PERFORMANCE_OUTPUT="+output) {
					t.Fatalf("performance output escaped its explicit owner: %#v", call)
				}
				return emitPerformanceSummary(t, output, test.emit, test.summary)
			})
			wantCalls := len(want)
			if test.failAt != 0 {
				wantCalls = test.failAt
			}
			if (err == nil) != test.accept || calls != wantCalls {
				t.Fatalf("%s: %v", test.name, err)
			}
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatalf("native scratch survived %s: %v", test.name, err)
			}
			if test.emit {
				if _, err := os.Stat(filepath.Join(output, "summary.json")); err != nil {
					t.Fatalf("performance evidence not retained: %v", err)
				}
			}
		})
	}
}

func emitPerformanceSummary(t *testing.T, output string, emit bool, summary string) error {
	if !emit {
		return nil
	}
	if err := os.Mkdir(output, 0o700); err != nil {
		return err
	}
	if summary == "valid" {
		writeQualifiedPerformanceSummary(t, output)
		return nil
	}
	return os.WriteFile(filepath.Join(output, "summary.json"), []byte(summary), 0o600)
}

func TestNativePerformanceOutputAdmission(t *testing.T) {
	output := t.TempDir()
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
	err := acceptNative(request, "", os.Getenv("AIGW_ACCEPTANCE_BASELINE"), NativeAcceptance{Performance: output}, func(call toolCall) error {
		t.Fatalf("invalid output admitted an external command: %#v", call)
		return nil
	})
	if err == nil || err.Error() != "performance output must be a new directory" {
		t.Fatalf("existing performance output accepted: %s: %v", output, err)
	}
}

func TestNativePerformanceSummaryBindsNativeEvidence(t *testing.T) {
	for name, reason := range map[string]string{
		"null-exit":        "explicit integer exit codes",
		"wrong-command":    "differs from its requested command",
		"declared-command": "does not select its declared executable",
		"declared-case":    "differs from its declared workload",
		"released-program": "differs from its released program",
		"workload-budget":  "changed its qualifying budget",
		"memory-growth":    "peak memory requires review",
		"block-budget":     "measurements are not qualified",
		"program-growth":   "executable size growth requires review",
		"native-platform":  "differs from its native consumer",
		"native-arch":      "differs from its native consumer",
		"raw-path":         "raw sample path is invalid",
	} {
		t.Run(name, func(t *testing.T) {
			output := t.TempDir()
			summary := writeQualifiedPerformanceSummary(t, output)
			switch name {
			case "null-exit", "wrong-command", "declared-command":
				changePerformanceRawEvidence(t, output, &summary.Blocks[0], name)
			case "released-program":
				summary.Programs[1].SHA256 = strings.Repeat("c", 64)
			case "workload-budget":
				for index := range summary.Blocks {
					if summary.Blocks[index].Case == "status" {
						summary.Blocks[index].Budget = 1
					}
				}
			case "memory-growth":
				inflatePerformanceMemory(summary.Memory)
			case "block-budget":
				inflatePerformanceBlock(t, output, summary.Blocks)
			case "program-growth":
				summary.Programs[1].Bytes = 2 << 20
				for index := range summary.Blocks {
					row := &summary.Blocks[index]
					if row.Variant == "candidate" {
						row.Executable.Bytes = summary.Programs[1].Bytes
						for process := range row.Execution {
							row.Execution[process].Image.Bytes = summary.Programs[1].Bytes
						}
					}
				}
			case "native-platform":
				summary.OS = "other"
			case "native-arch":
				summary.Arch = "other"
			case "raw-path":
				summary.Blocks[0].Raw = "../foreign-status.json"
			case "declared-case":
				index := slices.IndexFunc(summary.Blocks, func(row performance.Measurement) bool { return row.Case == "setup" })
				changePerformanceRawEvidence(t, output, &summary.Blocks[index], name)
			}
			var err error
			summary.Pooled, err = performance.Pooled(summary.Blocks)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(summary)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(output, "summary.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := performance.ReadSummary(output, false); err == nil || !strings.Contains(err.Error(), reason) {
				t.Fatalf("%s acquired native qualification: %v", name, err)
			}
		})
	}
}

func TestNativePerformanceSummaryOwnsWindowsMeasurements(t *testing.T) {
	for _, name := range []string{"mixed-native-blocks", "reused-verifier", "foreign-controller"} {
		t.Run(name, func(t *testing.T) {
			summary := writeQualifiedPerformanceSummary(t, t.TempDir())
			summary.OS = "windows"
			if err := summary.Review(); err != nil {
				t.Fatalf("one verifier's x64-on-ARM64 measurements were refused: %v", err)
			}
			for index := range summary.Blocks {
				row := &summary.Blocks[index]
				row.ControllerExecution = slices.Clone(row.ControllerExecution)
				switch name {
				case "mixed-native-blocks":
					if row.Block != 2 {
						continue
					}
					row.ControllerExecution[0].NativeMachine, row.ControllerExecution[0].WOW64Machine = 0x8664, 0
					row.Execution = slices.Clone(row.Execution)
					for process := range row.Execution {
						row.Execution[process].NativeMachine, row.Execution[process].WOW64Machine = 0x8664, 0
					}
				case "reused-verifier":
					row.ControllerExecution[0].ParentCreated++
				case "foreign-controller":
					row.ControllerExecution[0].ParentPID = 9
				}
			}
			if err := summary.Review(); err == nil || !strings.Contains(err.Error(), "owned by its observed verifier") {
				t.Fatalf("%s acquired native performance qualification: %v", name, err)
			}
		})
	}
}

func TestNativePerformanceSummaryRetainsItsQualificationContract(t *testing.T) {
	for _, name := range []string{"controller", "controller-image", "workload-image", "scope", "program", "predecessor", "diagnostic", "samples", "pooled", "workload", "memory", "memory-observation"} {
		t.Run(name, func(t *testing.T) {
			summary := writeQualifiedPerformanceSummary(t, t.TempDir())
			if err := summary.Review(); err != nil {
				t.Fatalf("complete native performance evidence was refused: %v", err)
			}
			switch name {
			case "controller":
				delete(summary.Controllers, "verifier")
			case "controller-image":
				selected := summary.Controllers["verifier"]
				selected.SHA256 = ""
				summary.Controllers["verifier"] = selected
			case "workload-image":
				summary.Blocks[0].Executable = nil
			case "scope":
				summary.ClientScope = ""
			case "program":
				summary.Programs[1].Variant = "unreleased"
			case "predecessor":
				summary.Programs[1].SHA256 = summary.Programs[0].SHA256
			case "diagnostic":
				summary.Blocks[0].Diagnostics = true
			case "samples":
				summary.Blocks[0].Samples.Times = nil
			case "pooled":
				summary.Pooled = nil
			case "workload":
				summary.Blocks = slices.DeleteFunc(summary.Blocks, func(row performance.Measurement) bool { return row.Case == "sync" })
				var err error
				summary.Pooled, err = performance.Pooled(summary.Blocks)
				if err != nil {
					t.Fatal(err)
				}
			case "memory":
				summary.Memory = nil
			case "memory-observation":
				summary.Memory[0].Bytes[0] = 0
			}
			if err := summary.Review(); err == nil {
				t.Fatalf("%s evidence loss retained native performance qualification", name)
			}
		})
	}
}

func inflatePerformanceMemory(rows []performance.Memory) {
	for index := range rows {
		if rows[index].Variant == "candidate" {
			for sample := range rows[index].Bytes {
				rows[index].Bytes[sample] = 21 << 20
			}
		}
	}
}

func changePerformanceRawEvidence(t *testing.T, output string, row *performance.Measurement, kind string) {
	t.Helper()
	path := filepath.Join(output, row.Raw)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	command, err := json.Marshal(row.Samples.Command)
	if err != nil {
		t.Fatal(err)
	}
	replacement := "different command"
	switch kind {
	case "null-exit":
		data = bytes.Replace(data, []byte(`"exit_code":0`), []byte(`"exit_code":null`), 1)
	case "declared-case":
		row.Command = []string{row.Executable.Path, "--version"}
		row.Samples.Command = performance.Argv(row.Command...)
		replacement = row.Samples.Command
	case "declared-command":
		row.Command = []string{"different command"}
		row.Samples.Command = performance.Argv(row.Command...)
		replacement = row.Samples.Command
	}
	if kind != "null-exit" {
		encoded, err := json.Marshal(replacement)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.Replace(data, command, encoded, 1)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func inflatePerformanceBlock(t *testing.T, output string, rows []performance.Measurement) {
	t.Helper()
	for index := range rows {
		row := &rows[index]
		if row.Variant != "candidate" || row.Case != "credential" || row.Block != 1 {
			continue
		}
		for sample := 37; sample < 40; sample++ {
			row.Samples.Times[sample] = 0.2
		}
		var err error
		row.P95, err = row.Samples.Percentile()
		if err != nil {
			t.Fatal(err)
		}
		writeHyperfineSamples(t, filepath.Join(output, row.Raw), row.Samples)
	}
}

func writeQualifiedPerformanceSummary(t *testing.T, output string) performance.Summary {
	t.Helper()
	image := performance.Identity{Path: "measured-image", Format: "PE", Machine: 0x8664, Arch: "amd64", SHA256: strings.Repeat("a", 64), Bytes: 1}
	observed := []performance.Execution{{PID: 2, ParentPID: 1, Created: 20, ParentCreated: 10, Role: "controller", Image: image,
		Machine: 0x8664, Arch: "amd64", WOW64Machine: 0x8664, NativeMachine: 0xaa64, Attributes: 1}}
	verifier := []performance.Execution{{PID: 1, ParentPID: 9, Created: 10, ParentCreated: 1, Role: "controller", Image: image,
		Machine: 0x8664, Arch: "amd64", WOW64Machine: 0x8664, NativeMachine: 0xaa64, Attributes: 1}}
	summary := performance.Summary{Qualification: true, Scope: "full-performance", OS: runtime.GOOS, Arch: runtime.GOARCH, Tool: "Hyperfine fixture",
		IdentityScope: "fixture", MemoryScope: "fixture", ClientScope: "fixture",
		Controllers:         map[string]performance.Identity{"hyperfine": image, "verifier": image},
		ControllerExecution: map[string][]performance.Execution{"hyperfine": observed, "verifier": verifier},
		Programs:            []performance.Program{{Variant: "baseline", Path: image.Path, SHA256: image.SHA256, Bytes: 1}, {Variant: "candidate", Path: image.Path, SHA256: strings.Repeat("b", 64), Bytes: 2}}}
	for _, variant := range []string{"baseline", "candidate"} {
		selected := image
		if variant == "candidate" {
			selected.Path, selected.SHA256, selected.Bytes = "candidate-image", strings.Repeat("b", 64), 2
		}
		observed := []performance.Execution{{PID: 3, ParentPID: 2, Created: 30, ParentCreated: 20, Role: "workload", Image: selected,
			Machine: 0x8664, Arch: "amd64", WOW64Machine: 0x8664, NativeMachine: 0xaa64, Attributes: 1}}
		for block := range 2 {
			for _, name := range []string{"credential", "projection", "setup", "sync", "version", "help", "status", "export"} {
				budget := 0.1
				if name == "projection" || name == "setup" || name == "sync" {
					budget = 0.25
				}
				row := performance.Measurement{Variant: variant, Backend: "env", Case: name, Block: block + 1, Budget: budget, P95: 0.01,
					Raw: fmt.Sprintf("%s-%s-%d.json", variant, name, block), Executable: &selected, Execution: slices.Clone(observed), ControllerExecution: summary.ControllerExecution["hyperfine"],
					Samples: performance.Samples{Command: performance.Argv(selected.Path, name), Times: make([]float64, 40), ExitCodes: make([]int, 40)}}
				row.Command = []string{selected.Path, name}
				switch name {
				case "credential":
					row.Command = []string{selected.Path, "-c", "projected credential helper"}
				case "projection":
					row.Command = []string{selected.Path, "use", "--for", "claude", "selected-route"}
				case "setup":
					row.Command = []string{selected.Path, "setup", "--from", "team.toml", "--account", "account"}
				case "version", "help":
					row.Command[1] = "--" + name
				case "status":
					row.Command = append(row.Command, "--json")
				case "export":
					row.Command = []string{selected.Path, "config", "export"}
				}
				row.Samples.Command = performance.Argv(row.Command...)
				if name == "credential" {
					row.Reader = &selected
					row.Execution = append(row.Execution, performance.Execution{PID: 4, ParentPID: 3, Created: 40, ParentCreated: 30, Role: "reader", Image: selected,
						Machine: 0x8664, Arch: "amd64", WOW64Machine: 0x8664, NativeMachine: 0xaa64, Attributes: 1})
				}
				for index := range row.Samples.Times {
					row.Samples.Times[index] = 0.01
				}
				writeHyperfineSamples(t, filepath.Join(output, row.Raw), row.Samples)
				summary.Blocks = append(summary.Blocks, row)
			}
			peak := make([]uint64, 40)
			for index := range peak {
				peak[index] = 16 << 20
			}
			summary.Memory = append(summary.Memory, performance.Memory{Variant: variant, Case: "status", Block: block + 1, Bytes: peak})
		}
	}
	var err error
	summary.Pooled, err = performance.Pooled(summary.Blocks)
	if err != nil {
		t.Fatal(err)
	}
	if err := summary.Review(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "summary.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return summary
}

func writeHyperfineSamples(t *testing.T, path string, samples performance.Samples) {
	t.Helper()
	type metric struct {
		Value float64 `json:"value"`
		Unit  string  `json:"unit"`
	}
	type measurement struct {
		Time     metric `json:"time_wall_clock"`
		ExitCode int    `json:"exit_code"`
	}
	type result struct {
		Command      string        `json:"command"`
		Measurements []measurement `json:"measurements"`
	}
	observed := result{Command: samples.Command}
	for index, duration := range samples.Times {
		observed.Measurements = append(observed.Measurements, measurement{Time: metric{Value: duration, Unit: "second"}, ExitCode: samples.ExitCodes[index]})
	}
	data, err := json.Marshal(struct {
		SchemaVersion int      `json:"schema_version"`
		PrimaryMetric string   `json:"primary_metric"`
		Results       []result `json:"results"`
	}{SchemaVersion: 2, PrimaryMetric: "time_wall_clock", Results: []result{observed}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
