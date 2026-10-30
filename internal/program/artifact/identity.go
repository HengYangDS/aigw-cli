package artifact

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Target identifies the operating system and architecture of a portable archive.
type Target struct {
	OS   string
	Arch string
}

// targets is the release's platform matrix, in the order of its archives.
var targets = []Target{
	{OS: "darwin", Arch: "amd64"},
	{OS: "darwin", Arch: "arm64"},
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "windows", Arch: "amd64"},
	{OS: "windows", Arch: "arm64"},
}

// Targets returns the platforms that every release archives, in the matrix's order.
func Targets() []Target {
	return slices.Clone(targets)
}

// ArchiveName returns the release asset name for this target and version.
func (target Target) ArchiveName(version string) string {
	extension := ".tar.gz"
	if target.OS == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("aigw_%s_%s_%s%s", version, target.OS, target.Arch, extension)
}

// Version validates the target and returns the strict semantic version in an asset name.
func (target Target) Version(name string) (string, error) {
	const prefix = "aigw_"
	suffix := strings.TrimPrefix(target.ArchiveName(""), prefix)
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return "", fmt.Errorf("verified local candidate archive must target %s/%s", target.OS, target.Arch)
	}
	version := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	if _, err := semver.StrictNewVersion(version); err != nil {
		return "", fmt.Errorf("invalid release version %q: %w", version, err)
	}
	return version, nil
}

func (target Target) programPath(version string) string {
	binary := "aigw"
	if target.OS == "windows" {
		binary += ".exe"
	}
	return fmt.Sprintf("aigw_%s_%s_%s/%s", version, target.OS, target.Arch, binary)
}

// ReadProgram verifies an archive and reads its unique platform-specific executable.
func (target Target) ReadProgram(archivePath, checksumsPath, version string) ([]byte, error) {
	if _, err := VerifyChecksum(archivePath, checksumsPath, filepath.Base(archivePath)); err != nil {
		return nil, err
	}
	return extractBinary(archivePath, target.programPath(version))
}
