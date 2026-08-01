package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunArchive(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	output := filepath.Join(tmp, "release.zip")
	if err := os.WriteFile(source, []byte("payload"), 0o640); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := run([]string{"-format", "zip", "-output", output, "-root", "release", "-epoch", "1", "-entry", "bin/aigw=" + source}, &stderr)
	if code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	if len(reader.File) != 2 || reader.File[0].Name != "release/" || reader.File[1].Name != "release/bin/aigw" {
		t.Fatalf("archive members = %#v, want release root and payload", reader.File)
	}
}

func TestRunArchiveRejectsInvalidArguments(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	validPrefix := []string{"-format", "zip", "-output", filepath.Join(tmp, "out.zip"), "-root", "release", "-epoch", "1"}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "usage", args: nil, want: "usage: archive"},
		{name: "unknown flag", args: []string{"-unknown"}, want: "flag provided but not defined"},
		{name: "malformed entry", args: append(append([]string(nil), validPrefix...), "-entry", "payload"), want: "invalid archive entry"},
		{name: "unsupported format", args: append(append([]string(nil), validPrefix...), "-format", "rar", "-entry", "payload="+source), want: "unsupported archive format"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(test.args, &stderr); code != 2 {
				t.Fatalf("run() code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("run() stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestEntryFlagAndParseEntries(t *testing.T) {
	var values entryFlag
	if err := values.Set("a=one"); err != nil {
		t.Fatal(err)
	}
	if err := values.Set("b=two"); err != nil {
		t.Fatal(err)
	}
	if got := values.String(); got != "a=one,b=two" {
		t.Fatalf("String() = %q", got)
	}
	entries, err := parseEntries(values)
	if err != nil || len(entries) != 2 || entries[1].Name != "b" || entries[1].Source != "two" {
		t.Fatalf("parseEntries() = %#v, %v", entries, err)
	}
	for _, raw := range [][]string{{"missing-separator"}, {"missing-source="}} {
		if _, err := parseEntries(raw); err == nil {
			t.Fatalf("parseEntries(%q) error = nil", raw)
		}
	}
}

func TestValidateArchiveName(t *testing.T) {
	for _, value := range []string{"", "/absolute", "a/../b", ".", "../escape", `a\b`} {
		if err := validateArchiveName(value); err == nil {
			t.Errorf("validateArchiveName(%q) error = nil", value)
		}
	}
	if err := validateArchiveName("bin/aigw"); err != nil {
		t.Fatalf("validateArchiveName(valid) error = %v", err)
	}
}

func TestNormalizeEntriesRejectsInvalidInputs(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		entries []archiveEntry
	}{
		{name: "empty"},
		{name: "invalid name", entries: []archiveEntry{{Name: "../payload", Source: source}}},
		{name: "duplicate", entries: []archiveEntry{{Name: "payload", Source: source}, {Name: "payload", Source: source}}},
		{name: "missing source", entries: []archiveEntry{{Name: "payload", Source: filepath.Join(tmp, "missing")}}},
		{name: "non regular source", entries: []archiveEntry{{Name: "payload", Source: tmp}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := normalizeEntries(test.entries); err == nil {
				t.Fatal("normalizeEntries() error = nil")
			}
		})
	}
}

func TestWriteArchiveReportsCreateFailure(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeArchive(filepath.Join(tmp, "missing", "out.zip"), formatZip, "release", time.Unix(1, 0).UTC(), []archiveEntry{{Name: "payload", Source: source}})
	if err == nil {
		t.Fatal("writeArchive() error = nil")
	}
	if err := writeArchive(filepath.Join(tmp, "out.zip"), formatZip, "../release", time.Unix(1, 0).UTC(), nil); err == nil {
		t.Fatal("writeArchive(invalid root) error = nil")
	}
}

func TestArchiveWritersReportInputAndOutputErrors(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := archiveEntry{Name: "missing", Source: filepath.Join(tmp, "missing")}
	epoch := time.Unix(1, 0).UTC()

	t.Run("tar root output", func(t *testing.T) {
		if err := writeTarGz(closedFile(t), "release", epoch, nil); err == nil {
			t.Fatal("writeTarGz() error = nil")
		}
	})
	t.Run("tar missing entry", func(t *testing.T) {
		if err := writeTarGz(&bytes.Buffer{}, "release", epoch, []archiveEntry{missing}); err == nil {
			t.Fatal("writeTarGz() error = nil")
		}
	})
	t.Run("tar closed writer", func(t *testing.T) {
		writer := tar.NewWriter(&bytes.Buffer{})
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := writeTarEntry(writer, "release", epoch, archiveEntry{Name: "payload", Source: source}); err == nil {
			t.Fatal("writeTarEntry() error = nil")
		}
	})
	t.Run("tar unreadable entry", func(t *testing.T) {
		writer := tar.NewWriter(&bytes.Buffer{})
		if err := writeTarEntry(writer, "release", epoch, archiveEntry{Name: "directory", Source: tmp}); err == nil {
			t.Fatal("writeTarEntry() error = nil")
		}
	})
	t.Run("zip root output", func(t *testing.T) {
		if err := writeZip(closedFile(t), "release", epoch, nil); err == nil {
			t.Fatal("writeZip() error = nil")
		}
	})
	t.Run("zip oversized root", func(t *testing.T) {
		if err := writeZip(&bytes.Buffer{}, strings.Repeat("x", 1<<16), epoch, nil); err == nil {
			t.Fatal("writeZip() error = nil")
		}
	})
	t.Run("zip missing entry", func(t *testing.T) {
		if err := writeZip(&bytes.Buffer{}, "release", epoch, []archiveEntry{missing}); err == nil {
			t.Fatal("writeZip() error = nil")
		}
	})
	t.Run("zip oversized name", func(t *testing.T) {
		writer := zip.NewWriter(&bytes.Buffer{})
		if err := writeZipEntry(writer, "release", epoch, archiveEntry{Name: strings.Repeat("x", 1<<16), Source: source}); err == nil {
			t.Fatal("writeZipEntry() error = nil")
		}
	})
	t.Run("zip unreadable entry", func(t *testing.T) {
		writer := zip.NewWriter(&bytes.Buffer{})
		if err := writeZipEntry(writer, "release", epoch, archiveEntry{Name: "directory", Source: tmp}); err == nil {
			t.Fatal("writeZipEntry() error = nil")
		}
	})
}

func closedFile(t *testing.T) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file
}

type writeCloserStub struct {
	io.Writer
	closeErr error
}

func (w writeCloserStub) Close() error { return w.closeErr }

type readCloserStub struct {
	io.Reader
	closeErr error
}

func (r readCloserStub) Close() error { return r.closeErr }

// failAfterWriter accepts writes until the allowed byte budget is exhausted,
// then reports an error. It lets tests drive a real *tar.Writer into its
// Close() failure path deterministically, without depending on any
// platform-specific behavior.
type failAfterWriter struct {
	allowed int
	written int
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.allowed {
		return 0, errors.New("write limit exceeded")
	}
	w.written += len(p)
	return len(p), nil
}

func TestWriteArchiveContentsReportsOutputCloseFailure(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := normalizeEntries([]archiveEntry{{Name: "payload", Source: source}})
	if err != nil {
		t.Fatal(err)
	}
	output := writeCloserStub{Writer: &bytes.Buffer{}, closeErr: errors.New("close failed")}
	err = writeArchiveContents(output, formatZip, "release", time.Unix(1, 0).UTC(), entries)
	if err == nil || !strings.Contains(err.Error(), "close output archive") {
		t.Fatalf("writeArchiveContents() error = %v", err)
	}
}

func TestWriteTarStreamReportsCloseFailure(t *testing.T) {
	writer := &failAfterWriter{allowed: 512}
	if err := writeTarStream(writer, "release", time.Unix(1, 0).UTC(), nil); err == nil {
		t.Fatal("writeTarStream() error = nil")
	}
}

func TestWriteTarEntryContentsReportsSourceCloseFailure(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	file := readCloserStub{Reader: strings.NewReader("payload"), closeErr: errors.New("close failed")}
	writer := tar.NewWriter(&bytes.Buffer{})
	err = writeTarEntryContents(writer, "release", time.Unix(1, 0).UTC(), archiveEntry{Name: "payload"}, info, file)
	if err == nil || !strings.Contains(err.Error(), "close archive entry") {
		t.Fatalf("writeTarEntryContents() error = %v", err)
	}
}

func TestWriteZipEntryContentsReportsSourceCloseFailure(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	file := readCloserStub{Reader: strings.NewReader("payload"), closeErr: errors.New("close failed")}
	writer := zip.NewWriter(&bytes.Buffer{})
	err = writeZipEntryContents(writer, "release", time.Unix(1, 0).UTC(), archiveEntry{Name: "payload"}, info, file)
	if err == nil || !strings.Contains(err.Error(), "close archive entry") {
		t.Fatalf("writeZipEntryContents() error = %v", err)
	}
}
