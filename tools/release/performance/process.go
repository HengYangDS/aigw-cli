package performance

import (
	"errors"
	"fmt"
)

func processArchitecture(machine uint16) string {
	return map[uint16]string{0x8664: "amd64", 0xaa64: "arm64", 0x014c: "386"}[machine]
}

func reviewExecution(selected *Identity, observed []Execution, platform string) error {
	if selected == nil || selected.Path == "" || len(selected.SHA256) != 64 || selected.Bytes < 1 || selected.Arch == "" {
		return errors.New("performance execution identity is incomplete")
	}
	if platform != "windows" {
		return nil
	}
	if len(observed) == 0 {
		return errors.New("performance execution identity is incomplete")
	}
	parents := make(map[uint32]Execution)
	for index, process := range observed {
		if err := process.review(); err != nil {
			return err
		}
		if index != 0 {
			parent, ok := parents[process.ParentPID]
			if !ok || !parent.owns(process) {
				return errors.New("performance process relationship is unproved")
			}
		}
		if _, repeated := parents[process.PID]; repeated {
			return errors.New("performance process identity is repeated")
		}
		parents[process.PID] = process
	}
	if !selected.sameFile(observed[0].Image) {
		return errors.New("performance process image differs from its selected file")
	}
	return nil
}

func (process Execution) review() error {
	arch := processArchitecture(process.Machine)
	if process.PID == 0 || process.ParentPID == 0 || process.Created == 0 || process.ParentCreated == 0 || process.ParentCreated > process.Created || process.Role == "" ||
		arch == "" || process.Arch != arch || processArchitecture(process.NativeMachine) == "" || process.Attributes&1 == 0 ||
		process.WOW64Machine != 0 && process.WOW64Machine != process.Machine ||
		len(process.Image.SHA256) != 64 || process.Image.Path == "" || process.Image.Bytes < 1 {
		return errors.New("performance execution identity is incomplete")
	}
	return nil
}

func (process Execution) owns(child Execution) bool {
	return process.PID == child.ParentPID && process.Created == child.ParentCreated && process.NativeMachine == child.NativeMachine
}

func reviewWorkloadExecution(row Measurement, controller Identity, platform string) error {
	if err := reviewExecution(row.Executable, row.Execution, platform); err != nil || platform != "windows" {
		return err
	}
	if err := reviewExecution(&controller, row.ControllerExecution, platform); err != nil {
		return err
	}
	root, host := row.Execution[0], row.ControllerExecution[0]
	if len(row.ControllerExecution) != 1 || host.Role != "controller" || root.Role != "workload" || !host.owns(root) {
		return errors.New("performance workload is not owned by its observed controller")
	}
	if row.Case != "credential" {
		return nil
	}
	var reader *Execution
	worker := false
	for index := 1; index < len(row.Execution); index++ {
		process := &row.Execution[index]
		switch process.Role {
		case "reader":
			if reader != nil || row.Reader == nil || !row.Reader.sameFile(process.Image) || !root.owns(*process) {
				return errors.New("performance projected reader is unproved")
			}
			reader = process
		case "credential-worker":
			if reader == nil || !row.Reader.sameFile(process.Image) || !reader.owns(*process) {
				return errors.New("performance native credential worker is unproved")
			}
			worker = true
		}
	}
	if reader == nil || row.Backend == "keyring" && !worker {
		return fmt.Errorf("performance credential chain is incomplete for %s", row.Backend)
	}
	return nil
}
