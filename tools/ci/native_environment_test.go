package main

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Each case reproduces one branch of the former GitLab shell that assembled the
// accept-native arguments from AIGW_* job inputs.
func TestNativeAcceptanceArgumentsFromGitLabInputs(t *testing.T) {
	base := map[string]string{"CI_PROJECT_URL": "https://gitlab.example/aigw", "CI_PIPELINE_SOURCE": "push"}
	for name, entry := range map[string]struct {
		env  map[string]string
		want []string
		err  string
	}{
		"default": {want: []string{"--peer", "gitlab", "--repository", "https://gitlab.example/aigw"}},
		"baseline and candidate": {
			env: map[string]string{
				"AIGW_BASELINE_TAG": "v0.4.6", "AIGW_BASELINE_ARTIFACTS": "/b",
				"AIGW_CANDIDATE_TAG": "v0.4.7", "AIGW_CANDIDATE_ARTIFACTS": "/c", "AIGW_CANDIDATE_SOURCE": "abc",
			},
			want: []string{
				"--peer", "gitlab", "--repository", "https://gitlab.example/aigw",
				"--baseline-tag", "v0.4.6", "--baseline-artifacts", "/b", "--tag", "v0.4.7",
				"--artifacts", "/c", "--candidate", "--candidate-source", "abc",
			},
		},
		"input package": {
			env:  map[string]string{"AIGW_NATIVE_INPUT_PACKAGE": "pkg", "AIGW_NATIVE_INPUT_SHA256": "ff"},
			want: []string{"--peer", "gitlab", "--repository", "https://gitlab.example/aigw", "--input-package", "pkg", "--input-sha256", "ff", "--candidate"},
		},
		"performance": {
			env:  map[string]string{"CI_PIPELINE_SOURCE": "web", "AIGW_NATIVE_PERFORMANCE": "true", "AIGW_NATIVE_PERFORMANCE_ATTRIBUTION": "true"},
			want: []string{"--peer", "gitlab", "--repository", "https://gitlab.example/aigw", "--performance", filepath.FromSlash("/work/build/verification/performance"), "--performance-attribution"},
		},
		"empty repository argument is kept": {
			env:  map[string]string{"CI_PROJECT_URL": "", "AIGW_CANDIDATE_TAG": ""},
			want: []string{"--peer", "gitlab", "--repository", ""},
		},
		"diagnostic client wins over clients": {
			env:  map[string]string{"CI_PIPELINE_SOURCE": "api", "AIGW_NATIVE_DIAGNOSTIC_CLIENT": "codex", "AIGW_NATIVE_CLIENTS": "true"},
			want: []string{"--peer", "gitlab", "--repository", "https://gitlab.example/aigw", "--diagnostic-client", "codex"},
		},
		"clients": {
			env:  map[string]string{"AIGW_NATIVE_CLIENTS": "true", "AIGW_BASELINE_TAG": "v0.4.6"},
			want: []string{"--peer", "gitlab", "--repository", "https://gitlab.example/aigw", "--baseline-tag", "v0.4.6", "--clients"},
		},
		"clients need a baseline":        {env: map[string]string{"AIGW_NATIVE_CLIENTS": "true"}, err: "real-client succession requires AIGW_BASELINE_TAG"},
		"diagnostics need a manual push": {env: map[string]string{"AIGW_NATIVE_DIAGNOSTIC_CLIENT": "codex"}, err: "require a manual pipeline"},
		"performance needs a manual push": {
			env: map[string]string{"AIGW_NATIVE_PERFORMANCE": "true"}, err: "require a manual pipeline",
		},
		"attribution needs a manual push": {
			env: map[string]string{"AIGW_NATIVE_PERFORMANCE_ATTRIBUTION": "true"}, err: "require a manual pipeline",
		},
	} {
		t.Run(name, func(t *testing.T) {
			job, err := nativeJobFromEnvironment(environmentLookup(base, entry.env), "linux", "/work")
			if entry.err != "" {
				if err == nil || !strings.Contains(err.Error(), entry.err) {
					t.Fatalf("error = %v, want %q", err, entry.err)
				}
				return
			}
			if err != nil || !slices.Equal(job.arguments, entry.want) {
				t.Fatalf("arguments = %q, %v; want %q", job.arguments, err, entry.want)
			}
		})
	}
}

func TestNativeJobSelectsPlatformBehaviorFromGitLabInputs(t *testing.T) {
	base := map[string]string{"CI_PROJECT_URL": "https://gitlab.example/aigw", "CI_PIPELINE_SOURCE": "push", "CI_PROJECT_DIR": "/builds/aigw"}
	for name, entry := range map[string]struct {
		platform string
		env      map[string]string
		want     nativeMode
	}{
		"plain source run": {platform: "linux"},
		"full quality":     {platform: "darwin", env: map[string]string{"AIGW_FULL_NATIVE_QUALITY": "true"}, want: nativeMode{fullQuality: true}},
		"lock refresh":     {platform: "windows", env: map[string]string{"AIGW_REFRESH_LOCKS": "true"}, want: nativeMode{refreshLocks: true}},
		"macOS review probe": {
			platform: "darwin", env: map[string]string{"CI_COMMIT_REF_PROTECTED": "false", "AIGW_REVIEW_PROTECTED_FILE": "/protected"},
			want: nativeMode{protectedFile: "/protected"},
		},
		"macOS protected ref skips the probe": {
			platform: "darwin", env: map[string]string{"CI_COMMIT_REF_PROTECTED": "true", "AIGW_REVIEW_PROTECTED_FILE": "/protected"},
		},
		"Windows has no review probe": {
			platform: "windows", env: map[string]string{"CI_COMMIT_REF_PROTECTED": "false", "AIGW_REVIEW_PROTECTED_FILE": "/protected"},
		},
		"Linux release input runs in a private Secret Service": {
			platform: "linux", env: map[string]string{"AIGW_BASELINE_ARTIFACTS": "/b"}, want: nativeMode{secretService: true},
		},
		"Linux source run stays outside the session": {platform: "linux", env: map[string]string{"AIGW_CANDIDATE_SOURCE": "abc"}},
		"macOS release input needs no session":       {platform: "darwin", env: map[string]string{"AIGW_BASELINE_TAG": "v0.4.6"}},
		"Windows input with clients supplies itself": {
			platform: "windows", env: map[string]string{"AIGW_NATIVE_INPUT_PACKAGE": "pkg", "AIGW_NATIVE_CLIENTS": "true", "AIGW_BASELINE_TAG": "v0.4.6"},
			want: nativeMode{windowsInput: true},
		},
		"Windows input without clients uses the package": {platform: "windows", env: map[string]string{"AIGW_NATIVE_INPUT_PACKAGE": "pkg"}},
		"Linux input with clients uses the package": {
			platform: "linux", env: map[string]string{"AIGW_NATIVE_INPUT_PACKAGE": "pkg", "AIGW_NATIVE_CLIENTS": "true", "AIGW_BASELINE_TAG": "v0.4.6"},
			want: nativeMode{secretService: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			job, err := nativeJobFromEnvironment(environmentLookup(base, entry.env), entry.platform, "/work")
			if err != nil {
				t.Fatal(err)
			}
			if job.nativeMode != entry.want {
				t.Fatalf("job = %+v, want %+v", job.nativeMode, entry.want)
			}
		})
	}
}

func TestWindowsInputWithClientsPassesTheVerifiedArchive(t *testing.T) {
	project := filepath.Join(t.TempDir(), "aigw")
	lookup := environmentLookup(map[string]string{
		"CI_PROJECT_URL": "https://gitlab.example/aigw", "CI_PIPELINE_SOURCE": "api", "CI_PROJECT_DIR": project, "CI_JOB_ID": "77",
	}, map[string]string{"AIGW_NATIVE_INPUT_PACKAGE": "pkg", "AIGW_NATIVE_INPUT_SHA256": "ff", "AIGW_NATIVE_DIAGNOSTIC_CLIENT": "codex"})
	job, err := nativeJobFromEnvironment(lookup, "windows", "/work")
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(filepath.Dir(project), "aigw-ci-mise-77", "native-input.tar")
	want := []string{"--peer", "gitlab", "--repository", "https://gitlab.example/aigw", "--input-archive", archive, "--input-sha256", "ff", "--candidate", "--diagnostic-client", "codex"}
	if !job.windowsInput || !slices.Equal(job.arguments, want) || windowsJobPath(lookup, "native-input.tar") != archive {
		t.Fatalf("Windows input job = %+v, want arguments %q", job, want)
	}
}

func TestNativeTrustDefaultsKeepExplicitFiles(t *testing.T) {
	for name, entry := range map[string]struct {
		env  map[string]string
		want []string
	}{
		"defaults from the GitLab file variables": {
			env:  map[string]string{"AIGW_RELEASE_ALLOWED_SIGNERS": "/source", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS": "/artifact"},
			want: []string{"AIGW_RELEASE_ALLOWED_SIGNERS_FILE=/source", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE=/artifact"},
		},
		"explicit files win": {
			env: map[string]string{
				"AIGW_RELEASE_ALLOWED_SIGNERS": "/source", "AIGW_RELEASE_ALLOWED_SIGNERS_FILE": "/kept",
				"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS": "/artifact", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE": "/kept-artifact",
			},
		},
		"no trust input": {},
	} {
		t.Run(name, func(t *testing.T) {
			if got := nativeTrustDefaults(environmentLookup(nil, entry.env)); !slices.Equal(got, entry.want) {
				t.Fatalf("trust defaults = %q, want %q", got, entry.want)
			}
		})
	}
}

func TestNativeJobFromEnvironmentPreparesSourceQualification(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	bootstrap := command{Name: "mise", Args: []string{"run", "bootstrap"}}
	resolve := command{Name: "mise", Args: []string{"run", "dependencies:resolve"}}
	for name, entry := range map[string]struct {
		env   map[string]string
		setup []command
	}{
		"source":       {setup: []command{bootstrap}},
		"lock refresh": {env: map[string]string{"AIGW_REFRESH_LOCKS": "true"}, setup: []command{bootstrap, resolve}},
		"full quality": {env: map[string]string{"AIGW_FULL_NATIVE_QUALITY": "true"}, setup: []command{bootstrap}},
	} {
		t.Run(name, func(t *testing.T) {
			selected := map[string]string{"AIGW_RELEASE_ALLOWED_SIGNERS": "/source", "CI_PROJECT_URL": "https://gitlab.example/aigw"}
			maps.Copy(selected, entry.env)
			nativeJobEnvironment(t, selected)
			var calls []command
			if err := run([]string{"native", "--from-env"}, &bytes.Buffer{}, func(call command) error {
				calls = append(calls, call)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(calls) <= len(entry.setup) {
				t.Fatalf("native job ran no source qualification: %#v", calls)
			}
			for index, call := range calls {
				if index < len(entry.setup) && (call.Name != entry.setup[index].Name || !slices.Equal(call.Args, entry.setup[index].Args)) {
					t.Fatalf("native job setup %d = %#v, want %#v", index, call, entry.setup[index])
				}
				if len(call.Env) == 0 || call.Env[0] != "AIGW_RELEASE_ALLOWED_SIGNERS_FILE=/source" {
					t.Fatalf("native job command lost its trust default: %#v", call)
				}
				if slices.Contains(call.Args, "accept-native") {
					t.Fatalf("source job ran a second release lifecycle: %#v", call)
				}
			}
		})
	}
}

func TestNativeJobFromEnvironmentKeepsPrebuiltAcceptanceNarrow(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_PIPELINE_SOURCE": "web", "CI_PROJECT_URL": "https://gitlab.example/aigw", "AIGW_CANDIDATE_ARTIFACTS": "/candidate"})
	var calls []command
	err := runNativeJob(nativeHost{getenv: os.Getenv, platform: "darwin", workdir: "/work"}, &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	})
	want := []string{"run", "./tools/release", "accept-native", "--peer", "gitlab", "--repository", "https://gitlab.example/aigw", "--artifacts", "/candidate", "--candidate"}
	if err != nil || len(calls) != 1 || calls[0].Name != "go" || !slices.Equal(calls[0].Args, want) {
		t.Fatalf("prebuilt native job = %#v, %v; want only the release owner", calls, err)
	}
}

func TestLinuxReleaseInputRunsInAPrivateSecretService(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_PROJECT_URL": "https://gitlab.example/aigw", "AIGW_CANDIDATE_ARTIFACTS": "/candidate"})
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var wrapped []command
	host := nativeHost{getenv: os.Getenv, platform: "linux", workdir: "/work", attach: func(call command) error {
		wrapped = append(wrapped, call)
		return nil
	}}
	if err := runNativeJob(host, &bytes.Buffer{}, func(call command) error {
		t.Fatalf("release input ran outside its private session: %#v", call)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session := []string{"--", self, "native", "--platform", "linux", "--from-env", "--secret-service-session"}
	if len(wrapped) != 1 || wrapped[0].Name != "dbus-run-session" || !slices.Equal(wrapped[0].Args, session) {
		t.Fatalf("private session = %#v, want %q", wrapped, session)
	}
	for name, reply := range map[string]string{"open": "(objectpath '" + secretServiceCollection + "',)\n", "absent": "(objectpath '/',)\n"} {
		t.Run(name, func(t *testing.T) {
			var inquiries, calls []command
			host.session = true
			host.inquiry = func(call command) ([]byte, error) {
				inquiries = append(inquiries, call)
				return []byte(reply), nil
			}
			err := runNativeJob(host, &bytes.Buffer{}, func(call command) error {
				calls = append(calls, call)
				return nil
			})
			if name == "absent" {
				if err == nil || len(inquiries) != 1 || len(calls) != 0 {
					t.Fatalf("unavailable session collection reached acceptance: %v, %#v", err, calls)
				}
				return
			}
			read := []string{"call", "--session", "--dest", "org.freedesktop.secrets", "--object-path", "/org/freedesktop/secrets", "--method", "org.freedesktop.Secret.Service.ReadAlias", "session"}
			alias := append(slices.Clone(read[:7]), "org.freedesktop.Secret.Service.SetAlias", "default", secretServiceCollection)
			if err != nil || len(inquiries) != 2 || !slices.Equal(inquiries[0].Args, read) || !slices.Equal(inquiries[1].Args, alias) || inquiries[1].Timeout == 0 {
				t.Fatalf("private session aliases = %#v, %v", inquiries, err)
			}
			if len(calls) != 1 || !slices.Contains(calls[0].Env, "AIGW_VERIFY_SYSTEM_KEYRING=1") || !slices.Contains(calls[0].Args, "accept-native") {
				t.Fatalf("release acceptance left the system credential store unverified: %#v", calls)
			}
		})
	}
}

func TestMacOSReviewJobProbesItsProtectedResource(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_COMMIT_REF_PROTECTED": "false", "AIGW_REVIEW_PROTECTED_FILE": "manager-config"})
	problem := "exact absolute protected resource"
	if runtime.GOOS == "windows" {
		problem = "protected-file qualification requires a Unix host"
	}
	calls := 0
	err := runNativeJob(nativeHost{getenv: os.Getenv, platform: "darwin", workdir: "/work"}, &bytes.Buffer{}, func(command) error { calls++; return nil })
	if err == nil || !strings.Contains(err.Error(), problem) || calls != 0 {
		t.Fatalf("review job skipped its protected resource probe: %v, calls=%d", err, calls)
	}
}

func TestNativeJobFromEnvironmentRefusesItsGuardsBeforeExecution(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	nativeJobEnvironment(t, map[string]string{"CI_PIPELINE_SOURCE": "push", "AIGW_NATIVE_DIAGNOSTIC_CLIENT": "codex"})
	for _, arguments := range [][]string{
		{"native", "--from-env"},
		{"native", "--from-env", "--full-quality"},
		{"native", "--from-env", "--protected-file", "/protected"},
		{"native", "--from-env", "--", "--clients"},
		{"native", "--secret-service-session"},
	} {
		calls := 0
		if err := run(arguments, &bytes.Buffer{}, func(command) error { calls++; return nil }); err == nil || calls != 0 {
			t.Fatalf("%q reached native execution: %v, calls=%d", arguments, err, calls)
		}
	}
}

func TestNativeJobPerformanceOutputUsesThePhysicalCheckout(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	physical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := physicalWorkdir(); err != nil || got != physical {
		t.Fatalf("physical checkout = %q, %v; want %q", got, err, physical)
	}
}

func TestAttachedCommandPreservesItsResult(t *testing.T) {
	if err := runAttached(command{Name: "git", Args: []string{"--version"}}); err != nil {
		t.Fatal(err)
	}
	var exit interface{ ExitCode() int }
	if err := runAttached(command{Name: "git", Args: []string{"config", "--get", "aigw.missing-key"}}); !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("attached failure = %v", err)
	}
}

// nativeJobEnvironment clears every GitLab input a native job reads, then
// selects only the entries the test names.
func nativeJobEnvironment(t *testing.T, selected map[string]string) {
	t.Helper()
	for _, name := range []string{
		"CI_PIPELINE_SOURCE", "CI_PROJECT_URL", "CI_PROJECT_DIR", "CI_JOB_ID", "CI_COMMIT_REF_PROTECTED",
		"AIGW_BASELINE_TAG", "AIGW_BASELINE_ARTIFACTS", "AIGW_CANDIDATE_TAG", "AIGW_CANDIDATE_ARTIFACTS", "AIGW_CANDIDATE_SOURCE",
		"AIGW_NATIVE_INPUT_PACKAGE", "AIGW_NATIVE_INPUT_SHA256", "AIGW_NATIVE_PERFORMANCE", "AIGW_NATIVE_PERFORMANCE_ATTRIBUTION",
		"AIGW_NATIVE_DIAGNOSTIC_CLIENT", "AIGW_NATIVE_CLIENTS", "AIGW_FULL_NATIVE_QUALITY", "AIGW_REFRESH_LOCKS",
		"AIGW_REVIEW_PROTECTED_FILE", "AIGW_RELEASE_ALLOWED_SIGNERS", "AIGW_RELEASE_ALLOWED_SIGNERS_FILE",
		"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE", "AIGW_ACCEPTANCE_BASELINE",
	} {
		t.Setenv(name, selected[name])
	}
}

func environmentLookup(base, overrides map[string]string) func(string) string {
	return func(key string) string {
		if value, ok := overrides[key]; ok {
			return value
		}
		return base[key]
	}
}
