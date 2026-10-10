package construction

import (
	"aigw-cli/tools/release/artifact"
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

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

func TestNativeCarrierRefusalStopsBeforeDownload(t *testing.T) {
	root, producer, _ := nativeInputFixture(t)
	carrier, err := resolveGitObject(root, "HEAD", executeTool(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	for _, stopped := range []int{0, 1, 2, 3, 4} {
		t.Run(fmt.Sprintf("admission=%d", stopped), func(t *testing.T) {
			if stopped == 0 {
				t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
			}
			input := NativeAcceptance{InputRelease: "native-inputs-" + carrier, CandidateSource: producer}
			step := 0
			err := input.verifyInputCarrier(root, func(call toolCall) error {
				step++
				if call.Name != "git" {
					t.Fatalf("unproved carrier started transport: %#v", call)
				}
				if step == stopped || stopped == 4 && step == 3 {
					return context.Canceled
				}
				if stopped == 3 && step == 2 {
					_, err := io.WriteString(call.Stdout, producer+"\n")
					return err
				}
				return executeTool(t.Context())(call)
			})
			if err == nil || stopped == 0 && step != 0 || stopped != 0 && step > 3 {
				t.Fatalf("carrier refusal continued admission: %v, steps=%d", err, step)
			}
			if stopped != 0 && stopped != 3 && !errors.Is(err, context.Canceled) {
				t.Fatalf("carrier refusal lost its interruption cause: %v", err)
			}
		})
	}
}

func TestNativePartialInputNeverRunsAJourney(t *testing.T) {
	root, producer, contents := nativeInputFixture(t)
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
				t.Fatalf("partial native input executed a dependent operation: %#v", call)
				return nil
			})
			if err == nil {
				t.Fatal("incomplete native matrix was accepted")
			}
		})
	}
}

func TestNativeInputCancellationAndJourneyFailurePreserveCallerBytes(t *testing.T) {
	root, producer, contents := nativeInputFixture(t)
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

func TestNativeAcceptancePublicEntryRejectsVerifierAsArtifactProducer(t *testing.T) {
	root, _, contents := nativeInputFixture(t)
	t.Chdir(root)
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
	if err := AcceptNative(t.Context(), input); err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("public acceptance relabeled producer bytes as the verifier: %v", err)
	}
	if data, err := os.ReadFile(archive); err != nil || !bytes.Equal(data, contents) {
		t.Fatalf("rejected public acceptance changed caller input: %v", err)
	}
	for _, path := range []string{"VERSION", "CHANGELOG.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := AcceptNative(t.Context(), input); err == nil {
			t.Fatalf("public acceptance proceeded without %s", path)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeRetainedPackageOwnsExtractionAndPublishedBaseline(t *testing.T) {
	t.Setenv("AIGW_VERIFY_SYSTEM_KEYRING", "1")
	root, producer, contents := nativeInputFixture(t)
	archive := filepath.Join(t.TempDir(), "public-inputs.tar")
	if err := os.WriteFile(archive, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	input, err := ParseNativeAcceptance([]string{"--candidate", "--candidate-source", producer,
		"--input-archive", archive, "--input-sha256", fmt.Sprintf("%x", sha256.Sum256(contents)), "--baseline-tag", "v1.2.3"})
	if err != nil {
		t.Fatalf("exact native package refused: %v", err)
	}
	var baseline, candidate string
	err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
		if call.Name != "go" {
			t.Fatalf("local package requested transport or rebuild: %#v", call)
		}
		for _, setting := range call.Env {
			if value, ok := strings.CutPrefix(setting, "AIGW_ACCEPTANCE_BASELINE="); ok {
				baseline = value
			}
			if value, ok := strings.CutPrefix(setting, "AIGW_ACCEPTANCE_RELEASE="); ok {
				candidate = value
			}
		}
		data, err := os.ReadFile(baseline)
		if err != nil || string(data) != "native candidate" {
			t.Fatalf("native lifecycle lost the signed published predecessor: %q, %v", data, err)
		}
		if filepath.Base(filepath.Dir(baseline)) != "aigw_1.2.3_"+runtime.GOOS+"_"+runtime.GOARCH {
			t.Fatalf("wrong predecessor archive selected: %s", baseline)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{baseline, candidate} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned native extraction survived acceptance: %s, %v", path, err)
		}
	}
	if data, err := os.ReadFile(archive); err != nil || !bytes.Equal(data, contents) {
		t.Fatalf("caller-owned native package changed: %v", err)
	}
	input.InputSHA256 = strings.Repeat("0", 64)
	err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(toolCall) error {
		t.Fatal("invalid package executed native tests")
		return errors.New("unreachable")
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("changed package checksum accepted: %v", err)
	}
}

func TestNativePackageTransportBindsSourceAndReclaimsInterruptedInput(t *testing.T) {
	root, producer, contents := nativeInputFixture(t)
	t.Chdir(root)
	carrier, err := resolveGitObject(root, "HEAD", executeTool(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	for _, peer := range []struct{ name, flag, input, tool string }{
		{"gitlab", "--input-package", "native-inputs", "glab"},
		{"github", "--input-release", "native-inputs-" + carrier, "gh"},
	} {
		for _, stopped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/interrupted=%t", peer.name, stopped), func(t *testing.T) {
				input, err := ParseNativeAcceptance([]string{peer.flag, peer.input, "--candidate", "--candidate-source", producer,
					"--input-sha256", fmt.Sprintf("%x", sha256.Sum256(contents)), "--baseline-tag", "v1.2.3", "--peer", peer.name, "--repository", "group/product"})
				if err != nil {
					t.Fatalf("exact native transport refused: %v", err)
				}
				journeys := 0
				var download toolCall
				transportResult := error(nil)
				if stopped {
					transportResult = context.Canceled
				}
				err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
					if call.Name == "git" {
						return executeTool(t.Context())(call)
					}
					if call.Name == "go" {
						journeys++
						return nil
					}
					download = call
					if err := os.WriteFile(call.Args[len(call.Args)-1], contents, 0o600); err != nil {
						return err
					}
					return transportResult
				})
				if !errors.Is(err, transportResult) || (journeys > 0) == stopped {
					t.Fatalf("native transport result: %v, journeys=%d", err, journeys)
				}
				archive := download.Args[len(download.Args)-1]
				workspace := filepath.Dir(archive)
				want := []string{"packages", "download", "--repo", "group/product", "--name", "native-inputs", "--version", producer, "--filename", "public-inputs.tar", "--path", archive}
				noPrompt := "GLAB_NO_PROMPT=1"
				if peer.name == "github" {
					want = []string{"release", "download", "native-inputs-" + carrier, "--repo", "group/product", "--pattern", "public-inputs.tar", "--output", archive}
					noPrompt = "GH_PROMPT_DISABLED=1"
				}
				if download.Name != peer.tool || !slices.Equal(download.Args, want) || download.Directory != root || download.Timeout != 2*time.Minute || !filepath.IsAbs(archive) || !slices.Contains(download.Env, noPrompt) {
					t.Fatalf("native transport lost exact input, no-prompt policy or deadline: %#v", download)
				}
				if _, err := os.Stat(workspace); !os.IsNotExist(err) {
					t.Fatalf("native transport retained its owned input workspace: %v", err)
				}
			})
		}
	}
}

func TestNativePublishedInputsUseTheSelectedPeerAndActualBaseline(t *testing.T) {
	root, _, contents := nativeInputFixture(t)
	t.Chdir(root)
	files := make(map[string][]byte)
	reader := tar.NewReader(bytes.NewReader(contents))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name, selected := strings.CutPrefix(header.Name, "baseline/")
		if !selected {
			continue
		}
		files[name], err = io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, peer := range []string{"github", "gitlab"} {
		t.Run(peer, func(t *testing.T) {
			input, err := ParseNativeAcceptance([]string{"--tag", "v1.2.3", "--baseline-tag", "v1.2.3", "--peer", peer, "--repository", "group/product"})
			if err != nil {
				t.Fatalf("published native inputs refused: %v", err)
			}
			var journeys []toolCall
			err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.3", Epoch: "1784246400"}, func(call toolCall) error {
				if call.Name == "go" {
					journeys = append(journeys, call)
					return nil
				}
				directory := call.Args[slices.Index(call.Args, "--dir")+1]
				want := []string{"release", "download", "v1.2.3", "--repo", "group/product", "--dir", directory}
				tool := map[string]string{"github": "gh", "gitlab": "glab"}[peer]
				want = append(want, map[string][]string{"gitlab": {"--asset-name", "aigw_*", "--asset-name", "checksums.txt*"}}[peer]...)
				if call.Name != tool || !slices.Equal(call.Args, want) || call.Timeout != 2*time.Minute || call.Directory != root {
					t.Fatalf("published input used the wrong transport or deadline: %#v", call)
				}
				for name, data := range files {
					if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil || len(journeys) == 0 {
				t.Fatalf("published inputs did not reach their native owner: %v", err)
			}
			for _, call := range journeys {
				index := slices.IndexFunc(call.Env, func(value string) bool { return strings.HasPrefix(value, "AIGW_ACCEPTANCE_BASELINE=") })
				baseline := strings.TrimPrefix(call.Env[index], "AIGW_ACCEPTANCE_BASELINE=")
				if _, err := os.Stat(baseline); !os.IsNotExist(err) {
					t.Fatalf("published baseline extraction survived acceptance: %v", err)
				}
			}
		})
	}
}

func TestNativeSourceSelectionRetainsPublishedBaselineClientsAndPerformance(t *testing.T) {
	root, _, contents := nativeInputFixture(t)
	t.Chdir(root)
	matrices := t.TempDir()
	if err := extractNativePackage(t.Context(), bytes.NewReader(contents), matrices, "1.2.4", "1.2.3"); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "performance")
	input, err := ParseNativeAcceptance([]string{"--baseline-tag", "v1.2.3", "--baseline-artifacts", filepath.Join(matrices, "baseline"), "--clients", "--performance", output})
	if err != nil {
		t.Fatal(err)
	}
	var builds int
	var stage string
	var journeys []toolCall
	err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
		if call.Name == "goreleaser" {
			builds++
			if !slices.Contains(call.Env, "AIGW_BUILD_OS="+runtime.GOOS) {
				t.Fatal("source selection did not build the actual host")
			}
			stage = goReleaserStage(t, call.Args)
			writeNativeVersionArchive(t, stage, "1.2.4")
			return nil
		}
		journeys = append(journeys, call)
		if !slices.Contains(call.Env, "AIGW_ACCEPTANCE_RELEASE="+stage) {
			t.Fatal("selected journey lost its built candidate")
		}
		if slices.Contains(call.Env, "AIGW_PERFORMANCE_OUTPUT="+output) {
			if err := os.Mkdir(output, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(output, "summary.json"), []byte(`{"blocks":[{}]}`), 0o600)
		}
		return nil
	})
	if err != nil || builds != 1 || len(journeys) != 4 {
		t.Fatalf("source selection builds=%d journeys=%d: %v", builds, len(journeys), err)
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("owned source candidate survived acceptance: %v", err)
	}
}

func nativeInputFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	root, key := releaseRoot(t), signingKey(t)
	git := func(args ...string) string {
		t.Helper()
		prefix := []string{"-C", root, "-c", "core.hooksPath=" + filepath.Join(root, ".git", "hooks"),
			"-c", "user.name=Native Test", "-c", "user.email=native@test.invalid", "-c", "gpg.format=ssh",
			"-c", "gpg.ssh.program=ssh-keygen", "-c", "user.signingkey=" + key}
		output, err := exec.CommandContext(t.Context(), "git", append(prefix, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git: %v: %s", err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q", "-b", "main")
	var contents bytes.Buffer
	writer := tar.NewWriter(&contents)
	for _, entry := range []struct{ directory, version string }{{"baseline", "1.2.3"}, {"candidate", "1.2.4"}} {
		for name, body := range map[string]string{"VERSION": entry.version + "\n", "CHANGELOG.md": "# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## [Unreleased]\n\n## [" + entry.version + "] - 2026-07-17\n\n### Fixed\n\n- Preserve native inputs.\n", "go.mod": "module example.invalid/native\n", "go.sum": "sum\n",
			"package-lock.json": "{}\n", "mise.lock": "lockfile_version = 2\n", "mise.toml": "[tools]\ngo = \"1.27.1\"\n"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		git("add", ".")
		git("commit", "-q", "-S", "-m", "test: native release "+entry.version)
		if entry.directory == "baseline" {
			git("tag", "-s", "-m", "published baseline", "v"+entry.version)
		}
		directory := t.TempDir()
		writeNativeVersionArchive(t, directory, entry.version)
		for _, name := range artifact.Names(entry.version) {
			if _, err := os.Stat(filepath.Join(directory, name)); os.IsNotExist(err) {
				if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := artifact.WriteProvenance(root, directory, filepath.Join(directory, "aigw_"+entry.version+".provenance.json"), entry.version, git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")); err != nil {
			t.Fatal(err)
		}
		if err := artifact.RewriteChecksums(directory, entry.version); err != nil {
			t.Fatal(err)
		}
		manifest := filepath.Join(directory, "checksums.txt")
		if err := os.Remove(manifest + ".sig"); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.CommandContext(t.Context(), "ssh-keygen", "-Y", "sign", "-f", key, "-n", artifact.SignatureNamespace, manifest).CombinedOutput(); err != nil {
			t.Fatalf("fixture manifest signature: %v: %s", err, output)
		}
		for _, name := range artifact.Names(entry.version) {
			data, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteHeader(&tar.Header{Name: entry.directory + "/" + name, Mode: 0o600, Size: int64(len(data))}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
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
	producer := git("rev-parse", "HEAD")
	git("commit", "-q", "-S", "--allow-empty", "-m", "test: separate native verifier")
	carrier := git("rev-parse", "HEAD")
	git("tag", "-s", "-m", "native acceptance input", "native-inputs-"+carrier)
	return root, producer, slices.Clone(contents.Bytes())
}
