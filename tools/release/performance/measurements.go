package performance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"
)

// Identity describes the selected file, not an executed process or shell redirection.
type Identity struct {
	Path          string   `json:"path"`
	Format        string   `json:"format"`
	Machine       uint32   `json:"machine"`
	Arch          string   `json:"arch"`
	SHA256        string   `json:"sha256"`
	Bytes         int      `json:"bytes"`
	Architectures []string `json:"architectures,omitempty"`
}

// Identify uses native executable parsers on the same bytes it hashes.
func Identify(path string) (Identity, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return Identity{}, fmt.Errorf("selected executable is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Identity{}, err
	}
	identity := Identity{Path: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Bytes: len(data)}
	switch {
	case bytes.HasPrefix(data, []byte("MZ")):
		file, err := pe.NewFile(bytes.NewReader(data))
		if err != nil {
			return Identity{}, err
		}
		identity.Format, identity.Machine = "PE", uint32(file.Machine)
		identity.Arch = map[uint16]string{pe.IMAGE_FILE_MACHINE_AMD64: "amd64", pe.IMAGE_FILE_MACHINE_ARM64: "arm64", pe.IMAGE_FILE_MACHINE_I386: "386"}[file.Machine]
	case bytes.HasPrefix(data, []byte("\x7fELF")):
		file, err := elf.NewFile(bytes.NewReader(data))
		if err != nil {
			return Identity{}, err
		}
		identity.Format, identity.Machine = "ELF", uint32(file.Machine)
		identity.Arch = map[elf.Machine]string{elf.EM_X86_64: "amd64", elf.EM_AARCH64: "arm64", elf.EM_386: "386"}[file.Machine]
	default:
		if fat, err := macho.NewFatFile(bytes.NewReader(data)); err == nil {
			identity.Format, identity.Arch = "Mach-O", "universal"
			for _, slice := range fat.Arches {
				arch := map[macho.Cpu]string{macho.CpuAmd64: "amd64", macho.CpuArm64: "arm64", macho.Cpu386: "386"}[slice.Cpu]
				if arch == "" {
					return Identity{}, fmt.Errorf("unsupported universal Mach-O CPU %#x", slice.Cpu)
				}
				identity.Architectures = append(identity.Architectures, arch)
			}
			return identity, nil
		}
		file, err := macho.NewFile(bytes.NewReader(data))
		if err != nil {
			return Identity{}, err
		}
		identity.Format, identity.Machine = "Mach-O", uint32(file.Cpu)
		identity.Arch = map[macho.Cpu]string{macho.CpuAmd64: "amd64", macho.CpuArm64: "arm64", macho.Cpu386: "386"}[file.Cpu]
	}
	if identity.Arch == "" {
		return Identity{}, fmt.Errorf("selected executable has unsupported %s machine %#x", identity.Format, identity.Machine)
	}
	return identity, nil
}

// Command binds one Hyperfine invocation to caller-owned output and environment.
type Command struct {
	Tool, Directory, Output           string
	Arguments, Environment, Sensitive []string
	Measurement                       Measurement
}

// Measure retains raw samples and separate redacted streams before validation.
func Measure(parent context.Context, input Command) (Measurement, error) {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	stdout, stderr, runErr := (process.Runner{StdoutLimit: 4 << 20}).RunCaptureStreams(ctx, process.Plan{
		Executable: input.Tool, Args: input.Arguments, Env: input.Environment, Directory: input.Directory,
	})
	for stream, log := range map[string][]byte{"stdout": stdout, "stderr": stderr} {
		if err := os.WriteFile(strings.TrimSuffix(input.Output, ".json")+"."+stream, []byte(redaction.Text(string(log), input.Sensitive...)), 0o600); err != nil {
			return Measurement{}, err
		}
	}
	if runErr != nil {
		return Measurement{}, fmt.Errorf("Hyperfine: %s; separate redacted streams retained", redaction.Text(runErr.Error(), input.Sensitive...))
	}
	data, err := os.ReadFile(input.Output)
	if err != nil {
		return Measurement{}, err
	}
	var report struct {
		Results []Samples `json:"results"`
	}
	if err := json.Unmarshal(data, &report); err != nil || len(report.Results) != 1 {
		return Measurement{}, errors.New("Hyperfine must produce one nonempty result")
	}
	row := input.Measurement
	row.Samples = report.Results[0]
	row.P95, err = row.Samples.Percentile()
	if err != nil {
		return Measurement{}, err
	}
	row.Raw, row.Diagnostics = filepath.Base(input.Output), process.DiagnosticFailure(stderr)
	return row, nil
}

// MeasureOperation times the existing native API with its own bounded worker
// environment. Native subprocess launch is included; Hyperfine and shell are not.
func MeasureOperation(ctx context.Context, output string, row Measurement, operation func() error) (Measurement, error) {
	var stop error
	for index := range 45 {
		if err := ctx.Err(); err != nil {
			stop = err
			break
		}
		start := time.Now()
		err := operation()
		elapsed := time.Since(start).Seconds()
		if index >= 5 {
			row.Samples.Times = append(row.Samples.Times, elapsed)
			code := 0
			if err != nil {
				code = 1
			}
			row.Samples.ExitCodes = append(row.Samples.ExitCodes, code)
		}
		if err != nil {
			stop = err
			break
		}
	}
	row.Raw = filepath.Base(output)
	data, err := json.MarshalIndent(struct {
		Results []Samples `json:"results"`
	}{[]Samples{row.Samples}}, "", "  ")
	if err != nil {
		return Measurement{}, err
	}
	if err := os.WriteFile(output, append(data, '\n'), 0o600); err != nil {
		return Measurement{}, err
	}
	row.P95, err = row.Samples.Percentile()
	return row, errors.Join(stop, err)
}

// Argv renders exact arguments for Hyperfine's shell-free command parser.
func Argv(args ...string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
}

// Workload binds the original command, optional preparation and declared budget.
type Workload struct {
	Name, Command, Prepare string
	Budget                 float64
}

// Arguments retains the official five-warmup, forty-sample Hyperfine contract.
func (c Workload) Arguments(raw string) []string {
	args := []string{"--shell=none", "--warmup", "5", "--runs", "40", "--output=inherit", "--style", "basic", "--export-json", raw}
	if c.Prepare != "" {
		args = append(args, "--prepare", c.Prepare)
	}
	return append(args, c.Command)
}
