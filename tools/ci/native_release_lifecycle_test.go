package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func runNativeReleaseLifecycle(t *testing.T, root, oldArtifact, newVersion, endpoint string) {
	t.Helper()
	const oldVersion = "0.0.0"
	newArtifact := buildNativeProgram(t, root, newVersion)
	archive, checksums := writeNativeArchive(t, newArtifact, newVersion)
	journey := newNativeJourney(t, oldArtifact, endpoint, true)
	journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
	before, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}

	journey.requireVersion(oldVersion)
	journey.updateFrom(archive, checksums)
	journey.requireHealthyVersion(newVersion)
	journey.run("update", "--rollback")
	journey.requireHealthyVersion(oldVersion)
	journey.updateFrom(archive, checksums)
	journey.requireHealthyVersion(newVersion)

	journey.uninstallAndRequireOwnedFilesAbsent()
	retained, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	expected := before.Clone()
	clear(expected.Adapters)
	if !reflect.DeepEqual(retained, expected) {
		t.Fatalf("capability configuration changed across program lifecycle\nwant: %#v\ngot: %#v", expected, retained)
	}
}

func (j *journeyFixture) updateFrom(archive, checksums string) {
	j.testing.Helper()
	j.run("update", "--candidate", archive, "--checksums", checksums)
}

func (j *journeyFixture) requireHealthyVersion(version string) {
	j.testing.Helper()
	j.requireVersion(version)
	if got := strings.TrimSpace(string(j.run("credential", "claude"))); got != "native-journey-token" {
		j.testing.Fatalf("credential = %q", got)
	}
	j.requireInstalledProgramFiles()
	j.run("check")
}

func (j *journeyFixture) requireVersion(version string) {
	j.testing.Helper()
	output := strings.TrimSpace(string(j.run("--version")))
	if !strings.Contains(output, version) {
		j.testing.Fatalf("installed version = %q, want %q", output, version)
	}
}

func (j *journeyFixture) requireInstalledProgramFiles() {
	j.testing.Helper()
	entries, err := os.ReadDir(filepath.Dir(j.binary))
	if err != nil {
		j.testing.Fatal(err)
	}
	want := map[string]bool{filepath.Base(j.binary): true, ".aigw.previous": true}
	if runtime.GOOS == "windows" {
		delete(want, ".aigw.previous")
		want[".aigw.previous.exe"] = true
	}
	if len(entries) != len(want) {
		j.testing.Fatalf("installed program directory contains residue: %#v", entryNames(entries))
	}
	for _, entry := range entries {
		if !want[entry.Name()] {
			j.testing.Fatalf("installed program directory contains residue %q", entry.Name())
		}
	}
}

func writeNativeArchive(t *testing.T, binary, version string) (string, string) {
	t.Helper()
	directory := t.TempDir()
	baseName := fmt.Sprintf("aigw_%s_%s_%s", version, runtime.GOOS, runtime.GOARCH)
	archiveName := baseName + ".tar.gz"
	if runtime.GOOS == "windows" {
		archiveName = baseName + ".zip"
	}
	archive := filepath.Join(directory, archiveName)
	payload := readFile(t, binary)
	if runtime.GOOS == "windows" {
		writeZipArchive(t, archive, baseName+"/"+executableName(), payload)
	} else {
		writeTarGzipArchive(t, archive, baseName+"/"+executableName(), payload)
	}
	digest := sha256.Sum256(readFile(t, archive))
	checksums := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksums, []byte(fmt.Sprintf("%x  %s\n", digest, archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	return archive, checksums
}

func writeTarGzipArchive(t *testing.T, path, member string, payload []byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: member, Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	closeArchive(t, tarWriter.Close, gzipWriter.Close, file.Close)
}

func writeZipArchive(t *testing.T, path, member string, payload []byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	header := &zip.FileHeader{Name: member, Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := archive.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(payload); err != nil {
		t.Fatal(err)
	}
	closeArchive(t, archive.Close, file.Close)
}

func closeArchive(t *testing.T, closers ...func() error) {
	t.Helper()
	for _, close := range closers {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
