package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatrixRejectsMissingExtraAndCorruptFiles(t *testing.T) {
	version := "1.2.3"
	if err := ValidateMatrix(filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing matrix accepted")
	}
	directory := writeFixture(t, version)
	if err := ValidateMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, Names(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("missing artifact=%v", err)
	}

	directory = writeFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("extra artifact=%v", err)
	}

	directory = writeFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, Names(version)[0]), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt artifact=%v", err)
	}

	directory = writeFixture(t, version)
	checksumPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumPath, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "invalid checksum") {
		t.Fatalf("invalid checksum manifest=%v", err)
	}

	directory = writeFixture(t, version)
	content, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(string(content), "\n", 2)[0]
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(first+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "duplicate checksum") {
		t.Fatalf("duplicate checksum=%v", err)
	}

	directory = writeFixture(t, version)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(strings.Repeat("0", 64)+"  unknown\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected entries") {
		t.Fatalf("unexpected checksum=%v", err)
	}
}

func TestCompareMatrices(t *testing.T) {
	version := "1.2.3"
	left, right := writeFixture(t, version), writeFixture(t, version)
	if err := CompareMatrices(left, right, version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, Names(version)[0]), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RewriteChecksums(right, version); err != nil {
		t.Fatal(err)
	}
	if err := CompareMatrices(left, right, version); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different matrix=%v", err)
	}
}

func writeFixture(t *testing.T, version string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range Names(version) {
		if name == "checksums.txt" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	return directory
}
