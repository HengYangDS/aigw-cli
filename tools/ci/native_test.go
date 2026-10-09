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

func TestNativeSourceQualificationUsesCanonicalCoverage(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			calls := nativeCommands(platform)
			quality := command{Name: "go", Args: []string{"run", "./tools/ci", "check-go", "."}}
			if len(calls) != 2 || !reflect.DeepEqual(calls[0], quality) || !slices.Contains(calls[1].Args, "--tags=native_resource_acceptance") {
				t.Fatalf("native source qualification = %#v, want static checks before one tagged coverage scope", calls)
			}
			if len(calls[1].Env) != 0 {
				t.Fatal("ordinary native acceptance enabled host credential integration")
			}
		})
	}
}

func TestNativeReviewProtectedResourceRefusesUnprovedDenial(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	root := t.TempDir()
	readable := filepath.Join(root, "manager-config")
	if err := os.WriteFile(readable, []byte("fixture-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, path, problem string
	}{
		{"readable", readable, "protected resource is readable"},
		{"missing", filepath.Join(root, "missing"), "protected resource identity is unproved"},
		{"relative", "manager-config", "exact absolute protected resource"},
		{"directory", root, "not a regular file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := qualifyProtectedResource(test.path, &stdout, func(command) ([]byte, error) {
				t.Fatal("unproved resource reached privilege inquiry")
				return nil, nil
			}); err == nil || !strings.Contains(err.Error(), test.problem) {
				t.Fatalf("native resource denial was unproved: %v", err)
			}
			problem := test.problem
			if runtime.GOOS == "windows" {
				problem = "protected-file qualification requires a Unix host"
			}
			ran := false
			err := run([]string{"native", "--protected-file", test.path, "--", "--artifacts=/candidate", "--candidate"}, &stdout, func(command) error {
				ran = true
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), problem) || ran {
				t.Fatalf("native review admitted an unproved denial: ran=%t error=%v", ran, err)
			}
			if strings.Contains(stdout.String(), "fixture-only") {
				t.Fatal("native review read protected content")
			}
		})
	}
}

func TestNativeReviewProtectedResourceRequiresNoElevationGrant(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "manager-config")
	if err := os.WriteFile(path, []byte("fixture-only"), 0o000); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("Root cannot represent a denied native review identity")
	}
	for _, test := range []struct {
		name, output string
		err          error
		accepted     bool
	}{
		{"no grant", "User review is not allowed to run sudo on this host.", errors.New("exit 1"), true},
		{"grant", "(ALL) ALL", nil, false},
		{"authorization pending", "sudo: a password is required", errors.New("exit 1"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := qualifyProtectedResource(path, &stdout, func(call command) ([]byte, error) {
				if call.Name != "sudo" || !slices.Equal(call.Args, []string{"-n", "-l"}) || call.Timeout == 0 || !slices.Contains(call.Env, "LC_ALL=C") {
					t.Fatalf("review permission inquiry is not native, bounded and noninteractive: %#v", call)
				}
				return []byte(test.output), test.err
			})
			if (err == nil) != test.accepted {
				t.Fatalf("review elevation qualification accepted=%t error=%v", test.accepted, err)
			}
			if test.accepted && !strings.Contains(stdout.String(), "direct_read=denied sudo_grant=none") {
				t.Fatal("native review qualification omitted its exact effect")
			}
		})
	}
}

func TestNativeSourceQualificationRunsLifecycleOnce(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "0")
	t.Setenv("AIGW_REFRESH_LOCKS", "")
	for _, arguments := range [][]string{
		{"native"}, {"native", "--full-quality"},
		{"native", "--", "--peer", "gitlab", "--repository", "https://gitlab.example/owner/product"},
		{"native", "--full-quality", "--", "--peer", "github", "--repository", "owner/product"},
	} {
		t.Run(strings.Join(arguments, "-"), func(t *testing.T) {
			var calls []command
			if err := run(arguments, &bytes.Buffer{}, func(call command) error {
				calls = append(calls, call)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			coverageCalls := 0
			for _, call := range calls {
				if slices.Contains(call.Args, "accept-native") {
					t.Fatalf("source lifecycle would run again in a second test process: %#v", call)
				}
				if slices.Contains(call.Args, "./tools/coverage") {
					coverageCalls++
					if !slices.Contains(call.Args, "--tags=native_resource_acceptance") {
						t.Fatalf("source coverage omitted native resource acceptance: %#v", call)
					}
				}
			}
			if coverageCalls != 1 {
				t.Fatalf("source coverage ran %d times, want once", coverageCalls)
			}
		})
	}
}

func TestNativeExplicitReleaseScopeRemainsIndependent(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	selected := []string{"--baseline-tag", "v0.3.1", "--peer", "github", "--repository", "owner/product"}
	var calls []command
	if err := run(append([]string{"native", "--"}, selected...), &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := command{Name: "go", Args: append([]string{"run", "./tools/release", "accept-native"}, selected...)}
	if len(calls) != 3 || !reflect.DeepEqual(calls[len(calls)-1], want) {
		t.Fatalf("explicit release scope was absorbed by source coverage: %#v", calls)
	}
}

func TestNativeSourceFixturesRetainIsolationAndCleanup(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	t.Setenv("AIGW_ACCEPTANCE_RELEASE", "/unrelated/candidate")
	t.Setenv("CI_JOB_TOKEN", "fixture-only")
	for _, failure := range []error{nil, errors.New("source coverage failed")} {
		var workspace string
		err := run([]string{"native"}, &bytes.Buffer{}, func(call command) error {
			if !slices.Contains(call.Args, "./tools/coverage") {
				return nil
			}
			for _, value := range call.Env {
				if after, ok := strings.CutPrefix(value, "TMPDIR="); ok {
					workspace = after
				}
			}
			if workspace == "" || !filepath.IsAbs(workspace) {
				t.Fatalf("source fixtures have no private workspace: %#v", call)
			}
			if _, err := os.Stat(workspace); err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{"TMP=" + workspace, "TEMP=" + workspace, "CI_JOB_TOKEN=", "GH_TOKEN=", "MISE_NETRC=false", "AIGW_ACCEPTANCE_RELEASE=", "AIGW_ACCEPTANCE_BASELINE="} {
				if !slices.Contains(call.Env, value) {
					t.Fatalf("source fixture isolation omitted %q: %#v", value, call)
				}
			}
			return failure
		})
		if !errors.Is(err, failure) {
			t.Fatalf("source result changed: %v, want %v", err, failure)
		}
		if _, err := os.Stat(workspace); !os.IsNotExist(err) {
			t.Fatalf("source fixture workspace survived: %s, %v", workspace, err)
		}
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
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
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
		}); err != nil || len(calls) != 2 {
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
		want := []string{"run", "./tools/coverage", "--tags=native_resource_acceptance"}
		if platform != "windows" {
			want = append(want, "--race")
		}
		want = append(want, "--profile-output", filepath.Join("build", "verification", "coverage", "profile.out"))
		name := "go"
		if platform == "linux" {
			name = "dbus-run-session"
			want = append([]string{"--", "env", "AIGW_VERIFY_LOCKED_SECRET_SERVICE=1", "AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host", "go"}, want...)
		}
		if call := nativeCommands(platform)[1]; call.Name != name || !slices.Equal(call.Args, want) {
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
	// The per-run fixture environment is qualified separately from this graph.
	if len(calls) == len(want) {
		calls[len(calls)-1].Env = nil
	}
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
		{"--input-package", "native-inputs", "--input-sha256", strings.Repeat("a", 64), "--candidate-source", strings.Repeat("b", 40), "--candidate", "--baseline-tag", "v0.3.1", "--peer", "gitlab", "--repository", "group/product"},
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
