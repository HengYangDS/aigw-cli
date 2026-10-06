package performance

import (
	"bytes"
	"context"
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
)

func TestIdentityMeasuresTheSelectedNativeFile(t *testing.T) {
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := Identify(path)
	if err != nil || identity.Path != path || identity.Arch != runtime.GOARCH || identity.Format == "" || identity.Machine == 0 || len(identity.SHA256) != 64 || identity.Bytes <= 0 {
		t.Fatalf("selected file identity is incomplete: %#v, %v", identity, err)
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
	if runtime.GOOS == "darwin" {
		shell, err := Identify("/bin/sh")
		if err != nil || shell.Format != "Mach-O" || (shell.Arch != runtime.GOARCH && (shell.Arch != "universal" || !slices.Contains(shell.Architectures, runtime.GOARCH))) {
			t.Fatalf("native shell file identity is incomplete: %#v, %v", shell, err)
		}
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
	if _, err := MeasureOperation(ctx, filepath.Join(t.TempDir(), "canceled.json"), Measurement{}, func() error { t.Fatal("canceled measurement executed"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled native API measurement continued: %v", err)
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
}

func TestArgvPreservesNativeArguments(t *testing.T) {
	if got := Argv(`C:\program files\aigw.exe`, "a'b", ""); got != `'C:\program files\aigw.exe' 'a'\''b' ''` {
		t.Fatalf("Hyperfine argv quoting = %s", got)
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
	for _, marker := range []string{"Working: benchmark-child", "[WARN] benchmark-child"} {
		t.Run(marker, func(t *testing.T) {
			raw := filepath.Join(t.TempDir(), "samples.json")
			benchmark := Workload{Command: Argv(program, "-test.run=^TestMeasureRetainsSeparateDiagnosticStreams$")}
			row, err := Measure(t.Context(), Command{
				Tool: hyperfine, Arguments: benchmark.Arguments(raw), Output: raw,
				Environment: append(os.Environ(), "AIGW_TEST_PERFORMANCE_DIAGNOSTIC="+marker),
				Sensitive:   []string{token + "\n"},
				Measurement: Measurement{Variant: "candidate", Backend: "env", Case: "credential", Block: 1, Budget: 0.1},
			})
			if err != nil {
				t.Fatal(err)
			}
			stdout, stdoutErr := os.ReadFile(strings.TrimSuffix(raw, ".json") + ".stdout")
			stderr, stderrErr := os.ReadFile(strings.TrimSuffix(raw, ".json") + ".stderr")
			if stdoutErr != nil || stderrErr != nil || !bytes.Contains(stdout, []byte("Warning: stdout data")) || !bytes.Contains(stderr, []byte(marker)) {
				t.Fatal("native benchmark lost its separate child output streams")
			}
			if strings.Contains(string(stdout)+string(stderr), token) || !bytes.Contains(stdout, []byte("[REDACTED]")) || !bytes.Contains(stderr, []byte("[REDACTED]")) {
				t.Fatal("native benchmark output leaked its bare Token")
			}
			if row.Diagnostics != strings.HasPrefix(marker, "[WARN]") || row.Raw != filepath.Base(raw) || row.P95 <= 0 || row.Variant != "candidate" || row.Budget != 0.1 || len(row.Samples.Times) != 40 {
				t.Fatalf("native benchmark lost its admitted measurement boundary: %#v", row)
			}
			var report struct {
				Results []Samples `json:"results"`
			}
			data, readErr := os.ReadFile(raw)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if err := json.Unmarshal(data, &report); err != nil || len(report.Results) != 1 {
				t.Fatal("native benchmark lost raw samples")
			}
			if _, err := report.Results[0].Percentile(); err != nil {
				t.Fatal(err)
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

func TestIdentityRecognizesPortableExecutableFormats(t *testing.T) {
	for _, target := range []struct{ os, arch, format string }{{"windows", "arm64", "PE"}, {"linux", "amd64", "ELF"}} {
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
}
