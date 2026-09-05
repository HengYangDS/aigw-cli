package main

import (
	"aigw-cli/tools/release/artifact"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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

func TestArtifactMatrixRejectsMissingExtraAndCorruptFiles(t *testing.T) {
	version := "1.2.3"
	if err := artifact.ValidateMatrix(filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing matrix accepted")
	}
	directory := writeArtifactFixture(t, version)
	if err := artifact.ValidateMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, artifact.Names(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("missing artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("extra artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, artifact.Names(version)[0]), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt artifact=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	checksumPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumPath, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "invalid checksum") {
		t.Fatalf("invalid checksum manifest=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	content, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(string(content), "\n", 2)[0]
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(first+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "duplicate checksum") {
		t.Fatalf("duplicate checksum=%v", err)
	}

	directory = writeArtifactFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(strings.Repeat("0", 64)+"  unknown\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected entries") {
		t.Fatalf("unexpected checksum=%v", err)
	}
}

func TestCompareArtifactMatrices(t *testing.T) {
	version := "1.2.3"
	left, right := writeArtifactFixture(t, version), writeArtifactFixture(t, version)
	if err := artifact.CompareMatrices(left, right, version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, artifact.Names(version)[0]), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.RewriteChecksums(right, version); err != nil {
		t.Fatal(err)
	}
	if err := artifact.CompareMatrices(left, right, version); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different matrix=%v", err)
	}
}

func TestRunArtifactCommands(t *testing.T) {
	version := "1.2.3"
	left, right := writeArtifactFixture(t, version), writeArtifactFixture(t, version)
	var output bytes.Buffer
	if err := run([]string{"validate-artifacts", left, version}, &output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"compare-artifacts", left, right, version}, &output); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"validate-artifacts"}, {"compare-artifacts"}} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
}

func TestRunReleasePolicyCommands(t *testing.T) {
	tmp := t.TempDir()
	module := filepath.Join(tmp, "go.mod")
	if err := os.WriteFile(module, []byte("module example\n\ngo "+strings.TrimPrefix(runtime.Version(), "go")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	document := filepath.Join(tmp, "readiness.md")
	if err := os.WriteFile(document, []byte("# readiness\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	for _, args := range [][]string{
		{"validate-toolchain", module},
		{"validate-readiness", "1.2.3-rc.1"},
		{"validate-readiness-doc", document},
	} {
		if err := run(args, &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"validate-toolchain"},
		{"validate-readiness"},
		{"validate-readiness-doc"},
	} {
		if err := run(args, &output); err == nil {
			t.Fatalf("invalid invocation accepted: %v", args)
		}
	}
}

func writeArtifactFixture(t *testing.T, version string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range artifact.Names(version) {
		if name == "checksums.txt" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := artifact.RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	return directory
}
