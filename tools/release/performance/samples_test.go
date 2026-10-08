package performance

import (
	"aigw-cli/internal/configuration"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestPercentileRequiresCompletedFiniteSamples(t *testing.T) {
	times := make([]float64, 40)
	for index := range times {
		times[index] = float64(index+1) / 1000
	}
	result := Samples{Times: times, ExitCodes: make([]int, 40)}
	before := slices.Clone(times)
	p95, err := result.Percentile()
	if err != nil || p95 != 0.038 || !slices.Equal(before, times) {
		t.Fatalf("p95=%v error=%v or raw samples changed", p95, err)
	}
	result.ExitCodes[0] = 1
	if _, err := result.Percentile(); err == nil {
		t.Fatal("a failed command qualified as performance evidence")
	}
	if _, err := (Samples{}).Percentile(); err == nil {
		t.Fatal("an empty result qualified as performance evidence")
	}
	result.ExitCodes[0] = 0
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		result.Times[0] = value
		if _, err := result.Percentile(); err == nil {
			t.Fatalf("invalid duration qualified: %v", value)
		}
	}
}

func TestPooledPreservesBlocksAndDiagnostics(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := Identify(program)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Measurement
	for block := range 2 {
		times := make([]float64, 40)
		for index := range times {
			times[index] = float64(block+1) / 100
		}
		selected := identity
		selected.Path = "independent-fixture-" + string(rune('1'+block))
		rows = append(rows, Measurement{Variant: "candidate", Backend: "env", Case: "status", Block: block + 1,
			P95: float64(block+1) / 100, Executable: &selected,
			Diagnostics: block == 1, Samples: Samples{Command: Argv(selected.Path, "status"), Times: times, ExitCodes: make([]int, 40)}})
	}
	pooled, err := Pooled(rows)
	if err != nil || len(pooled) != 1 || pooled[0].P95 != 0.02 || len(rows[0].Samples.Times) != 40 || !pooled[0].Diagnostics ||
		pooled[0].Executable == nil || pooled[0].Executable.SHA256 != identity.SHA256 || pooled[0].Executable.Path != "" || rows[0].Samples.Command == rows[1].Samples.Command {
		t.Fatalf("pooled result lost a block or changed its source: %#v, %v", pooled, err)
	}
	if _, err := Pooled(rows[:1]); err == nil {
		t.Fatal("an incomplete block qualified as pooled evidence")
	}
	for _, invalid := range [][]Measurement{
		nil, {rows[0], rows[0]},
		{{Variant: "candidate", Backend: "env", Case: "status", Block: 1, Samples: pooled[0].Samples}},
		{rows[0], {Variant: "candidate", Backend: "env", Case: "status", Block: 2, Budget: 1, Samples: rows[1].Samples}},
	} {
		if _, err := Pooled(invalid); err == nil {
			t.Fatal("pooled evidence admitted an incomplete or inconsistent block contract")
		}
	}
	for _, change := range []func(*Measurement){
		func(row *Measurement) { row.Executable = nil },
		func(row *Measurement) { row.Executable.SHA256 = "different-bytes" },
		func(row *Measurement) { row.Executable.Arch = "different-platform" },
		func(row *Measurement) { row.P95 = 0 },
	} {
		changed := rows[1]
		selected := *changed.Executable
		changed.Executable = &selected
		change(&changed)
		if _, err := Pooled([]Measurement{rows[0], changed}); err == nil {
			t.Fatal("pooled evidence admitted an unqualified or different executable block")
		}
	}
}

func TestPooledKeepsDirectAndForwardingBindingsSeparate(t *testing.T) {
	var rows []Measurement
	for _, mode := range []string{"direct", "forwarding"} {
		binding := configuration.Runtime{Client: configuration.ClientClaude, RouteID: "route", AccountID: "account",
			Protocol: configuration.ProtocolAnthropic, UpstreamEndpoint: "https://upstream.example/v1", Endpoint: "https://upstream.example/v1"}
		if mode == "forwarding" {
			binding.Endpoint = "http://127.0.0.1:18792/v1"
		}
		for block := 1; block <= 2; block++ {
			times := make([]float64, 40)
			for index := range times {
				times[index] = 0.01
			}
			row := Measurement{Variant: "candidate", Backend: "env", Case: "status", Block: block,
				P95: 0.01, Budget: Budget("status"), Samples: Samples{Times: times, ExitCodes: make([]int, 40)}, Mode: mode, Runtime: binding}
			rows = append(rows, row)
		}
	}
	pooled, err := Pooled(rows)
	if err != nil || len(pooled) != 2 {
		t.Fatalf("direct and forwarding blocks were merged: %#v, %v", pooled, err)
	}
}

func TestMemoryRequiresOneResolvedMeasurementBinding(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Summary)
		reject bool
	}{
		{name: "resolved forwarding"},
		{name: "candidate block drift", reject: true, mutate: func(summary *Summary) { summary.Memory[3].Runtime.AccountID = "different-account" }},
		{name: "predecessor block drift", reject: true, mutate: func(summary *Summary) { summary.Memory[1].Runtime.RouteID = "different-route" }},
		{name: "timing differs from memory", reject: true, mutate: func(summary *Summary) {
			for index := range summary.Memory {
				if summary.Memory[index].Variant == "candidate" {
					summary.Memory[index].Runtime.UpstreamEndpoint = "https://different-upstream.example/v1"
				}
			}
		}},
		{name: "status timing absent", reject: true, mutate: func(summary *Summary) { summary.Blocks = nil }},
		{name: "historical direct", mutate: func(summary *Summary) {
			summary.Scope = "full-performance"
			for index := range summary.Memory {
				summary.Memory[index].Mode, summary.Memory[index].Runtime, summary.Memory[index].Executable = "", configuration.Runtime{}, nil
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			programs := map[string]Program{
				"baseline":  {Variant: "baseline", SHA256: strings.Repeat("b", 64), Bytes: 100},
				"candidate": {Variant: "candidate", SHA256: strings.Repeat("c", 64), Bytes: 200},
			}
			summary := Summary{Scope: "forwarding-performance"}
			for _, variant := range []string{"baseline", "candidate"} {
				mode := "direct"
				binding := configuration.Runtime{Client: configuration.ClientClaude, RouteID: "route", AccountID: "account",
					Protocol: configuration.ProtocolAnthropic, Endpoint: "https://upstream.example/v1", UpstreamEndpoint: "https://upstream.example/v1"}
				if variant == "candidate" {
					mode, binding.Endpoint = "forwarding", "http://127.0.0.1:18792/v1"
				}
				identity := Identity{SHA256: programs[variant].SHA256, Bytes: programs[variant].Bytes}
				for block := 1; block <= 2; block++ {
					observations := make([]uint64, 40)
					for index := range observations {
						observations[index] = 1 << 20
					}
					summary.Memory = append(summary.Memory, Memory{Variant: variant, Case: "status", Block: block, Mode: mode, Runtime: binding, Executable: &identity, Bytes: observations})
					if variant == "candidate" {
						summary.Blocks = append(summary.Blocks, Measurement{Variant: variant, Backend: "env", Case: "status", Block: block, Mode: mode, Runtime: binding})
					}
				}
			}
			if test.mutate != nil {
				test.mutate(&summary)
			}
			if err := ReviewMemory(summary.Memory); err != nil {
				t.Fatal(err)
			}
			if err := summary.reviewMemoryBindings(programs); (err != nil) != test.reject {
				t.Fatalf("memory admission differs from the retained workload binding: %v", err)
			}
		})
	}
}

func TestForwardingWorkloadsMatchTheirDeclaredScope(t *testing.T) {
	programs := map[string]Program{"candidate": {Variant: "candidate"}}
	var rows []Measurement
	for _, name := range []string{"credential", "projection", "sync", "status", "export"} {
		rows = append(rows, Measurement{Variant: "candidate", Backend: "env", Case: name, Mode: "forwarding", Budget: Budget(name)})
	}
	if err := reviewWorkloads(rows, programs, "forwarding-performance"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"setup", "version", "help", "unowned"} {
		row := Measurement{Variant: "candidate", Backend: "env", Case: name, Mode: "forwarding", Budget: 0.1}
		if err := reviewWorkloads(append(slices.Clone(rows), row), programs, "forwarding-performance"); err == nil {
			t.Fatalf("forwarding admission included an undeclared workload: %s", name)
		}
	}
}

func TestReadSummaryRequiresNativeRawEvidence(t *testing.T) {
	directory := t.TempDir()
	if _, err := ReadSummary(directory, "full-performance"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing summary was not refused by its file owner: %v", err)
	}
	file := filepath.Join(directory, "summary.json")
	if err := os.WriteFile(file, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSummary(directory, "full-performance"); err == nil {
		t.Fatal("malformed summary qualified")
	}
	summary := Summary{OS: runtime.GOOS, Arch: runtime.GOARCH, Qualification: true, Scope: "full-performance",
		Blocks: []Measurement{{Raw: "missing-status.json"}}}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSummary(directory, "full-performance"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("qualified summary admitted absent raw samples: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "missing-status.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSummary(directory, "full-performance"); err == nil {
		t.Fatal("qualified summary admitted malformed raw samples")
	}
	summary.Qualification, summary.Scope = false, "component-attribution"
	summary.Pooled = slices.Clone(summary.Blocks)
	data, err = json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSummary(directory, "component-attribution"); err != nil {
		t.Fatalf("nonqualifying diagnostic evidence was refused: %v", err)
	}
	if _, err := ReadSummary(directory, "full-performance"); err == nil {
		t.Fatal("diagnostic evidence was promoted to performance qualification")
	}
}

func TestNativePeakMemoryBudget(t *testing.T) {
	const baseline = 16 << 20
	for _, test := range []struct {
		candidate uint64
		review    bool
	}{
		{baseline, false},
		{baseline + 4<<20, false},
		{baseline + 5<<20, true},
	} {
		var rows []Memory
		for block := 1; block <= 2; block++ {
			for _, program := range []struct {
				variant string
				peak    uint64
			}{{"baseline", baseline}, {"candidate", test.candidate}} {
				peaks := make([]uint64, 40)
				for index := range peaks {
					peaks[index] = program.peak
				}
				rows = append(rows, Memory{Variant: program.variant, Case: "status", Block: block, Bytes: peaks})
			}
		}
		if err := ReviewMemory(rows); (err != nil) != test.review {
			t.Fatalf("candidate peak %d: review=%t error=%v", test.candidate, test.review, err)
		}
	}
	if err := ReviewMemory(nil); err == nil {
		t.Fatal("empty evidence qualified as completed memory acceptance")
	}
	if err := ReviewMemory([]Memory{{Variant: "candidate", Case: "status", Block: 1, Bytes: []uint64{baseline}}}); err == nil {
		t.Fatal("candidate memory was accepted without a matching predecessor observation")
	}
}
