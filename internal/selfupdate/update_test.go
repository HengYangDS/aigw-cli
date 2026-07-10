package selfupdate_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/selfupdate"
)

type fakeRunner struct {
	archive  []byte
	checksum string
	calls    [][]string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "release" && args[1] == "list" {
		return []byte(`[{"tag_name":"v0.2.0"}]`), nil
	}
	if len(args) >= 2 && args[0] == "release" && args[1] == "download" {
		dir, asset := "", ""
		for i, arg := range args {
			if arg == "--dir" {
				dir = args[i+1]
			}
			if arg == "--asset-name" {
				asset = args[i+1]
			}
		}
		if asset == "checksums.txt" {
			return nil, os.WriteFile(filepath.Join(dir, asset), []byte(r.checksum), 0o600)
		}
		return nil, os.WriteFile(filepath.Join(dir, asset), r.archive, 0o600)
	}
	switch name {
	case "open", "sudo", "msiexec":
		return []byte("ok"), nil
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func TestUpdateDownloadsVerifiesAndAtomicallyReplacesBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{
		archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, name),
	}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "new-binary" || !strings.Contains(message, "v0.2.0") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
}

func TestUpdateRefusesChecksumMismatch(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_linux_amd64/aigw", []byte("new-binary"))
	binary := filepath.Join(t.TempDir(), "aigw")
	_ = os.WriteFile(binary, []byte("old-binary"), 0o755)
	runner := &fakeRunner{
		archive: archive, checksum: strings.Repeat("0", 64) + "  ./aigw_0.2.0_linux_amd64.tar.gz\n",
	}
	u := selfupdate.Updater{GOOS: "linux", GOARCH: "amd64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "old-binary" {
		t.Fatalf("old binary replaced after checksum failure: %q", got)
	}
}

func TestPackageManagedDebUpdateDownloadsVerifiesAndInvokesPackageManager(t *testing.T) {
	payload := []byte("deb package")
	sum := sha256.Sum256(payload)
	name := "aigw_0.2.0_linux_amd64.deb"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{archive: payload, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := selfupdate.Updater{GOOS: "linux", GOARCH: "amd64", Channel: selfupdate.ChannelDeb, Executable: binary, Runner: runner}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "old-binary" {
		t.Fatalf("package-managed update replaced binary directly: %q", got)
	}
	if !strings.Contains(message, "包管理器") {
		t.Fatalf("message = %q", message)
	}
	if !runner.called("sudo", "dpkg", "-i") {
		t.Fatalf("dpkg install was not invoked; calls=%v", runner.calls)
	}
}

func TestMacPackageUpdateUsesUniversalPkgAsset(t *testing.T) {
	payload := []byte("pkg package")
	sum := sha256.Sum256(payload)
	name := "aigw_0.2.0_darwin_universal.pkg"
	runner := &fakeRunner{archive: payload, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := selfupdate.Updater{GOOS: "darwin", GOARCH: "arm64", Channel: selfupdate.ChannelPKG, Executable: filepath.Join(t.TempDir(), "aigw"), Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if !runner.downloaded(name) {
		t.Fatalf("universal pkg asset was not downloaded; calls=%v", runner.calls)
	}
	if !runner.called("open") {
		t.Fatalf("macOS installer was not opened; calls=%v", runner.calls)
	}
}

func TestCurrentUsesBuildTimeInstallChannel(t *testing.T) {
	previous := selfupdate.InstallChannel
	t.Cleanup(func() { selfupdate.InstallChannel = previous })
	selfupdate.InstallChannel = "rpm"
	updater := selfupdate.Current("/usr/bin/aigw")
	if updater.Channel != selfupdate.ChannelRPM {
		t.Fatalf("channel = %q, want rpm", updater.Channel)
	}
}

func (r *fakeRunner) downloaded(asset string) bool {
	for _, call := range r.calls {
		for i, part := range call {
			if part == "--asset-name" && i+1 < len(call) && call[i+1] == asset {
				return true
			}
		}
	}
	return false
}

func (r *fakeRunner) called(prefix ...string) bool {
	for _, call := range r.calls {
		if len(call) < len(prefix) {
			continue
		}
		ok := true
		for i := range prefix {
			if call[i] != prefix[i] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func tarGz(t *testing.T, name string, data []byte) []byte {
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
