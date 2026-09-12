// Package artifact defines and verifies the portable release artifact matrix.
package artifact

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// SignatureNamespace separates release-asset signatures from other SSH signatures.
const SignatureNamespace = "aigw-release"

// SignatureTrust identifies the independently supplied authorization for artifact signatures.
type SignatureTrust struct {
	AllowedSigners string
	Principal      string
}

// VerifyMatrix checks complete artifact bytes against an explicitly authorized signer.
func VerifyMatrix(ctx context.Context, directory, version string, trust SignatureTrust) error {
	if trust.AllowedSigners == "" || trust.Principal == "" {
		return errors.New("release artifact authorization requires allowed-signers file and signer principal")
	}
	if _, err := verifiedDigests(directory, version); err != nil {
		return err
	}
	return verifySignature(ctx, directory, "verify", "-f", trust.AllowedSigners, "-I", trust.Principal)
}

// Archives returns the portable product archives for version.
func Archives(version string) []string {
	return []string{
		"aigw_" + version + "_darwin_amd64.tar.gz",
		"aigw_" + version + "_darwin_arm64.tar.gz",
		"aigw_" + version + "_linux_amd64.tar.gz",
		"aigw_" + version + "_linux_arm64.tar.gz",
		"aigw_" + version + "_windows_amd64.zip",
		"aigw_" + version + "_windows_arm64.zip",
	}
}

// Names returns the complete release artifact matrix for version.
func Names(version string) []string {
	return append(provenanceSubjects(version),
		"aigw_"+version+".provenance.json",
		"checksums.txt",
		"checksums.txt.sig",
	)
}

func provenanceSubjects(version string) []string {
	return append(Archives(version),
		"aigw_"+version+".spdx.json",
		"aigw_"+version+".vulnerabilities.json",
		"aigw_"+version+".licenses.json",
	)
}

// ValidateMatrix verifies membership, non-empty files, and checksums.
func ValidateMatrix(directory, version string) error {
	if _, err := verifiedDigests(directory, version); err != nil {
		return err
	}
	return verifySignature(context.Background(), directory, "check-novalidate")
}

func verifySignature(ctx context.Context, directory, operation string, arguments ...string) error {
	manifest := filepath.Join(directory, "checksums.txt")
	file, err := os.Open(manifest)
	if err != nil {
		return fmt.Errorf("release artifact matrix: read checksum manifest for signature verification: %w", err)
	}
	defer func() { _ = file.Close() }()
	args := append([]string{"-Y", operation,
		"-n", SignatureNamespace,
		"-s", filepath.Join(directory, "checksums.txt.sig"),
	}, arguments...)
	command := exec.CommandContext(ctx, "ssh-keygen", args...)
	command.Stdin = file
	if output, runErr := command.CombinedOutput(); runErr != nil {
		return fmt.Errorf("release artifact matrix: invalid detached signature: %w: %s", runErr, strings.TrimSpace(string(output)))
	}
	return nil
}

func verifiedDigests(directory, version string) (map[string]string, error) {
	wanted := Names(version)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("release artifact matrix: %w", err)
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			actual = append(actual, entry.Name())
		}
	}
	slices.Sort(actual)
	expected := slices.Clone(wanted)
	slices.Sort(expected)
	localDigests := make(map[string]string, len(wanted)-1)
	var manifest []byte
	for _, name := range wanted {
		data, readErr := os.ReadFile(filepath.Join(directory, name))
		if readErr != nil || len(data) == 0 {
			return nil, fmt.Errorf("release artifact matrix: missing or empty artifact: %s", name)
		}
		if name == "checksums.txt" {
			manifest = data
		} else {
			localDigests[name] = fmt.Sprintf("%x", sha256.Sum256(data))
		}
	}
	if !slices.Equal(actual, expected) {
		return nil, fmt.Errorf("release artifact matrix: unexpected or missing files: %v", actual)
	}
	digests := map[string]string{}
	for line := range strings.Lines(string(manifest)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != 64 {
			return nil, errors.New("release artifact matrix: invalid checksum manifest format")
		}
		name := strings.TrimPrefix(fields[1], "./")
		if _, duplicate := digests[name]; duplicate {
			return nil, fmt.Errorf("release artifact matrix: duplicate checksum entry: %s", name)
		}
		digests[name] = strings.ToLower(fields[0])
	}
	for _, name := range checksummedNames(version) {
		if digests[name] != localDigests[name] {
			return nil, fmt.Errorf("release artifact matrix: checksum mismatch for %s", name)
		}
	}
	if len(digests) != len(checksummedNames(version)) {
		return nil, errors.New("release artifact matrix: checksum manifest has unexpected entries")
	}
	digests["checksums.txt.sig"] = localDigests["checksums.txt.sig"]
	return digests, nil
}

// CompareMatrices verifies two complete matrices and compares their content.
func CompareMatrices(left, right, version string) error {
	leftDigests, err := verifiedDigests(left, version)
	if err != nil {
		return err
	}
	rightDigests, err := verifiedDigests(right, version)
	if err != nil {
		return err
	}
	for _, name := range comparableNames(version) {
		if leftDigests[name] != rightDigests[name] {
			return fmt.Errorf("release artifact differs across forge stages: %s", name)
		}
	}
	return nil
}

// RewriteChecksums writes the canonical checksum manifest for a matrix.
func RewriteChecksums(directory, version string) error {
	var output strings.Builder
	for _, name := range checksummedNames(version) {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&output, "%x  %s\n", sha256.Sum256(data), name)
	}
	return os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(output.String()), 0o600)
}

func checksummedNames(version string) []string {
	names := Names(version)
	return names[:len(names)-2]
}

func comparableNames(version string) []string {
	return append(checksummedNames(version), "checksums.txt.sig")
}
