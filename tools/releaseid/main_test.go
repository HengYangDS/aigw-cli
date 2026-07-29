package main

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
)

type rejectingWriter struct{}

func (rejectingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write rejected")
}

func TestUUIDV5IsStableAndVersionScoped(t *testing.T) {
	t.Parallel()

	const namespace = "6ba7b814-9dad-11d1-80b4-00c04fd430c8"
	first, err := uuidV5(namespace, "aigw/product/0.1.0-rc.55/windows/amd64")
	if err != nil {
		t.Fatalf("uuidV5() error = %v", err)
	}
	second, err := uuidV5(namespace, "aigw/product/0.1.0-rc.55/windows/amd64")
	if err != nil {
		t.Fatalf("uuidV5() second error = %v", err)
	}
	if first != second {
		t.Fatalf("uuidV5() is not stable: %q != %q", first, second)
	}
	other, err := uuidV5(namespace, "aigw/product/0.1.0-rc.56/windows/amd64")
	if err != nil {
		t.Fatalf("uuidV5() version change error = %v", err)
	}
	if first == other {
		t.Fatalf("uuidV5() did not scope identity by version: %q", first)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(first) {
		t.Fatalf("uuidV5() = %q, want RFC 4122 UUIDv5", first)
	}
}

func TestUUIDV5RejectsMalformedNamespace(t *testing.T) {
	t.Parallel()

	if _, err := uuidV5("not-a-uuid", "aigw"); err == nil {
		t.Fatal("uuidV5() error = nil for malformed namespace")
	}
}

func TestRunRejectsOutputFailure(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{
		"-namespace", "6ba7b814-9dad-11d1-80b4-00c04fd430c8",
		"-name", "aigw",
	}, rejectingWriter{}, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "write rejected") {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
}
