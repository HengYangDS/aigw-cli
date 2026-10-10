package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestNativeAcceptanceUsesReleaseOwner(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			calls := nativeCommands(platform)
			quality := command{Name: "go", Args: []string{"run", "./tools/ci", "check-go", "."}}
			release := command{Name: "go", Args: []string{"run", "./tools/release", "accept-native"}}
			if len(calls) != 3 || !reflect.DeepEqual(calls[0], quality) || !reflect.DeepEqual(calls[2], release) {
				t.Fatalf("native acceptance = %#v, want shared static checks before tests and release-owned lifecycle", calls)
			}
			if len(calls[1].Env) != 0 {
				t.Fatal("ordinary native acceptance enabled host credential integration")
			}
		})
	}
}

func TestNativeCandidateCIForwardsInputsWithoutRepeatingSourceQualification(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	arguments := []string{"native", "--platform", runtime.GOOS, "--", "--candidate", "--candidate-source", strings.Repeat("a", 40),
		"--input-archive", filepath.Join(t.TempDir(), "public-inputs.tar"), "--input-sha256", strings.Repeat("b", 64), "--baseline-tag", "v0.3.3"}
	var calls []command
	err := run(arguments, &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	})
	want := append([]string{"run", "./tools/release", "accept-native"}, arguments[4:]...)
	if err != nil || len(calls) != 1 || calls[0].Name != "go" || !slices.Equal(calls[0].Args, want) {
		t.Fatalf("explicit candidate input did not reach its native owner once: %v, %#v", err, calls)
	}
}

func TestNativeAcceptanceRequiresTheRealHostPlatform(t *testing.T) {
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "")
	root := repositoryRoot(t)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	for _, args := range [][]string{{"native"}, {"native", "--platform", runtime.GOOS}} {
		var calls []command
		if err := run(args, &bytes.Buffer{}, func(call command) error {
			calls = append(calls, call)
			return nil
		}); err != nil || len(calls) != 3 {
			t.Fatalf("native host args=%v error=%v calls=%d", args, err, len(calls))
		}
		if got := calls[1]; !reflect.DeepEqual(got, nativeCommands(runtime.GOOS)[1]) {
			t.Fatalf("native coverage dispatch = %#v", got)
		}
	}
	other := "linux"
	if runtime.GOOS == other {
		other = "darwin"
	}
	if err := run([]string{"native", "--platform", other}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "requires "+other+" host") {
		t.Fatalf("cross-host native acceptance error = %v", err)
	}
}

func TestNativeAcceptanceRefusesARepositoryWithoutVersionTruth(t *testing.T) {
	for _, version := range []string{"", "not-semver", "01.2.3", "1.2.3-rc.01"} {
		t.Run(version, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if version != "" {
				if err := os.WriteFile("VERSION", []byte(version+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			err := run([]string{"native", "--platform", runtime.GOOS}, &bytes.Buffer{}, func(command) error {
				calls++
				return nil
			})
			if err == nil || calls != 0 {
				t.Fatalf("invalid VERSION %q reached native execution: error=%v calls=%d", version, err, calls)
			}
		})
	}
}

func TestNativeCommandsKeepSourceEvidenceDistinct(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			want := []string{"run", "./tools/coverage"}
			if platform != "windows" {
				want = append(want, "--race")
			}
			profile := filepath.Join("build", "acceptance", "coverage-"+platform+".out")
			want = append(want, "--profile-output", profile)
			got := nativeCommands(platform)[1]
			if got.Name != "go" || !slices.Equal(got.Args, want) {
				t.Fatalf("%s source verification = %#v, want strict coverage with its native profile", platform, got)
			}
		})
	}
}

func TestNativeFullQualityUsesTheExistingGateOnce(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "")
	// Native tool qualification does not reinterpret source-publication inputs.
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "source-owner@example.test")
	want := append(slices.Clone(qualityCommands), nativeCommands(runtime.GOOS)[1:]...)
	var calls []command
	err := run([]string{"native", "--full-quality"}, &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	})
	if err != nil || !reflect.DeepEqual(calls, want) {
		t.Fatalf("full native quality = %#v, %v; want existing gate then native tests and lifecycle", calls, err)
	}
	static := command{Name: "go", Args: []string{"run", "./tools/ci", "check-go", "."}}
	count := 0
	for _, call := range calls {
		if reflect.DeepEqual(call, static) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("native Go static check ran %d times, want once", count)
	}
}

func TestNativeFullQualityStopsAtEachFailedGate(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "")
	failure := errors.New("native quality gate failed")
	for index, expected := range qualityCommands {
		t.Run(fmt.Sprintf("gate-%d", index), func(t *testing.T) {
			calls := 0
			err := run([]string{"native", "--full-quality"}, &bytes.Buffer{}, func(call command) error {
				calls++
				if calls == index+1 {
					if !reflect.DeepEqual(call, expected) {
						t.Fatalf("gate %d = %#v, want %#v", index, call, expected)
					}
					return failure
				}
				return nil
			})
			if calls != index+1 || !errors.Is(err, failure) {
				t.Fatalf("gate %d failure: calls=%d error=%v", index, calls, err)
			}
		})
	}
}

func TestNativeAcceptanceStopsBeforeTestsWhenStaticChecksFail(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "")
	failure := errors.New("native source lint failed")
	calls := 0
	err := run([]string{"native"}, &bytes.Buffer{}, func(call command) error {
		calls++
		if call.Name != "go" || !slices.Equal(call.Args, []string{"run", "./tools/ci", "check-go", "."}) {
			t.Fatalf("first native check = %#v", call)
		}
		return failure
	})
	if calls != 1 || !errors.Is(err, failure) {
		t.Fatalf("native failure propagation: calls=%d error=%v", calls, err)
	}
}

func TestRejectsUnknownCommandsAndPlatforms(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"native", "--platform", "plan9"}} {
		if err := run(args, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}
