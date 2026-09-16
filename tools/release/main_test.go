package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestLocalDeliveryUsesExistingCommandsAndSourceAdmission(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{
		{"build", "--local", "dist"},
		{"verify-artifacts", "--local", "dist"},
		{"accept-native", "--local", "--artifacts", "dist"},
	} {
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "read VERSION") {
			t.Fatalf("local delivery must enter source admission: args=%v error=%v", args, err)
		}
	}
}

func TestExecuteReturnsPortableProcessStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if status := execute([]string{"unknown"}, &stdout, &stderr); status != 2 || !strings.Contains(stderr.String(), "unknown release command") {
		t.Fatalf("failure status=%d stderr=%q", status, stderr.String())
	}
	stderr.Reset()
	if status := execute([]string{"validate-readiness", "1.2.3-rc.1"}, &stdout, &stderr); status != 0 {
		t.Fatalf("success status=%d stderr=%q", status, stderr.String())
	}
}
