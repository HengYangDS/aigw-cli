package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.githubAPIURL(source, "releases?per_page=100"), nil)
	if err != nil {
		return "", fmt.Errorf("create GitHub prerelease metadata request: %w", err)
	}
	if err := u.authorizeGitHubRequest(request); err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := u.githubHTTPClient().Do(request)
	if err != nil {
		return "", unavailable(fmt.Errorf("query GitHub prerelease metadata: %w", err))
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
			return "", httpFailure(ReleaseProviderGitHub, "query prerelease metadata", response)
		}
		return "", latestErr
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&releases); err != nil {
		return "", fmt.Errorf("parse GitHub prerelease metadata: %w", err)
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
		if !isGitHubNotFound(prereleaseErr) {
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
	release, err := u.githubReleaseWithCLI(ctx, source, "releases/latest")
	if err == nil {
		if release.TagName == "" {
			return "", fmt.Errorf("no AIGW release is available")
		}
		return release.TagName, nil
	}
	output, listErr := u.runGitHubCLI(ctx, "api", "repos/"+source.Repository+"/releases?per_page=100")
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

func (u Updater) githubReleaseWithCLI(ctx context.Context, source ReleaseSource, path string) (githubRelease, error) {
	output, err := u.runGitHubCLI(ctx, "api", "repos/"+source.Repository+"/"+path)
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.githubAPIURL(source, path), nil)
	if err != nil {
		return githubRelease{}, fmt.Errorf("create GitHub release metadata request: %w", err)
	}
	if err := u.authorizeGitHubRequest(request); err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := u.githubHTTPClient().Do(request)
	if err != nil {
		return githubRelease{}, unavailable(fmt.Errorf("query GitHub release metadata: %w", err))
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return githubRelease{}, httpFailure(ReleaseProviderGitHub, "query release metadata", response)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("parse GitHub release metadata: %w", err)
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
		if err := u.downloadGitHubAsset(ctx, assetURL, filepath.Join(directory, asset)); err != nil {
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
		if _, err := u.runGitHubCLI(ctx, "release", "download", tag, "--repo", source.Repository, "--pattern", asset, "--dir", directory); err != nil {
			return fmt.Errorf("download GitHub release asset %s through gh: %w", asset, err)
		}
		info, err := os.Stat(filepath.Join(directory, asset))
		if err != nil || info.IsDir() || info.Size() == 0 {
			return fmt.Errorf("GitHub CLI did not write release asset %s", asset)
		}
	}
	return nil
}

func (u Updater) downloadGitHubAsset(ctx context.Context, rawURL, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create GitHub asset request: %w", err)
	}
	if err := u.authorizeGitHubRequest(request); err != nil {
		return err
	}
	response, err := u.githubHTTPClient().Do(request)
	if err != nil {
		return unavailable(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return httpFailure(ReleaseProviderGitHub, "download release asset", response)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create downloaded asset: %w", err)
	}
	_, copyErr := io.Copy(file, io.LimitReader(response.Body, 1<<30))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write downloaded asset: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close downloaded asset: %w", closeErr)
	}
	return nil
}

func isGitHubUnavailable(err error) bool { return isSourceUnavailable(err) }

func (u Updater) githubAPIURL(source ReleaseSource, path string) string {
	origin := strings.TrimRight(source.Origin, "/")
	if strings.EqualFold(origin, "https://github.com") {
		origin = "https://api.github.com"
	}
	return origin + "/repos/" + source.Repository + "/" + path
}

func (u Updater) githubHTTPClient() *http.Client {
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
		if len(previous) > 0 && !strings.EqualFold(request.URL.Host, previous[0].URL.Host) {
			request.Header.Del("Authorization")
			request.Header.Del("PRIVATE-TOKEN")
		}
		if len(previous) > 0 && previous[0].URL.Scheme == "https" && request.URL.Scheme != "https" {
			return fmt.Errorf("refusing GitHub update redirect from HTTPS to HTTP")
		}
		if defaultCheckRedirect != nil {
			return defaultCheckRedirect(request, previous)
		}
		return nil
	}
	return &client
}

func (u *Updater) authorizeGitHubRequest(request *http.Request) error {
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

func (u Updater) runGitHubCLI(ctx context.Context, args ...string) ([]byte, error) {
	return u.Runner.Run(ctx, "gh", args...)
}

func githubCLIFallbackAllowed(source ReleaseSource) bool {
	return strings.EqualFold(strings.TrimRight(source.Origin, "/"), "https://github.com")
}

func isGitHubNotFound(err error) bool {
	return isHTTPStatus(err, http.StatusNotFound)
}
