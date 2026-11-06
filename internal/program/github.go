package program

import (
	"aigw-cli/internal/platform/process"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (u Updater) latestPrereleaseTagFromGitHub(ctx context.Context, source ReleaseSource, latestErr error) (string, error) {
	if !isHTTPStatus(latestErr, http.StatusNotFound) {
		return "", latestErr
	}
	var releases []githubRelease
	err := u.releaseMetadata(ctx, releaseRequest{
		provider: ReleaseProviderGitHub, address: u.githubAPIURL(source, "releases?per_page=100"),
		accept: "application/vnd.github+json", authorize: u.authorizeGitHubRequest,
		create: "create GitHub prerelease metadata request", transfer: "query GitHub prerelease metadata", status: "query prerelease metadata",
	}, 4<<20, "parse GitHub prerelease metadata", &releases)
	// A status that leaves the source available, unlike a rate limit or a
	// server failure, keeps the original 404.
	if _, answered := errors.AsType[releaseHTTPError](err); answered && !isSourceUnavailable(err) {
		return "", latestErr
	}
	if err != nil {
		return "", err
	}
	latest := latestPublishedGitHubPrerelease(releases)
	if latest == "" {
		return "", latestErr
	}
	return latest, nil
}

func latestPublishedGitHubPrerelease(releases []githubRelease) string {
	latest := ""
	for _, release := range releases {
		if release.Draft || !release.Prerelease || strings.TrimSpace(release.PublishedAt) == "" {
			continue
		}
		if _, err := parseVersion(release.TagName); err != nil {
			continue
		}
		if latest == "" {
			latest = release.TagName
			continue
		}
		comparison, err := compareVersions(release.TagName, latest)
		if err == nil && comparison > 0 {
			latest = release.TagName
		}
	}
	return latest
}

// latestTagFromGitHubRelease resolves the normal GitHub latest-release route
// first, then falls back to the published prerelease list only when GitHub has
// no stable release. Source builds and tests use this through the same update
// path as released binaries.
func (u Updater) latestTagFromGitHubRelease(ctx context.Context, source ReleaseSource) (string, error) {
	release, err := u.githubRelease(ctx, source, "releases/latest")
	if err != nil {
		tag, prereleaseErr := u.latestPrereleaseTagFromGitHub(ctx, source, err)
		if prereleaseErr == nil {
			return tag, nil
		}
		if !isHTTPStatus(prereleaseErr, http.StatusNotFound) {
			return "", prereleaseErr
		}
		if tag, cliErr := u.latestTagFromGitHubCLI(ctx, source); cliErr == nil {
			return tag, nil
		}
		return "", prereleaseErr
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no AIGW release is available")
	}
	return release.TagName, nil
}

// latestTagFromGitHubCLI uses gh's existing, OS-managed authentication only
// after the official GitHub API returned the deliberate private-repository 404
// shape. It never reads, exports, or persists gh credentials, and it is never
// used for a custom GitHub-compatible endpoint.
func (u Updater) latestTagFromGitHubCLI(ctx context.Context, source ReleaseSource) (string, error) {
	if !githubCLIFallbackAllowed(source) {
		return "", fmt.Errorf("GitHub CLI fallback is unavailable for this release source")
	}
	release, err := u.latestGitHubReleaseWithCLI(ctx, source)
	if err == nil {
		if release.TagName == "" {
			return "", fmt.Errorf("no AIGW release is available")
		}
		return release.TagName, nil
	}
	output, listErr := u.runGitHubCLI(ctx, source, "api", "repos/"+source.Repository+"/releases?per_page=100")
	if listErr != nil {
		return "", fmt.Errorf("query GitHub release metadata through gh: %w", listErr)
	}
	var releases []githubRelease
	if err := json.Unmarshal(output, &releases); err != nil {
		return "", fmt.Errorf("parse GitHub release metadata through gh: %w", err)
	}
	if tag := latestPublishedGitHubPrerelease(releases); tag != "" {
		return tag, nil
	}
	return "", err
}

func (u Updater) latestGitHubReleaseWithCLI(ctx context.Context, source ReleaseSource) (githubRelease, error) {
	output, err := u.runGitHubCLI(ctx, source, "api", "repos/"+source.Repository+"/releases/latest")
	if err != nil {
		return githubRelease{}, err
	}
	var release githubRelease
	if err := json.Unmarshal(output, &release); err != nil {
		return githubRelease{}, fmt.Errorf("parse GitHub release metadata through gh: %w", err)
	}
	return release, nil
}

func (u Updater) githubRelease(ctx context.Context, source ReleaseSource, path string) (githubRelease, error) {
	var release githubRelease
	if err := u.releaseMetadata(ctx, releaseRequest{
		provider: ReleaseProviderGitHub, address: u.githubAPIURL(source, path),
		accept: "application/vnd.github+json", authorize: u.authorizeGitHubRequest,
		create: "create GitHub release metadata request", transfer: "query GitHub release metadata", status: "query release metadata",
	}, 4<<20, "parse GitHub release metadata", &release); err != nil {
		return githubRelease{}, err
	}
	return release, nil
}

func (u Updater) downloadGitHubReleaseAssets(ctx context.Context, release githubRelease, directory string, assets ...string) error {
	urls := make(map[string]string, len(release.Assets))
	for _, asset := range release.Assets {
		if filepath.Base(asset.Name) == asset.Name && asset.BrowserDownloadURL != "" {
			urls[asset.Name] = asset.BrowserDownloadURL
		}
	}
	for _, asset := range assets {
		if filepath.Base(asset) != asset {
			return fmt.Errorf("invalid release asset name %q", asset)
		}
		assetURL := urls[asset]
		if assetURL == "" {
			return fmt.Errorf("GitHub release metadata does not include %s", asset)
		}
		request := releaseRequest{
			provider: ReleaseProviderGitHub, address: assetURL, authorize: u.authorizeGitHubRequest,
			create: "create GitHub asset request", status: "download release asset",
		}
		if err := u.downloadAsset(ctx, request, filepath.Join(directory, asset), "asset"); err != nil {
			return fmt.Errorf("download GitHub release asset %s: %w", asset, err)
		}
	}
	return nil
}

func (u Updater) downloadReleaseAssetsFromGitHub(ctx context.Context, source ReleaseSource, tag, directory string, assets ...string) error {
	release, err := u.githubRelease(ctx, source, "releases/tags/"+url.PathEscape(tag))
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) && githubCLIFallbackAllowed(source) {
			return u.downloadGitHubReleaseAssetsWithCLI(ctx, source, tag, directory, assets...)
		}
		return err
	}
	if release.TagName != tag {
		return fmt.Errorf("GitHub release metadata tag %q does not match requested tag %q", release.TagName, tag)
	}
	return u.downloadGitHubReleaseAssets(ctx, release, directory, assets...)
}

func (u Updater) downloadGitHubReleaseAssetsWithCLI(ctx context.Context, source ReleaseSource, tag, directory string, assets ...string) error {
	if !githubCLIFallbackAllowed(source) {
		return fmt.Errorf("GitHub CLI fallback is unavailable for this release source")
	}
	for _, asset := range assets {
		if filepath.Base(asset) != asset {
			return fmt.Errorf("invalid release asset name %q", asset)
		}
		if _, err := u.runGitHubCLI(ctx, source, "release", "download", tag, "--repo", source.Repository, "--pattern", asset, "--dir", directory); err != nil {
			return fmt.Errorf("download GitHub release asset %s through gh: %w", asset, err)
		}
		info, err := os.Stat(filepath.Join(directory, asset))
		if err != nil || info.IsDir() || info.Size() == 0 {
			return fmt.Errorf("GitHub CLI did not write release asset %s", asset)
		}
	}
	return nil
}

func (u Updater) githubAPIURL(source ReleaseSource, path string) string {
	origin := strings.TrimRight(source.Origin, "/")
	if strings.EqualFold(origin, "https://github.com") {
		origin = "https://api.github.com"
	}
	return origin + "/repos/" + source.Repository + "/" + path
}

func (u Updater) authorizeGitHubRequest(request *http.Request) error {
	for _, name := range []string{"AIGW_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		token := os.Getenv(name)
		if token == "" {
			continue
		}
		if strings.ContainsAny(token, "\r\n") {
			return fmt.Errorf("GitHub token contains a control character")
		}
		request.Header.Set("Authorization", "Bearer "+token)
		break
	}
	return nil
}

func (u Updater) runGitHubCLI(ctx context.Context, source ReleaseSource, args ...string) ([]byte, error) {
	_, host, _ := strings.Cut(strings.TrimRight(source.Origin, "/"), "://")
	return u.captureReleaseCommand(ctx, process.Plan{Executable: "gh", Args: args, Env: append(os.Environ(), "GH_PROMPT_DISABLED=1", "GH_HOST="+host)})
}

func githubCLIFallbackAllowed(source ReleaseSource) bool {
	return strings.EqualFold(strings.TrimRight(source.Origin, "/"), "https://github.com")
}
