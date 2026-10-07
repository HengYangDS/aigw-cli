package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

type identityCaptureRunner struct {
	output []byte
	stderr []byte
	err    error
	plan   process.Plan
}

func (runner *identityCaptureRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plan = plan
	return runner.output, runner.err
}

func (runner *identityCaptureRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	return output, runner.stderr, err
}

func TestIdentifyExecutableRejectsNativeDiagnostics(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range []string{"warning: invalid configuration\n", "ERROR client startup failed\n"} {
		runner := &identityCaptureRunner{output: []byte("codex-cli 1.2.3\n"), stderr: []byte(diagnostic)}
		if _, err := IdentifyExecutable(t.Context(), runner, executable, t.TempDir(), t.TempDir()); err == nil {
			t.Fatal("native diagnostics qualified executable identity")
		}
	}
}

func TestIdentifyExecutableRejectsIncompleteOrUnobservableIdentity(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-codex")
	for _, test := range []struct {
		name       string
		executable string
		runner     process.VerificationRunner
		want       string
	}{
		{name: "missing executable", runner: &identityCaptureRunner{}, want: "not configured"},
		{name: "missing runner", executable: missing, want: "runner is unavailable"},
		{name: "unreadable executable", executable: missing, runner: &identityCaptureRunner{}, want: "read Codex executable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := IdentifyExecutable(context.Background(), test.runner, test.executable, t.TempDir(), t.TempDir())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("IdentifyExecutable() error = %v, want %q", err, test.want)
			}
		})
	}

	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	failed := &identityCaptureRunner{err: errors.New("version failed")}
	if _, err := IdentifyExecutable(context.Background(), failed, executable, t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "inspect Codex version") {
		t.Fatalf("version command error = %v", err)
	}
	empty := &identityCaptureRunner{output: []byte(" \n")}
	if _, err := IdentifyExecutable(context.Background(), empty, executable, t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "reported no version") {
		t.Fatalf("empty version error = %v", err)
	}
}

func TestIdentifyExecutableUsesTheConfiguredHome(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	runner := &identityCaptureRunner{output: []byte(" codex-cli 1.2.3 \n")}
	identity, err := IdentifyExecutable(context.Background(), runner, executable, home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if identity.Version != "codex-cli 1.2.3" || identity.SHA256 == "" {
		t.Fatalf("identity = %#v", identity)
	}
	if runner.plan.Executable != executable || !slices.Equal(runner.plan.Args, []string{"--version"}) {
		t.Fatalf("version plan = %#v", runner.plan)
	}
	found := false
	for _, value := range runner.plan.Env {
		if value == "CODEX_HOME="+home {
			found = true
		}
	}
	if !found {
		t.Fatalf("version environment does not contain CODEX_HOME=%s", home)
	}
}

func TestFileSHA256RejectsADirectory(t *testing.T) {
	if _, err := fileSHA256(t.TempDir()); err == nil {
		t.Fatal("fileSHA256() accepted a directory")
	}
}

func TestVerificationPlanRejectsIncompleteInputs(t *testing.T) {
	runtime := configuration.Runtime{RouteID: "codex", Model: "gpt-test"}
	for _, test := range []struct {
		name       string
		executable string
		configPath string
		outputPath string
		runtime    configuration.Runtime
		want       string
	}{
		{name: "missing executable", configPath: "config.toml", outputPath: "output.txt", runtime: runtime, want: "not configured"},
		{name: "missing config", executable: "codex", outputPath: "output.txt", runtime: runtime, want: "target is not configured"},
		{name: "missing model", executable: "codex", configPath: "config.toml", outputPath: "output.txt", runtime: configuration.Runtime{RouteID: "codex"}, want: "has no Codex model"},
		{name: "missing output", executable: "codex", configPath: "config.toml", runtime: runtime, want: "output path is not configured"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := VerificationPlan(test.executable, test.configPath, test.outputPath, test.runtime, "Read challenge.txt", "")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("VerificationPlan() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestVerificationPlanPreservesCredentialHelperEnvironment(t *testing.T) {
	t.Setenv("AIGW_SECRET_BACKEND", "env")
	t.Setenv("AIGW_TOKEN_GATEWAY", "fixture-token")
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "ambient-home"))
	home := t.TempDir()
	plan, err := VerificationPlan("codex", filepath.Join(home, "config.toml"), filepath.Join(t.TempDir(), "output.txt"), configuration.Runtime{RouteID: "codex", Model: "gpt-test"}, "Read challenge.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"AIGW_SECRET_BACKEND=env", "AIGW_TOKEN_GATEWAY=fixture-token", "CODEX_HOME=" + home} {
		if !slices.Contains(plan.Env, entry) {
			t.Fatalf("verification child lost required environment entry %q", entry)
		}
	}
	if got := slices.DeleteFunc(append([]string(nil), plan.Env...), func(entry string) bool {
		return !strings.HasPrefix(entry, "CODEX_HOME=")
	}); !slices.Equal(got, []string{"CODEX_HOME=" + home}) {
		t.Fatalf("verification home selection = %q", got)
	}
}

func TestVerificationPlanOwnsNativeTemporaryRoot(t *testing.T) {
	parent := t.TempDir()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, parent)
	}
	home := filepath.Join(parent, "codex-home")
	temporary := filepath.Join(parent, "verification-output")
	plan, err := VerificationPlan("codex", filepath.Join(home, "config.toml"), filepath.Join(temporary, "response.txt"), configuration.Runtime{RouteID: "codex", Model: "gpt-test"}, "Read challenge.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		if !slices.Contains(plan.Env, key+"="+filepath.Join(home, "tmp")) || slices.Contains(plan.Env, key+"="+parent) {
			t.Fatalf("native temporary environment %s did not select the private Codex home's child workspace", key)
		}
	}
	if os.Getenv("TMPDIR") != parent {
		t.Fatal("native temporary selection changed the parent process")
	}
}

func TestReadOnlyCodexCommandsRejectNativeDiagnostics(t *testing.T) {
	runner := &identityCaptureRunner{
		output: []byte(`{"models":[]}`),
		stderr: []byte("WARNING: native catalogue probe failed /private/operator canary-token\n"),
	}
	_, err := runCodexReadOnly(t.Context(), runner, "codex", t.TempDir(), t.TempDir(), "debug", "models", "--bundled")
	if err == nil || !strings.Contains(err.Error(), "warning") {
		t.Fatalf("native catalogue diagnostics qualified a successful result: %v", err)
	}
	for _, private := range []string{"/private/operator", "canary-token"} {
		if strings.Contains(err.Error(), private) {
			t.Fatalf("catalogue error exposed private diagnostic %q", private)
		}
	}
}

func TestCatalogProbePreservesCleanupFailure(t *testing.T) {
	primary := errors.New("catalog probe failed")
	for _, failure := range []error{nil, primary} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			scratch := t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, scratch)
			}
			remove := removeCatalogProbe
			t.Cleanup(func() { removeCatalogProbe = remove })
			var removed, home string
			removeCatalogProbe = func(path string) error {
				removed = path
				return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
			}
			err := withCatalogProbe(func(_ context.Context, path, temporary string) error {
				home = path
				if filepath.Dir(temporary) != path || os.Getenv("TMPDIR") != scratch {
					t.Fatal("probe temporary root is not owned or changed the parent environment")
				}
				return failure
			})
			if home == "" || home == scratch || filepath.Dir(home) != scratch || removed != home {
				t.Fatalf("probe ownership = %q, cleanup = %q, parent = %q", home, removed, scratch)
			}
			if failure != nil && !errors.Is(err, failure) {
				t.Fatalf("probe lost primary cause: %v", err)
			}
			if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), home) {
				t.Fatalf("probe lost cleanup cause: %v", err)
			}
			if _, err := os.Stat(home); err != nil {
				t.Fatalf("injected cleanup unexpectedly removed probe home: %v", err)
			}
		})
	}
}
