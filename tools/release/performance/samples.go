// Package performance owns retained native measurement execution and evidence.
package performance

import (
	"errors"
	"fmt"
	"math"
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
	Variant     string    `json:"variant"`
	Backend     string    `json:"backend"`
	Case        string    `json:"case"`
	Block       int       `json:"block"`
	P95         float64   `json:"p95_seconds"`
	Budget      float64   `json:"budget_seconds"`
	Raw         string    `json:"raw"`
	Diagnostics bool      `json:"diagnostics"`
	Samples     Samples   `json:"-"`
	Executable  *Identity `json:"selected_executable,omitempty"`
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
			group.Block, group.Raw, group.Samples = 0, "", Samples{}
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
