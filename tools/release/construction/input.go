package construction

import (
	"aigw-cli/tools/release/artifact"
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (input *NativeAcceptance) hasPackage() bool {
	return input.InputPackage != "" || input.InputRelease != "" || input.InputArchive != ""
}

func (input *NativeAcceptance) validatePackage() error {
	if !input.hasPackage() {
		if input.InputSHA256 != "" {
			return errors.New("native input checksum requires its package")
		}
		return nil
	}
	if input.InputPackage != "" && (input.InputArchive != "" || input.Peer != "gitlab" || input.Repository == "") {
		return errors.New("native input package requires an explicit GitLab transport without a competing archive")
	}
	if input.InputRelease != "" && (input.InputPackage != "" || input.InputArchive != "" || input.Peer != "github" || input.Repository == "") {
		return errors.New("native input release requires an explicit GitHub transport without a competing package or archive")
	}
	if input.InputArchive != "" && !filepath.IsAbs(input.InputArchive) {
		return errors.New("native input archive requires an absolute caller-owned path")
	}
	if !input.Candidate || input.BaselineTag == "" || input.Artifacts != "" || input.Tag != "" || input.BaselineArtifacts != "" {
		return errors.New("native input package requires an exact candidate and published predecessor without competing artifact inputs")
	}
	for _, value := range []struct {
		name, text string
		size       int
	}{
		{"checksum", input.InputSHA256, sha256.Size},
		{"source", input.CandidateSource, 20},
	} {
		if err := validateInputIdentity(value.name, value.text, value.size); err != nil {
			return err
		}
	}
	if input.InputRelease == "" {
		return nil
	}
	carrier, ok := strings.CutPrefix(input.InputRelease, "native-inputs-")
	if !ok {
		return errors.New("native input release must name its exact lowercase carrier commit")
	}
	return validateInputIdentity("carrier commit", carrier, 20)
}

func validateInputIdentity(name, text string, size int) error {
	decoded, err := hex.DecodeString(text)
	if err != nil || len(decoded) != size || text != strings.ToLower(text) {
		return fmt.Errorf("native input package requires exact lowercase hexadecimal %s", name)
	}
	return nil
}

func (input *NativeAcceptance) preparePackage(ctx context.Context, request buildRequest, workspace string, run toolRunner) (result error) {
	if !input.hasPackage() {
		return nil
	}
	if err := input.verifyInputCarrier(request.Root, run); err != nil {
		return err
	}
	archive := input.InputArchive
	if archive == "" {
		archive = filepath.Join(workspace, "public-inputs.tar")
		call := toolCall{
			Name: "glab", Directory: request.Root, Timeout: 2 * time.Minute,
			Args: []string{"packages", "download", "--repo", input.Repository, "--name", input.InputPackage, "--version", input.CandidateSource, "--filename", "public-inputs.tar", "--path", archive},
			Env:  []string{"GLAB_NO_PROMPT=1", "GLAB_ENABLE_CI_AUTOLOGIN=true", "GLAB_CONFIG_DIR=" + filepath.Join(workspace, "glab")},
		}
		if input.InputRelease != "" {
			call.Name = "gh"
			call.Args = []string{"release", "download", input.InputRelease, "--repo", input.Repository, "--pattern", "public-inputs.tar", "--output", archive}
			call.Env = []string{"GH_PROMPT_DISABLED=1"}
		}
		if err := run(call); err != nil {
			return fmt.Errorf("download native input package: %w", err)
		}
	}
	info, err := os.Lstat(archive)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("native input archive must be a regular file")
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return err
	}
	if fmt.Sprintf("%x", digest.Sum(nil)) != input.InputSHA256 {
		return errors.New("native input package checksum mismatch")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := extractNativePackage(ctx, file, workspace, request.Version, strings.TrimPrefix(input.BaselineTag, "v")); err != nil {
		return err
	}
	input.Artifacts, input.BaselineArtifacts = filepath.Join(workspace, "candidate"), filepath.Join(workspace, "baseline")
	return nil
}

func (input *NativeAcceptance) verifyInputCarrier(root string, run toolRunner) error {
	if input.InputRelease == "" {
		return nil
	}
	ref := "refs/tags/" + input.InputRelease
	git := toolCall{
		Name: "git", Directory: root, Timeout: time.Minute,
		Args: []string{"-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen", "-c", "gpg.ssh.allowedSignersFile=" + os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE"), "verify-tag", ref},
	}
	if err := run(git); err != nil {
		return fmt.Errorf("verify native input transport tag: %w", err)
	}
	var source bytes.Buffer
	git.Args, git.Stdout = []string{"rev-parse", "--verify", ref + "^{commit}"}, &source
	if err := run(git); err != nil {
		return fmt.Errorf("resolve native input transport source: %w", err)
	}
	carrier := strings.TrimPrefix(input.InputRelease, "native-inputs-")
	if strings.TrimSpace(source.String()) != carrier {
		return errors.New("native input transport tag does not select its named carrier commit")
	}
	git.Args, git.Stdout = []string{"merge-base", "--is-ancestor", input.CandidateSource, carrier}, nil
	if err := run(git); err != nil {
		return fmt.Errorf("native input producer is not an ancestor of its carrier: %w", err)
	}
	return nil
}

func extractNativePackage(ctx context.Context, file io.Reader, workspace, candidate, baseline string) error {
	expected := make(map[string]bool)
	for _, input := range []struct{ directory, version string }{{"candidate", candidate}, {"baseline", baseline}} {
		if err := os.Mkdir(filepath.Join(workspace, input.directory), 0o700); err != nil {
			return err
		}
		for _, name := range artifact.Names(input.version) {
			expected[input.directory+"/"+name] = false
		}
	}
	reader := tar.NewReader(file)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		seen, selected := expected[header.Name]
		if !selected {
			continue
		}
		if seen || header.Typeflag != tar.TypeReg {
			return fmt.Errorf("native input matrix requires one regular file for %s", header.Name)
		}
		if err := writeNativeInputFile(reader, filepath.Join(workspace, filepath.FromSlash(header.Name))); err != nil {
			return err
		}
		expected[header.Name] = true
	}
	for name, seen := range expected {
		if !seen {
			return fmt.Errorf("native input matrix is missing %s", name)
		}
	}
	return nil
}

func writeNativeInputFile(reader io.Reader, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, reader)
	return errors.Join(copyErr, file.Close())
}
