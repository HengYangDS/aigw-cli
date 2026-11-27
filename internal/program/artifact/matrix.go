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

// PublishedNames returns the matrix that a published release's own checksum manifest names: its
// checksummed files in the manifest's order, then the manifest and its signature. A predecessor is
// verified by it, because an earlier release may have published platforms that Names no longer lists.
// Every name must be a file of version, and the provenance must close the manifest, as
// RewriteChecksums writes it.
func PublishedNames(directory, version string) ([]string, error) {
	manifest, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		return nil, fmt.Errorf("published release matrix: %w", err)
	}
	if _, err := parseChecksums(manifest); err != nil {
		return nil, fmt.Errorf("published release matrix: %w", err)
	}
	var names []string
	for line := range strings.Lines(string(manifest)) {
		_, name, _ := checksumLine(line)
		if !FileOf(name, version) {
			return nil, fmt.Errorf("published release matrix: %s is not a file of release %s", name, version)
		}
		names = append(names, name)
	}
	if provenance := "aigw_" + version + ".provenance.json"; len(names) == 0 || names[len(names)-1] != provenance {
		return nil, fmt.Errorf("published release matrix: the manifest must end with %s", provenance)
	}
	return append(names, "checksums.txt", "checksums.txt.sig"), nil
}

// PublishedSubjects returns the provenance subjects of a published release: the names of its
// manifest without the provenance itself, in the manifest's order.
func PublishedSubjects(directory, version string) ([]string, error) {
	names, err := PublishedNames(directory, version)
	if err != nil {
		return nil, err
	}
	return names[:len(names)-3], nil
}

// FileOf reports whether name is a file of the release version: a plain file name that begins with
// aigw_<version> followed by "_" or ".".
func FileOf(name, version string) bool {
	prefix := "aigw_" + version
	return !strings.ContainsAny(name, `/\`) && (strings.HasPrefix(name, prefix+"_") || strings.HasPrefix(name, prefix+"."))
}

// ValidateMatrix verifies membership, non-empty files, checksums and the signature
// envelope. It does not decide whether the signer is authorized.
func ValidateMatrix(directory, version string) error {
	return validateMatrix(directory, Names(version))
}

// ValidatePublishedMatrix verifies a published release by the matrix that its own manifest names,
// with the same checks as ValidateMatrix.
func ValidatePublishedMatrix(directory, version string) error {
	wanted, err := PublishedNames(directory, version)
	if err != nil {
		return err
	}
	return validateMatrix(directory, wanted)
}

func validateMatrix(directory string, wanted []string) error {
	if _, err := verifiedDigests(directory, wanted); err != nil {
		return err
	}
	manifest := filepath.Join(directory, "checksums.txt")
	if err := verifyEnvelope(manifest, manifest+".sig"); err != nil {
		return fmt.Errorf("release artifact matrix: invalid detached signature: %w", err)
	}
	return nil
}

// verifiedDigests checks a directory against wanted, a matrix whose last two names are the manifest and
// its signature, and returns the digest of each wanted file.
func verifiedDigests(directory string, wanted []string) (map[string]string, error) {
	checksummed := wanted[:len(wanted)-2]
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
	for _, name := range checksummed {
		if digests[name] != localDigests[name] {
			return nil, fmt.Errorf("release artifact matrix: checksum mismatch for %s", name)
		}
	}
	if len(digests) != len(checksummed) {
		return nil, errors.New("release artifact matrix: checksum manifest has unexpected entries")
	}
	return localDigests, nil
}

// CompareMatrices verifies two complete matrices and compares their content.
func CompareMatrices(left, right, version string) error {
	leftDigests, err := verifiedDigests(left, Names(version))
	if err != nil {
		return err
	}
	rightDigests, err := verifiedDigests(right, Names(version))
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
