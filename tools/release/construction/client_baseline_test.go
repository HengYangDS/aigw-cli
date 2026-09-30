package construction

import (
	releaseartifact "aigw-cli/tools/release/artifact"
	"context"
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
	err := acceptNative(request, artifacts, os.Getenv("AIGW_ACCEPTANCE_BASELINE"), true, "", func(call toolCall) error {
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
		})
	}
}

func TestReleaseToolDeadlineLeavesItsParentLive(t *testing.T) {
	if os.Getenv("AIGW_TEST_DEADLINE_CHILD") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	err = executeTool(parent)(toolCall{Name: executable, Args: []string{"-test.run=^TestReleaseToolDeadlineLeavesItsParentLive$"}, Env: []string{"AIGW_TEST_DEADLINE_CHILD=1"}, Timeout: 100 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || parent.Err() != nil {
		t.Fatalf("child deadline failed or cancelled its parent: %v, %v", err, parent.Err())
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
					if !slices.Contains(call.Args, "^TestNativePublishedPredecessorJourney$") {
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
	if output, err := exec.CommandContext(t.Context(), "ssh-keygen", "-Y", "sign", "-f", key, "-n", releaseartifact.SignatureNamespace, manifest).CombinedOutput(); err != nil {
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
