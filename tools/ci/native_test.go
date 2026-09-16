package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestKeychainIntegrationHasExplicitBuildScope(t *testing.T) {
	expected := map[string]string{
		"aigw-cli/internal/secrets":          "keyring_darwin_test.go",
		"aigw-cli/internal/secrets/keychain": "integration_darwin_test.go",
		"aigw-cli/tools/release":             "credentials_darwin_test.go",
	}
	for _, selected := range []bool{false, true} {
		args := []string{"list", "-json"}
		if selected {
			args = append(args, "-tags=keychain_integration")
		}
		args = append(args, "./internal/secrets", "./internal/secrets/keychain", "./tools/release")
		call := exec.CommandContext(t.Context(), "go", args...)
		call.Dir = repositoryRoot(t)
		call.Env = append(os.Environ(), "GOOS=darwin", "GOFLAGS=")
		output, err := call.Output()
		if err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(bytes.NewReader(output))
		seen := make(map[string]bool)
		for {
			var pkg struct {
				ImportPath  string   `json:"ImportPath"`
				TestGoFiles []string `json:"TestGoFiles"`
			}
			if err := decoder.Decode(&pkg); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			file := expected[pkg.ImportPath]
			if file == "" || slices.Contains(pkg.TestGoFiles, file) != selected {
				t.Fatalf("integration selection=%t package=%s test files=%v", selected, pkg.ImportPath, pkg.TestGoFiles)
			}
			seen[pkg.ImportPath] = true
		}
		if len(seen) != len(expected) {
			t.Fatalf("integration inventory returned %d packages, want %d", len(seen), len(expected))
		}
	}
}

func TestNativeDarwinRequiresAnEphemeralCredentialHost(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS native admission")
	}
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "1")
	for _, fullQuality := range []bool{false, true} {
		for _, scope := range []string{"", "operator-host", "ephemeral-host"} {
			t.Run(fmt.Sprintf("full=%t/scope=%s", fullQuality, scope), func(t *testing.T) {
				t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", scope)
				args := []string{"native", "--platform", "darwin"}
				wantCalls := 3
				if fullQuality {
					args = append(args, "--full-quality")
					wantCalls = len(qualityCommands) + 2
				}
				calls, integrationCalls := 0, 0
				err := run(args, &bytes.Buffer{}, func(call command) error {
					calls++
					if slices.Contains(call.Env, "GOFLAGS=-tags=keychain_integration") {
						integrationCalls++
						if !slices.Contains(call.Args, "./tools/coverage") {
							t.Fatal("host integration escaped the single native coverage execution")
						}
					}
					return nil
				})
				if scope == "ephemeral-host" {
					if err != nil || calls != wantCalls || integrationCalls != 1 {
						t.Fatalf("admitted native host: calls=%d integration=%d error=%v", calls, integrationCalls, err)
					}
				} else if err == nil || calls != 0 || !strings.Contains(err.Error(), "ephemeral-host") {
					t.Fatalf("unadmitted host reached native execution: calls=%d error=%v", calls, err)
				}
			})
		}
	}
}

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

func TestNativeDarwinSeparatesPrivateContractsFromPublisherQualification(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS native admission")
	}
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "ephemeral-host")
	integrationCalls := 0
	err := run([]string{"native", "--platform", "darwin"}, &bytes.Buffer{}, func(call command) error {
		if slices.Contains(call.Env, "GOFLAGS=-tags=keychain_integration") {
			integrationCalls++
		}
		if slices.Contains(call.Env, "AIGW_VERIFY_SYSTEM_KEYRING=1") {
			t.Fatal("private Keychain coverage opted into publisher-bound lifecycle")
		}
		return nil
	})
	if err != nil || integrationCalls != 1 {
		t.Fatalf("private native coverage calls=%d error=%v", integrationCalls, err)
	}
}

func TestNativeAcceptanceRequiresTheRealHostPlatform(t *testing.T) {
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
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
		if runtime.GOOS == "windows" {
			if got := calls[1]; got.Name != "go" || !slices.Equal(got.Args, []string{"test", "-json", "./..."}) {
				t.Fatalf("native Windows test command = %#v", got)
			}
		} else {
			wantProfile := filepath.Join("build", "acceptance", "coverage-"+runtime.GOOS+".out")
			if got := calls[1]; got.Name != "go" || !slices.Equal(got.Args, []string{"run", "./tools/coverage", "--race", "--profile-output", wantProfile}) {
				t.Fatalf("native coverage command = %#v", got)
			}
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
	windows := nativeCommands("windows")
	if !slices.Equal(windows[1].Args, []string{"test", "-json", "./..."}) {
		t.Fatalf("Windows source verification = %#v", windows[1])
	}
	linux := nativeCommands("linux")
	want := []string{"run", "./tools/coverage", "--race", "--profile-output", filepath.Join("build", "acceptance", "coverage-linux.out")}
	if !slices.Equal(linux[1].Args, want) {
		t.Fatalf("Linux source verification = %#v", linux[1])
	}
}

func TestNativeFullQualityUsesTheExistingGateOnce(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
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
