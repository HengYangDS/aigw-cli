package main

import (
	"bytes"
	"errors"
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
		})
	}
}

func TestNativeAcceptanceRequiresTheRealHostPlatform(t *testing.T) {
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
			if got := calls[1]; got.Name != "go" || !slices.Equal(got.Args, []string{"test", "./..."}) {
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
	if !slices.Equal(windows[1].Args, []string{"test", "./..."}) {
		t.Fatalf("Windows source verification = %#v", windows[1])
	}
	linux := nativeCommands("linux")
	want := []string{"run", "./tools/coverage", "--race", "--profile-output", filepath.Join("build", "acceptance", "coverage-linux.out")}
	if !slices.Equal(linux[1].Args, want) {
		t.Fatalf("Linux source verification = %#v", linux[1])
	}
}

func TestNativeAcceptanceStopsBeforeTestsWhenStaticChecksFail(t *testing.T) {
	t.Chdir(repositoryRoot(t))
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
