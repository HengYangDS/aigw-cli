package keychain

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
	if handled, code := RunWorker(os.Args[1:], os.Stdout, "AIGW_TOKEN"); handled {
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
		{name: "locked", code: deniedExit, want: ErrDenied},
		{name: "denied", code: failureExit, want: ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := fixtureReader{code: test.code}
			value, err := read(context.Background(), reader, "/owned/aigw", workerCommand, "AIGW_TOKEN", "team", nil)
			if !errors.Is(err, test.want) || test.code == 0 && value != "exact-token" {
				t.Fatalf("read result = %q, %v", value, err)
			}
			if test.code != 0 && value != "" {
				t.Fatal("failed read returned secret output")
			}
		})
	}
}

type fixtureReader struct{ code int }

func (r fixtureReader) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("credential read lacks a deadline")
	}
	if plan.Executable != "/owned/aigw" || strings.Join(plan.Args, " ") != workerCommand+" AIGW_TOKEN team" || plan.Stdin != "" {
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
	value, err := read(ctx, waitReader{plan: command}, os.Args[0], workerCommand, "AIGW_TOKEN", "team", nil)
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
		{workerCommand},
		{workerCommand, "foreign-service", "team"},
		{workerCommand, "AIGW_TOKEN", ""},
		{workerCommand, "AIGW_TOKEN", "team", "extra"},
	} {
		var out bytes.Buffer
		handled, code := dispatch(args, &out, "AIGW_TOKEN", func(string, string, bool) ([]byte, error) {
			t.Fatal("invalid worker input reached Keychain")
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
		{name: "locked", err: ErrDenied, code: deniedExit},
		{name: "failure", err: errors.New("private backend diagnostic"), code: failureExit},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			value := []byte("synthetic-token")
			handled, code := dispatch([]string{workerCommand, "AIGW_TOKEN", "team"}, &out, "AIGW_TOKEN", func(string, string, bool) ([]byte, error) { return value, test.err })
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
	if handled, _ := RunWorker([]string{"--version"}, io.Discard, "AIGW_TOKEN"); handled {
		t.Fatal("ordinary command intercepted")
	}
	for _, out := range []io.Writer{shortWriter{}, failedWriter{}} {
		_, code := dispatch([]string{workerCommand, "AIGW_TOKEN", "team"}, out, "AIGW_TOKEN", func(string, string, bool) ([]byte, error) { return []byte("value"), nil })
		if code != failureExit {
			t.Fatal("failed stdout write returned success")
		}
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestStoredValueDecodingPreservesCurrentKeyringGrammar(t *testing.T) {
	for _, test := range []struct {
		stored, want string
		bad          bool
	}{
		{stored: " raw-token\n", want: "raw-token"},
		{stored: "go-keyring-base64:dG9rZW4=", want: "token"},
		{stored: "go-keyring-encoded:746f6b656e", want: "token"},
		{stored: "go-keyring-base64:not-base64", bad: true},
		{stored: "go-keyring-encoded:zz", bad: true},
	} {
		got, err := DecodeStoredValue(test.stored)
		if (err != nil) != test.bad || !test.bad && got != test.want {
			t.Fatalf("stored grammar mismatch: %v", err)
		}
	}
}

func TestReadFailsClosedWhenWorkerCannotStart(t *testing.T) {
	value, err := read(t.Context(), process.Runner{}, filepath.Join(t.TempDir(), "absent"), workerCommand, "AIGW_TOKEN", "team", nil)
	if value != "" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("startup failure: %v", err)
	}
}

func TestWorkerObservationDoesNotRequestSecretBytes(t *testing.T) {
	var out bytes.Buffer
	called := false
	handled, code := dispatch([]string{workerCommand, "AIGW_TOKEN", "team", "observe"}, &out, "AIGW_TOKEN", func(string, string, bool) ([]byte, error) {
		called = true
		return []byte("credential"), nil
	})
	if !handled || called || out.Len() != 0 || code == 0 {
		t.Fatalf("unadmitted operation reached value reader: handled=%v code=%d", handled, code)
	}
}

func TestWorkerObservationUsesTheMetadataOperation(t *testing.T) {
	var out bytes.Buffer
	observed := false
	handled, code := dispatch([]string{observeCommand, "AIGW_TOKEN", "team"}, &out, "AIGW_TOKEN", func(_, _ string, metadata bool) ([]byte, error) {
		observed = metadata
		return nil, nil
	})
	if !handled || code != 0 || !observed || out.Len() != 0 {
		t.Fatalf("metadata worker: %v, %d", handled, code)
	}
}
