package performance

import (
	"slices"
	"strings"
	"testing"
)

func TestExecutionRequiresNativeProcessOwnership(t *testing.T) {
	image := Identity{Path: "C:\\owned\\program.exe", Format: "PE", Machine: 0x8664, Arch: "amd64", SHA256: strings.Repeat("a", 64), Bytes: 1}
	observed := []Execution{{PID: 2, ParentPID: 1, Created: 20, ParentCreated: 10, Role: "controller", Image: image,
		Machine: 0x8664, Arch: "amd64", NativeMachine: 0xaa64, Attributes: 1}}
	if err := reviewExecution(&image, observed, "windows"); err != nil {
		t.Fatalf("native x64-on-ARM was inferred to be invalid: %v", err)
	}
	for name, change := range map[string]func(*Execution){
		"missing-creation":           func(p *Execution) { p.Created = 0 },
		"missing-parent":             func(p *Execution) { p.ParentPID = 0 },
		"missing-parent-creation":    func(p *Execution) { p.ParentCreated = 0 },
		"reused-parent":              func(p *Execution) { p.ParentCreated = p.Created + 1 },
		"missing-role":               func(p *Execution) { p.Role = "" },
		"missing-native-machine":     func(p *Execution) { p.NativeMachine = 0 },
		"inconsistent-wow64":         func(p *Execution) { p.WOW64Machine = 0x014c },
		"missing-machine-attributes": func(p *Execution) { p.Attributes = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := slices.Clone(observed)
			change(&changed[0])
			if err := reviewExecution(&image, changed, "windows"); err == nil {
				t.Fatal("incomplete native process ownership qualified")
			}
		})
	}
}

func TestExecutionRequiresTheOriginalCredentialChain(t *testing.T) {
	image := Identity{Path: "C:\\owned\\program.exe", Format: "PE", Machine: 0x8664, Arch: "amd64", SHA256: strings.Repeat("a", 64), Bytes: 1}
	process := func(pid, parent uint32, created, parentCreated uint64, role string) Execution {
		return Execution{PID: pid, ParentPID: parent, Created: created, ParentCreated: parentCreated, Role: role, Image: image,
			Machine: 0x8664, Arch: "amd64", NativeMachine: 0xaa64, Attributes: 1}
	}
	row := Measurement{Case: "credential", Backend: "keyring", Executable: &image, Reader: &image,
		ControllerExecution: []Execution{process(2, 1, 20, 10, "controller")},
		Execution:           []Execution{process(3, 2, 30, 20, "workload"), process(4, 3, 40, 30, "reader"), process(5, 4, 50, 40, "credential-worker")}}
	if err := reviewWorkloadExecution(row, image, "windows"); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Measurement){
		"missing-reader":       func(r *Measurement) { r.Execution[1].Role = "descendant" },
		"missing-worker":       func(r *Measurement) { r.Execution = r.Execution[:2] },
		"different-controller": func(r *Measurement) { r.ControllerExecution[0].PID = 10 },
		"reused-parent":        func(r *Measurement) { r.Execution[2].ParentCreated++ },
		"parallel-worker":      func(r *Measurement) { r.Execution[2].ParentPID, r.Execution[2].ParentCreated = 3, 30 },
		"reader-image":         func(r *Measurement) { r.Execution[1].Image.SHA256 = strings.Repeat("b", 64) },
		"workload-machine":     func(r *Measurement) { r.Execution[0].NativeMachine = 0x8664 },
		"reader-machine":       func(r *Measurement) { r.Execution[1].NativeMachine = 0x8664 },
		"worker-machine":       func(r *Measurement) { r.Execution[2].NativeMachine = 0x8664 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := row
			changed.Execution = slices.Clone(row.Execution)
			changed.ControllerExecution = slices.Clone(row.ControllerExecution)
			change(&changed)
			if err := reviewWorkloadExecution(changed, image, "windows"); err == nil {
				t.Fatal("unproved native credential chain qualified")
			}
		})
	}
	row.Backend, row.Execution = "env", row.Execution[:2]
	if err := reviewWorkloadExecution(row, image, "windows"); err != nil {
		t.Fatalf("environment credentials incorrectly require a native worker: %v", err)
	}
}
