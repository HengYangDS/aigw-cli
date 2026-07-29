package selfupdate

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

func TestFileSHA256RejectsUnreadableSource(t *testing.T) {
	if _, err := fileSHA256(t.TempDir()); err == nil {
		t.Fatal("fileSHA256 accepted a directory as input")
	}
}

func TestFileSHA256ComputesDigestOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
}

func TestCompareVersionsPropagatesLeftParseError(t *testing.T) {
	if _, err := compareVersions("not-a-version", "1.0.0"); err == nil {
		t.Fatal("compareVersions accepted a malformed left version")
	}
}

func TestCompareVersionsPropagatesRightParseError(t *testing.T) {
	if _, err := compareVersions("1.0.0", "not-a-version"); err == nil {
		t.Fatal("compareVersions accepted a malformed right version")
	}
}

func TestCompareVersionsTreatsPrereleaseAsOlderThanRelease(t *testing.T) {
	got, err := compareVersions("1.0.0", "1.0.0-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("compareVersions(release, prerelease) = %d, want 1", got)
	}
}

func TestCompareVersionsTreatsReleaseAsNewerThanPrerelease(t *testing.T) {
	got, err := compareVersions("1.0.0-alpha", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if got != -1 {
		t.Fatalf("compareVersions(prerelease, release) = %d, want -1", got)
	}
}

func TestCompareVersionsComparesCoreComponents(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.2.0", "1.1.0", 1},
		{"1.1.2", "1.1.1", 1},
		{"1.0.0", "1.0.0", 0},
	}
	for _, tc := range cases {
		got, err := compareVersions(tc.left, tc.right)
		if err != nil {
			t.Fatalf("compareVersions(%q,%q) error = %v", tc.left, tc.right, err)
		}
		if got != tc.want {
			t.Fatalf("compareVersions(%q,%q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestParseVersionRejectsEmptyValue(t *testing.T) {
	if _, err := parseVersion(""); err == nil {
		t.Fatal("parseVersion accepted an empty value")
	}
	if _, err := parseVersion("v"); err == nil {
		t.Fatal("parseVersion accepted a bare v prefix")
	}
}

func TestParseVersionRejectsWrongNumberOfCoreComponents(t *testing.T) {
	if _, err := parseVersion("1.2"); err == nil {
		t.Fatal("parseVersion accepted a two-component version")
	}
	if _, err := parseVersion("1.2.3.4"); err == nil {
		t.Fatal("parseVersion accepted a four-component version")
	}
}

func TestParseVersionRejectsNonNumericCoreComponent(t *testing.T) {
	if _, err := parseVersion("1.a.3"); err == nil {
		t.Fatal("parseVersion accepted a non-numeric core component")
	}
}

func TestParseVersionRejectsInvalidPrerelease(t *testing.T) {
	if _, err := parseVersion("1.2.3-alpha!"); err == nil {
		t.Fatal("parseVersion accepted a prerelease with an invalid character")
	}
}

func TestParseVersionAcceptsValidPrerelease(t *testing.T) {
	parsed, err := parseVersion("v1.2.3-alpha.1")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.core != [3]uint64{1, 2, 3} || parsed.pre != "alpha.1" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestValidPrereleaseRejectsEmptySegment(t *testing.T) {
	if validPrerelease("alpha..1") {
		t.Fatal("validPrerelease accepted an empty segment")
	}
	if validPrerelease(".alpha") {
		t.Fatal("validPrerelease accepted a leading empty segment")
	}
}

func TestValidPrereleaseRejectsInvalidCharacter(t *testing.T) {
	if validPrerelease("alpha!") {
		t.Fatal("validPrerelease accepted an invalid character")
	}
	if validPrerelease("alpha_beta") {
		t.Fatal("validPrerelease accepted an underscore")
	}
}

func TestValidPrereleaseRejectsOversizedNumericSegment(t *testing.T) {
	// 20 nines overflow uint64 (max ~1.8e19) while still being all-digits, so
	// this exercises the allDigits fallback rejection after prereleaseNumber
	// fails to parse it as a bounded integer.
	if validPrerelease("99999999999999999999") {
		t.Fatal("validPrerelease accepted an oversized all-digit segment")
	}
}

func TestValidPrereleaseAcceptsNumericAndAlphanumericSegments(t *testing.T) {
	if !validPrerelease("alpha.1.beta-2") {
		t.Fatal("validPrerelease rejected a valid prerelease")
	}
}

func TestVerifyChecksumSkipsMalformedAndMismatchedLines(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	content := []byte("archive-bytes")
	if err := os.WriteFile(archivePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fileSHA256ForTest(t, archivePath)
	checksums := strings.Join([]string{
		"not-enough-fields",
		"nothex-nothex-nothex  " + archiveName,
		strings.Repeat("a", 64) + "  other-name",
		"./" + sum + "  should-not-match", // wrong field order, ignored
		sum + "  ./" + archiveName,
	}, "\n")
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(checksums), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archivePath, checksumsPath, archiveName); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyChecksumRejectsDuplicateEntry(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.WriteFile(archivePath, []byte("archive-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fileSHA256ForTest(t, archivePath)
	checksums := sum + "  " + archiveName + "\n" + sum + "  " + archiveName + "\n"
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(checksums), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archivePath, checksumsPath, archiveName); err == nil || !strings.Contains(err.Error(), "duplicate checksum entry") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsHashMismatch(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.WriteFile(archivePath, []byte("archive-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archivePath, checksumsPath, archiveName); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsUnreadableArchiveContent(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.Mkdir(archivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archivePath, checksumsPath, archiveName); err == nil || !strings.Contains(err.Error(), "hash update archive") {
		t.Fatalf("error = %v", err)
	}
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
