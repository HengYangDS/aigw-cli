package performance

import (
	"aigw-cli/internal/configuration"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"aigw-cli/internal/process"
)

func TestWorkloadAcceptsItsResolvedForwardingProjection(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := Identify(program)
	if err != nil {
		t.Fatal(err)
	}
	binding := configuration.Runtime{Client: configuration.ClientClaude, RouteID: "selected-route", AccountID: "account",
		Protocol: configuration.ProtocolAnthropic, UpstreamEndpoint: "https://upstream.example/v1", Endpoint: "http://127.0.0.1:18792/v1"}
	workload := Workload{Name: "projection", Command: []string{program, "use", "--for", "claude", "selected-route", "--forwarding-endpoint", binding.Endpoint}, Mode: "forwarding", Runtime: binding}
	if err := workload.Review(&selected); err != nil {
		t.Fatalf("the declared native forwarding projection was refused: %v", err)
	}
	workload.Command[len(workload.Command)-1] = "http://127.0.0.1:18793/v1"
	if err := workload.Review(&selected); err == nil {
		t.Fatal("a forwarding command detached from its resolved binding qualified")
	}
}

func currentTestImage(t *testing.T) Identity {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := Identify(path)
	if err != nil || image.Path != path {
		t.Fatalf("selected test file identity differs from the executable: %#v, %v", image, err)
	}
	return image
}

func TestIdentityMeasuresTheSelectedNativeFile(t *testing.T) {
	identity := currentTestImage(t)
	if identity.Arch != runtime.GOARCH || identity.Format == "" || identity.Machine == 0 || len(identity.SHA256) != 64 || identity.Bytes <= 0 {
		t.Fatalf("selected file identity is incomplete: %#v", identity)
	}
	invalid := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(invalid, []byte("not a native executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{invalid, invalid + ".missing", filepath.Dir(invalid)} {
		if _, err := Identify(path); err == nil {
			t.Fatal("a missing, invalid or nonregular file acquired native identity")
		}
	}
	for _, header := range [][]byte{[]byte("MZ"), []byte("\x7fELF")} {
		if err := os.WriteFile(invalid, header, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Identify(invalid); err == nil {
			t.Fatal("a truncated native executable acquired file identity")
		}
	}
	if runtime.GOOS == "darwin" {
		shell, err := Identify("/bin/sh")
		if err != nil || shell.Format != "Mach-O" || (shell.Arch != runtime.GOARCH && (shell.Arch != "universal" || !slices.Contains(shell.Architectures, runtime.GOARCH))) {
			t.Fatalf("native shell file identity is incomplete: %#v, %v", shell, err)
		}
	}
}

func TestIdentityRejectsUnrecognizedNativeMachine(t *testing.T) {
	header := elf.Header64{Type: uint16(elf.ET_EXEC), Machine: uint16(elf.EM_NONE), Version: uint32(elf.EV_CURRENT), Ehsize: 64}
	copy(header.Ident[:], elf.ELFMAG)
	header.Ident[elf.EI_CLASS], header.Ident[elf.EI_DATA], header.Ident[elf.EI_VERSION] = byte(elf.ELFCLASS64), byte(elf.ELFDATA2LSB), byte(elf.EV_CURRENT)
	var data bytes.Buffer
	if err := binary.Write(&data, binary.LittleEndian, header); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "unrecognized-machine")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Identify(path); err == nil || !strings.Contains(err.Error(), "unsupported ELF machine") {
		t.Fatalf("unrecognized native machine authorization = %v", err)
	}
}

func TestMeasureOperationRetainsExactCompletedSamples(t *testing.T) {
	for _, fail := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "native-api.json")
		calls := 0
		row, err := MeasureOperation(t.Context(), path, Measurement{}, func() error {
			calls++
			time.Sleep(time.Millisecond)
			if fail && calls == 6 {
				return errors.New("controlled synthetic failure")
			}
			return nil
		})
		if (err != nil) != fail || (!fail && (calls != 45 || len(row.Samples.Times) != 40)) || (fail && calls != 6) {
			t.Fatalf("native API sample contract differs: calls=%d row=%#v err=%v", calls, row, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Results []Samples `json:"results"`
		}
		if err := json.Unmarshal(data, &report); err != nil || len(report.Results) != 1 {
			t.Fatalf("raw native samples lost: %v", err)
		}
		want := 40
		if fail {
			want = 1
		}
		if len(report.Results[0].Times) != want {
			t.Fatal("raw native API samples lost completed attempts")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	canceled := filepath.Join(t.TempDir(), "canceled.json")
	if _, err := MeasureOperation(ctx, canceled, Measurement{}, func() error { t.Fatal("canceled measurement executed"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled native API measurement continued: %v", err)
	}
	data, err := os.ReadFile(canceled)
	if err != nil || !bytes.Contains(data, []byte(`"times": []`)) || !bytes.Contains(data, []byte(`"exit_codes": []`)) {
		t.Fatalf("never-completed observations must retain empty arrays: %s, %v", data, err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	path := filepath.Join(t.TempDir(), "interrupted.json")
	calls := 0
	row, err := MeasureOperation(ctx, path, Measurement{}, func() error {
		calls++
		time.Sleep(time.Millisecond)
		if calls == 6 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || len(row.Samples.Times) != 1 {
		t.Fatalf("interruption discarded completed sample evidence: %#v %v", row, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("interrupted native samples were not retained")
	}
	if _, err := MeasureOperation(t.Context(), filepath.Join(t.TempDir(), "missing", "samples.json"), Measurement{}, func() error { return nil }); err == nil {
		t.Fatal("unretained native samples acquired a measurement")
	}
}

func TestArgvPreservesNativeArguments(t *testing.T) {
	prepared := len(os.Args) == 7 && os.Args[6] == "prepare"
	if prepared && !slices.Equal(os.Args[3:6], []string{"", "a'b", `C:\native\`}) {
		t.Fatal("native preparation changed its empty, apostrophe, or trailing-backslash argument")
	}
	if prepared {
		file, err := os.OpenFile(os.Args[2], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintln(file, "prepared"); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	if got := Argv(`C:\program files\aigw.exe`, "a'b", ""); got != `'C:\program files\aigw.exe' 'a'\''b' ''` {
		t.Fatalf("Hyperfine argv quoting = %s", got)
	}
	workload := Workload{Command: []string{"measured command"}, Prepare: []string{"prepare command"}}
	want := []string{"--shell=none", "--metrics=time", "--warmup", "5", "--runs", "40", "--output=inherit", "--style", "basic", "--export-json", "samples.json", "--prepare", preparationCommand(workload.Prepare), Argv(workload.Command...)}
	if got := workload.Arguments("samples.json"); !slices.Equal(got, want) {
		t.Fatalf("prepared native workload = %q, want %q", got, want)
	}
}

func TestMeasureRetainsSeparateDiagnosticStreams(t *testing.T) {
	const token = "synthetic-performance-diagnostic-token"
	if marker := os.Getenv("AIGW_TEST_PERFORMANCE_DIAGNOSTIC"); marker != "" {
		_, _ = fmt.Fprintln(os.Stdout, "Warning: stdout data", token)
		_, _ = fmt.Fprintln(os.Stderr, marker, token)
		time.Sleep(20 * time.Millisecond)
		os.Exit(0)
	}
	hyperfine, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ marker, prepared string }{
		{"Working: benchmark-child", "prepared space's.log"},
		{"[WARN] benchmark-child", "prepared&native.log"},
		{"Warning: benchmark-child", "prepared-warning.log"},
	} {
		t.Run(test.marker, func(t *testing.T) {
			raw := filepath.Join(t.TempDir(), "samples.json")
			prepared := filepath.Join(filepath.Dir(raw), test.prepared)
			benchmark := Workload{
				Command: []string{program, "-test.run=^TestMeasureRetainsSeparateDiagnosticStreams$"},
				Prepare: []string{program, "-test.run=^TestArgvPreservesNativeArguments$", test.prepared, "", "a'b", `C:\native\`, "prepare"},
			}
			// Only the synchronous fixture child omits the race runtime's exit delay.
			environment := append(os.Environ(), "AIGW_TEST_PERFORMANCE_DIAGNOSTIC="+test.marker,
				"GORACE="+strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0"))
			row, err := Measure(t.Context(), Command{
				Tool: hyperfine, Arguments: benchmark.Arguments(raw), Output: raw, Directory: filepath.Dir(raw),
				Environment: environment,
				Sensitive:   []string{token + "\n"},
				Measurement: Measurement{Variant: "candidate", Backend: "env", Case: "credential", Block: 1, Budget: 0.1},
			})
			if err != nil {
				for _, stream := range []string{"stdout", "stderr"} {
					content, readErr := os.ReadFile(strings.TrimSuffix(raw, ".json") + "." + stream)
					t.Logf("Hyperfine %s (redacted; read error %v):\n%s", stream, readErr, content)
				}
				t.Fatal(err)
			}
			data, err := os.ReadFile(prepared)
			if err != nil || string(data) != strings.Repeat("prepared\n", 45) {
				t.Fatalf("native preparation lost its exact arguments or five warmups and forty samples: %v", err)
			}
			stdout, stdoutErr := os.ReadFile(strings.TrimSuffix(raw, ".json") + ".stdout")
			stderr, stderrErr := os.ReadFile(strings.TrimSuffix(raw, ".json") + ".stderr")
			if stdoutErr != nil || stderrErr != nil || !bytes.Contains(stdout, []byte("Warning: stdout data")) || !bytes.Contains(stderr, []byte(test.marker)) {
				t.Fatal("native benchmark lost its separate child output streams")
			}
			if strings.Contains(string(stdout)+string(stderr), token) || !bytes.Contains(stdout, []byte("[REDACTED]")) || !bytes.Contains(stderr, []byte("[REDACTED]")) {
				t.Fatal("native benchmark output leaked its bare Token")
			}
			if row.Diagnostics != process.DiagnosticFailure(stderr) {
				t.Fatalf("native benchmark diagnostic decision=%t differs from complete redacted stderr:\n%s", row.Diagnostics, stderr)
			}
			if row.Raw != filepath.Base(raw) || row.P95 <= 0 || row.Variant != "candidate" || row.Budget != 0.1 || len(row.Samples.Times) != 40 {
				t.Fatalf("native benchmark lost its admitted measurement boundary: %#v", row)
			}
			data, readErr := os.ReadFile(raw)
			if readErr != nil {
				t.Fatal(readErr)
			}
			_, decodeErr := decodeSamples(data, Argv(benchmark.Command...))
			if decodeErr != nil {
				t.Fatalf("native benchmark lost schema 2 raw samples: %v", decodeErr)
			}
		})
	}
}

func TestMeasureRejectsUnprovedNativeEvidence(t *testing.T) {
	hyperfine, err := exec.LookPath("hyperfine")
	if err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"missing-output": "", "invalid-json": "invalid", "empty-result": `{"results":[]}`,
		"incomplete-samples": `{"results":[{"times":[0.1],"exit_codes":[0]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := filepath.Join(t.TempDir(), "samples.json")
			if contents != "" {
				if err := os.WriteFile(raw, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Measure(t.Context(), Command{Tool: hyperfine, Arguments: []string{"--version"}, Output: raw}); err == nil {
				t.Fatal("unproved raw native evidence acquired a measurement")
			}
		})
	}
	root := t.TempDir()
	for _, raw := range []string{filepath.Join(root, "failed.json"), filepath.Join(root, "missing", "failed.json")} {
		if _, err := Measure(t.Context(), Command{Tool: filepath.Join(root, "missing-tool"), Output: raw}); err == nil {
			t.Fatal("a failed native tool or unavailable stream destination acquired a measurement")
		}
	}
}

func TestMeasureBindsRawEvidenceAndRetainsFailure(t *testing.T) {
	if output := os.Getenv("AIGW_TEST_PERFORMANCE_EXPORT"); output != "" {
		os.Exit(writePerformanceExportFixture(output))
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"matched", "wrong-command", "failed-tool", "stale-output", "extra-sample", "stale-identity", "changed-file", "unbound-file", "null-exit", "null-time", "wrong-unit", "wrong-schema", "wrong-metric"} {
		t.Run(name, func(t *testing.T) {
			input := performanceExportFixture(t, program, name)
			row, err := Measure(t.Context(), input)
			switch name {
			case "matched":
				if err != nil || row.P95 != 0.01 {
					t.Fatalf("owned command did not qualify: %#v, %v", row, err)
				}
			case "stale-identity", "unbound-file":
				if _, observed := os.Stat(input.Output); !errors.Is(observed, os.ErrNotExist) || err == nil || row.P95 != 0 {
					t.Fatal("an unbound executable ran before refusal")
				}
			case "stale-output":
				data, observed := os.ReadFile(input.Output)
				if err == nil || observed != nil || string(data) != "previous evidence" {
					t.Fatal("stale evidence was overwritten before refusal")
				}
			default:
				count := 40
				if name == "extra-sample" {
					count++
				}
				if err == nil || row.Raw != "samples.json" || row.Variant != "candidate" || row.P95 != 0 || len(row.Samples.Times) != count || row.Samples.Command == "" {
					t.Fatalf("failed measurement lost its exact partial evidence: %#v, %v", row, err)
				}
			}
		})
	}
}

func writePerformanceExportFixture(output string) int {
	name := os.Getenv("AIGW_TEST_PERFORMANCE_CASE")
	command := os.Getenv("AIGW_TEST_PERFORMANCE_COMMAND")
	count := 40
	if name == "wrong-command" {
		command = "different command"
	}
	if name == "extra-sample" {
		count++
	}
	measurements := make([]map[string]any, count)
	for index := range measurements {
		measurements[index] = map[string]any{
			"time_wall_clock": map[string]any{"value": 0.01, "unit": "second"}, "exit_code": 0,
		}
	}
	report := map[string]any{"schema_version": 2, "primary_metric": "time_wall_clock", "results": []any{map[string]any{
		"command": command, "measurements": measurements,
	}}}
	switch name {
	case "null-exit":
		measurements[0]["exit_code"] = nil
	case "null-time":
		measurements[0]["time_wall_clock"] = map[string]any{"value": nil, "unit": "second"}
	case "wrong-unit":
		measurements[0]["time_wall_clock"] = map[string]any{"value": 0.01, "unit": "millisecond"}
	case "wrong-schema":
		report["schema_version"] = 1
	case "wrong-metric":
		report["primary_metric"] = "memory_peak_resident"
	}
	data, err := json.Marshal(report)
	if err != nil || os.WriteFile(output, data, 0o600) != nil {
		return 5
	}
	if path := os.Getenv("AIGW_TEST_PERFORMANCE_CHANGED_FILE"); path != "" {
		if err := os.WriteFile(path, []byte("replaced during measurement"), 0o600); err != nil {
			return 6
		}
	}
	if name == "failed-tool" {
		return 4
	}
	return 0
}

func performanceExportFixture(t *testing.T, program, name string) Command {
	t.Helper()
	output := filepath.Join(t.TempDir(), "samples.json")
	selected, changed := program, ""
	if name == "changed-file" {
		data, err := os.ReadFile(program)
		if err != nil {
			t.Fatal(err)
		}
		selected = filepath.Join(filepath.Dir(output), "selected-executable")
		if err := os.WriteFile(selected, data, 0o700); err != nil {
			t.Fatal(err)
		}
		changed = selected
	}
	identity, err := Identify(selected)
	if err != nil {
		t.Fatal(err)
	}
	command := Argv(selected, "status")
	switch name {
	case "stale-identity":
		identity.SHA256 = strings.Repeat("0", 64)
	case "unbound-file":
		command = Argv("different-executable", "status")
	case "stale-output":
		if err := os.WriteFile(output, []byte("previous evidence"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Command{
		Tool: program, Arguments: []string{"-test.run=^TestMeasureBindsRawEvidenceAndRetainsFailure$", "--", command}, Output: output,
		Environment: append(os.Environ(), "AIGW_TEST_PERFORMANCE_EXPORT="+output, "AIGW_TEST_PERFORMANCE_CASE="+name,
			"AIGW_TEST_PERFORMANCE_COMMAND="+command, "AIGW_TEST_PERFORMANCE_CHANGED_FILE="+changed,
			"GORACE="+strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0")),
		Measurement: Measurement{Variant: "candidate", Backend: "env", Case: "credential", Block: 1, Executable: &identity},
	}
}

func TestIdentityRecognizesPortableExecutableFormats(t *testing.T) {
	for _, target := range []struct{ os, arch, format string }{{"windows", "arm64", "PE"}, {"linux", "amd64", "ELF"}, {"darwin", "arm64", "Mach-O"}} {
		t.Run(target.format, func(t *testing.T) {
			root := t.TempDir()
			source, executable := filepath.Join(root, "main.go"), filepath.Join(root, "selected-executable")
			if err := os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-o", executable, source)
			command.Env = append(os.Environ(), "GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("build native parser fixture: %s: %v", output, err)
			}
			identity, err := Identify(executable)
			if err != nil || identity.Format != target.format || identity.Arch != target.arch || identity.Machine == 0 || identity.Bytes <= 0 || len(identity.SHA256) != 64 {
				t.Fatalf("selected %s native file identity is incomplete: %#v, %v", target.format, identity, err)
			}
		})
	}
	for _, test := range []struct {
		name      string
		cpu       macho.Cpu
		wantError string
	}{{"universal", macho.CpuArm64, ""}, {"unsupported universal CPU", macho.Cpu(0x0100ffff), "unsupported universal Mach-O CPU"}} {
		t.Run(test.name, func(t *testing.T) {
			var data bytes.Buffer
			if err := binary.Write(&data, binary.BigEndian, []uint32{macho.MagicFat, 2, uint32(macho.CpuAmd64), 0, 48, 32, 4, uint32(test.cpu), 0, 80, 32, 4}); err != nil {
				t.Fatal(err)
			}
			for _, cpu := range []macho.Cpu{macho.CpuAmd64, test.cpu} {
				if err := binary.Write(&data, binary.LittleEndian, []uint32{macho.Magic64, uint32(cpu), 0, uint32(macho.TypeExec), 0, 0, 0, 0}); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(t.TempDir(), "universal-executable")
			if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			identity, err := Identify(path)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("universal identity admitted an unsupported CPU: %#v, %v", identity, err)
				}
				return
			}
			if err != nil || identity.Format != "Mach-O" || identity.Arch != "universal" || !slices.Equal(identity.Architectures, []string{"amd64", "arm64"}) || identity.Bytes != data.Len() || identity.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data.Bytes())) {
				t.Fatalf("universal native file identity is incomplete: %#v, %v", identity, err)
			}
		})
	}
}
