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
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			var workspace string
			want := errors.New("injected acceptance failure")
			err := acceptNative(request, "", false, "", func(call toolCall) error {
				if call.Name != "go" || call.Directory != request.Root || !slices.Equal(call.Args, []string{"test", "./tools/release", "-run", "^(TestNativeProductJourney|TestNativeRollbackConfigurationAdmission|TestNativeTeamManifestJourney)$", "-count=1", "-v"}) {
					t.Fatalf("source acceptance escaped its product test owner: %#v", call)
				}
				workspace = strings.TrimPrefix(call.Env[1], "TMPDIR=")
				if !filepath.IsAbs(workspace) || !slices.Equal(call.Env, []string{"AIGW_ACCEPTANCE_RELEASE=", "TMPDIR=" + workspace, "TMP=" + workspace, "TEMP=" + workspace}) {
					t.Fatalf("source acceptance environment = %#v", call.Env)
				}
				if err := os.WriteFile(filepath.Join(workspace, "test-owned-output"), []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
				if fail {
					return want
				}
				return nil
			})
			if (fail && !errors.Is(err, want)) || (!fail && err != nil) {
				t.Fatalf("failure=%t error=%v", fail, err)
			}
			if workspace == "" {
				t.Fatal("source acceptance never executed")
			}
			if _, err := os.Stat(workspace); !os.IsNotExist(err) {
				t.Fatalf("native acceptance workspace survived: %s, %v", workspace, err)
			}
		})
	}
}

func TestNativeClientSourceAcceptanceSelectsOneHostBuild(t *testing.T) {
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "0"}
	var stage string
	calls := 0
	err := acceptNative(request, "", true, "", func(call toolCall) error {
		calls++
		if calls == 1 {
			if call.Name != "goreleaser" || !slices.Contains(call.Env, "AIGW_BUILD_OS="+runtime.GOOS) {
				t.Fatalf("client source acceptance selected wrong build: %#v", call)
			}
			stage = goReleaserStage(t, call.Args)
			writeNativeArchive(t, stage)
			return nil
		}
		if call.Name != "go" || !slices.Contains(call.Env, "AIGW_ACCEPTANCE_RELEASE="+stage) {
			t.Fatalf("client acceptance lost built artifact identity: %#v", call)
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("native client build calls=%d error=%v", calls, err)
	}
	if _, err := os.Stat(filepath.Dir(stage)); !os.IsNotExist(err) {
		t.Fatalf("client build workspace survived: %s, %v", stage, err)
	}
}

func TestBuildNativeRejectsInvalidInputsWithoutOwningCallerWorkspace(t *testing.T) {
	for _, failure := range []string{"changelog", "version", "configuration", "workspace"} {
		t.Run(failure, func(t *testing.T) {
			root, workspace, version := releaseRoot(t), t.TempDir(), "1.2.3"
			if failure == "version" {
				version = "invalid"
			}
			if failure != "changelog" {
				if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("## ["+version+"] - 2026-09-15\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "configuration" {
				if err := os.Remove(filepath.Join(root, ".config", "release", "goreleaser.yaml")); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "workspace" {
				workspace = filepath.Join(workspace, "not-a-directory")
				if err := os.WriteFile(workspace, []byte("caller-owned"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stage, err := BuildNative(t.Context(), root, workspace, version)
			if err == nil || stage != "" {
				t.Fatalf("%s: stage=%q error=%v", failure, stage, err)
			}
			if _, err := os.Stat(workspace); err != nil {
				t.Fatalf("builder removed caller workspace: %v", err)
			}
		})
	}
}

func TestNativeBuildNeedsNoPublisherCredentials(t *testing.T) {
	for _, platform := range []string{"linux", "windows", "darwin", ""} {
		t.Run(platform, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "0", TargetOS: platform}
			t.Setenv("AIGW_MACOS_SIGNING_IDENTITY", strings.Repeat("a", 40))
			for _, name := range []string{"AIGW_MACOS_SIGNING_P12", "AIGW_MACOS_SIGNING_PASSWORD_FILE", "AIGW_MACOS_SIGNING_REQUIREMENTS"} {
				t.Setenv(name, "")
			}
			calls := 0
			_, err := buildArchives(request, t.TempDir(), func(call toolCall) error {
				calls++
				if !slices.Contains(call.Env, "AIGW_MACOS_SIGNING_IDENTITY=") {
					t.Fatal("native build inherited publisher identity")
				}
				if !slices.Contains(call.Env, "AIGW_BUILD_OS="+platform) {
					t.Fatal("build selection was not passed to GoReleaser")
				}

				if slices.Contains(call.Args, "--skip=homebrew") != (platform == "windows") {
					t.Fatalf("Homebrew generation does not match target %q: %v", platform, call.Args)
				}
				return nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("platform=%q calls=%d error=%v", platform, calls, err)
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
			source := t.TempDir()
			writeNativeArchive(t, source)
			var stage string
			var calls []toolCall
			want := errors.New("acceptance failed")
			err := acceptNative(request, source, true, "", func(call toolCall) error {
				if call.Name != "go" {
					t.Fatalf("published client acceptance rebuilt artifacts: %#v", call)
				}
				stage = strings.TrimPrefix(call.Env[0], "AIGW_ACCEPTANCE_RELEASE=")
				calls = append(calls, call)
				if len(calls) == test.failure {
					return want
				}
				return nil
			})
			if (test.failure == 0 && err != nil) || (test.failure != 0 && !errors.Is(err, want)) {
				t.Fatalf("acceptance outcome = %v", err)
			}
			if len(calls) != test.calls || stage == source || !filepath.IsAbs(stage) {
				t.Fatalf("executed %d commands with stage %q", len(calls), stage)
			}
			for _, call := range calls {
				if call.Directory != request.Root || !slices.Equal(call.Env, []string{"AIGW_ACCEPTANCE_RELEASE=" + stage, "TMPDIR=" + stage, "TMP=" + stage, "TEMP=" + stage}) {
					t.Fatalf("acceptance lost stage ownership: %#v", call)
				}
			}
			if len(calls) == 2 && !slices.Equal(calls[1].Args, []string{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClientJourney$", "-count=1", "-v"}) {
				t.Fatalf("real-client command = %#v", calls[1])
			}
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
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
			err = acceptNative(request, source, clients, "", func(call toolCall) error {
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

func TestNativePerformanceOwnsResultsAndCleanup(t *testing.T) {
	for _, test := range []struct {
		name   string
		failAt int
		emit   bool
	}{
		{"success", 0, true}, {"lifecycle failure", 1, false},
		{"performance failure", 2, false}, {"missing results", 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			source := t.TempDir()
			writeNativeArchive(t, source)
			output := filepath.Join(t.TempDir(), "measurements")
			var stage string
			calls := 0
			err := acceptNative(request, source, false, output, func(call toolCall) error {
				calls++
				stage = strings.TrimPrefix(call.Env[0], "AIGW_ACCEPTANCE_RELEASE=")
				if call.Name != "go" || call.Directory != request.Root || stage == source {
					t.Fatalf("performance escaped native stage: %#v", call)
				}
				if calls == test.failAt {
					return errors.New(test.name)
				}
				if calls == 1 {
					return nil
				}
				want := []string{"test", "-tags=performance_acceptance", "./tools/release", "-run", "^TestNativePerformance$", "-count=1", "-v"}
				if !slices.Equal(call.Args, want) || !slices.Contains(call.Env, "AIGW_PERFORMANCE_OUTPUT="+output) {
					t.Fatalf("performance dispatch = %#v", call)
				}
				if !test.emit {
					return nil
				}
				if err := os.Mkdir(output, 0o700); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(output, "summary.json"), []byte(`{"blocks":[{}]}`), 0o600)
			})
			if (err == nil) != test.emit {
				t.Fatalf("%s: %v", test.name, err)
			}
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatalf("native scratch survived %s: %v", test.name, err)
			}
			if test.emit {
				if _, err := os.Stat(filepath.Join(output, "summary.json")); err != nil {
					t.Fatalf("performance evidence not retained: %v", err)
				}
			}
		})
	}
}

func TestNativePerformanceOutputAdmission(t *testing.T) {
	for _, output := range []string{"relative-performance", t.TempDir()} {
		request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
		err := acceptNative(request, "", false, output, func(call toolCall) error {
			t.Fatalf("invalid output admitted an external command: %#v", call)
			return nil
		})
		if err == nil {
			t.Fatalf("invalid performance output accepted: %s", output)
		}
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

func TestPublisherSigningIdentityIsExplicitAndNative(t *testing.T) {
	identity := strings.Repeat("a", 40)
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "0", MacOSSigningIdentity: identity}
	_, err := buildArchives(request, t.TempDir(), func(call toolCall) error {
		if !slices.Contains(call.Env, "AIGW_MACOS_SIGNING_IDENTITY="+identity) {
			t.Fatal("publisher identity missing")
		}
		return nil
	})
	if runtime.GOOS == "darwin" && err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "darwin" && err == nil {
		t.Fatal("native signing accepted on another operating system")
	}
	for _, value := range []string{"publisher name", `a" --anything`, strings.Repeat("g", 40)} {
		request.MacOSSigningIdentity = value
		if err := validateRequest(request); err == nil {
			t.Fatalf("invalid fingerprint accepted: %q", value)
		}
	}
	request.MacOSSigningIdentity = identity
	request.TargetOS = "windows"
	if err := validateRequest(request); err == nil {
		t.Fatal("irrelevant signing identity accepted for Windows-only build")
	}
}
