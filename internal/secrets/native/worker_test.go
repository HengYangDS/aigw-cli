package native

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/process"
)

func TestMain(m *testing.M) {
	if handled, code := RunWorker(os.Args[1:], os.Stdin, os.Stdout, "AIGW_TOKEN"); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestReadPreservesNativeResultAndClassifiesFailure(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
		want error
	}{
		{name: "success"},
		{name: "missing", code: missingExit, want: ErrNotFound},
		{name: "denied", code: failureExit, want: ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := fixtureReader{code: test.code}
			value, err := execute(context.Background(), reader, process.Plan{Executable: "/owned/aigw", Args: []string{readCommand, "AIGW_TOKEN", "team"}})
			if !errors.Is(err, test.want) || test.code == 0 && value != "exact-token" {
				t.Fatalf("read result = %q, %v", value, err)
			}
			if test.code != 0 && value != "" {
				t.Fatal("failed read returned secret output")
			}
		})
	}
}

func TestInvokeRequiresTheOwningProductExecutable(t *testing.T) {
	if _, err := invoke("", readCommand, "AIGW_TOKEN", "team", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty product executable error = %v, want ErrUnavailable", err)
	}
}

type fixtureReader struct{ code int }

func (r fixtureReader) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("credential read lacks a deadline")
	}
	if plan.Executable != "/owned/aigw" || strings.Join(plan.Args, " ") != readCommand+" AIGW_TOKEN team" || plan.Stdin != "" {
		return nil, errors.New("credential worker target drift")
	}
	if r.code == 0 {
		return []byte("exact-token"), nil
	}
	command := exec.Command(os.Args[0], "-test.run=^TestCredentialExitFixture$")
	command.Env = append(os.Environ(), "AIGW_TEST_CREDENTIAL_EXIT="+strconv.Itoa(r.code))
	err := command.Run()
	return []byte("private diagnostic must not escape"), err
}

func TestCredentialExitFixture(t *testing.T) {
	value := os.Getenv("AIGW_TEST_CREDENTIAL_EXIT")
	if value != "" {
		code, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		os.Exit(code)
	}
}

func TestReadTerminatesAndReapsUnresponsiveWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	command := process.Plan{Executable: os.Args[0], Args: []string{"-test.run=^TestCredentialWaitFixture$"}, Env: append(os.Environ(), "AIGW_TEST_CREDENTIAL_WAIT=1")}
	started := time.Now()
	value, err := execute(ctx, waitReader{plan: command}, process.Plan{Executable: os.Args[0], Args: []string{readCommand, "AIGW_TOKEN", "team"}})
	if value != "" || !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 3*time.Second {
		t.Fatalf("unbounded credential read: %q, %v", value, err)
	}
}

type waitReader struct{ plan process.Plan }

func (r waitReader) RunCapture(ctx context.Context, _ process.Plan) ([]byte, error) {
	return (process.Runner{}).RunCapture(ctx, r.plan)
}

func TestCredentialWaitFixture(t *testing.T) {
	if os.Getenv("AIGW_TEST_CREDENTIAL_WAIT") == "1" {
		time.Sleep(time.Minute)
	}
}

func TestWorkerRestrictsIdentityAndKeepsFailuresOffStandardOutput(t *testing.T) {
	for _, args := range [][]string{
		{readCommand},
		{readCommand, "foreign-service", "team"},
		{readCommand, "AIGW_TOKEN", ""},
		{readCommand, "AIGW_TOKEN", "team", "extra"},
	} {
		var out bytes.Buffer
		handled, code := dispatch(args, strings.NewReader(""), &out, "AIGW_TOKEN", func(string, string, string, []byte) ([]byte, error) {
			t.Fatal("invalid worker input reached the native credential service")
			return nil, nil
		})
		if !handled || code == 0 || out.Len() != 0 {
			t.Fatalf("worker result = %v, %d, %q", handled, code, &out)
		}
	}
}

func TestWorkerResultOwnsExitStatusAndSecretOutput(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code int
	}{
		{name: "success"},
		{name: "missing", err: ErrNotFound, code: missingExit},
		{name: "failure", err: errors.New("private backend diagnostic"), code: failureExit},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			value := []byte("synthetic-token")
			handled, code := dispatch([]string{readCommand, "AIGW_TOKEN", "team"}, strings.NewReader(""), &out, "AIGW_TOKEN", func(string, string, string, []byte) ([]byte, error) { return value, test.err })
			if !handled || code != test.code || test.err != nil && out.Len() != 0 || test.err == nil && out.String() != "synthetic-token" {
				t.Fatalf("worker output: handled=%v code=%d", handled, code)
			}
			for _, b := range value {
				if b != 0 {
					t.Fatal("worker buffer was retained")
				}
			}
		})
	}
	if handled, _ := RunWorker([]string{"--version"}, strings.NewReader(""), io.Discard, "AIGW_TOKEN"); handled {
		t.Fatal("ordinary command intercepted")
	}
	for _, out := range []io.Writer{shortWriter{}, failedWriter{}} {
		_, code := dispatch([]string{readCommand, "AIGW_TOKEN", "team"}, strings.NewReader(""), out, "AIGW_TOKEN", func(string, string, string, []byte) ([]byte, error) { return []byte("value"), nil })
		if code != failureExit {
			t.Fatal("failed stdout write returned success")
		}
	}
}

func TestWorkerReturnsOnlyCredentialPresenceMetadata(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  []byte
		output string
	}{
		{name: "present", value: []byte("1"), output: "1"},
		{name: "absent", value: []byte("0"), output: "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			handled, code := dispatch(
				[]string{existsCommand, "AIGW_TOKEN", "team"},
				strings.NewReader(""),
				&out,
				"AIGW_TOKEN",
				func(operation, service, account string, data []byte) ([]byte, error) {
					if operation != existsCommand || service != "AIGW_TOKEN" || account != "team" || len(data) != 0 {
						t.Fatal("credential metadata request changed")
					}
					return append([]byte(nil), test.value...), nil
				},
			)
			if !handled || code != 0 || out.String() != test.output {
				t.Fatalf("metadata result: handled=%v code=%d output=%q", handled, code, &out)
			}
		})
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWorkerMutationAdmitsBoundedStdinAndErasesBuffers(t *testing.T) {
	for _, test := range []struct {
		operation, input string
		accepted         bool
	}{
		{writeCommand, "synthetic-token", true},
		{writeCommand, strings.Repeat("x", maxStoredValue), true},
		{writeCommand, "", false},
		{writeCommand, strings.Repeat("x", maxStoredValue+1), false},
		{deleteCommand, "", true},
	} {
		var input, output []byte
		var out bytes.Buffer
		called := false
		handled, code := dispatch([]string{test.operation, "AIGW_TOKEN", "team"}, strings.NewReader(test.input), &out, "AIGW_TOKEN", func(operation, service, account string, data []byte) ([]byte, error) {
			called = true
			if operation != test.operation || service != "AIGW_TOKEN" || account != "team" || string(data) != test.input {
				t.Fatal("mutation target or stdin changed")
			}
			input, output = data, []byte("unexpected-backend-output")
			return output, nil
		})
		if !handled || called != test.accepted || (code == 0) != test.accepted || out.Len() != 0 {
			t.Fatalf("mutation admission: handled=%v called=%v code=%d", handled, called, code)
		}
		if bytes.Count(input, []byte{0}) != len(input) || bytes.Count(output, []byte{0}) != len(output) {
			t.Fatal("worker retained credential buffers")
		}
	}
	_, code := dispatch([]string{writeCommand, "AIGW_TOKEN", "team"}, failedReader{}, io.Discard, "AIGW_TOKEN", func(string, string, string, []byte) ([]byte, error) {
		t.Fatal("failed input reached the native credential service")
		return nil, nil
	})
	if code != failureExit {
		t.Fatal("failed stdin accepted")
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWorkerMutationTransportsCredentialsOnlyThroughStdin(t *testing.T) {
	for _, operation := range []string{writeCommand, deleteCommand} {
		runner := mutationRunner{operation: operation}
		if _, err := execute(t.Context(), runner, process.Plan{Executable: "/owned/aigw", Args: []string{operation, "AIGW_TOKEN", "team"}, Stdin: "synthetic-token", Env: []string{"HOME=/owned"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := execute(t.Context(), mutationRunner{}, process.Plan{Stdin: strings.Repeat("x", maxStoredValue+1)}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("oversized stdin reached worker")
	}
}

type mutationRunner struct{ operation string }

func (r mutationRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("mutation lacks deadline")
	}
	if plan.Executable != "/owned/aigw" || strings.Join(plan.Args, " ") != r.operation+" AIGW_TOKEN team" || plan.Stdin != "synthetic-token" || strings.Join(plan.Env, " ") != "HOME=/owned" {
		return nil, errors.New("mutation ownership or credential transport drift")
	}
	return nil, nil
}

func TestReadFailsClosedWhenWorkerCannotStart(t *testing.T) {
	value, err := execute(t.Context(), process.Runner{}, process.Plan{Executable: filepath.Join(t.TempDir(), "absent"), Args: []string{readCommand, "AIGW_TOKEN", "team"}})
	if value != "" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("startup failure: %v", err)
	}
}
