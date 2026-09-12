package artifact

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
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

func zipArchive(t *testing.T, entries ...[]byte) []byte {
	t.Helper()
	if len(entries)%2 != 0 {
		t.Fatal("zip fixture needs name/data pairs")
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for len(entries) > 0 {
		name, data := string(entries[0]), entries[1]
		entries = entries[2:]
		entryWriter, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entryWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestExtractZipBinaryReturnsMatchingEntry(t *testing.T) {
	data := zipArchive(t, []byte("aigw_1.2.3_windows_amd64/aigw.exe"), []byte("binary-bytes"))
	path := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	binary, err := extractZipBinary(path, "aigw_1.2.3_windows_amd64/aigw.exe")
	if err != nil {
		t.Fatal(err)
	}
	if string(binary) != "binary-bytes" {
		t.Fatalf("binary = %q", binary)
	}
}

func TestExtractZipBinarySkipsDirectoryEntries(t *testing.T) {
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	if _, err := writer.Create("aigw_1.2.3_windows_amd64/"); err != nil {
		t.Fatal(err)
	}
	fileWriter, err := writer.Create("aigw_1.2.3_windows_amd64/aigw.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fileWriter.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	binary, err := extractZipBinary(path, "aigw_1.2.3_windows_amd64/aigw.exe")
	if err != nil {
		t.Fatal(err)
	}
	if string(binary) != "payload" {
		t.Fatalf("binary = %q", binary)
	}
}

func TestExtractZipBinaryRejectsMultipleMatches(t *testing.T) {
	data := zipArchive(t,
		[]byte("aigw_1.2.3_windows_amd64/aigw.exe"), []byte("one"),
	)
	// Craft a duplicate entry by concatenating raw writer output twice under
	// the same name using a manual writer with two Create calls.
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for range 2 {
		fileWriter, err := writer.Create("aigw_1.2.3_windows_amd64/aigw.exe")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fileWriter.Write([]byte("dup")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	_ = data
	path := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractZipBinary(path, "aigw_1.2.3_windows_amd64/aigw.exe"); err == nil || !strings.Contains(err.Error(), "multiple expected AIGW binaries") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractZipBinaryRejectsMissingEntry(t *testing.T) {
	data := zipArchive(t, []byte("aigw_1.2.3_windows_amd64/other"), []byte("payload"))
	path := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractZipBinary(path, "aigw_1.2.3_windows_amd64/aigw.exe"); err == nil || !strings.Contains(err.Error(), "is missing from update archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractZipBinaryRejectsUnsupportedCompression(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	writer.RegisterCompressor(99, func(destination io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(destination, flate.NoCompression)
	})
	header := &zip.FileHeader{Name: "aigw_1.2.3_windows_amd64/aigw.exe", Method: 99}
	if _, err := writer.CreateHeader(header); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(path, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractZipBinary(path, header.Name); err == nil || !strings.Contains(err.Error(), "open AIGW binary in zip") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractZipBinaryRejectsUnreadableArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(path, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractZipBinary(path, "aigw/aigw"); err == nil || !strings.Contains(err.Error(), "open zip archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractZipBinaryRejectsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.zip")
	if _, err := extractZipBinary(path, "aigw/aigw"); err == nil {
		t.Fatal("extractZipBinary accepted a missing archive")
	}
}

func TestExtractBinaryDispatchesOnZipSuffix(t *testing.T) {
	data := zipArchive(t, []byte("aigw_1.2.3_windows_amd64/aigw.exe"), []byte("zip-binary"))
	path := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	binary, err := extractBinary(path, "aigw_1.2.3_windows_amd64/aigw.exe")
	if err != nil {
		t.Fatal(err)
	}
	if string(binary) != "zip-binary" {
		t.Fatalf("binary = %q", binary)
	}
}
