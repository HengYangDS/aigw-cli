// Package upgrade owns dual-Forge release resolution, verified artifact
// retrieval, channel-aware installation, and portable rollback.
package upgrade

import (
	"aigw-cli/internal/upgrade/artifact"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aigw-cli/internal/process"

	"github.com/rogpeppe/go-internal/robustio"
)

const releaseRequestTimeout = 30 * time.Second

// Match [net/http.Client]'s default when composing its unexported redirect policy.
const releaseRedirectLimit = 10

// Updater resolves, verifies, installs, and rolls back signed portable AIGW releases.
type Updater struct {
	GOOS       string
	GOARCH     string
	Executable string
	Runner     process.CaptureRunner
	HTTPClient *http.Client
	GitLab     ReleaseSource
	GitHub     ReleaseSource
}

func (u Updater) releaseHTTPClient() *http.Client {
	base := u.HTTPClient
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	if client.Timeout == 0 {
		client.Timeout = releaseRequestTimeout
	}
	defaultCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, previous []*http.Request) error {
		if defaultCheckRedirect != nil {
			if err := defaultCheckRedirect(request, previous); err != nil {
				return err
			}
		} else if len(previous) >= releaseRedirectLimit {
			return fmt.Errorf("stopped after %d redirects", releaseRedirectLimit)
		}
		for _, prior := range previous {
			if prior.URL.Scheme == "https" && request.URL.Scheme != "https" {
				return fmt.Errorf("refusing release update redirect from HTTPS to HTTP")
			}
			if request.URL.Scheme != prior.URL.Scheme || !strings.EqualFold(request.URL.Host, prior.URL.Host) {
				request.Header.Del("Authorization")
				request.Header.Del("Private-Token")
			}
		}
		return nil
	}
	return &client
}

type releaseHTTPError struct {
	provider   ReleaseProvider
	operation  string
	statusCode int
	status     string
}

func (e releaseHTTPError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.provider, e.operation, e.status)
}

func httpFailure(provider ReleaseProvider, operation string, response *http.Response) error {
	err := releaseHTTPError{
		provider:   provider,
		operation:  operation,
		statusCode: response.StatusCode,
		status:     response.Status,
	}
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
		return unavailable(err)
	}
	return err
}

func isHTTPStatus(err error, statusCode int) bool {
	var target releaseHTTPError
	return errors.As(err, &target) && target.statusCode == statusCode
}

type resolvedRelease struct {
	Source ReleaseSource
	Tag    string
}

type downloadedRelease struct {
	Source ReleaseSource
	Asset  string
	Digest string
}

// CandidateArchive is an explicit local-only update input. It has no provider
// configuration and cannot be promoted into a remote source implicitly.
type CandidateArchive struct {
	ArchivePath   string
	ChecksumsPath string
}

// Current constructs an Updater for the current executable and platform.
func Current(executable string) Updater {
	return Updater{
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Executable: executable,
		Runner:     process.Runner{},
		GitLab: ReleaseSource{
			Provider:   ReleaseProviderGitLab,
			Origin:     strings.TrimSpace(BuildGitLabReleaseOrigin),
			Repository: strings.TrimSpace(BuildGitLabReleaseRepository),
		},
		GitHub: ReleaseSource{
			Provider:   ReleaseProviderGitHub,
			Origin:     strings.TrimSpace(BuildGitHubReleaseOrigin),
			Repository: strings.TrimSpace(BuildGitHubReleaseRepository),
		},
	}
}

// Update installs a strictly newer verified release and returns its version.
func (u Updater) Update(ctx context.Context, currentVersion string) (string, error) {
	if u.Runner == nil {
		u.Runner = process.Runner{}
	}
	sources := u.forgeSources()
	if err := validateReleaseSources(sources...); err != nil {
		return "", err
	}
	resolved, err := u.resolvePeerReleases(ctx, sources...)
	if err != nil {
		return "", err
	}
	return u.updateFromResolvedPeers(ctx, resolved, currentVersion)
}

func (u Updater) resolvePeerReleases(ctx context.Context, sources ...ReleaseSource) ([]resolvedRelease, error) {
	resolved := make([]resolvedRelease, 0, len(sources))
	unavailableSources := make([]error, 0, len(sources))
	for _, source := range sources {
		if source.empty() {
			continue
		}
		tag, sourceUnavailable, err := u.latestTagFromSource(ctx, source)
		if err == nil {
			resolved = append(resolved, resolvedRelease{Source: source, Tag: tag})
			continue
		}
		if sourceUnavailable {
			unavailableSources = append(unavailableSources, fmt.Errorf("%s: %w", source.Provider, err))
			continue
		}
		return nil, fmt.Errorf("%s release metadata failed: %w", source.Provider, err)
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("no configured release source is reachable: %w", errors.Join(unavailableSources...))
	}
	for _, candidate := range resolved[1:] {
		if candidate.Tag != resolved[0].Tag {
			return nil, fmt.Errorf("configured release sources disagree on latest tag: %s=%s, %s=%s", resolved[0].Source.Provider, resolved[0].Tag, candidate.Source.Provider, candidate.Tag)
		}
	}
	return resolved, nil
}

func (u Updater) updateFromResolvedPeers(ctx context.Context, releases []resolvedRelease, currentVersion string) (result string, resultErr error) {
	selected := releases[0]
	comparison, err := compareVersions(selected.Tag, currentVersion)
	if err != nil {
		return "", err
	}
	if comparison == 0 {
		return "already running the latest version " + selected.Tag, nil
	}
	if comparison < 0 {
		return "", fmt.Errorf("refusing to replace %s with older release %s", currentVersion, selected.Tag)
	}
	asset := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).ArchiveName(normalizeVersion(selected.Tag))
	directory, err := os.MkdirTemp("", "aigw-update-")
	if err != nil {
		return "", fmt.Errorf("create update workspace: %w", err)
	}
	defer func() {
		if err := robustio.RemoveAll(directory); err != nil {
			cleanupErr := fmt.Errorf("remove update workspace %s: %w", directory, err)
			if resultErr == nil {
				cleanupErr = fmt.Errorf("program update completed; %w", cleanupErr)
			}
			resultErr = errors.Join(resultErr, cleanupErr)
		}
	}()
	downloads, err := u.downloadPeerAssets(ctx, releases, asset, directory)
	if err != nil {
		return "", err
	}
	for _, candidate := range downloads[1:] {
		if candidate.Digest != downloads[0].Digest {
			return "", fmt.Errorf("reachable release sources disagree on %s asset bytes: %s != %s", asset, downloads[0].Source.Provider, candidate.Source.Provider)
		}
	}
	archive := downloads[0].Asset
	checksums := filepath.Join(filepath.Dir(archive), "checksums.txt")
	if err := u.installPortableArchive(ctx, archive, checksums, normalizeVersion(selected.Tag)); err != nil {
		return "", err
	}
	return "updated to " + selected.Tag + " verified from " + releaseProviders(downloads), nil
}

func (u Updater) downloadPeerAssets(ctx context.Context, releases []resolvedRelease, asset, root string) ([]downloadedRelease, error) {
	downloads := make([]downloadedRelease, 0, len(releases))
	unavailableSources := make([]error, 0, len(releases))
	for _, release := range releases {
		directory, err := os.MkdirTemp(root, string(release.Source.Provider)+"-")
		if err != nil {
			return nil, fmt.Errorf("create peer download directory: %w", err)
		}
		unavailable, err := u.downloadReleaseAssetsFromExactSource(ctx, release.Source, release.Tag, directory, asset, "checksums.txt")
		if err != nil {
			if unavailable {
				unavailableSources = append(unavailableSources, fmt.Errorf("%s: %w", release.Source.Provider, err))
				continue
			}
			return nil, fmt.Errorf("%s release assets failed: %w", release.Source.Provider, err)
		}
		path := filepath.Join(directory, asset)
		digest, err := artifact.VerifyChecksum(path, filepath.Join(directory, "checksums.txt"), asset)
		if err != nil {
			return nil, fmt.Errorf("%s release checksum failed: %w", release.Source.Provider, err)
		}
		downloads = append(downloads, downloadedRelease{Source: release.Source, Asset: path, Digest: digest})
	}
	if len(downloads) == 0 {
		return nil, fmt.Errorf("all reachable release sources failed while downloading %s: %w", asset, errors.Join(unavailableSources...))
	}
	return downloads, nil
}

func releaseProviders(downloads []downloadedRelease) string {
	providers := make([]string, 0, len(downloads))
	for _, download := range downloads {
		providers = append(providers, string(download.Source.Provider))
	}
	return strings.Join(providers, " and ")
}

func (u Updater) captureReleaseCommand(ctx context.Context, plan process.Plan) ([]byte, error) {
	output, err := u.Runner.RunCapture(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", plan.Executable, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
