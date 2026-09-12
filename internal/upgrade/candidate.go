package upgrade

import (
	"aigw-cli/internal/upgrade/artifact"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateCandidate installs an explicitly supplied local archive. It never
// consults a release source or HTTP client.
func (u Updater) UpdateCandidate(ctx context.Context, currentVersion string, candidate CandidateArchive) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	archivePath, checksumPath := strings.TrimSpace(candidate.ArchivePath), strings.TrimSpace(candidate.ChecksumsPath)
	if archivePath == "" || checksumPath == "" {
		return "", fmt.Errorf("verified local candidate requires both archive and checksums paths")
	}
	archiveName := filepath.Base(archivePath)
	version, err := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).Version(archiveName)
	if err != nil {
		return "", err
	}
	comparison, err := compareVersions("v"+version, currentVersion)
	if err != nil {
		return "", err
	}
	if comparison == 0 {
		program, err := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).ReadProgram(archivePath, checksumPath, version)
		if err != nil {
			return "", err
		}
		current, err := os.ReadFile(u.Executable)
		if err != nil {
			return "", fmt.Errorf("read current AIGW executable: %w", err)
		}
		if !bytes.Equal(program, current) {
			return "", fmt.Errorf("candidate version v%s has different program bytes from the current executable", version)
		}
		return "verified local candidate already matches the current program at version v" + version, ctx.Err()
	}
	if comparison < 0 {
		return "", fmt.Errorf("refusing to replace %s with older verified local candidate v%s", currentVersion, version)
	}
	if err := u.installPortableArchive(ctx, archivePath, checksumPath, version); err != nil {
		return "", err
	}
	return "updated to v" + version + " from a verified local candidate", nil
}
