package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Finding is one stable, path-addressable architecture violation.
type Finding struct {
	Rule    string   `json:"rule"`
	Path    string   `json:"path"`
	Line    int      `json:"line,omitempty"`
	Message string   `json:"message"`
	Files   []string `json:"files,omitempty"`
	Prefix  string   `json:"prefix,omitempty"`
	Count   int      `json:"count,omitempty"`
	Name    string   `json:"name,omitempty"`
	Package string   `json:"package,omitempty"`
}

// Report is the stable JSON document emitted by the gate.
type Report struct {
	OK       bool           `json:"ok"`
	Policy   string         `json:"policy"`
	Root     string         `json:"root"`
	Summary  map[string]int `json:"summary"`
	Findings []Finding      `json:"findings"`
}

func newReport(policyPath, root string) Report {
	return Report{
		OK:       true,
		Policy:   toPOSIX(policyPath),
		Root:     toPOSIX(root),
		Summary:  map[string]int{},
		Findings: []Finding{},
	}
}

func (r *Report) addFinding(f Finding) {
	r.Findings = append(r.Findings, f)
	r.OK = false
	r.Summary[f.Rule]++
	r.Summary["total"]++
}

func (r *Report) finalize() {
	sort.SliceStable(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Prefix != b.Prefix {
			return a.Prefix < b.Prefix
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Message < b.Message
	})
	if r.Summary == nil {
		r.Summary = map[string]int{}
	}
	if _, ok := r.Summary["total"]; !ok {
		r.Summary["total"] = 0
	}
}

func writeReport(w io.Writer, report Report) error {
	report.finalize()
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}

func toPOSIX(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}
