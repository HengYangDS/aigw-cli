package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarEntryForTest struct {
	name string
	data []byte
	dir  bool
}

func gzipBytesForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := gzip.NewWriter(&output)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func tarGzWithEntriesForTest(t *testing.T, entries []tarEntryForTest) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.data))}
		if entry.dir {
			header.Typeflag = tar.TypeDir
			header.Size = 0
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if !entry.dir {
			if _, err := tarWriter.Write(entry.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestExtractBinaryRejectsUnreadableArchive(t *testing.T) {
	if _, err := extractBinary(filepath.Join(t.TempDir(), "missing.tar.gz"), "a"); err == nil || !strings.Contains(err.Error(), "open update archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractBinaryRejectsInvalidGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(path, []byte("not-gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractBinary(path, "a"); err == nil || !strings.Contains(err.Error(), "open gzip archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractBinaryRejectsCorruptTarStream(t *testing.T) {
	// A well-formed gzip stream wrapping bytes that are not a valid tar
	// stream triggers the tar-read error branch.
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(path, gzipBytesForTest(t, []byte("not-a-tar-stream-but-long-enough-to-parse-partially")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractBinary(path, "a"); err == nil {
		t.Fatal("extractBinary accepted a corrupt tar stream")
	}
}

func TestExtractBinarySkipsNonMatchingEntriesAndDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	archive := tarGzWithEntriesForTest(t, []tarEntryForTest{
		{name: "other/file", data: []byte("skip-me")},
		{name: "dir/", dir: true},
		{name: "aigw_1.2.3/aigw", data: []byte("binary-bytes")},
	})
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	binary, err := extractBinary(path, "aigw_1.2.3/aigw")
	if err != nil {
		t.Fatal(err)
	}
	if string(binary) != "binary-bytes" {
		t.Fatalf("binary = %q", binary)
	}
}

func TestExtractBinaryRejectsMultipleMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	archive := tarGzWithEntriesForTest(t, []tarEntryForTest{
		{name: "aigw_1.2.3/aigw", data: []byte("one")},
		{name: "aigw_1.2.3/aigw", data: []byte("two")},
	})
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractBinary(path, "aigw_1.2.3/aigw"); err == nil || !strings.Contains(err.Error(), "multiple expected AIGW binaries") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractBinaryRejectsMissingEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	archive := tarGzWithEntriesForTest(t, []tarEntryForTest{
		{name: "aigw_1.2.3/other", data: []byte("payload")},
	})
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractBinary(path, "aigw_1.2.3/aigw"); err == nil || !strings.Contains(err.Error(), "is missing from update archive") {
		t.Fatalf("error = %v", err)
	}
}
