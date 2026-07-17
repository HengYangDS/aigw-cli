package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteArchiveIgnoresSourceMTime(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	source := filepath.Join(tmp, "aigw")
	if err := os.WriteFile(source, []byte("deterministic payload\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := archiveEntry{Name: "aigw", Source: source}
	epoch := time.Unix(1784246400, 0).UTC()

	for _, format := range []archiveFormat{formatTarGz, formatZip} {
		format := format
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			first := filepath.Join(tmp, string(format)+"-first")
			second := filepath.Join(tmp, string(format)+"-second")
			if err := os.Chtimes(source, epoch.Add(-24*time.Hour), epoch.Add(-24*time.Hour)); err != nil {
				t.Fatal(err)
			}
			if err := writeArchive(first, format, "aigw_0.1.0-test", epoch, []archiveEntry{entry}); err != nil {
				t.Fatalf("first writeArchive() error = %v", err)
			}
			if err := os.Chtimes(source, epoch.Add(24*time.Hour), epoch.Add(24*time.Hour)); err != nil {
				t.Fatal(err)
			}
			if err := writeArchive(second, format, "aigw_0.1.0-test", epoch, []archiveEntry{entry}); err != nil {
				t.Fatalf("second writeArchive() error = %v", err)
			}
			firstBytes, err := os.ReadFile(first)
			if err != nil {
				t.Fatal(err)
			}
			secondBytes, err := os.ReadFile(second)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(firstBytes, secondBytes) {
				t.Fatalf("%s archives differ after only source mtime changed", format)
			}
		})
	}
}

func TestWriteArchiveRejectsUnsafeEntries(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	source := filepath.Join(tmp, "payload")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeArchive(filepath.Join(tmp, "out.zip"), formatZip, "release", time.Unix(1, 0).UTC(), []archiveEntry{{Name: "../escape", Source: source}})
	if err == nil {
		t.Fatal("writeArchive() error = nil for unsafe archive path")
	}
}
