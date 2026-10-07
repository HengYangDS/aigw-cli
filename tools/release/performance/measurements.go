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
	"slices"
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

func (i Identity) sameFile(other Identity) bool {
	return i.Format == other.Format && i.Machine == other.Machine && i.Arch == other.Arch &&
		i.SHA256 == other.SHA256 && i.Bytes == other.Bytes && slices.Equal(i.Architectures, other.Architectures)
}

func (i Identity) verify() error {
	observed, err := Identify(i.Path)
	if err != nil {
		return err
	}
	if !i.sameFile(observed) {
		return errors.New("selected executable bytes or platform identity changed")
	}
	return nil
}

// Command binds one Hyperfine invocation to caller-owned output and environment.
type Command struct {
	Tool, Directory, Output           string
	Arguments, Environment, Sensitive []string
	Measurement                       Measurement
}

// Measure retains raw samples and separate redacted streams before validation.
func Measure(parent context.Context, input Command) (Measurement, error) {
	row := input.Measurement
	row.Raw, row.P95, row.Samples = "", 0, Samples{}
	if _, err := os.Lstat(input.Output); !errors.Is(err, os.ErrNotExist) {
		return row, errors.New("performance output must be new")
	}
	if len(input.Arguments) == 0 {
		return row, errors.New("performance requires an explicit measured command")
	}
	command := input.Arguments[len(input.Arguments)-1]
	if len(row.Command) != 0 && command != Argv(row.Command...) {
		return row, errors.New("performance export differs from its requested command")
	}
	if row.Executable != nil {
		if err := selectsExecutable(command, row.Executable); err != nil {
			return row, err
		}
		if err := row.Executable.verify(); err != nil {
			return row, err
		}
	}
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	stdout, stderr, runErr := (process.Runner{StdoutLimit: 4 << 20}).RunCaptureStreams(ctx, process.Plan{
		Executable: input.Tool, Args: input.Arguments, Env: input.Environment, Directory: input.Directory,
	})
	if row.Executable != nil {
		runErr = errors.Join(runErr, row.Executable.verify())
	}
	for stream, log := range map[string][]byte{"stdout": stdout, "stderr": stderr} {
		if err := os.WriteFile(strings.TrimSuffix(input.Output, ".json")+"."+stream, []byte(redaction.Text(string(log), input.Sensitive...)), 0o600); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}
	row.Diagnostics = process.DiagnosticFailure(stderr)
	if runErr != nil {
		runErr = fmt.Errorf("Hyperfine: %s; separate redacted streams retained", redaction.Text(runErr.Error(), input.Sensitive...))
	}
	data, err := os.ReadFile(input.Output)
	if err != nil {
		return row, errors.Join(runErr, err)
	}
	row.Raw = filepath.Base(input.Output)
	row.Samples, err = decodeSamples(data, command)
	if err = errors.Join(runErr, err); err != nil {
		return row, err
	}
	row.P95, err = row.Samples.Percentile()
	return row, err
}

func selectsExecutable(command string, selected *Identity) error {
	if selected == nil || selected.Path == "" {
		return errors.New("measured command does not select its declared executable")
	}
	executable := Argv(selected.Path)
	if command != executable && !strings.HasPrefix(command, executable+" ") {
		return errors.New("measured command does not select its declared executable")
	}
	return nil
}

func decodeSamples(data []byte, command string) (Samples, error) {
	var report struct {
		Results []struct {
			Samples
			ExitCodes []*int `json:"exit_codes"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &report); err != nil || len(report.Results) != 1 {
		return Samples{}, errors.New("Hyperfine must produce one nonempty result")
	}
	samples := report.Results[0].Samples
	samples.ExitCodes = []int{}
	for _, code := range report.Results[0].ExitCodes {
		if code == nil {
			return samples, errors.New("performance export requires explicit integer exit codes")
		}
		samples.ExitCodes = append(samples.ExitCodes, *code)
	}
	if samples.Command != command {
		return samples, errors.New("Hyperfine export differs from its requested command")
	}
	if len(samples.Times) != 40 {
		return samples, errors.New("Hyperfine must retain exactly forty measured samples")
	}
	_, err := samples.Percentile()
	return samples, err
}

// MeasureOperation times the existing native API with its own bounded worker
// environment. Native subprocess launch is included; Hyperfine and shell are not.
func MeasureOperation(ctx context.Context, output string, row Measurement, operation func() error) (Measurement, error) {
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return row, errors.New("performance output must be new")
	}
	row.P95, row.Samples = 0, Samples{Command: row.Samples.Command, Times: []float64{}, ExitCodes: []int{}}
	var stop error
	if row.Executable != nil {
		stop = row.Executable.verify()
	}
	for index := range 45 {
		if stop != nil {
			break
		}
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
	if row.Executable != nil {
		stop = errors.Join(stop, row.Executable.verify())
	}
	row.Raw = filepath.Base(output)
	data, err := json.MarshalIndent(struct {
		Results []Samples `json:"results"`
	}{[]Samples{row.Samples}}, "", "  ")
	if err != nil {
		return row, errors.Join(stop, err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0o600); err != nil {
		return row, errors.Join(stop, err)
	}
	p95, err := row.Samples.Percentile()
	if err := errors.Join(stop, err); err != nil {
		return row, err
	}
	row.P95 = p95
	return row, nil
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
	Name             string
	Command, Prepare []string
	Budget           float64
}

// Review binds a qualifying case to its native operation and selected executable.
// Dynamic paths and identifiers stay invocation inputs, not another workload list.
func (c Workload) Review(selected *Identity) error {
	if err := selectsExecutable(Argv(c.Command...), selected); err != nil {
		return err
	}
	args := c.Command[1:]
	valid := false
	switch c.Name {
	case "projection":
		valid = len(args) == 4 && slices.Equal(args[:2], []string{"use", "--for"}) && args[2] != "" && args[3] != ""
	case "setup":
		valid = len(args) == 5 && slices.Equal(args[:2], []string{"setup", "--from"}) && args[2] != "" && args[3] == "--account" && args[4] != ""
	case "sync":
		valid = slices.Equal(args, []string{"sync"})
	case "version":
		valid = slices.Equal(args, []string{"--version"})
	case "help":
		valid = slices.Equal(args, []string{"--help"})
	case "status":
		valid = slices.Equal(args, []string{"status", "--json"})
	case "export":
		valid = slices.Equal(args, []string{"config", "export"})
	case "credential":
		valid = len(args) == 2 && args[0] == "-c" && args[1] != "" || len(args) == 3 && slices.Equal(args[:2], []string{"/d", "/c"}) && args[2] != ""
	}
	if !valid {
		return errors.New("performance command differs from its declared workload")
	}
	return nil
}

// Arguments retains the official five-warmup, forty-sample Hyperfine contract.
func (c Workload) Arguments(raw string) []string {
	args := []string{"--shell=none", "--warmup", "5", "--runs", "40", "--output=inherit", "--style", "basic", "--export-json", raw}
	if len(c.Prepare) != 0 {
		args = append(args, "--prepare", Argv(c.Prepare...))
	}
	return append(args, Argv(c.Command...))
}

// Budget is the original native workload ceiling; other cases are diagnostic.
func Budget(name string) float64 {
	switch name {
	case "projection", "setup", "sync":
		return 0.25
	case "credential", "version", "help", "status", "export":
		return 0.1
	default:
		return 0
	}
}
