package program

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/program/artifact"
)

// TransitionAvailable is the state of an update preview that found a newer
// release, which Update or UpdateCandidate would install.
const TransitionAvailable = "available"

// Preview reports what Update would do from public release metadata alone:
// it sends no token, runs no other program, downloads nothing and writes
// nothing. The newest release that every reachable source names is compared
// with the current version.
func (u Updater) Preview(ctx context.Context, currentVersion string) (Transition, error) {
	if err := RequirePortableOwnership(u.Executable); err != nil {
		return Transition{}, err
	}
	sources := u.forgeSources()
	if err := validateReleaseSources(sources...); err != nil {
		return Transition{}, err
	}
	resolved := make([]resolvedRelease, 0, len(sources))
	unreadable := make([]error, 0, len(sources))
	for _, source := range sources {
		if source.empty() {
			continue
		}
		tag, err := u.publicLatestTag(ctx, source)
		switch {
		case err == nil:
			resolved = append(resolved, resolvedRelease{Source: source, Tag: tag})
		case isSourceUnavailable(err) || isHTTPStatus(err, http.StatusUnauthorized) || isHTTPStatus(err, http.StatusForbidden) || isHTTPStatus(err, http.StatusNotFound):
			unreadable = append(unreadable, fmt.Errorf("%s: %w", source.Provider, err))
		default:
			return Transition{}, fmt.Errorf("%s public release metadata failed: %w", source.Provider, err)
		}
	}
	if len(resolved) == 0 {
		return Transition{}, fmt.Errorf("no configured release source publishes its metadata without authentication: %w", errors.Join(unreadable...))
	}
	for _, candidate := range resolved[1:] {
		if candidate.Tag != resolved[0].Tag {
			return Transition{}, fmt.Errorf("configured release sources disagree on latest tag: %s=%s, %s=%s", resolved[0].Source.Provider, resolved[0].Tag, candidate.Source.Provider, candidate.Tag)
		}
	}
	return u.previewReplacement(previewVersion(resolved[0].Tag, currentVersion, "release"))
}

// PreviewCandidate reports what UpdateCandidate would do from the archive's
// name alone: it neither reads, verifies nor runs the archive. An archive at
// the current version reports the current program, because only the update
// compares their bytes.
func (u Updater) PreviewCandidate(currentVersion string, candidate CandidateArchive) (Transition, error) {
	if err := RequirePortableOwnership(u.Executable); err != nil {
		return Transition{}, err
	}
	archivePath, checksumsPath := strings.TrimSpace(candidate.ArchivePath), strings.TrimSpace(candidate.ChecksumsPath)
	if archivePath == "" || checksumsPath == "" {
		return Transition{}, fmt.Errorf("verified local candidate requires both archive and checksums paths")
	}
	for _, path := range []string{archivePath, checksumsPath} {
		info, err := os.Stat(path)
		if err != nil {
			return Transition{}, fmt.Errorf("inspect verified local candidate: %w", err)
		}
		if !info.Mode().IsRegular() {
			return Transition{}, fmt.Errorf("verified local candidate %s is not a regular file", path)
		}
	}
	version, err := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).Version(filepath.Base(archivePath))
	if err != nil {
		return Transition{}, err
	}
	return u.previewReplacement(previewVersion("v"+version, currentVersion, "verified local candidate"))
}

// previewReplacement completes a preview as the update would proceed: an
// update keeps the replaced program at the rollback copy's path, so a
// directory there refuses the preview as it refuses the update.
func (u Updater) previewReplacement(result Transition, err error) (Transition, error) {
	if err != nil || result.State != TransitionAvailable {
		return result, err
	}
	if err := refuseRollbackDirectory(RollbackPath(u.Executable)); err != nil {
		return Transition{}, err
	}
	return result, nil
}

// previewVersion compares a release tag with the current version as an
// update would: an older release is refused.
func previewVersion(tag, currentVersion, source string) (Transition, error) {
	comparison, err := compareVersions(tag, currentVersion)
	if err != nil {
		return Transition{}, err
	}
	version := normalizeVersion(tag)
	switch {
	case comparison == 0:
		return Transition{State: TransitionCurrent, Version: version, Summary: "v" + version + " is already the current version"}, nil
	case comparison < 0:
		return Transition{}, fmt.Errorf("refusing to replace %s with older %s v%s", currentVersion, source, version)
	default:
		return Transition{State: TransitionAvailable, Version: version, Summary: "would update to " + source + " v" + version}, nil
	}
}

// publicLatestTag reads a source's latest release tag without authentication.
// A GitHub repository without a stable release offers its newest published
// prerelease, as Update does.
func (u Updater) publicLatestTag(ctx context.Context, source ReleaseSource) (string, error) {
	switch source.Provider {
	case ReleaseProviderGitHub:
		var release githubRelease
		err := u.publicReleaseMetadata(ctx, source.Provider, u.githubAPIURL(source, "releases/latest"), &release)
		if isHTTPStatus(err, http.StatusNotFound) {
			var releases []githubRelease
			if listErr := u.publicReleaseMetadata(ctx, source.Provider, u.githubAPIURL(source, "releases?per_page=100"), &releases); listErr != nil {
				return "", listErr
			}
			if tag := latestPublishedGitHubPrerelease(releases); tag != "" {
				return tag, nil
			}
			return "", err
		}
		if err != nil {
			return "", err
		}
		if release.TagName == "" {
			return "", fmt.Errorf("no AIGW release is available")
		}
		return release.TagName, nil
	case ReleaseProviderGitLab:
		gitlab := u
		gitlab.GitLab = source
		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := u.publicReleaseMetadata(ctx, source.Provider, gitlab.gitLabAPIURL("releases/permalink/latest"), &release); err != nil {
			return "", err
		}
		if release.TagName == "" {
			return "", fmt.Errorf("no AIGW release is available")
		}
		return release.TagName, nil
	default:
		return "", fmt.Errorf("unsupported release provider %q", source.Provider)
	}
}

// publicReleaseMetadata decodes one release metadata document that a source
// serves without authentication.
func (u Updater) publicReleaseMetadata(ctx context.Context, provider ReleaseProvider, address string, document any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return fmt.Errorf("create public release metadata request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := u.releaseHTTPClient().Do(request)
	if err != nil {
		return unavailable(fmt.Errorf("query public release metadata: %w", err))
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return httpFailure(provider, "query public release metadata", response)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(document); err != nil {
		return fmt.Errorf("parse public release metadata: %w", err)
	}
	return nil
}
