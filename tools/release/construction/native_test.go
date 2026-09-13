package construction

import (
	"aigw-cli/internal/upgrade/artifact"
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestNativeAcceptanceOwnsBuildConsumptionAndCleanup(t *testing.T) {
	for _, failure := range []string{"", "build", "archive", "acceptance"} {
		t.Run(failure, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			var stage string
			var accepted bool
			want := errors.New("injected failure")
			err := acceptNative(request, "", false, func(call toolCall) error {
				if call.Name == "goreleaser" {
					stage = goReleaserStage(t, call.Args)
					if failure == "build" {
						return want
					}
					writeNativeArchive(t, stage)
					if failure == "archive" {
						return os.Remove(filepath.Join(stage, "checksums.txt"))
					}
					return nil
				}
				accepted = true
				if call.Name != "go" || !slices.Equal(call.Args, []string{"test", "./tools/release", "-run", "^(TestNativeProductJourney|TestNativeRollbackConfigurationAdmission|TestNativeTeamManifestJourney)$", "-count=1", "-v"}) {
					t.Fatalf("native acceptance escaped its existing test owner: %#v", call)
				}
				if !slices.Equal(call.Env, []string{"AIGW_ACCEPTANCE_RELEASE=" + stage}) {
					t.Fatalf("native acceptance environment = %#v", call.Env)
				}
				program := "aigw"
				if runtime.GOOS == "windows" {
					program += ".exe"
				}
				data, err := os.ReadFile(filepath.Join(stage, "aigw_1.2.3_"+runtime.GOOS+"_"+runtime.GOARCH, program))
				if err != nil || string(data) != "native candidate" {
					t.Fatalf("selected native bytes = %q, %v", data, err)
				}
				if failure == "acceptance" {
					return want
				}
				return nil
			})
			if (err != nil) != (failure != "") || accepted != (failure == "" || failure == "acceptance") {
				t.Fatalf("failure=%q accepted=%t error=%v", failure, accepted, err)
			}
			if _, err := os.Stat(filepath.Dir(stage)); !os.IsNotExist(err) {
				t.Fatalf("native build workspace survived: %s, %v", stage, err)
			}
		})
	}
}

func TestNativeArchivePreparationRequiresVerifiedBytes(t *testing.T) {
	for _, scenario := range []string{"valid", "missing", "corrupt", "collision"} {
		t.Run(scenario, func(t *testing.T) {
			stage := t.TempDir()
			writeNativeArchive(t, stage)
			target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
			var err error
			switch scenario {
			case "missing":
				err = os.Remove(filepath.Join(stage, target.ArchiveName("1.2.3")))
			case "corrupt":
				err = os.WriteFile(filepath.Join(stage, target.ArchiveName("1.2.3")), []byte("changed"), 0o600)
			case "collision":
				err = os.WriteFile(filepath.Join(stage, "aigw_1.2.3_"+runtime.GOOS+"_"+runtime.GOARCH), nil, 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = prepareNativeBinary(stage, "1.2.3")
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("archive %s: %v", scenario, err)
			}
		})
	}
}

func TestNativeClientAcceptanceSharesStageAndPropagatesFailure(t *testing.T) {
	for _, test := range []struct {
		name           string
		failure, calls int
	}{
		{"success", 0, 2},
		{"lifecycle failure", 1, 1},
		{"client failure", 2, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			var stage string
			var calls []toolCall
			want := errors.New("acceptance failed")
			err := acceptNative(request, "", true, func(call toolCall) error {
				if call.Name == "goreleaser" {
					stage = goReleaserStage(t, call.Args)
					writeNativeArchive(t, stage)
					return nil
				}
				calls = append(calls, call)
				if len(calls) == test.failure {
					return want
				}
				return nil
			})
			if (test.failure == 0 && err != nil) || (test.failure != 0 && !errors.Is(err, want)) {
				t.Fatalf("acceptance outcome = %v", err)
			}
			if len(calls) != test.calls {
				t.Fatalf("executed %d acceptance commands", len(calls))
			}
			for _, call := range calls {
				if call.Directory != request.Root || !slices.Equal(call.Env, []string{"AIGW_ACCEPTANCE_RELEASE=" + stage}) {
					t.Fatalf("acceptance lost stage ownership: %#v", call)
				}
			}
			if len(calls) == 2 && !slices.Equal(calls[1].Args, []string{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClientJourney$", "-count=1", "-v"}) {
				t.Fatalf("real-client command = %#v", calls[1])
			}
			if _, err := os.Stat(filepath.Dir(stage)); !os.IsNotExist(err) {
				t.Fatalf("acceptance stage survived: %v", err)
			}
		})
	}
}

func TestNativeAcceptanceConsumesExistingArchives(t *testing.T) {
	for _, clients := range []bool{false, true} {
		t.Run(fmt.Sprint(clients), func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			source := t.TempDir()
			writeNativeArchive(t, source)
			before, err := os.ReadFile(filepath.Join(source, "checksums.txt"))
			if err != nil {
				t.Fatal(err)
			}
			var stage string
			calls := 0
			want := errors.New("acceptance failed")
			err = acceptNative(request, source, clients, func(call toolCall) error {
				if call.Name != "go" {
					t.Fatalf("existing artifact acceptance rebuilt: %#v", call)
				}
				stage = strings.TrimPrefix(call.Env[0], "AIGW_ACCEPTANCE_RELEASE=")
				if stage == source {
					t.Fatal("acceptance must own scratch rather than mutate source artifacts")
				}
				calls++
				if calls == 2 || !clients {
					return want
				}
				return nil
			})
			if !errors.Is(err, want) || calls != map[bool]int{false: 1, true: 2}[clients] {
				t.Fatalf("calls=%d result=%v", calls, err)
			}
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatalf("acceptance scratch retained: %v", err)
			}
			after, err := os.ReadFile(filepath.Join(source, "checksums.txt"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("source artifacts changed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(source, "aigw_1.2.3_"+runtime.GOOS+"_"+runtime.GOARCH)); !os.IsNotExist(err) {
				t.Fatalf("source gained extracted state: %v", err)
			}
		})
	}
}

func writeNativeArchive(t *testing.T, stage string) {
	t.Helper()
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	archive := filepath.Join(stage, target.ArchiveName("1.2.3"))
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("aigw_1.2.3_%s_%s/aigw", target.OS, target.Arch)
	payload := []byte("native candidate")
	var entry io.Writer
	closers := []io.Closer{file}
	if target.OS == "windows" {
		writer := zip.NewWriter(file)
		entry, err = writer.Create(name + ".exe")
		closers = append([]io.Closer{writer}, closers...)
	} else {
		compressed := gzip.NewWriter(file)
		writer := tar.NewWriter(compressed)
		err = writer.WriteHeader(&tar.Header{Name: name, Mode: 0o700, Size: int64(len(payload))})
		entry = writer
		closers = append([]io.Closer{writer, compressed}, closers...)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(payload); err != nil {
		t.Fatal(err)
	}
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	checksums := fmt.Sprintf("%x  %s\n", sha256.Sum256(data), filepath.Base(archive))
	if err := os.WriteFile(filepath.Join(stage, "checksums.txt"), []byte(checksums), 0o600); err != nil {
		t.Fatal(err)
	}
}
