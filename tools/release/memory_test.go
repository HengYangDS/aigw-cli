//go:build performance_acceptance

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"aigw-cli/tools/release/performance"
)

type memoryMeasurement struct {
	Variant string   `json:"variant"`
	Case    string   `json:"case"`
	Block   int      `json:"block"`
	Bytes   []uint64 `json:"peak_resident_bytes"`
}

func (j *journeyFixture) measureMemory(variant string, block int) (memoryMeasurement, error) {
	j.testing.Helper()
	row := memoryMeasurement{Variant: variant, Case: "status", Block: block, Bytes: []uint64{}}
	for sample := range 45 {
		command := exec.CommandContext(j.testing.Context(), j.binary, "status", "--json")
		command.Env, command.Dir = j.environment, j.root
		var output bytes.Buffer
		command.Stdout, command.Stderr = &output, &output
		peak, err := measurePeakMemory(command)
		if err != nil {
			return row, fmt.Errorf("configured status memory observation: %w", err)
		}
		if !json.Valid(output.Bytes()) {
			return row, errors.New("configured status memory observation did not return JSON")
		}
		if sample >= 5 {
			row.Bytes = append(row.Bytes, peak)
		}
	}
	return row, nil
}

func reviewPeakMemory(rows []memoryMeasurement) error {
	type boundary struct {
		variant string
		block   int
	}
	if len(rows) != 4 {
		return errors.New("peak-memory acceptance requires two candidate and predecessor blocks")
	}
	peaks := make(map[boundary]uint64)
	for _, row := range rows {
		key := boundary{row.Variant, row.Block}
		if row.Case != "status" || (row.Variant != "candidate" && row.Variant != "baseline") ||
			row.Block < 1 || row.Block > 2 || len(row.Bytes) != 40 || slices.Contains(row.Bytes, 0) || peaks[key] != 0 {
			return errors.New("peak-memory acceptance requires unique status blocks with forty positive native observations")
		}
		peaks[key] = slices.Max(row.Bytes)
	}
	for block := 1; block <= 2; block++ {
		baseline, candidate := peaks[boundary{"baseline", block}], peaks[boundary{"candidate", block}]
		if baseline == 0 || candidate == 0 {
			return errors.New("peak-memory review requires matched predecessor and candidate blocks")
		}
		if candidate > baseline && candidate-baseline > 4<<20 && float64(candidate) > 1.2*float64(baseline) {
			return fmt.Errorf("status block %d peak memory requires review: %d -> %d bytes exceeds both 20%% and 4 MiB", block, baseline, candidate)
		}
	}
	return nil
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
		var rows []memoryMeasurement
		for block := 1; block <= 2; block++ {
			for _, program := range []struct {
				variant string
				peak    uint64
			}{{"baseline", baseline}, {"candidate", test.candidate}} {
				peaks := make([]uint64, 40)
				for index := range peaks {
					peaks[index] = program.peak
				}
				rows = append(rows, memoryMeasurement{Variant: program.variant, Case: "status", Block: block, Bytes: peaks})
			}
		}
		if err := reviewPeakMemory(rows); (err != nil) != test.review {
			t.Fatalf("candidate peak %d: review=%t error=%v", test.candidate, test.review, err)
		}
	}
	if err := reviewPeakMemory(nil); err == nil {
		t.Fatal("empty evidence qualified as completed memory acceptance")
	}
	if err := reviewPeakMemory([]memoryMeasurement{{Variant: "candidate", Case: "status", Block: 1, Bytes: []uint64{baseline}}}); err == nil {
		t.Fatal("candidate memory was accepted without a matching predecessor observation")
	}
}

func TestNativePeakMemory(t *testing.T) {
	parentMemory := make([]byte, 128<<20)
	for offset := 0; offset < len(parentMemory); offset += os.Getpagesize() {
		parentMemory[offset] = 1
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var peaks []uint64
	for _, size := range []string{"16", "64", "16"} {
		command := exec.CommandContext(t.Context(), program, "-test.run=^TestNativePeakMemoryChild$")
		command.Env = append(os.Environ(), "AIGW_MEMORY_ALLOCATION="+size)
		peak, err := measurePeakMemory(command)
		if err != nil || peak == 0 {
			t.Fatalf("native peak %s MiB = %d, %v", size, peak, err)
		}
		peaks = append(peaks, peak)
	}
	runtime.KeepAlive(parentMemory)
	t.Logf("independent child peaks with a 128 MiB parent allocation: %v", peaks)
	if peaks[1] < peaks[0]+32<<20 || peaks[1] < peaks[2]+32<<20 {
		t.Fatalf("per-process peaks lost allocation or inherited previous child usage: %v", peaks)
	}
	if _, err := measurePeakMemory(exec.CommandContext(t.Context(), filepath.Join(t.TempDir(), "missing"))); err == nil {
		t.Fatal("missing executable produced memory evidence")
	}
}

func TestNativePeakMemoryChild(t *testing.T) {
	value := os.Getenv("AIGW_MEMORY_ALLOCATION")
	if value == "" {
		return
	}
	size, err := strconv.Atoi(value)
	if err != nil || size < 1 || size > 64 {
		t.Fatal("invalid synthetic memory allocation")
	}
	pages := make([]byte, size<<20)
	for offset := 0; offset < len(pages); offset += os.Getpagesize() {
		pages[offset] = 1
	}
	runtime.KeepAlive(pages)
}

func TestNativeMemoryRetainsInterruptedObservations(t *testing.T) {
	if output := os.Getenv("AIGW_TEST_PARTIAL_MEMORY"); output != "" {
		program, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		journey := &journeyFixture{testing: t, binary: program, root: output,
			environment: append(os.Environ(), "AIGW_TEST_MEMORY_OBSERVATIONS="+filepath.Join(output, "observations"))}
		row, err := journey.measureMemory("candidate", 1)
		if err != nil {
			t.Error(err)
		}
		rows := []performance.Measurement{{Variant: "candidate", Backend: "env", Case: "status", Block: 1, Raw: "partial.json"}}
		writeNativePerformanceSummary(t, output, os.Getenv("AIGW_TEST_HYPERFINE"), nil, rows, []memoryMeasurement{row}, false)
		return
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	tool, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	command := exec.CommandContext(t.Context(), program, "-test.run=^TestNativeMemoryRetainsInterruptedObservations$")
	command.Env = append(os.Environ(), "AIGW_TEST_PARTIAL_MEMORY="+output,
		"AIGW_TEST_HYPERFINE="+tool,
		"GORACE="+strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0"))
	if err := command.Run(); err == nil {
		t.Fatal("interrupted memory block qualified")
	}
	var summary struct {
		Qualification bool                      `json:"qualification"`
		Blocks        []performance.Measurement `json:"blocks"`
		Pooled        []performance.Measurement `json:"pooled"`
		Memory        []memoryMeasurement       `json:"memory"`
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(output, "summary.json")), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Qualification || len(summary.Blocks) != 1 || summary.Blocks[0].Raw != "partial.json" || summary.Pooled == nil || len(summary.Pooled) != 0 || len(summary.Memory) != 1 {
		t.Fatalf("failed performance summary lost or qualified partial blocks: %#v", summary)
	}
	row := summary.Memory[0]
	if row.Variant != "candidate" || row.Case != "status" || row.Block != 1 || len(row.Bytes) != 2 || slices.Contains(row.Bytes, 0) {
		t.Fatalf("interrupted block lost its two completed native observations: %#v", row)
	}
	if err := reviewPeakMemory([]memoryMeasurement{row}); err == nil {
		t.Fatal("partial memory observations satisfied qualification")
	}
}
