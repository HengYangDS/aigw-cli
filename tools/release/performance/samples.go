// Package performance owns retained native measurement execution and evidence.
package performance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Samples retains successful command durations without credential values.
type Samples struct {
	Command   string    `json:"command,omitempty"`
	Times     []float64 `json:"times"`
	ExitCodes []int     `json:"exit_codes"`
}

// Percentile returns nearest-rank p95 without mutating raw evidence.
func (s Samples) Percentile() (float64, error) {
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

// Measurement binds one retained timing block to its workload and budget.
// A zero budget denotes component diagnosis, never performance qualification.
type Measurement struct {
	Variant             string      `json:"variant"`
	Backend             string      `json:"backend"`
	Case                string      `json:"case"`
	Command             []string    `json:"command"`
	Block               int         `json:"block"`
	P95                 float64     `json:"p95_seconds"`
	Budget              float64     `json:"budget_seconds"`
	Raw                 string      `json:"raw"`
	Diagnostics         bool        `json:"diagnostics"`
	Samples             Samples     `json:"-"`
	Executable          *Identity   `json:"selected_executable,omitempty"`
	Reader              *Identity   `json:"selected_credential_reader,omitempty"`
	Execution           []Execution `json:"execution,omitempty"`
	ControllerExecution []Execution `json:"controller_execution,omitempty"`
}

// Pooled combines exactly two forty-sample blocks for each measured boundary.
func Pooled(measurements []Measurement) ([]Measurement, error) {
	if len(measurements) == 0 {
		return nil, errors.New("pooled performance requires measured blocks")
	}
	groups := make(map[string]Measurement)
	blocks := make(map[string]uint8)
	for _, row := range measurements {
		key := row.Variant + "/" + row.Backend + "/" + row.Case
		if row.Block < 1 || row.Block > 2 || len(row.Samples.Times) != 40 {
			return nil, fmt.Errorf("%s requires two distinct forty-sample blocks", key)
		}
		if p95, err := row.Samples.Percentile(); err != nil || row.P95 != p95 {
			return nil, errors.Join(err, fmt.Errorf("%s has an unqualified measurement block", key))
		}
		bit := uint8(1 << (row.Block - 1))
		if blocks[key]&bit != 0 {
			return nil, fmt.Errorf("%s repeats block %d", key, row.Block)
		}
		blocks[key] |= bit
		group, exists := groups[key]
		if exists && group.Budget != row.Budget {
			return nil, fmt.Errorf("%s changed its measured budget", key)
		}
		if exists && ((group.Executable == nil) != (row.Executable == nil) ||
			(group.Executable != nil && !group.Executable.sameFile(*row.Executable))) {
			return nil, fmt.Errorf("%s changed its executable byte or platform identity", key)
		}
		if !exists {
			group = row
			group.Block, group.Raw, group.Command, group.Samples, group.Execution, group.ControllerExecution = 0, "", nil, Samples{}, nil, nil
			if row.Executable != nil {
				selected := *row.Executable
				selected.Path = ""
				group.Executable = &selected
			}
		}
		group.Diagnostics = group.Diagnostics || row.Diagnostics
		group.Samples.Times = append(group.Samples.Times, row.Samples.Times...)
		group.Samples.ExitCodes = append(group.Samples.ExitCodes, row.Samples.ExitCodes...)
		groups[key] = group
	}
	var pooled []Measurement
	for key, row := range groups {
		if blocks[key] != 3 {
			return nil, fmt.Errorf("%s lacks two complete forty-sample blocks", key)
		}
		var err error
		row.P95, err = row.Samples.Percentile()
		if err != nil {
			return nil, err
		}
		pooled = append(pooled, row)
	}
	slices.SortFunc(pooled, func(a, b Measurement) int {
		return strings.Compare(a.Variant+a.Backend+a.Case, b.Variant+b.Backend+b.Case)
	})
	return pooled, nil
}

// Program binds a predecessor or candidate to its measured release bytes.
type Program struct {
	Variant string `json:"variant"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Bytes   int    `json:"bytes"`
}

// Memory retains per-child native peak observations for one workload block.
type Memory struct {
	Variant string   `json:"variant"`
	Case    string   `json:"case"`
	Block   int      `json:"block"`
	Bytes   []uint64 `json:"bytes"`
}

// ReviewMemory applies the retained two-block native peak-memory budget.
func ReviewMemory(rows []Memory) error {
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

// Execution describes a live Windows process, independently of its PE file header.
type Execution struct {
	PID           uint32   `json:"pid"`
	ParentPID     uint32   `json:"parent_pid"`
	Created       uint64   `json:"creation_time"`
	ParentCreated uint64   `json:"parent_creation_time"`
	Role          string   `json:"role"`
	Image         Identity `json:"image"`
	Machine       uint16   `json:"process_machine"`
	Arch          string   `json:"process_arch"`
	WOW64Machine  uint16   `json:"wow64_machine"`
	NativeMachine uint16   `json:"native_machine"`
	Attributes    uint32   `json:"machine_attributes"`
}

// Summary is the single producer and consumer contract for native performance.
type Summary struct {
	Qualification       bool                   `json:"qualification"`
	Scope               string                 `json:"scope"`
	Controllers         map[string]Identity    `json:"selected_controller_files"`
	ControllerExecution map[string][]Execution `json:"controller_execution,omitempty"`
	IdentityScope       string                 `json:"identity_scope"`
	OS                  string                 `json:"os"`
	Arch                string                 `json:"arch"`
	Tool                string                 `json:"tool"`
	MemoryScope         string                 `json:"memory_scope"`
	ClientScope         string                 `json:"client_scope"`
	Programs            []Program              `json:"programs"`
	Blocks              []Measurement          `json:"blocks"`
	Pooled              []Measurement          `json:"pooled"`
	Memory              []Memory               `json:"memory"`
}

// Review checks measured evidence rather than trusting the qualification flag.
func (s Summary) Review() error {
	for _, row := range s.Blocks {
		if err := reviewWorkloadExecution(row, s.Controllers["hyperfine"], s.OS); err != nil {
			return err
		}
	}
	for _, name := range []string{"verifier", "hyperfine"} {
		selected, ok := s.Controllers[name]
		if !ok {
			return errors.New("performance controller identity is incomplete")
		}
		if err := reviewExecution(&selected, s.ControllerExecution[name], s.OS); err != nil {
			return err
		}
	}
	if s.Scope != "full-performance" || s.OS == "" || s.Arch == "" || s.Tool == "" || s.IdentityScope == "" || s.MemoryScope == "" || s.ClientScope == "" || len(s.Programs) != 2 {
		return errors.New("performance summary lacks its complete native scope")
	}
	programs, err := reviewPrograms(s.Programs)
	if err != nil {
		return err
	}
	for _, row := range s.Blocks {
		if err := row.review(programs[row.Variant]); err != nil {
			return err
		}
	}
	pooled, err := Pooled(s.Blocks)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(pooled)
	declared, _ := json.Marshal(s.Pooled)
	if !bytes.Equal(encoded, declared) {
		return errors.New("performance summary differs from its retained samples")
	}
	return errors.Join(reviewWorkloads(pooled, programs), ReviewMemory(s.Memory))
}

func reviewWorkloads(pooled []Measurement, programs map[string]Program) error {
	groups := make(map[string]bool)
	for _, row := range pooled {
		if row.Diagnostics || row.Budget <= 0 || row.Variant == "candidate" && row.P95 > row.Budget || programs[row.Variant].Variant == "" {
			return errors.New("performance measurements are not qualified")
		}
		groups[row.Variant+"/"+row.Backend+"/"+row.Case] = true
	}
	for _, variant := range []string{"baseline", "candidate"} {
		for _, name := range []string{"credential", "projection", "setup", "sync", "version", "help", "status", "export"} {
			if !groups[variant+"/env/"+name] {
				return errors.New("performance summary omits a native workload")
			}
		}
	}
	return nil
}

func reviewPrograms(rows []Program) (map[string]Program, error) {
	programs := make(map[string]Program)
	for _, program := range rows {
		if (program.Variant != "baseline" && program.Variant != "candidate") || program.Path == "" || len(program.SHA256) != 64 || program.Bytes < 1 {
			return nil, errors.New("performance summary has an invalid released program")
		}
		programs[program.Variant] = program
	}
	if len(programs) != 2 || programs["baseline"].SHA256 == programs["candidate"].SHA256 {
		return nil, errors.New("performance summary requires distinct predecessor and candidate")
	}
	baseline, candidate := programs["baseline"].Bytes, programs["candidate"].Bytes
	if candidate-baseline > 1<<20 && float64(candidate) > float64(baseline)*1.1 {
		return nil, errors.New("executable size growth requires review: exceeds both 10% and 1 MiB")
	}
	return programs, nil
}

func (row Measurement) review(program Program) error {
	if len(row.Command) == 0 || row.Samples.Command != Argv(row.Command...) {
		return errors.New("performance export differs from its requested command")
	}
	if err := (Workload{Name: row.Case, Command: row.Command}).Review(row.Executable); err != nil {
		return err
	}
	if budget := Budget(row.Case); budget == 0 || row.Budget != budget {
		return errors.New("performance workload changed its qualifying budget")
	}
	if row.Diagnostics || row.Variant == "candidate" && row.P95 > row.Budget {
		return errors.New("performance measurements are not qualified")
	}
	selected := row.Executable
	if row.Case == "credential" {
		selected = row.Reader
	}
	if selected == nil || selected.SHA256 != program.SHA256 || selected.Bytes != program.Bytes {
		return errors.New("performance workload differs from its released program")
	}
	return nil
}

// ReadSummary rehydrates retained samples and enforces the same native contract.
func ReadSummary(directory string, attribution bool) (Summary, error) {
	var summary Summary
	data, err := os.ReadFile(filepath.Join(directory, "summary.json"))
	if err != nil {
		return summary, err
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		return summary, err
	}
	if summary.OS != runtime.GOOS || summary.Arch != runtime.GOARCH {
		return summary, errors.New("performance summary differs from its native consumer")
	}
	if attribution {
		if summary.Qualification || summary.Scope != "component-attribution" || len(summary.Blocks) == 0 || len(summary.Pooled) == 0 {
			return summary, errors.New("performance attribution requires nonqualifying diagnostic evidence")
		}
		return summary, nil
	}
	if !summary.Qualification || summary.Scope != "full-performance" {
		return summary, errors.New("performance acceptance requires a qualified full-performance summary")
	}
	for index := range summary.Blocks {
		row := &summary.Blocks[index]
		if row.Raw == "" || filepath.Base(row.Raw) != row.Raw {
			return summary, errors.New("performance raw sample path is invalid")
		}
		data, err := os.ReadFile(filepath.Join(directory, row.Raw))
		if err != nil {
			return summary, err
		}
		samples, err := decodeSamples(data, Argv(row.Command...))
		if err != nil {
			return summary, err
		}
		row.Samples = samples
	}
	return summary, summary.Review()
}
