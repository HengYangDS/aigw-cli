package artifact

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Archives returns the portable product archives for version, one for each target.
func Archives(version string) []string {
	archives := make([]string, 0, len(targets))
	for _, target := range targets {
		archives = append(archives, target.ArchiveName(version))
	}
	return archives
}

// Names returns the complete release artifact matrix for version.
func Names(version string) []string {
	return append(ProvenanceSubjects(version),
		"aigw_"+version+".provenance.json",
		"checksums.txt",
		"checksums.txt.sig",
	)
}

// ProvenanceSubjects returns the artifacts that the release provenance names as its subjects.
func ProvenanceSubjects(version string) []string {
	return append(Archives(version),
		"aigw_"+version+".spdx.json",
		"aigw_"+version+".vulnerabilities.json",
		"aigw_"+version+".licenses.json",
	)
}

// ValidateMatrix verifies membership, non-empty files, checksums and the signature
// envelope. It does not decide whether the signer is authorized.
func ValidateMatrix(directory, version string) error {
	if _, err := verifiedDigests(directory, version); err != nil {
		return err
	}
	manifest := filepath.Join(directory, "checksums.txt")
	if err := verifyEnvelope(manifest, manifest+".sig"); err != nil {
		return fmt.Errorf("release artifact matrix: invalid detached signature: %w", err)
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
	localDigests := make(map[string]string, len(wanted))
	var manifest []byte
	for _, name := range wanted {
		data, readErr := os.ReadFile(filepath.Join(directory, name))
		if readErr != nil || len(data) == 0 {
			return nil, fmt.Errorf("release artifact matrix: missing or empty artifact: %s", name)
		}
		localDigests[name] = fmt.Sprintf("%x", sha256.Sum256(data))
		if name == "checksums.txt" {
			manifest = data
		}
	}
	if !slices.Equal(actual, expected) {
		return nil, fmt.Errorf("release artifact matrix: unexpected or missing files: %v", actual)
	}
	digests, err := parseChecksums(manifest)
	if err != nil {
		return nil, fmt.Errorf("release artifact matrix: %w", err)
	}
	for _, name := range checksummedNames(version) {
		if digests[name] != localDigests[name] {
			return nil, fmt.Errorf("release artifact matrix: checksum mismatch for %s", name)
		}
	}
	if len(digests) != len(checksummedNames(version)) {
		return nil, errors.New("release artifact matrix: checksum manifest has unexpected entries")
	}
	return localDigests, nil
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
	for _, name := range Names(version) {
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
