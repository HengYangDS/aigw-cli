//go:build performance_acceptance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/redaction"
	"aigw-cli/tools/release/performance"
)

type processMeasurement struct {
	Variant       string        `json:"variant"`
	Backend       string        `json:"backend"`
	Block         int           `json:"block"`
	Phase         string        `json:"phase"`
	Index         int           `json:"index"`
	Warmup        bool          `json:"warmup"`
	PID           int           `json:"pid"`
	ExitCode      int           `json:"exit_code"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	WallSeconds   float64       `json:"wall_seconds"`
	UserSeconds   float64       `json:"user_seconds"`
	SystemSeconds float64       `json:"system_seconds"`
	Usage         *processUsage `json:"native_usage"`
}

type processUsage struct {
	MaxRSS       int64  `json:"max_rss"`
	RSSUnit      string `json:"max_rss_unit"`
	MinorFaults  int64  `json:"minor_faults"`
	MajorFaults  int64  `json:"major_faults"`
	InputBlocks  int64  `json:"input_blocks"`
	OutputBlocks int64  `json:"output_blocks"`
	Voluntary    int64  `json:"voluntary_context_switches"`
	Involuntary  int64  `json:"involuntary_context_switches"`
}

// measureWorkloadProcesses observes an exact prepared workload inside accept-native's
// existing bounded suite group/Job. It adds no independent process controller.
func (j *journeyFixture) measureWorkloadProcesses(parent context.Context, path string, row performance.Measurement, workload performance.Workload) (result performance.Measurement, resultErr error) {
	if row.Executable == nil || len(workload.Command) == 0 || workload.Command[0] != row.Executable.Path || len(workload.Prepare) != 0 && workload.Prepare[0] != row.Executable.Path {
		return row, errors.New("workload diagnosis requires one exact executable identity")
	}
	verify := func() error {
		observed, err := performance.Identify(row.Executable.Path)
		if err == nil && !reflect.DeepEqual(observed, *row.Executable) {
			err = errors.New("workload diagnostic executable identity changed")
		}
		return err
	}
	if err := verify(); err != nil {
		return row, err
	}
	recordPath := strings.TrimSuffix(path, ".json") + ".processes.json"
	recordFile, err := os.OpenFile(recordPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return row, err
	}
	defer func() { resultErr = errors.Join(resultErr, recordFile.Close()) }()
	var records []processMeasurement
	index := 0
	run := func(phase string, argv []string) error {
		ctx, cancel := context.WithTimeout(parent, 5*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, argv[0], argv[1:]...)
		command.Dir, command.Env = j.root, j.environment
		command.WaitDelay = 2 * time.Second
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		start := time.Now()
		runErr := command.Run()
		end := time.Now()
		if state := command.ProcessState; state != nil {
			sample := index
			if sample >= 5 {
				sample -= 5
			}
			records = append(records, processMeasurement{
				Variant: row.Variant, Backend: row.Backend, Block: row.Block, Phase: phase, Index: sample, Warmup: index < 5,
				PID: state.Pid(), ExitCode: state.ExitCode(), StartedAt: start.UTC(), FinishedAt: end.UTC(), WallSeconds: end.Sub(start).Seconds(),
				UserSeconds: state.UserTime().Seconds(), SystemSeconds: state.SystemTime().Seconds(), Usage: completedProcessUsage(state),
			})
		}
		for stream, data := range map[string][]byte{"stdout": stdout.Bytes(), "stderr": stderr.Bytes()} {
			file, err := os.OpenFile(strings.TrimSuffix(path, ".json")+"."+stream, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return errors.Join(runErr, err)
			}
			_, writeErr := fmt.Fprintf(file, "%s warmup=%t index=%d\n%s", phase, index < 5, index, redaction.Text(string(data), j.sensitiveInputs...))
			runErr = errors.Join(runErr, writeErr, file.Close())
		}
		return errors.Join(runErr, ctx.Err())
	}
	row.Samples.Command = performance.Argv(workload.Command...)
	row, runErr := performance.MeasureOperation(parent, path, row, func() error {
		if len(workload.Prepare) != 0 {
			if err := run("prepare", workload.Prepare); err != nil {
				return err
			}
		}
		err := run("sample", workload.Command)
		index++
		return err
	})
	cpuScope := "completed process and waited-for descendants, as reported by native ProcessState"
	if runtime.GOOS == "windows" {
		cpuScope = "completed process only; descendant credential workers are excluded"
	}
	data, encodeErr := json.MarshalIndent(struct {
		Qualification bool                  `json:"qualification"`
		Scope         string                `json:"scope"`
		Case          string                `json:"case"`
		CPUAccounting string                `json:"cpu_accounting"`
		Limits        string                `json:"limits"`
		Executable    *performance.Identity `json:"executable"`
		Records       []processMeasurement  `json:"records"`
	}{false, "workload-process-attribution", row.Case, cpuScope,
		"Row timings include prepare, sample and evidence writes; sample records alone measure each completed command. Neither is Hyperfine qualification. Wall-minus-CPU does not distinguish scheduler, storage or IPC waits. Zero or unavailable counters do not prove absence of work.", row.Executable, records}, "", "  ")
	_, writeErr := recordFile.Write(append(data, '\n'))
	return row, errors.Join(runErr, verify(), encodeErr, writeErr)
}

func TestNativePreparedWorkloadRetainsCompletedProcessObservations(t *testing.T) {
	if phase := os.Getenv("AIGW_TEST_PROCESS_PHASE"); phase != "" {
		if code := map[string]int{"failure/sample": 7, "prepare-failure/prepare": 7}[phase+"/"+os.Args[len(os.Args)-1]]; code != 0 {
			os.Exit(code)
		}
		if phase == "deadline" {
			time.Sleep(time.Minute)
		}
		return
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := performance.Identify(program)
	if err != nil {
		t.Fatal(err)
	}
	deadlineExit := -1
	if runtime.GOOS == "windows" {
		deadlineExit = 1
	}
	for _, test := range []struct {
		phase       string
		workload    string
		backend     string
		count, exit int
		timeout     time.Duration
	}{
		{"success", "projection", "env", 90, 0, 30 * time.Second},
		{"success", "sync", "keyring", 90, 0, 30 * time.Second},
		{"failure", "sync", "keyring", 2, 7, 30 * time.Second},
		{"prepare-failure", "sync", "keyring", 1, 7, 30 * time.Second},
		{"deadline", "sync", "keyring", 1, deadlineExit, 100 * time.Millisecond},
	} {
		t.Run(test.phase+"/"+test.workload, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), test.timeout)
			defer cancel()
			journey := &journeyFixture{testing: t, root: t.TempDir(), binary: program,
				environment: append(os.Environ(), "AIGW_TEST_PROCESS_PHASE="+test.phase, "GORACE=atexit_sleep_ms=0")}
			path := filepath.Join(journey.root, "projection.json")
			base := []string{program, "-test.run=^TestNativePreparedWorkloadRetainsCompletedProcessObservations$"}
			workload := performance.Workload{Name: test.workload, Command: append(slices.Clone(base), "sample"), Prepare: append(slices.Clone(base), "prepare")}
			row, runErr := journey.measureWorkloadProcesses(ctx, path, performance.Measurement{Variant: "candidate", Backend: test.backend, Case: test.workload, Block: 2, Executable: &identity}, workload)
			if test.phase == "deadline" && !errors.Is(runErr, context.DeadlineExceeded) {
				t.Fatalf("native deadline was not retained: %v", runErr)
			}
			if (runErr != nil) != (test.exit != 0) {
				t.Fatalf("process result was lost: %v", runErr)
			}
			var report struct {
				Qualification bool                 `json:"qualification"`
				Scope         string               `json:"scope"`
				Case          string               `json:"case"`
				Records       []processMeasurement `json:"records"`
			}
			if err := json.Unmarshal(readFile(t, strings.TrimSuffix(path, ".json")+".processes.json"), &report); err != nil {
				t.Fatal(err)
			}
			if test.exit == 0 && (len(row.Samples.Times) != 40 || row.Raw != "projection.json") {
				t.Fatalf("measured samples lost their output binding: %#v", row)
			}
			if report.Qualification || report.Scope != "workload-process-attribution" || report.Case != test.workload || len(report.Records) != test.count {
				t.Fatalf("process evidence was lost or qualified: %#v", report)
			}
			requireProcessMeasurements(t, report.Records, test.backend, test.exit)
		})
	}
}

// requireProcessMeasurements checks the same native record contract on successful
// and interrupted prepared journeys, including partial final observations.
func requireProcessMeasurements(t *testing.T, records []processMeasurement, backend string, finalExit int) {
	t.Helper()
	for index, record := range records {
		if record.PID <= 0 || record.WallSeconds <= 0 || record.FinishedAt.Before(record.StartedAt) || record.Phase != []string{"prepare", "sample"}[index%2] || record.Warmup != (index/2 < 5) || record.Block != 2 || record.Variant != "candidate" || record.Backend != backend {
			t.Fatalf("completed process binding is incomplete: %#v", record)
		}
		wantExit := 0
		if index == len(records)-1 {
			wantExit = finalExit
		}
		if record.ExitCode != wantExit {
			t.Fatalf("native exit status differs: %#v", record)
		}
	}
}

func TestNativeAttributionKeepsTheProjectionWorkload(t *testing.T) {
	journey := &journeyFixture{binary: "installed program"}
	qualified := journey.performanceCases([]string{"projected helper"}, "env", "test preparer")[1]
	for _, test := range journey.attributionCases([]string{"projected helper"}, "selected shell", "copied reader", "exact-scope") {
		if test.Name == "projection" {
			if !reflect.DeepEqual(test, qualified) {
				t.Fatalf("projection diagnosis changed its qualified workload: got=%#v want=%#v", test, qualified)
			}
			return
		}
	}
	t.Fatal("component attribution omits the measured projection workload")
}

func TestNativeAttributionSummaryRetainsItsNonqualifyingScope(t *testing.T) {
	tool, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	var rows []performance.Measurement
	for block := range 2 {
		times := make([]float64, 40)
		for index := range times {
			times[index] = 0.2
		}
		rows = append(rows, performance.Measurement{Variant: "candidate", Backend: "env", Case: "credential",
			Block: block + 1, Budget: 0.1, P95: 0.2, Raw: fmt.Sprintf("credential-%d.json", block+1), Diagnostics: true,
			Samples: performance.Samples{Command: "observed helper", Times: times, ExitCodes: make([]int, 40)}})
	}
	output := t.TempDir()
	writeNativePerformanceSummary(t, output, tool, nil, rows, nil, true)
	var summary struct {
		Qualification bool                      `json:"qualification"`
		Scope         string                    `json:"scope"`
		Blocks        []performance.Measurement `json:"blocks"`
		Pooled        []performance.Measurement `json:"pooled"`
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(output, "summary.json")), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Qualification || summary.Scope != "component-attribution" || len(summary.Blocks) != 2 || len(summary.Pooled) != 1 ||
		!summary.Pooled[0].Diagnostics || summary.Pooled[0].Budget != 0.1 || summary.Pooled[0].P95 != 0.2 || summary.Blocks[0].Raw != rows[0].Raw {
		t.Fatalf("diagnosis claimed qualification or discarded its evidence: %#v", summary)
	}
}

func (j *journeyFixture) measureMemory(variant string, block int) (performance.Memory, error) {
	j.testing.Helper()
	row := performance.Memory{Variant: variant, Case: "status", Block: block, Bytes: []uint64{}}
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
		var rows []performance.Memory
		for block := 1; block <= 2; block++ {
			for _, program := range []struct {
				variant string
				peak    uint64
			}{{"baseline", baseline}, {"candidate", test.candidate}} {
				peaks := make([]uint64, 40)
				for index := range peaks {
					peaks[index] = program.peak
				}
				rows = append(rows, performance.Memory{Variant: program.variant, Case: "status", Block: block, Bytes: peaks})
			}
		}
		if err := performance.ReviewMemory(rows); (err != nil) != test.review {
			t.Fatalf("candidate peak %d: review=%t error=%v", test.candidate, test.review, err)
		}
	}
	if err := performance.ReviewMemory(nil); err == nil {
		t.Fatal("empty evidence qualified as completed memory acceptance")
	}
	if err := performance.ReviewMemory([]performance.Memory{{Variant: "candidate", Case: "status", Block: 1, Bytes: []uint64{baseline}}}); err == nil {
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
		writeNativePerformanceSummary(t, output, os.Getenv("AIGW_TEST_HYPERFINE"), nil, rows, []performance.Memory{row}, false)
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
		Memory        []performance.Memory      `json:"memory"`
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
	if err := performance.ReviewMemory([]performance.Memory{row}); err == nil {
		t.Fatal("partial memory observations satisfied qualification")
	}
}

func TestNativePerformanceSummaryRejectsUnprovedExecution(t *testing.T) {
	if os.Getenv("AIGW_TEST_UNPROVED_PERFORMANCE") == "1" {
		var rows []performance.Measurement
		for block := range 2 {
			times := make([]float64, 40)
			for index := range times {
				times[index] = 0.01
			}
			rows = append(rows, performance.Measurement{Variant: "candidate", Backend: "env", Case: "credential",
				Block: block + 1, Budget: 0.1, P95: 0.01, Raw: fmt.Sprintf("credential-%d.json", block+1),
				Samples: performance.Samples{Command: "unobserved helper", Times: times, ExitCodes: make([]int, 40)}})
		}
		writeNativePerformanceSummary(t, os.Getenv("AIGW_TEST_PERFORMANCE_OUTPUT"), os.Getenv("AIGW_TEST_HYPERFINE"), nil, rows, nil, false)
		return
	}
	tool, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	command := exec.CommandContext(t.Context(), program, "-test.run=^TestNativePerformanceSummaryRejectsUnprovedExecution$")
	command.Env = append(os.Environ(), "AIGW_TEST_UNPROVED_PERFORMANCE=1", "AIGW_TEST_PERFORMANCE_OUTPUT="+output, "AIGW_TEST_HYPERFINE="+tool)
	result, runErr := command.CombinedOutput()
	var summary struct {
		Qualification bool   `json:"qualification"`
		Scope         string `json:"scope"`
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(output, "summary.json")), &summary); err != nil {
		t.Fatal(err)
	}
	if runErr == nil || summary.Qualification || summary.Scope != "full-performance" || !strings.Contains(string(result), "performance execution identity is incomplete") {
		t.Fatalf("unproved execution qualified: %#v, %v\n%s", summary, runErr, result)
	}
}
