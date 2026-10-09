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
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNativePackagedInputsShareSignedMatrixAndOwnedLifecycle(t *testing.T) {
	root, source, contents := nativePackageFixture(t)
	input, err := ParseNativeAcceptance(nativePackageArguments(source, contents))
	if err != nil {
		t.Fatalf("explicit native package input was refused: %v", err)
	}
	if !input.UsesPrebuiltArtifacts() {
		t.Fatal("native package input repeated source qualification")
	}
	downloads, journeys := 0, 0
	var workspace string
	err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
		switch call.Name {
		case "git":
			return nil
		case "glab":
			downloads++
			index := slices.Index(call.Args, "--path")
			if index < 0 {
				t.Fatalf("native package has no exact download path: %#v", call)
			}
			archive := call.Args[index+1]
			workspace = filepath.Dir(archive)
			want := []string{"packages", "download", "--repo", "group/product", "--name", "native-inputs", "--version", source, "--filename", "public-inputs.tar", "--path", archive}
			if !slices.Equal(call.Args, want) || call.Directory != root || call.Timeout != 2*time.Minute {
				t.Fatalf("native package identity or deadline changed: %#v", call)
			}
			for _, setting := range []string{"GLAB_NO_PROMPT=1", "GLAB_ENABLE_CI_AUTOLOGIN=true", "GLAB_CONFIG_DIR=" + filepath.Join(workspace, "glab")} {
				if !slices.Contains(call.Env, setting) {
					t.Fatalf("native package discarded quiet isolated authentication: %#v", call.Env)
				}
			}
			return os.WriteFile(archive, contents, 0o600)
		case "go":
			journeys++
			for _, directory := range []string{"candidate", "baseline"} {
				entries, err := os.ReadDir(filepath.Join(workspace, directory))
				if err != nil || len(entries) != len(artifact.Names("1.2.4")) {
					t.Fatalf("native package did not select the complete matrix: %s, %v, %v", directory, entries, err)
				}
			}
			if _, err := os.Stat(filepath.Join(workspace, "suppliers")); !os.IsNotExist(err) {
				t.Fatalf("product acceptance extracted independent client supplies: %v", err)
			}
			if !slices.Contains(call.Args, "^TestNativePublishedPredecessor(Journey|ForwardingJourney)$") {
				return nil
			}
			index := slices.IndexFunc(call.Env, func(setting string) bool { return strings.HasPrefix(setting, "AIGW_ACCEPTANCE_BASELINE=") })
			if index < 0 {
				t.Fatal("native package discarded its published predecessor")
			}
			_, baseline, _ := strings.Cut(call.Env[index], "=")
			data, err := os.ReadFile(baseline)
			if err != nil || string(data) != "native candidate" {
				t.Fatalf("published predecessor bytes changed: %q, %v", data, err)
			}
		default:
			t.Fatalf("unexpected native package tool: %#v", call)
		}
		return nil
	})
	if err != nil || downloads != 1 || journeys != 2 {
		t.Fatalf("native package lifecycle=%v, downloads=%d, journeys=%d", err, downloads, journeys)
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("native package retained its exact workspace: %v", err)
	}
}

func TestNativeGitHubInputReleasePreservesSignedIdentityAndOwnedCleanup(t *testing.T) {
	root, source, contents := nativePackageFixture(t)
	carrier, err := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	args := nativePackageArguments(source, contents)
	args[0], args[1], args[10] = "--input-release", "native-inputs-"+strings.TrimSpace(string(carrier)), "github"
	input, err := ParseNativeAcceptance(args)
	if err != nil || !input.UsesPrebuiltArtifacts() {
		t.Fatalf("signed GitHub input release was refused: %v", err)
	}
	downloads, journeys, stopped := 0, 0, false
	var archive string
	run := func(call toolCall) error {
		switch call.Name {
		case "git":
			switch call.Args[0] {
			case "-c", "rev-parse", "merge-base":
				return executeTool(t.Context())(call)
			}
		case "gh":
			downloads++
			archive = call.Args[slices.Index(call.Args, "--output")+1]
			want := []string{"release", "download", input.InputRelease, "--repo", "group/product", "--pattern", "public-inputs.tar", "--output", archive}
			if !slices.Equal(call.Args, want) || call.Directory != root || call.Timeout != 2*time.Minute || !filepath.IsAbs(archive) {
				t.Fatalf("GitHub input identity or deadline changed: %#v", call)
			}
			if !slices.Contains(call.Env, "GH_PROMPT_DISABLED=1") {
				t.Fatal("GitHub input transport permits authentication prompts")
			}
			if writeErr := os.WriteFile(archive, contents, 0o600); writeErr != nil {
				return writeErr
			}
			if stopped {
				return context.Canceled
			}
		case "go":
			journeys++
		default:
			t.Fatalf("unexpected GitHub input tool: %#v", call)
		}
		return nil
	}
	for _, interrupted := range []bool{false, true} {
		temp := t.TempDir()
		for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
			t.Setenv(key, temp)
		}
		downloads, journeys, stopped = 0, 0, interrupted
		err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, run)
		if stopped {
			if !errors.Is(err, context.Canceled) || journeys != 0 {
				t.Fatalf("stopped download reached acceptance: %d, %v", journeys, err)
			}
		} else if err != nil || journeys != 2 {
			t.Fatalf("GitHub matrices did not reach both native journeys: %d, %v", journeys, err)
		}
		if downloads != 1 {
			t.Fatalf("GitHub input downloads = %d, want one", downloads)
		}
		if _, err := os.Stat(filepath.Dir(archive)); !os.IsNotExist(err) {
			t.Fatalf("GitHub input retained owned scratch: %v", err)
		}
	}
}

func TestNativeGitHubInputReleaseRejectsUnboundCarrier(t *testing.T) {
	root, source, contents := nativePackageFixture(t)
	carrier, err := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	args := nativePackageArguments(source, contents)
	args[0], args[1], args[10] = "--input-release", "native-inputs-"+strings.TrimSpace(string(carrier)), "github"
	input, err := ParseNativeAcceptance(args)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"tag target", "producer ancestry"} {
		t.Run(invalid, func(t *testing.T) {
			temp := t.TempDir()
			for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(key, temp)
			}
			err := acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
				if call.Name != "git" {
					t.Fatalf("unbound carrier reached acquisition or native acceptance: %#v", call)
				}
				if invalid == "tag target" && slices.Contains(call.Args, "rev-parse") {
					_, err := io.WriteString(call.Stdout, source+"\n")
					return err
				}
				if invalid == "producer ancestry" && slices.Contains(call.Args, "merge-base") {
					return errors.New("producer is outside carrier history")
				}
				return executeTool(t.Context())(call)
			})
			if err == nil {
				t.Fatal("unbound GitHub input reached native acceptance")
			}
			if matches, err := filepath.Glob(filepath.Join(temp, "aigw-native-inputs-*")); err != nil || len(matches) != 0 {
				t.Fatalf("refused carrier retained owned scratch: %v, %v", matches, err)
			}
		})
	}
}

func TestNativeGitHubInputReleaseRequiresOneExactCarrier(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	source := strings.Repeat("a", 40)
	args := nativePackageArguments(source, nil)
	args[0], args[1], args[10] = "--input-release", "native-inputs-"+strings.Repeat("b", 40), "github"
	if _, err := ParseNativeAcceptance(args); err != nil {
		t.Fatalf("exact GitHub native input was refused: %v", err)
	}
	for _, conflict := range [][]string{
		{"--peer", "gitlab"}, {"--input-release", "v1.2.4"},
		{"--input-release", "native-inputs-" + strings.Repeat("B", 40)},
		{"--input-release", "native-inputs-" + strings.Repeat("b", 39)},
		{"--input-package", "native-inputs"}, {"--input-archive", filepath.Join(t.TempDir(), "foreign.tar")},
		{"--candidate-source", strings.ToUpper(source)}, {"--candidate=false"},
	} {
		if _, err := ParseNativeAcceptance(append(slices.Clone(args), conflict...)); err == nil {
			t.Errorf("GitHub input accepted competing or unbound identity: %q", conflict)
		}
	}
}

func TestNativePackageAdmissionRequiresOneExactSourceAndTransport(t *testing.T) {
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	args := nativePackageArguments(strings.Repeat("a", 40), nil)
	if _, err := ParseNativeAcceptance(args); err != nil {
		t.Fatalf("complete native package identity was refused: %v", err)
	}
	output := filepath.Join(t.TempDir(), "performance")
	if _, err := ParseNativeAcceptance(append(slices.Clone(args), "--performance", output)); err != nil {
		t.Fatalf("native package performance output was refused: %v", err)
	}
	if _, err := ParseNativeAcceptance(append(slices.Clone(args), "--performance", "manager-relative-build/performance")); err == nil || err.Error() != "performance output must be an absolute directory" {
		t.Fatalf("relative performance input reached package acquisition: %v", err)
	}
	for _, missing := range []string{"--input-sha256", "--candidate-source", "--baseline-tag", "--peer", "--repository"} {
		selected := slices.Clone(args)
		index := slices.Index(selected, missing)
		selected = slices.Delete(selected, index, index+2)
		if _, err := ParseNativeAcceptance(selected); err == nil {
			t.Errorf("native package accepted missing %s", missing)
		}
	}
	for _, conflicting := range [][]string{
		{"--peer", "github"}, {"--artifacts", "/foreign"}, {"--tag", "v1.2.4"},
		{"--baseline-artifacts", "/foreign"}, {"--input-sha256", strings.Repeat("g", 64)},
		{"--candidate-source", "HEAD"}, {"--candidate=false"}, {"--input-archive", filepath.Join(t.TempDir(), "retained.tar")},
	} {
		if _, err := ParseNativeAcceptance(append(slices.Clone(args), conflicting...)); err == nil {
			t.Errorf("native package accepted competing or unbound input: %q", conflicting)
		}
	}
	if _, err := ParseNativeAcceptance([]string{"--input-sha256", strings.Repeat("a", 64)}); err == nil {
		t.Fatal("package checksum was accepted without its package")
	}
	local := slices.Clone(args[:len(args)-4])
	local[0], local[1] = "--input-archive", "public-inputs.tar"
	if _, err := ParseNativeAcceptance(local); err == nil {
		t.Fatal("native input accepted a relative caller-owned archive")
	}
}

func TestNativeRetainedPackagePreservesCallerArchiveWithoutTransport(t *testing.T) {
	root, source, contents := nativePackageFixture(t)
	archive := filepath.Join(t.TempDir(), "retained native input.tar")
	if err := os.WriteFile(archive, contents, 0o400); err != nil {
		t.Fatal(err)
	}
	args := nativePackageArguments(source, contents)
	args[0], args[1] = "--input-archive", archive
	args = args[:len(args)-4]
	input, err := ParseNativeAcceptance(args)
	if err != nil {
		t.Fatalf("retained native input was refused: %v", err)
	}
	journeys := 0
	err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
		if call.Name == "git" {
			return nil
		}
		if call.Name != "go" {
			t.Fatalf("retained native input caused another download: %#v", call)
		}
		journeys++
		return nil
	})
	if err != nil || journeys != 2 {
		t.Fatalf("retained native input journeys=%d, error=%v", journeys, err)
	}
	readback, err := os.ReadFile(archive)
	if err != nil || !bytes.Equal(readback, contents) {
		t.Fatalf("native acceptance changed its caller-owned archive: %v", err)
	}
	input.InputArchive = t.TempDir()
	err = acceptNativeInput(t.Context(), input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
		if call.Name != "git" {
			t.Fatalf("nonregular caller input reached execution: %#v", call)
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("native retained input accepted a directory: %v", err)
	}
}

func TestNativePackageFailuresNeverExecuteAndReclaimOwnedScratch(t *testing.T) {
	root, source, valid := nativePackageFixture(t)
	regular := &tar.Header{Name: "candidate/checksums.txt", Mode: 0o600}
	link := &tar.Header{Name: regular.Name, Typeflag: tar.TypeSymlink, Linkname: "../../foreign"}
	for _, failure := range []struct {
		name, checksum  string
		contents        []byte
		transport, want error
		omit, cancel    bool
	}{
		{name: "download", contents: valid, transport: errors.New("owned transport stopped")},
		{name: "checksum", checksum: strings.Repeat("0", 64), contents: valid},
		{name: "missing matrix"},
		{name: "selected link", contents: nativePackageHeaders(t, link)},
		{name: "duplicate matrix", contents: nativePackageHeaders(t, regular, regular)},
		{name: "interrupted download", contents: valid, transport: context.Canceled},
		{name: "missing download", contents: valid, want: os.ErrNotExist, omit: true},
		{name: "corrupt header", contents: bytes.Repeat([]byte{1}, 512), want: tar.ErrHeader},
		{name: "truncated member", contents: valid[:513], want: io.ErrUnexpectedEOF},
		{name: "interrupted extraction", contents: valid, want: context.Canceled, cancel: true},
	} {
		t.Run(failure.name, func(t *testing.T) {
			temp := t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, temp)
			}
			args := nativePackageArguments(source, failure.contents)
			if failure.checksum != "" {
				args[slices.Index(args, "--input-sha256")+1] = failure.checksum
			}
			input, err := ParseNativeAcceptance(args)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			err = acceptNativeInput(ctx, input, buildRequest{Root: root, Version: "1.2.4", Epoch: "1784246400"}, func(call toolCall) error {
				if call.Name == "git" {
					return nil
				}
				if call.Name != "glab" {
					t.Fatalf("invalid native package reached execution: %#v", call)
				}
				archive := call.Args[slices.Index(call.Args, "--path")+1]
				if !failure.omit {
					if err := os.WriteFile(archive, failure.contents, 0o600); err != nil {
						return err
					}
				}
				if failure.cancel {
					cancel()
				}
				return failure.transport
			})
			want := failure.want
			if want == nil {
				want = failure.transport
			}
			if err == nil || want != nil && !errors.Is(err, want) {
				t.Fatalf("native package failure changed identity: %v", err)
			}
			if matches, err := filepath.Glob(filepath.Join(temp, "aigw-native-inputs-*")); err != nil || len(matches) != 0 {
				t.Fatalf("failed native package retained owned scratch: %v, %v", matches, err)
			}
		})
	}
}

func nativePackageHeaders(t *testing.T, headers ...*tar.Header) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for _, header := range headers {
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func nativePackageArguments(source string, contents []byte) []string {
	return []string{"--input-package", "native-inputs", "--input-sha256", fmt.Sprintf("%x", sha256.Sum256(contents)), "--candidate-source", source, "--candidate", "--baseline-tag", "v1.2.3", "--peer", "gitlab", "--repository", "group/product"}
}

func nativePackageFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
	root, key := releaseRoot(t), signingKey(t)
	baseline := signedNativeInputFixture(t, root, "1.2.3", key)
	candidate := signedNativeInputFixture(t, root, "1.2.4", key)
	if output, err := exec.CommandContext(t.Context(), "git", "-C", root, "tag", "-d", "v1.2.4").CombinedOutput(); err != nil {
		t.Fatalf("untagged candidate fixture failed: %v, %s", err, output)
	}
	output, err := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	prefix := []string{"-C", root, "-c", "core.hooksPath=" + filepath.Join(root, ".git", "hooks"), "-c", "user.name=Native Test", "-c", "user.email=native@test.invalid", "-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen", "-c", "user.signingkey=" + key}
	if signed, err := exec.CommandContext(t.Context(), "git", append(slices.Clone(prefix), "commit", "--allow-empty", "-S", "-m", "test: accepted transport carrier")...).CombinedOutput(); err != nil {
		t.Fatalf("synthetic carrier commit failed: %v, %s", err, signed)
	}
	carrier, err := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	arguments := append(slices.Clone(prefix), "tag", "-s", "-a", "native-inputs-"+strings.TrimSpace(string(carrier)), "-m", "native transport input")
	if signed, err := exec.CommandContext(t.Context(), "git", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("synthetic transport fixture failed: %v, %s", err, signed)
	}
	var contents bytes.Buffer
	writer := tar.NewWriter(&contents)
	for _, input := range []struct{ directory, path, version string }{{"candidate", candidate, "1.2.4"}, {"baseline", baseline, "1.2.3"}} {
		for _, name := range artifact.Names(input.version) {
			data, err := os.ReadFile(filepath.Join(input.path, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteHeader(&tar.Header{Name: input.directory + "/" + name, Mode: 0o600, Size: int64(len(data))}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.WriteHeader(&tar.Header{Name: "suppliers/ignored.tar", Size: 1, Mode: 0o600}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(writer, "x"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return root, strings.TrimSpace(string(output)), contents.Bytes()
}
