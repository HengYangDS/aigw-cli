package construction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Ported from the maintenance branch (c9393953): native acceptance selects
// exactly one input transport with exact identities and refuses every other
// combination before any work starts.
func TestNativeInputSelectionRequiresOneTransportAndExactIdentities(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	producer := strings.Repeat("a", 40)
	archive := filepath.Join(t.TempDir(), "public-inputs.tar")
	selection := []string{"--candidate", "--candidate-source", producer, "--input-sha256", strings.Repeat("b", 64), "--baseline-tag", "v1.2.3"}
	for _, transport := range [][]string{
		{"--input-archive", archive},
		{"--input-package", "native-inputs", "--peer", "gitlab", "--repository", "group/product"},
		{"--input-release", "native-inputs-" + producer, "--peer", "github", "--repository", "group/product"},
	} {
		input, err := ParseNativeAcceptance(append(slices.Clone(selection), transport...))
		if err != nil || input.CandidateSource != producer || input.BaselineTag != "v1.2.3" || !input.Candidate {
			t.Fatalf("valid native selection lost producer or predecessor: %#v, %v", input, err)
		}
	}
	for _, arguments := range [][]string{
		{"--unknown"}, {"positional"}, {"--input-sha256", strings.Repeat("b", 64)},
		{"--input-package", "native-inputs", "--peer", "github", "--repository", "group/product"},
		{"--input-release", "native-inputs-" + producer, "--peer", "gitlab", "--repository", "group/product"},
		append(slices.Clone(selection), "--input-archive", "relative.tar"),
		{"--input-archive", archive},
		{"--candidate", "--candidate-source", producer, "--input-archive", archive, "--input-sha256", "invalid", "--baseline-tag", "v1.2.3"},
		append(slices.Clone(selection), "--input-release", "unnamed-carrier", "--peer", "github", "--repository", "group/product"),
		append(slices.Clone(selection), "--input-release", "native-inputs-"+strings.Repeat("A", 40), "--peer", "github", "--repository", "group/product"),
		{"--peer", "github"}, {"--candidate-source", producer}, {"--baseline-artifacts", archive},
		{"--tag", "v1.2.3"}, {"--artifacts", archive, "--tag", "1.2.3"},
		{"--baseline-tag", "v1.2.3"}, {"--performance", t.TempDir()},
	} {
		if input, err := ParseNativeAcceptance(arguments); err == nil {
			t.Fatalf("ambiguous or unbound native selection accepted: %q, %#v", arguments, input)
		}
	}
}

// Ported from the maintenance branch (c9393953): an unproved carrier stops its
// admission before any transport, and an interruption keeps its cause. Here
// the carrier is admitted in two Git steps: tag verification, then its commit.
func TestNativeCarrierRefusalStopsBeforeDownload(t *testing.T) {
	root, producer, _ := nativePackageFixture(t)
	tags, err := exec.CommandContext(t.Context(), "git", "-C", root, "tag", "--list", "native-inputs-*").Output()
	if err != nil {
		t.Fatal(err)
	}
	release := strings.TrimSpace(string(tags))
	for _, stopped := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("admission=%d", stopped), func(t *testing.T) {
			if stopped == 0 {
				t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
			}
			input := NativeAcceptance{InputRelease: release, CandidateSource: producer}
			step := 0
			err := input.verifyInputCarrier(root, func(call toolCall) error {
				step++
				if call.Name != "git" {
					t.Fatalf("unproved carrier started transport: %#v", call)
				}
				if step == stopped {
					return context.Canceled
				}
				if stopped == 3 && step == 2 {
					// The tag resolves to another commit than the one it names.
					_, err := io.WriteString(call.Stdout, producer+"\n")
					return err
				}
				return executeTool(t.Context())(call)
			})
			switch {
			case err == nil || step > 2:
				t.Fatalf("carrier refusal continued admission: %v, steps=%d", err, step)
			case stopped == 1 || stopped == 2:
				if !errors.Is(err, context.Canceled) || step != stopped {
					t.Fatalf("carrier refusal lost its interruption cause: %v, steps=%d", err, step)
				}
			case stopped == 3:
				if !strings.Contains(err.Error(), "does not select its named carrier commit") {
					t.Fatalf("a mismatched carrier was not refused for its commit: %v", err)
				}
			}
		})
	}
}

// Ported from the maintenance branch (c9393953): a missing, unreadable or
// partial input package never runs a dependent operation.
func TestNativePartialInputNeverRunsAJourney(t *testing.T) {
	root, producer, contents := nativePackageFixture(t)
	for _, state := range []string{"missing", "directory", "empty", "header", "truncated"} {
		t.Run(state, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "public-inputs.tar")
			data := []byte(nil)
			switch state {
			case "header":
				data = bytes.Repeat([]byte("invalid"), 100)
			case "truncated":
				data = contents[:513]
			}
			if state == "directory" {
				if err := os.Mkdir(archive, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if state != "missing" {
				if err := os.WriteFile(archive, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			input := NativeAcceptance{InputArchive: archive, InputSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Candidate: true, CandidateSource: producer, BaselineTag: "v1.2.3"}
			err := acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
				if call.Name == "git" {
					return executeTool(t.Context())(call)
				}
				t.Fatalf("partial native input executed a dependent operation: %#v", call)
				return nil
			})
			if err == nil {
				t.Fatal("incomplete native matrix was accepted")
			}
		})
	}
}

// Ported from the maintenance branch (c9393953): an interrupted or cancelled
// journey leaves the caller's archive bytes unchanged and no owned scratch.
func TestNativeInputCancellationAndJourneyFailurePreserveCallerBytes(t *testing.T) {
	root, producer, contents := nativePackageFixture(t)
	t.Chdir(root)
	archive := filepath.Join(t.TempDir(), "public-inputs.tar")
	if err := os.WriteFile(archive, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := ParseNativeAcceptance([]string{"--candidate", "--candidate-source", producer,
		"--input-archive", archive, "--input-sha256", fmt.Sprintf("%x", sha256.Sum256(contents)), "--baseline-tag", "v1.2.3", "--clients"})
	if err != nil {
		t.Fatal(err)
	}
	interrupted := errors.New("native journey interrupted")
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%t", canceled), func(t *testing.T) {
			scratch := t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, scratch)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			want, calls := interrupted, 1
			if canceled {
				cancel()
				want, calls = context.Canceled, 0
			}
			journeys := 0
			err := acceptNativeInput(ctx, input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
				if call.Name == "git" {
					return executeTool(ctx)(call)
				}
				if call.Name != "go" {
					t.Fatalf("caller-owned input requested transport or rebuild: %#v", call)
				}
				journeys++
				return interrupted
			})
			if !errors.Is(err, want) || journeys != calls {
				t.Fatalf("native interruption continued dependent journeys: %v, calls=%d", err, journeys)
			}
			if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
				t.Fatalf("interrupted native input retained owned resources: %v, %v", entries, err)
			}
			if data, err := os.ReadFile(archive); err != nil || !bytes.Equal(data, contents) {
				t.Fatalf("interrupted native input changed caller bytes: %v", err)
			}
		})
	}
}

// Ported from the maintenance branch (c9393953): the public entry refuses the
// verifier's own HEAD relabelled as the artifact producer and leaves the
// caller's input unchanged. A signed verifier commit after the producer makes
// the two differ, so only the provenance binding can refuse it.
func TestNativeAcceptancePublicEntryRejectsVerifierAsArtifactProducer(t *testing.T) {
	root, _, contents := nativePackageFixture(t)
	t.Chdir(root)
	key := signingKey(t)
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []string{"AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"} {
		signers := os.Getenv(variable)
		if signers == "" {
			continue
		}
		file, err := os.OpenFile(signers, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = fmt.Fprintf(file, "\nnative@test.invalid %s", public)
		if err = errors.Join(err, file.Close()); err != nil {
			t.Fatal(err)
		}
	}
	changelog := "# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## [Unreleased]\n\n## [1.2.4] - 2026-07-17\n\n### Fixed\n\n- Preserve native inputs.\n"
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root, "-c", "core.hooksPath=" + filepath.Join(root, ".git", "hooks"),
			"-c", "user.name=Native Test", "-c", "user.email=native@test.invalid", "-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen",
			"-c", "user.signingkey=" + key}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("verifier commit fixture failed: %v, %s", err, output)
		}
	}
	git("add", "CHANGELOG.md")
	git("commit", "-q", "-S", "-m", "test: verifier head")
	verifier, err := resolveGitObject(root, "HEAD", executeTool(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "public-inputs.tar")
	if err := os.WriteFile(archive, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := ParseNativeAcceptance([]string{"--candidate", "--candidate-source", verifier,
		"--input-archive", archive, "--input-sha256", fmt.Sprintf("%x", sha256.Sum256(contents)), "--baseline-tag", "v1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if err := AcceptNative(t.Context(), input); err == nil || !strings.Contains(err.Error(), "provenance does not match") {
		t.Fatalf("public acceptance relabeled producer bytes as the verifier: %v", err)
	}
	if data, err := os.ReadFile(archive); err != nil || !bytes.Equal(data, contents) {
		t.Fatalf("rejected public acceptance changed caller input: %v", err)
	}
}
