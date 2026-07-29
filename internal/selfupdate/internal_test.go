package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rejectingLimitedWriter struct{}

func (rejectingLimitedWriter) Write([]byte) (int, error) {
	return 0, errors.New("write rejected")
}

func TestLimitedWriterPropagatesUnderlyingError(t *testing.T) {
	writer := &limitedWriter{writer: rejectingLimitedWriter{}, limit: 16}
	if count, err := writer.Write([]byte("payload")); count != 0 || err == nil || !strings.Contains(err.Error(), "write rejected") {
		t.Fatalf("Write() = (%d, %v)", count, err)
	}
}

// tarGzForTest builds a single-entry tar.gz fixture for internal (whitebox)
// tests. It mirrors the external test package's tarGz helper.
func tarGzForTest(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// TestComparePrereleaseAllBranches exercises every comparison shape that
// comparePrerelease can encounter: numeric-vs-numeric, numeric-vs-lexical,
// lexical-vs-lexical (equal and unequal), and differing segment counts.
func TestComparePrereleaseAllBranches(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"1", "2", -1},
		{"2", "1", 1},
		{"1", "alpha", -1},
		{"alpha", "1", 1},
		{"alpha", "alpha", 0},
		{"alpha", "beta", -1},
		{"beta", "alpha", 1},
		{"alpha.1", "alpha.1", 0},
		{"alpha", "alpha.1", -1},
		{"alpha.1", "alpha", 1},
	}
	for _, tc := range cases {
		got, err := comparePrerelease(tc.left, tc.right)
		if err != nil {
			t.Fatalf("comparePrerelease(%q,%q) error = %v", tc.left, tc.right, err)
		}
		if got != tc.want {
			t.Fatalf("comparePrerelease(%q,%q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestAllDigitsRejectsEmptyValue(t *testing.T) {
	if allDigits("") {
		t.Fatal("allDigits accepted an empty value")
	}
	if !allDigits("123") {
		t.Fatal("allDigits rejected a numeric value")
	}
	if allDigits("12a") {
		t.Fatal("allDigits accepted a mixed value")
	}
}

func TestIsSHA256RejectsMalformedValues(t *testing.T) {
	if isSHA256(strings.Repeat("a", 63)) {
		t.Fatal("isSHA256 accepted a short value")
	}
	if isSHA256(strings.Repeat("g", 64)) {
		t.Fatal("isSHA256 accepted a non-hex value")
	}
	if !isSHA256(strings.Repeat("a", 64)) {
		t.Fatal("isSHA256 rejected a valid value")
	}
	if !isSHA256(strings.Repeat("F", 64)) {
		t.Fatal("isSHA256 rejected uppercase hex")
	}
}

func TestArchiveVersionRejectsWrongTarget(t *testing.T) {
	if _, err := archiveVersion("aigw_1.2.3_linux_amd64.tar.gz", "darwin", "arm64"); err == nil {
		t.Fatal("archiveVersion accepted a mismatched target")
	}
	if _, err := archiveVersion("aigw_bogus_darwin_arm64.tar.gz", "darwin", "arm64"); err == nil {
		t.Fatal("archiveVersion accepted an invalid embedded version")
	}
	version, err := archiveVersion("aigw_1.2.3_windows_amd64.zip", "windows", "amd64")
	if err != nil || version != "1.2.3" {
		t.Fatalf("version = %q, err = %v", version, err)
	}
}

func TestExpectedBinaryPathFormatsComponents(t *testing.T) {
	got := expectedBinaryPath("1.2.3", "windows", "amd64", "aigw.exe")
	want := "aigw_1.2.3_windows_amd64/aigw.exe"
	if got != want {
		t.Fatalf("expectedBinaryPath = %q, want %q", got, want)
	}
}

func TestRollbackPathVariants(t *testing.T) {
	if got := rollbackPath("/opt/aigw/aigw"); got != "/opt/aigw/.aigw.previous" {
		t.Fatalf("rollbackPath = %q", got)
	}
	if got := rollbackPath("/opt/aigw/aigw.EXE"); got != "/opt/aigw/.aigw.previous.exe" {
		t.Fatalf("rollbackPath = %q", got)
	}
	if got := rollbackPath(`C:\aigw\aigw.exe`); got != `C:\aigw\.aigw.previous.exe` {
		t.Fatalf("rollbackPath = %q", got)
	}
	if got := rollbackPath(`C:\aigw\aigw`); got != `C:\aigw\.aigw.previous` {
		t.Fatalf("rollbackPath = %q", got)
	}
}

func TestWindowsRollbackStagePathAppendsSuffix(t *testing.T) {
	if got := windowsRollbackStagePath(`C:\aigw\aigw.exe`); got != `C:\aigw\aigw.exe.rollback` {
		t.Fatalf("windowsRollbackStagePath = %q", got)
	}
}

func TestParseChannelRejectsUnknownValue(t *testing.T) {
	if _, ok := parseChannel("bogus"); ok {
		t.Fatal("parseChannel accepted an unknown channel")
	}
	if channel, ok := parseChannel(" DEB "); !ok || channel != ChannelDeb {
		t.Fatalf("channel = %q, ok = %v", channel, ok)
	}
}

func TestDetectChannelPrefersBuildTimeInstallChannel(t *testing.T) {
	previous := InstallChannel
	t.Cleanup(func() { InstallChannel = previous })
	InstallChannel = "rpm"
	if got := detectChannel("/usr/bin/aigw"); got != ChannelRPM {
		t.Fatalf("channel = %q, want rpm", got)
	}
}

func TestDetectChannelFallsBackToEnvironmentVariable(t *testing.T) {
	previous := InstallChannel
	t.Cleanup(func() { InstallChannel = previous })
	InstallChannel = "bogus"
	t.Setenv("AIGW_INSTALL_CHANNEL", "msi")
	if got := detectChannel("/usr/bin/aigw"); got != ChannelMSI {
		t.Fatalf("channel = %q, want msi", got)
	}
}

func TestDetectChannelIgnoresInvalidEnvironmentVariable(t *testing.T) {
	previous := InstallChannel
	t.Cleanup(func() { InstallChannel = previous })
	InstallChannel = "bogus"
	t.Setenv("AIGW_INSTALL_CHANNEL", "also-bogus")
	got := detectChannel("/opt/aigw/aigw")
	if got != ChannelPortable {
		t.Fatalf("channel = %q, want portable", got)
	}
}

func TestPackageAssetNameAllChannels(t *testing.T) {
	cases := []struct {
		channel Channel
		goos    string
		goarch  string
		want    string
	}{
		{ChannelPKG, "darwin", "arm64", "aigw_1.2.3_darwin_universal.pkg"},
		{ChannelPKG, "linux", "arm64", ""},
		{ChannelDeb, "linux", "amd64", "aigw_1.2.3_linux_amd64.deb"},
		{ChannelDeb, "darwin", "amd64", ""},
		{ChannelRPM, "linux", "amd64", "aigw_1.2.3_linux_amd64.rpm"},
		{ChannelRPM, "darwin", "amd64", ""},
		{ChannelMSI, "windows", "amd64", "aigw_1.2.3_windows_amd64.msi"},
		{ChannelMSI, "linux", "amd64", ""},
		{ChannelPortable, "darwin", "amd64", ""},
	}
	for _, tc := range cases {
		u := Updater{Channel: tc.channel, GOOS: tc.goos, GOARCH: tc.goarch}
		if got := u.packageAssetName("1.2.3"); got != tc.want {
			t.Fatalf("channel=%s goos=%s: packageAssetName = %q, want %q", tc.channel, tc.goos, got, tc.want)
		}
	}
}

type fakeCommandRunner struct {
	calls [][]string
	fail  map[string]error
}

func (r *fakeCommandRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	if err, ok := r.fail[name]; ok {
		return nil, err
	}
	return []byte("ok"), nil
}

func TestRunPackageInstallerAllChannels(t *testing.T) {
	cases := []struct {
		channel Channel
		prefix  []string
	}{
		{ChannelPKG, []string{"open"}},
		{ChannelDeb, []string{"sudo", "dpkg", "-i"}},
		{ChannelRPM, []string{"sudo", "rpm", "-Uvh"}},
		{ChannelMSI, []string{"msiexec", "/i"}},
	}
	for _, tc := range cases {
		runner := &fakeCommandRunner{}
		u := Updater{Channel: tc.channel, Runner: runner}
		if err := u.runPackageInstaller(context.Background(), "/tmp/pkg"); err != nil {
			t.Fatalf("channel=%s error = %v", tc.channel, err)
		}
		want := append(append([]string{}, tc.prefix...), "/tmp/pkg")
		if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != strings.Join(want, " ") {
			t.Fatalf("channel=%s calls = %v, want %v", tc.channel, runner.calls, want)
		}
	}
}

func TestRunPackageInstallerPropagatesFailureForEachChannel(t *testing.T) {
	cases := []struct {
		channel Channel
		command string
	}{
		{ChannelPKG, "open"},
		{ChannelDeb, "sudo"},
		{ChannelRPM, "sudo"},
		{ChannelMSI, "msiexec"},
	}
	for _, tc := range cases {
		runner := &fakeCommandRunner{fail: map[string]error{tc.command: errors.New("boom")}}
		u := Updater{Channel: tc.channel, Runner: runner}
		if err := u.runPackageInstaller(context.Background(), "/tmp/pkg"); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("channel=%s error = %v", tc.channel, err)
		}
	}
}

func TestRunPackageInstallerRejectsUnknownChannel(t *testing.T) {
	u := Updater{Channel: "bogus", Runner: &fakeCommandRunner{}}
	if err := u.runPackageInstaller(context.Background(), "/tmp/pkg"); err == nil || !strings.Contains(err.Error(), "unknown installation channel") {
		t.Fatalf("error = %v", err)
	}
}

// withFakeCmd installs a fake `cmd` executable on PATH for the duration of
// the test so scheduleWindowsReplacement/scheduleWindowsRollback can be
// exercised end-to-end without requiring an actual Windows host.
func withFakeCmd(t *testing.T, succeed bool) {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	if !succeed {
		script = "#!/bin/sh\nexit 1\n"
	}
	path := filepath.Join(directory, "cmd")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestScheduleWindowsReplacementStagesBinaryAndLaunchesHelper(t *testing.T) {
	withFakeCmd(t, true)
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	message, err := u.scheduleWindowsReplacement([]byte("new-binary"), "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "v1.2.3") {
		t.Fatalf("message = %q", message)
	}
	staged, err := os.ReadFile(executable + ".update")
	if err != nil || string(staged) != "new-binary" {
		t.Fatalf("staged=%q err=%v", staged, err)
	}
	if _, err := os.Stat(executable + ".update.cmd"); err != nil {
		t.Fatalf("update script missing: %v", err)
	}
}

func TestScheduleWindowsReplacementRejectsEmptyExecutable(t *testing.T) {
	t.Chdir(t.TempDir())
	withFakeCmd(t, true)
	u := Updater{Executable: " "}
	if _, err := u.scheduleWindowsReplacement([]byte("data"), "v1.0.0"); err == nil {
		t.Fatal("scheduleWindowsReplacement accepted an empty executable path")
	}
}

func TestScheduleWindowsReplacementFailsWhenStagingCannotBeWritten(t *testing.T) {
	withFakeCmd(t, true)
	directory := t.TempDir()
	executable := filepath.Join(directory, "missing-parent", "aigw.exe")
	u := Updater{Executable: executable}
	if _, err := u.scheduleWindowsReplacement([]byte("data"), "v1.0.0"); err == nil || !strings.Contains(err.Error(), "stage Windows update") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduleWindowsReplacementFailsWhenHelperCannotStart(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	if _, err := u.scheduleWindowsReplacement([]byte("data"), "v1.0.0"); err == nil || !strings.Contains(err.Error(), "start Windows update helper") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduleWindowsRollbackRestoresPreviousBinary(t *testing.T) {
	withFakeCmd(t, true)
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	message, err := u.scheduleWindowsRollback()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "scheduled") {
		t.Fatalf("message = %q", message)
	}
	staged, err := os.ReadFile(windowsRollbackStagePath(executable))
	if err != nil || string(staged) != "previous" {
		t.Fatalf("staged=%q err=%v", staged, err)
	}
}

func TestScheduleWindowsRollbackRequiresPreviousBinary(t *testing.T) {
	withFakeCmd(t, true)
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	if _, err := u.scheduleWindowsRollback(); err == nil || !strings.Contains(err.Error(), "no previous portable AIGW binary") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduleWindowsRollbackFailsWhenExecutableMissing(t *testing.T) {
	withFakeCmd(t, true)
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	if _, err := u.scheduleWindowsRollback(); err == nil || !strings.Contains(err.Error(), "inspect current AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduleWindowsRollbackFailsWhenHelperCannotStart(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	if _, err := u.scheduleWindowsRollback(); err == nil || !strings.Contains(err.Error(), "start Windows AIGW rollback helper") {
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
	for index := 0; index < len(entries); index += 2 {
		name, data := string(entries[index]), entries[index+1]
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
	for i := 0; i < 2; i++ {
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

func TestFileSHA256RejectsMissingFile(t *testing.T) {
	if _, err := fileSHA256(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("fileSHA256 accepted a missing file")
	}
}

func TestVerifyChecksumRejectsInvalidArchiveName(t *testing.T) {
	directory := t.TempDir()
	if err := verifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), "nested/name"); err == nil {
		t.Fatal("verifyChecksum accepted a nested archive name")
	}
	if err := verifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), ""); err == nil {
		t.Fatal("verifyChecksum accepted an empty archive name")
	}
}

func TestVerifyChecksumRejectsMissingChecksumsFile(t *testing.T) {
	directory := t.TempDir()
	if err := verifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "missing.txt"), "a"); err == nil || !strings.Contains(err.Error(), "read checksums") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsMissingEntry(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte("deadbeef  other-name\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), "a"); err == nil || !strings.Contains(err.Error(), "checksum entry missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyChecksumRejectsUnreadableArchive(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(strings.Repeat("a", 64)+"  a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(filepath.Join(directory, "a"), filepath.Join(directory, "checksums.txt"), "a"); err == nil || !strings.Contains(err.Error(), "open update archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreservePreviousBinaryRejectsMissingSource(t *testing.T) {
	if err := preservePreviousBinary(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("preservePreviousBinary accepted a missing source")
	}
}

func TestReplacePortableBinaryPropagatesPreserveFailure(t *testing.T) {
	if err := (Updater{Executable: filepath.Join(t.TempDir(), "missing")}).replacePortableBinary([]byte("data")); err == nil {
		t.Fatal("replacePortableBinary accepted a missing executable")
	}
}

func TestInstallPortableArchiveRejectsChecksumMismatch(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.WriteFile(archivePath, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(directory, "aigw")}
	if _, _, err := u.installPortableArchive(archivePath, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
}
