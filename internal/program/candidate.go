package program

import (
	"aigw-cli/internal/program/artifact"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrCandidateIdentity means an equal-version candidate differs from the installed program.
var ErrCandidateIdentity = errors.New("equal-version candidate has different program bytes")

// UpdateCandidate installs an explicitly supplied local archive. It never
// consults a release source or HTTP client.
func (u Updater) UpdateCandidate(ctx context.Context, currentVersion string, candidate CandidateArchive) (Transition, error) {
	if err := RequirePortableOwnership(u.Executable); err != nil {
		return Transition{}, err
	}
	if err := ctx.Err(); err != nil {
		return Transition{}, err
	}
	archivePath, checksumPath := strings.TrimSpace(candidate.ArchivePath), strings.TrimSpace(candidate.ChecksumsPath)
	if archivePath == "" || checksumPath == "" {
		return Transition{}, fmt.Errorf("verified local candidate requires both archive and checksums paths")
	}
	archiveName := filepath.Base(archivePath)
	version, err := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).Version(archiveName)
	if err != nil {
		return Transition{}, err
	}
	comparison, err := compareVersions("v"+version, currentVersion)
	if err != nil {
		return Transition{}, err
	}
	if err := ctx.Err(); err != nil {
		return Transition{}, err
	}
	if comparison == 0 {
		program, err := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).ReadProgram(archivePath, checksumPath, version)
		if err != nil {
			return Transition{}, err
		}
		current, err := os.ReadFile(u.Executable)
		if err != nil {
			return Transition{}, fmt.Errorf("read current AIGW executable: %w", err)
		}
		if !bytes.Equal(program, current) {
			return Transition{}, fmt.Errorf("candidate version v%s: %w", version, ErrCandidateIdentity)
		}
		return Transition{State: TransitionCurrent, Version: version,
			Summary: "verified local candidate already matches the current program at version v" + version}, nil
	}
	if comparison < 0 {
		return Transition{}, fmt.Errorf("refusing to replace %s with older verified local candidate v%s", currentVersion, version)
	}
	if err := refuseRollbackDirectory(RollbackPath(u.Executable)); err != nil {
		return Transition{}, err
	}
	activated, err := u.installPortableArchive(ctx, archivePath, checksumPath, version)
	if !activated {
		return Transition{}, err
	}
	return Transition{State: TransitionUpdated, Version: version, Summary: "updated to v" + version + " from a verified local candidate"}, err
}
