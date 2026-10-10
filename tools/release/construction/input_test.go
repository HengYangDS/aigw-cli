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
		for name, body := range map[string]string{"VERSION": entry.version + "\n", "go.mod": "module example.invalid/native\n", "go.sum": "sum\n",
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
