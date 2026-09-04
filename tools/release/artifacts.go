package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func artifactNames(version string) []string {
	return []string{
		"aigw_" + version + "_darwin_amd64.tar.gz",
		"aigw_" + version + "_darwin_arm64.tar.gz",
		"aigw_" + version + "_linux_amd64.tar.gz",
		"aigw_" + version + "_linux_arm64.tar.gz",
		"aigw_" + version + "_windows_amd64.zip",
		"aigw_" + version + "_windows_arm64.zip",
		"aigw_" + version + ".spdx.json",
		"checksums.txt",
	}
}

func validateArtifactMatrix(directory, version string) error {
	_, err := verifiedArtifactDigests(directory, version)
	return err
}

func verifiedArtifactDigests(directory, version string) (map[string]string, error) {
	wanted := artifactNames(version)
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
	for _, name := range wanted {
		information, statErr := os.Stat(filepath.Join(directory, name))
		if statErr != nil || information.Size() == 0 {
			return nil, fmt.Errorf("release artifact matrix: missing or empty artifact: %s", name)
		}
	}
	if !slices.Equal(actual, expected) {
		return nil, fmt.Errorf("release artifact matrix: unexpected or missing files: %v", actual)
	}
	manifest, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		return nil, err
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
	for _, name := range wanted[:len(wanted)-1] {
		data, readErr := os.ReadFile(filepath.Join(directory, name))
		if readErr != nil {
			return nil, readErr
		}
		actualDigest := fmt.Sprintf("%x", sha256.Sum256(data))
		if digests[name] != actualDigest {
			return nil, fmt.Errorf("release artifact matrix: checksum mismatch for %s", name)
		}
	}
	if len(digests) != len(wanted)-1 {
		return nil, errors.New("release artifact matrix: checksum manifest has unexpected entries")
	}
	return digests, nil
}

func compareArtifactMatrices(left, right, version string) error {
	leftDigests, err := verifiedArtifactDigests(left, version)
	if err != nil {
		return err
	}
	rightDigests, err := verifiedArtifactDigests(right, version)
	if err != nil {
		return err
	}
	for _, name := range artifactNames(version)[:len(artifactNames(version))-1] {
		if leftDigests[name] != rightDigests[name] {
			return fmt.Errorf("release artifact differs across forge stages: %s", name)
		}
	}
	return nil
}

func rewriteChecksums(directory, version string) error {
	var output strings.Builder
	for _, name := range artifactNames(version)[:len(artifactNames(version))-1] {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&output, "%x  %s\n", sha256.Sum256(data), name)
	}
	return os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(output.String()), 0o600)
}
