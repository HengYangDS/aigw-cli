package process

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestLimitedBufferWriteEnforcesLimitAndFlagsOverflow(t *testing.T) {
	buf := &limitedBuffer{limit: 8}
	n, err := buf.Write([]byte("1234"))
	if err != nil || n != 4 || buf.overflow {
		t.Fatalf("first write = (%d, %v), overflow=%v", n, err, buf.overflow)
	}
	n, err = buf.Write([]byte("567890"))
	if n != 4 || !errors.Is(err, errCapturedProcessOutputLimit) || !buf.overflow {
		t.Fatalf("partial overflow write = (%d, %v), overflow=%v", n, err, buf.overflow)
	}
	if buf.String() != "12345678" {
		t.Fatalf("buffer contents = %q, want truncated at the limit", buf.String())
	}
	n, err = buf.Write([]byte("x"))
	if n != 0 || !errors.Is(err, errCapturedProcessOutputLimit) {
		t.Fatalf("write past a full buffer = (%d, %v)", n, err)
	}
}

func TestRunnerRunRejectsMissingExecutable(t *testing.T) {
	err := (Runner{}).Run(context.Background(), Plan{
		Executable: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil {
		t.Fatal("Run() with a missing executable should fail")
	}
}

func TestRunnerRunCaptureSurfacesStartFailure(t *testing.T) {
	_, err := (Runner{}).RunCapture(context.Background(), Plan{
		Executable: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil || !strings.Contains(err.Error(), "start ") {
		t.Fatalf("RunCapture() error = %v", err)
	}
}
