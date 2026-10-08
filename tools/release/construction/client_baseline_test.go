package construction

import (
	upgradeartifact "aigw-cli/internal/upgrade/artifact"
	releaseartifact "aigw-cli/tools/release/artifact"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNativeClientAcceptancePreservesExplicitPublishedPredecessor(t *testing.T) {
	baseline := filepath.Join(t.TempDir(), "published-aigw")
	if err := os.WriteFile(baseline, []byte("published predecessor"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", baseline)
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
	artifacts := t.TempDir()
	writeNativeArchive(t, artifacts, "1.2.3")
	observed := false
	err := acceptNative(request, artifacts, os.Getenv("AIGW_ACCEPTANCE_BASELINE"), NativeAcceptance{Clients: true}, func(call toolCall) error {
		if slices.Contains(call.Args, "-tags=client_acceptance") {
			observed = true
			if !slices.Contains(call.Env, "AIGW_ACCEPTANCE_BASELINE="+baseline) {
				t.Error("real-client acceptance discarded the explicit published predecessor")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("real-client acceptance did not execute")
	}
}

func TestNativeAcceptanceRejectsCompetingPredecessorInputs(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "/explicit/published/aigw")
	err := AcceptNative(t.Context(), NativeAcceptance{BaselineTag: "v1.2.3", Peer: "gitlab", Repository: "group/product"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("competing published baselines were accepted: %v", err)
	}
}

func TestNativeClientSuccessionRequiresPublishedPredecessor(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	if _, err := ParseNativeAcceptance([]string{"--clients"}); err == nil || !strings.Contains(err.Error(), "published predecessor") {
		t.Fatalf("client succession accepted missing predecessor: %v", err)
	}
	for _, arguments := range [][]string{
		{"--peer=github", "--repository=team/product", "--baseline-tag=v1.2.3", "--tag=", "--clients=true"},
		{"--peer=github", "--repository=team/product", "--baseline-tag=", "--tag=", "--clients=false"},
	} {
		if _, err := ParseNativeAcceptance(arguments); err != nil {
			t.Fatalf("declared empty optional inputs were refused: %v", err)
		}
	}
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "/published/aigw")
	if _, err := ParseNativeAcceptance([]string{"--clients"}); err != nil {
		t.Fatalf("explicit retained predecessor was refused: %v", err)
	}
}

func TestNativeDiagnosticClientAdmission(t *testing.T) {
	root := t.TempDir()
	artifacts := "--artifacts=" + filepath.Join(root, "candidate")
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", filepath.Join(root, "published-aigw"))
	t.Setenv("AIGW_RELEASE_TAG", "")
	executable := filepath.Join(root, "client")
	if err := os.WriteFile(executable, []byte("test-owned executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, client := range nativeAcceptanceClients {
		t.Setenv("AIGW_ACCEPTANCE_"+strings.ToUpper(client), "")
	}
	for _, client := range nativeAcceptanceClients {
		if _, err := ParseNativeAcceptance([]string{artifacts, "--candidate", "--diagnostic-client=" + client}); err != nil {
			t.Errorf("explicit %s diagnostic was refused: %v", client, err)
		}
		key := "AIGW_ACCEPTANCE_" + strings.ToUpper(client)
		t.Setenv(key, executable)
		if err := requireNativeClients(client); err != nil {
			t.Errorf("%s diagnostic requires an unrelated executable: %v", client, err)
		}
		t.Setenv(key, "")
		if err := requireNativeClients(client); err == nil {
			t.Errorf("%s diagnostic admitted a missing executable", client)
		}
	}
	for _, arguments := range [][]string{
		{"--diagnostic-client=hermes"},
		{artifacts, "--candidate", "--diagnostic-client=unknown"},
		{artifacts, "--candidate", "--diagnostic-client=hermes|codex"},
		{artifacts, "--candidate", "--clients", "--diagnostic-client=hermes"},
		{artifacts, "--candidate", "--performance=" + filepath.Join(root, "samples"), "--diagnostic-client=hermes"},
	} {
		if _, err := ParseNativeAcceptance(arguments); err == nil {
			t.Errorf("ambiguous or unqualified diagnostic was admitted: %q", arguments)
		}
	}
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	if _, err := ParseNativeAcceptance([]string{artifacts, "--candidate", "--diagnostic-client=hermes"}); err == nil {
		t.Fatal("client diagnostic admitted a missing published predecessor")
	}
}

func TestNativePerformanceAttributionRequiresExplicitScope(t *testing.T) {
	root := t.TempDir()
	artifacts := "--artifacts=" + filepath.Join(root, "candidate")
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", filepath.Join(root, "published-aigw"))
	args := []string{artifacts, "--candidate", "--performance=" + filepath.Join(root, "samples"), "--performance-attribution"}
	if _, err := ParseNativeAcceptance(args); err != nil {
		t.Fatalf("bounded native attribution was refused: %v", err)
	}
	for _, arguments := range [][]string{
		{"--performance-attribution"},
		{artifacts, "--candidate", "--performance-attribution"},
		append(slices.Clone(args), "--clients"),
		append(slices.Clone(args), "--diagnostic-client=hermes"),
	} {
		if _, err := ParseNativeAcceptance(arguments); err == nil {
			t.Fatalf("attribution admitted ambiguous or unbound scope: %v", arguments)
		}
	}
}

func TestNativePerformanceAttributionRetainsOnlyItsOwnMeasurements(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "/published/aigw")
	output := filepath.Join(t.TempDir(), "measurements")
	input, err := ParseNativeAcceptance([]string{"--artifacts=/candidate", "--candidate", "--performance=" + output, "--performance-attribution"})
	if err != nil {
		t.Fatal(err)
	}
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
	calls := 0
	if err := acceptNative(request, "", "/published/aigw", input, func(call toolCall) error {
		calls++
		if slices.Contains(call.Args, "^TestMeasureRetainsSeparateDiagnosticStreams$") && calls == 1 {
			return nil
		}
		if !slices.Contains(call.Args, "^TestNativePerformance$") || !slices.Contains(call.Env, "AIGW_PERFORMANCE_ATTRIBUTION=1") {
			t.Fatalf("attribution repeated or weakened another native gate: %#v", call)
		}
		if err := os.Mkdir(output, 0o700); err != nil {
			return err
		}
		summary := writeQualifiedPerformanceSummary(t, output)
		summary.Qualification, summary.Scope = false, "component-attribution"
		data, err := json.Marshal(summary)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(output, "summary.json"), data, 0o600)
	}); err != nil || calls != 2 {
		t.Fatalf("native attribution calls=%d error=%v", calls, err)
	}
}

func TestNativePerformanceScopeCannotInheritAttribution(t *testing.T) {
	t.Setenv("AIGW_PERFORMANCE_ATTRIBUTION", "1")
	for _, attribution := range []bool{false, true} {
		output := filepath.Join(t.TempDir(), "measurements")
		input := NativeAcceptance{Performance: output, PerformanceAttribution: attribution}
		request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
		if err := acceptNative(request, "", "/published/aigw", input, func(call toolCall) error {
			want := "AIGW_PERFORMANCE_ATTRIBUTION=0"
			if attribution {
				want = "AIGW_PERFORMANCE_ATTRIBUTION=1"
			}
			if !slices.Contains(call.Env, want) {
				t.Fatalf("native performance inherited a different scope: %#v", call.Env)
			}
			if slices.Contains(call.Args, "^TestMeasureRetainsSeparateDiagnosticStreams$") {
				return nil
			}
			if err := os.Mkdir(output, 0o700); err != nil {
				return err
			}
			summary := writeQualifiedPerformanceSummary(t, output)
			if attribution {
				summary.Qualification, summary.Scope = false, "component-attribution"
			}
			data, err := json.Marshal(summary)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(output, "summary.json"), data, 0o600)
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeDiagnosticClientSelectsOnlyItsRetainedJourney(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "/published/aigw")
	for _, client := range nativeAcceptanceClients {
		t.Run(client, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			artifacts := t.TempDir()
			writeNativeArchive(t, artifacts, "1.2.3")
			want := errors.New("diagnosed client failure")
			var calls []toolCall
			err := acceptNative(request, artifacts, "/published/aigw", NativeAcceptance{DiagnosticClient: client}, func(call toolCall) error {
				calls = append(calls, call)
				return want
			})
			pattern := "^TestNativeClientJourney$/^" + client + "($|-)"
			if !errors.Is(err, want) || len(calls) != 1 || !slices.Contains(calls[0].Args, pattern) || !slices.Contains(calls[0].Args, "-tags=client_acceptance") || !slices.Contains(calls[0].Env, "AIGW_ACCEPTANCE_BASELINE=/published/aigw") {
				t.Fatalf("diagnostic changed scope, predecessor, or failure: %#v, %v", calls, err)
			}
			stage := strings.TrimPrefix(calls[0].Env[0], "AIGW_ACCEPTANCE_RELEASE=")
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatalf("diagnostic-owned stage survived: %s, %v", stage, err)
			}
		})
	}
}

func TestNativeReleaseDownloadUsesSelectedPeerAndBoundedNativeRunner(t *testing.T) {
	for _, peer := range []string{"github", "gitlab"} {
		t.Run(peer, func(t *testing.T) {
			input := NativeAcceptance{Peer: peer, Repository: "group/product"}
			directory := filepath.Join(t.TempDir(), "download")
			var calls []toolCall
			err := downloadNativeRelease("source", input, "v1.2.3", directory, func(call toolCall) error {
				calls = append(calls, call)
				return nil
			})
			if err != nil || len(calls) != 1 {
				t.Fatalf("download execution: %v, %#v", err, calls)
			}
			call := calls[0]
			name := "gh"
			if peer == "gitlab" {
				name = "glab"
			}
			if call.Name != name || call.Timeout != 2*time.Minute || !slices.Contains(call.Args, "v1.2.3") || !slices.Contains(call.Args, directory) {
				t.Fatalf("download escaped its selected peer or deadline: %#v", call)
			}
			if !slices.Contains(call.Env, "GH_PROMPT_DISABLED=1") || !slices.Contains(call.Env, "GLAB_NO_PROMPT=1") {
				t.Fatal("download permits interactive authentication")
			}
			err = downloadNativeRelease("source", input, "v1.2.3", directory, func(call toolCall) error {
				t.Fatalf("occupied destination reached a repeated download: %#v", call)
				return nil
			})
			if !errors.Is(err, os.ErrExist) {
				t.Fatalf("download did not preserve its occupied destination: %v", err)
			}
		})
	}
}

func TestNativeTaggedPeerInputsShareTheReleaseLifecycle(t *testing.T) {
	key := signingKey(t)
	root := releaseRoot(t)
	baseline := signedNativeInputFixture(t, root, "1.2.3", key)
	candidate := signedNativeInputFixture(t, root, "1.2.4", key)
	for _, peer := range []string{"github", "gitlab"} {
		t.Run(peer, func(t *testing.T) {
			request := buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}
			input := NativeAcceptance{Peer: peer, Repository: "group/product", Tag: "v1.2.4", BaselineTag: "v1.2.3"}
			downloads, journeys := 0, 0
			var scratch string
			err := acceptNativeInput(t.Context(), input, request, func(call toolCall) error {
				switch call.Name {
				case "git":
					return nil
				case "gh", "glab":
					downloads++
					index := slices.Index(call.Args, "--dir")
					if index < 0 || call.Timeout != 2*time.Minute {
						t.Fatalf("download input is incomplete: %#v", call)
					}
					from := candidate
					if slices.Contains(call.Args, "v1.2.3") {
						from = baseline
					}
					return os.CopyFS(call.Args[index+1], os.DirFS(from))
				case "go":
					journeys++
					scratch = strings.TrimPrefix(call.Env[0], "AIGW_ACCEPTANCE_RELEASE=")
					if !slices.Contains(call.Args, "^TestNativePublishedPredecessor(Journey|ForwardingJourney)$") {
						return nil
					}
					entry := slices.IndexFunc(call.Env, func(value string) bool { return strings.HasPrefix(value, "AIGW_ACCEPTANCE_BASELINE=") })
					if entry < 0 {
						t.Fatal("published predecessor input was discarded")
					}
					_, path, _ := strings.Cut(call.Env[entry], "=")
					data, err := os.ReadFile(path)
					if err != nil || string(data) != "native candidate" {
						t.Fatalf("published predecessor bytes changed: %q, %v", data, err)
					}
				default:
					t.Fatalf("unexpected acceptance tool: %#v", call)
				}
				return nil
			})
			if err != nil || downloads != 2 || journeys != 2 {
				t.Fatalf("peer acceptance=%v, downloads=%d, journeys=%d", err, downloads, journeys)
			}
			if _, err := os.Stat(scratch); !os.IsNotExist(err) {
				t.Fatalf("native acceptance retained its workspace: %v", err)
			}
		})
	}
}

func TestNativeInputFailuresReclaimExactWorkspaceWithoutRunningArtifacts(t *testing.T) {
	for _, failure := range []string{"candidate download", "candidate signature", "baseline download", "baseline signature"} {
		t.Run(failure, func(t *testing.T) {
			root, temp := releaseRoot(t), t.TempDir()
			t.Setenv("TMPDIR", temp)
			t.Setenv("TMP", temp)
			t.Setenv("TEMP", temp)
			input := NativeAcceptance{Tag: "v1.2.3", Peer: "gitlab", Repository: "group/product"}
			if strings.HasPrefix(failure, "baseline") {
				input.Tag, input.BaselineTag = "", "v1.2.2"
			}
			if failure == "baseline signature" {
				input.BaselineArtifacts = t.TempDir()
			}
			sentinel := errors.New("owned download failed")
			calls := 0
			err := acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.3", Epoch: "1784246400"}, func(call toolCall) error {
				switch call.Name {
				case "git":
					return nil
				case "glab":
					calls++
					if strings.HasSuffix(failure, "download") {
						return sentinel
					}
					return nil
				default:
					t.Fatalf("untrusted artifact reached execution: %#v", call)
					return nil
				}
			})
			if err == nil || strings.HasSuffix(failure, "download") && !errors.Is(err, sentinel) {
				t.Fatalf("native input failure changed identity: %v", err)
			}
			if calls == 0 && failure != "baseline signature" {
				t.Fatal("declared download was not attempted")
			}
			matches, err := filepath.Glob(filepath.Join(temp, "aigw-native-inputs-*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("failed native inputs retained owned scratch: %v, %v", matches, err)
			}
		})
	}
}

func TestNativeInputAdmissionFailurePrecedesArtifactsAndCleansScratch(t *testing.T) {
	for _, failure := range []string{"source", "scratch", "journey"} {
		t.Run(failure, func(t *testing.T) {
			root, temp := releaseRoot(t), t.TempDir()
			if failure == "scratch" {
				temp = filepath.Join(temp, "not-a-directory")
				if err := os.WriteFile(temp, []byte("caller-owned"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("TMPDIR", temp)
			t.Setenv("TMP", temp)
			t.Setenv("TEMP", temp)
			input := NativeAcceptance{Tag: "v1.2.3", Peer: "gitlab", Repository: "group/product"}
			if failure == "journey" {
				input = NativeAcceptance{}
			}
			sentinel := errors.New("owned admission failed")
			err := acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.3", Epoch: "1784246400"}, func(call toolCall) error {
				if call.Name == "git" && failure != "source" {
					return nil
				}
				if call.Name != "git" && call.Name != "go" {
					t.Fatalf("failed admission reached artifact transport: %#v", call)
				}
				return sentinel
			})
			if err == nil {
				t.Fatal("failed native admission was accepted")
			}
			if failure == "scratch" {
				if bytes, readErr := os.ReadFile(temp); readErr != nil || string(bytes) != "caller-owned" {
					t.Fatalf("invalid scratch admission changed caller input: %v", readErr)
				}
				return
			}
			if !errors.Is(err, sentinel) {
				t.Fatalf("native admission error identity changed: %v", err)
			}
			if matches, err := filepath.Glob(filepath.Join(temp, "aigw-native-*-*")); err != nil || len(matches) != 0 {
				t.Fatalf("failed native admission retained scratch: %v, %v", matches, err)
			}
		})
	}
}

func signedNativeInputFixture(t *testing.T, root, version, key string) string {
	t.Helper()
	for name, body := range map[string]string{
		"VERSION": version + "\n", "go.mod": "module example.invalid/native\n", "go.sum": "sum\n",
		"package-lock.json": "{}\n", "mise.lock": "lockfile_version = 2\n", "mise.toml": "[tools]\ngo = \"1.27.1\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		prefix := []string{"-C", root, "-c", "core.hooksPath=" + filepath.Join(root, ".git", "hooks"), "-c", "user.name=Native Test", "-c", "user.email=native@test.invalid", "-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen", "-c", "user.signingkey=" + key}
		output, err := exec.CommandContext(t.Context(), "git", append(prefix, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git failed: %v, %s", err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		git("init", "-q", "-b", "main")
	}
	git("add", ".")
	git("commit", "-q", "-S", "-m", "test: native release "+version)
	git("tag", "-s", "-m", "Release "+version, "v"+version)
	directory := t.TempDir()
	writeNativeArchive(t, directory, version)
	for _, name := range releaseartifact.Names(version) {
		if _, err := os.Stat(filepath.Join(directory, name)); os.IsNotExist(err) {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := releaseartifact.WriteProvenance(root, directory, filepath.Join(directory, "aigw_"+version+".provenance.json"), version, git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")); err != nil {
		t.Fatal(err)
	}
	if err := releaseartifact.RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, "checksums.txt")
	if err := os.Remove(manifest + ".sig"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(t.Context(), "ssh-keygen", "-Y", "sign", "-f", key, "-n", upgradeartifact.SignatureNamespace, manifest).CombinedOutput(); err != nil {
		t.Fatalf("fixture signing failed: %v, %s", err, output)
	}
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(t.TempDir(), "signers")
	if err := os.WriteFile(allowed, []byte(fmt.Sprintf("native@test.invalid %s", public)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE", allowed)
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", allowed)
	t.Setenv("AIGW_RELEASE_ARTIFACT_SIGNER", "native@test.invalid")
	return directory
}
