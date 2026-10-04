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

func TestNativeAcceptanceScopesPublishedBaselineToRelease(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "/published/aigw")
	for _, args := range [][]string{{"native"}, {"native", "--full-quality"}} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			var calls []command
			if err := run(args, &bytes.Buffer{}, func(call command) error {
				calls = append(calls, call)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(calls) < 3 {
				t.Fatalf("native command count = %d", len(calls))
			}
			for _, call := range calls[:len(calls)-1] {
				if !slices.Contains(call.Env, "AIGW_ACCEPTANCE_BASELINE=") {
					t.Fatalf("source gate inherited the published baseline: %#v", call)
				}
			}
			if slices.Contains(calls[len(calls)-1].Env, "AIGW_ACCEPTANCE_BASELINE=") {
				t.Fatalf("release acceptance lost the published baseline: %#v", calls[len(calls)-1])
			}
		})
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

func TestNativeCommandsRequireCoverageOnEveryPlatform(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		want := []string{"run", "./tools/coverage"}
		if platform != "windows" {
			want = append(want, "--race")
		}
		want = append(want, "--profile-output", filepath.Join("build", "verification", "coverage", "profile.out"))
		if call := nativeCommands(platform)[1]; call.Name != "go" || !slices.Equal(call.Args, want) {
			t.Fatalf("%s canonical coverage verification = %#v, want %v", platform, call, want)
		}
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

func TestNativeAcceptanceForwardsReleaseOwnedArguments(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "/published/aigw")
	selected := []string{"--artifacts", "/candidate with spaces", "--candidate", "--clients"}
	for _, sourceFlags := range [][]string{nil, {"--full-quality"}} {
		var calls []command
		args := append([]string{"native"}, sourceFlags...)
		args = append(append(args, "--"), selected...)
		err := run(args, &bytes.Buffer{}, func(call command) error {
			calls = append(calls, call)
			return nil
		})
		if err != nil || (len(sourceFlags) == 0 && len(calls) != 1) || (len(sourceFlags) != 0 && len(calls) < 3) {
			t.Fatalf("native acceptance discarded explicit release inputs: %v, %#v", err, calls)
		}
		final := calls[len(calls)-1]
		want := append([]string{"run", "./tools/release", "accept-native"}, selected...)
		if !slices.Equal(final.Args, want) || slices.Contains(final.Env, "AIGW_ACCEPTANCE_BASELINE=") {
			t.Fatalf("release inputs or predecessor changed: %#v", final)
		}
		for _, source := range calls[:len(calls)-1] {
			if slices.Contains(source.Args, "/candidate with spaces") || !slices.Contains(source.Env, "AIGW_ACCEPTANCE_BASELINE=") {
				t.Fatalf("release inputs escaped into source verification: %#v", source)
			}
		}
	}
}

func TestNativeDiagnosticClientForwardsOnlyTheAdmittedReleaseScope(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "/published/aigw")
	selected := []string{"--artifacts=/candidate", "--candidate", "--diagnostic-client=hermes"}
	var calls []command
	err := run(append([]string{"native", "--"}, selected...), &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	})
	want := command{Name: "go", Args: append([]string{"run", "./tools/release", "accept-native"}, selected...)}
	if err != nil || !reflect.DeepEqual(calls, []command{want}) {
		t.Fatalf("diagnostic scope changed or repeated source qualification: %#v, %v", calls, err)
	}
}

func TestNativePrebuiltAcceptanceDoesNotRepeatSourceQualification(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	for _, selected := range [][]string{
		{"--artifacts", "/candidate with spaces", "--candidate"},
		{"--artifacts=/candidate", "--candidate"},
		{"--tag", "v0.3.1", "--peer", "gitlab", "--repository", "group/product"},
	} {
		var calls []command
		err := run(append([]string{"native", "--"}, selected...), &bytes.Buffer{}, func(call command) error {
			calls = append(calls, call)
			return nil
		})
		want := command{Name: "go", Args: append([]string{"run", "./tools/release", "accept-native"}, selected...)}
		if err != nil || !reflect.DeepEqual(calls, []command{want}) {
			t.Fatalf("prebuilt native scope = %#v, %v; want only the existing artifact owner", calls, err)
		}
	}
	for _, source := range []struct {
		args    []string
		refresh string
	}{
		{[]string{"native", "--full-quality", "--", "--artifacts", "/candidate", "--candidate"}, ""},
		{[]string{"native", "--", "--baseline-tag", "v0.3.1", "--peer", "gitlab", "--repository", "group/product"}, ""},
		{[]string{"native", "--", "--artifacts", "/candidate", "--candidate"}, "true"},
	} {
		t.Setenv("AIGW_REFRESH_LOCKS", source.refresh)
		calls := 0
		err := run(source.args, &bytes.Buffer{}, func(command) error { calls++; return nil })
		if err != nil || calls < 3 {
			t.Fatalf("source qualification was narrowed: arguments=%q refresh=%q calls=%d error=%v", source.args, source.refresh, calls, err)
		}
	}
}

func TestNativeAcceptanceRequiresTheReleaseArgumentSeparator(t *testing.T) {
	calls := 0
	err := run([]string{"native", "unexpected"}, &bytes.Buffer{}, func(command) error { calls++; return nil })
	if err == nil || calls != 0 {
		t.Fatalf("unseparated release arguments reached native verification: %v, %d", err, calls)
	}
}
