package selfupdate

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// UpdateCandidate installs an explicitly supplied local archive. It never
// consults a release runner or HTTP client.
func (u Updater) UpdateCandidate(_ context.Context, currentVersion string, candidate CandidateArchive) (string, error) {
	if u.Channel == "" {
		u.Channel = ChannelPortable
	}
	if u.Channel != ChannelPortable {
		return "", fmt.Errorf("verified local candidate installation is available only for a portable installation; use the native package manager for %s", u.Channel)
	}
	archivePath, checksumPath := strings.TrimSpace(candidate.ArchivePath), strings.TrimSpace(candidate.ChecksumsPath)
	if archivePath == "" || checksumPath == "" {
		return "", fmt.Errorf("verified local candidate requires both archive and checksums paths")
	}
	archiveName := filepath.Base(archivePath)
	version, err := archiveVersion(archiveName, u.GOOS, u.GOARCH)
	if err != nil {
		return "", err
	}
	comparison, err := compareVersions("v"+version, currentVersion)
	if err != nil {
		return "", err
	}
	if comparison == 0 {
		return "verified local candidate already matches version v" + version, nil
	}
	if comparison < 0 {
		return "", fmt.Errorf("refusing to replace %s with older verified local candidate v%s", currentVersion, version)
	}
	if err := verifyChecksum(archivePath, checksumPath, archiveName); err != nil {
		return "", err
	}
	binaryName := "aigw"
	if u.GOOS == "windows" {
		binaryName = "aigw.exe"
	}
	binary, err := extractBinary(archivePath, expectedBinaryPath(version, u.GOOS, u.GOARCH, binaryName))
	if err != nil {
		return "", err
	}
	if u.GOOS == "windows" && runtime.GOOS == "windows" {
		return u.scheduleWindowsReplacement(binary, "v"+version)
	}
	if err := u.replacePortableBinary(binary); err != nil {
		return "", err
	}
	return "updated to v" + version + " from a verified local candidate", nil
}
