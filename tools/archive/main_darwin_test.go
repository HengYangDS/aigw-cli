package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"net"
	"os"
	"testing"
	"time"
)

func TestArchiveEntriesReportOpenFailures(t *testing.T) {
	socketFile, err := os.CreateTemp(".", ".archive-socket-*")
	if err != nil {
		t.Fatal(err)
	}
	socket := socketFile.Name()
	if err := socketFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(socket); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(socket) })
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	epoch := time.Unix(1, 0).UTC()
	entry := archiveEntry{Name: "socket", Source: socket}

	if err := writeTarEntry(tar.NewWriter(&bytes.Buffer{}), "release", epoch, entry); err == nil {
		t.Fatal("writeTarEntry(socket) error = nil")
	}
	if err := writeZipEntry(zip.NewWriter(&bytes.Buffer{}), "release", epoch, entry); err == nil {
		t.Fatal("writeZipEntry(socket) error = nil")
	}
}
