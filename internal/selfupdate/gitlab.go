package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (u Updater) downloadReleaseAssetsFromExactSource(ctx context.Context, source ReleaseSource, tag, directory string, assets ...string) (bool, error) {
	switch source.Provider {
	case ReleaseProviderGitLab:
		original := u.GitLab
		u.GitLab = source
		err := u.downloadReleaseAssets(ctx, tag, directory, assets...)
		u.GitLab = original
		return isGlabUnavailable(err), err
	case ReleaseProviderGitHub:
		err := u.downloadReleaseAssetsFromGitHub(ctx, source, tag, directory, assets...)
		return isGitHubUnavailable(err), err
	default:
		return false, fmt.Errorf("unsupported release provider %q", source.Provider)
	}
}

func (u Updater) downloadReleaseAssets(ctx context.Context, tag, directory string, assets ...string) error {
	for index, asset := range assets {
		path := filepath.Join(directory, asset)
		_, err := u.runGlab(ctx, "release", "download", tag, "-R", u.releaseProject(), "--asset-name", asset, "--dir", directory)
		if err == nil {
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() && info.Size() > 0 {
				continue
			}
			err = fmt.Errorf("glab reported success but did not write %s", asset)
		}
		if apiErr := u.downloadReleaseAssetsWithGlabAPI(ctx, tag, directory, assets[index:]...); apiErr == nil {
			return nil
		} else if !isGlabUnavailable(err) {
			if tokenErr := u.validateTokenFallbackHost(); tokenErr != nil {
				return fmt.Errorf("download release asset %s: %w; authenticated glab fallback failed: %v", asset, err, apiErr)
			}
		} else if tokenErr := u.validateTokenFallbackHost(); tokenErr != nil {
			return fmt.Errorf("%w; authenticated glab fallback failed: %v", err, apiErr)
		}
		for _, remaining := range assets[index:] {
			if err := u.downloadReleaseAssetFromGitLabAPI(ctx, tag, remaining, directory); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

func (u Updater) downloadReleaseAssetsWithGlabAPI(ctx context.Context, tag, directory string, assets ...string) error {
	if _, ok := u.Runner.(FileRunner); !ok {
		if _, ok := u.Runner.(EnvironmentFileRunner); !ok {
			return fmt.Errorf("authenticated glab asset download is unavailable")
		}
	}
	if len(assets) == 0 {
		return fmt.Errorf("authenticated glab asset download is unavailable")
	}
	output, err := u.runGlab(ctx, "api", "projects/"+u.releaseProjectPath()+"/releases/"+url.PathEscape(tag))
	if err != nil {
		return fmt.Errorf("query release metadata through glab: %w", err)
	}
	var release struct {
		Assets struct {
			Links []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"links"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(output, &release); err != nil {
		return fmt.Errorf("parse release metadata through glab: %w", err)
	}
	urls := make(map[string]string, len(release.Assets.Links))
	for _, link := range release.Assets.Links {
		if link.URL == "" {
			continue
		}
		if assetName := releaseAssetName(link.URL); assetName != "" {
			urls[assetName] = link.URL
		}
		if filepath.Base(link.Name) == link.Name {
			urls[link.Name] = link.URL
		}
	}
	for _, asset := range assets {
		if filepath.Base(asset) != asset {
			return fmt.Errorf("invalid release asset name %q", asset)
		}
		assetURL := urls[asset]
		if assetURL == "" {
			return fmt.Errorf("release metadata does not include %s", asset)
		}
		path := filepath.Join(directory, asset)
		if err := u.runGlabToFile(ctx, path, "api", assetURL); err != nil {
			return fmt.Errorf("download release asset %s through glab: %w", asset, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() == 0 {
			_ = os.Remove(path)
			if err != nil {
				return fmt.Errorf("inspect downloaded release asset %s: %w", asset, err)
			}
			return fmt.Errorf("glab API did not write %s", asset)
		}
	}
	return nil
}

func releaseAssetName(assetURL string) string {
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return ""
	}
	name := filepath.Base(parsed.Path)
	if name == "." || name == "/" || name == "" || filepath.Base(name) != name {
		return ""
	}
	return name
}

func (u Updater) downloadReleaseAssetFromGitLabAPI(ctx context.Context, tag, asset, directory string) error {
	if filepath.Base(asset) != asset {
		return fmt.Errorf("invalid release asset name %q", asset)
	}
	token, err := gitLabToken()
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.gitLabReleaseDownloadURL(tag, asset), nil)
	if err != nil {
		return fmt.Errorf("create GitLab release-download request: %w", err)
	}
	request.Header.Set("PRIVATE-TOKEN", token)
	response, err := u.gitLabHTTPClient().Do(request)
	if err != nil {
		return fmt.Errorf("download release asset %s: %w", asset, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download release asset %s: %s", asset, response.Status)
	}
	path := filepath.Join(directory, asset)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create downloaded release asset %s: %w", asset, err)
	}
	_, copyErr := io.Copy(file, io.LimitReader(response.Body, 1<<30))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write downloaded release asset %s: %w", asset, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close downloaded release asset %s: %w", asset, closeErr)
	}
	return nil
}

func (u Updater) latestTagFromSource(ctx context.Context, source ReleaseSource) (string, bool, error) {
	switch source.Provider {
	case ReleaseProviderGitLab:
		original := u.GitLab
		u.GitLab = source
		tag, err := u.latestTag(ctx)
		u.GitLab = original
		return tag, isGlabUnavailable(err), err
	case ReleaseProviderGitHub:
		tag, err := u.latestTagFromGitHubRelease(ctx, source)
		if err != nil {
			return "", isGitHubUnavailable(err), err
		}
		return tag, false, nil
	default:
		return "", false, fmt.Errorf("unsupported release provider %q", source.Provider)
	}
}

func (u Updater) latestTag(ctx context.Context) (string, error) {
	output, err := u.runGlab(ctx, "release", "list", "-R", u.releaseProject(), "--per-page", "1", "-F", "json", "--jq", ".[0].tag_name")
	if err != nil {
		if !isGlabUnavailable(err) {
			return "", fmt.Errorf("query latest release: %w", err)
		}
		if tokenErr := u.validateTokenFallbackHost(); tokenErr != nil {
			return "", tokenErr
		}
		tag, apiErr := u.latestTagFromGitLabAPI(ctx)
		if apiErr != nil && isSourceUnavailable(apiErr) {
			return "", unavailable(fmt.Errorf("GitLab release lookup failed through glab and API: %v; %w", err, apiErr))
		}
		return tag, apiErr
	}
	tag, err := releaseTagFromCLIOutput(output)
	if err != nil {
		return "", err
	}
	if tag == "" {
		return "", fmt.Errorf("no AIGW release is available")
	}
	return tag, nil
}

// releaseTagFromCLIOutput keeps the update contract independent of incidental
// client diagnostics. glab prints any configuration-location warnings before
// its --jq value, so the final non-empty line remains the sole authoritative
// tag. A malformed final value must still fail closed rather than being
// mistaken for an absent release.
func releaseTagFromCLIOutput(output []byte) (string, error) {
	tag := ""
	for _, line := range strings.FieldsFunc(string(output), func(r rune) bool { return r == '\n' || r == '\r' }) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		tag = strings.TrimSpace(line)
	}
	if tag == "" {
		return "", nil
	}
	if _, err := parseVersion(tag); err != nil {
		return "", err
	}
	return tag, nil
}

type sourceUnavailableError struct{ err error }

func (e sourceUnavailableError) Error() string { return e.err.Error() }

func (e sourceUnavailableError) Unwrap() error { return e.err }

func unavailable(err error) error {
	if err == nil {
		return nil
	}
	return sourceUnavailableError{err: err}
}

func isSourceUnavailable(err error) bool {
	var target sourceUnavailableError
	return errors.As(err, &target)
}

func isGlabUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if isSourceUnavailable(err) || errors.Is(err, exec.ErrNotFound) {
		return true
	}
	return strings.Contains(err.Error(), "authenticated glab") ||
		strings.Contains(err.Error(), "latest private release requires authenticated glab")
}

func (u Updater) latestTagFromGitLabAPI(ctx context.Context) (string, error) {
	token, err := gitLabToken()
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.gitLabAPIURL("releases/permalink/latest"), nil)
	if err != nil {
		return "", fmt.Errorf("create GitLab latest-release request: %w", err)
	}
	request.Header.Set("PRIVATE-TOKEN", token)
	response, err := u.gitLabHTTPClient().Do(request)
	if err != nil {
		return "", unavailable(fmt.Errorf("query GitLab latest release: %w", err))
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
		return "", unavailable(fmt.Errorf("query GitLab latest release: %s", response.Status))
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("query GitLab latest release: %s", response.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&release); err != nil {
		return "", fmt.Errorf("parse GitLab latest release: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no AIGW release is available")
	}
	return release.TagName, nil
}

func (u Updater) runGlab(ctx context.Context, args ...string) ([]byte, error) {
	if runner, ok := u.Runner.(EnvironmentRunner); ok {
		return runner.RunWithEnv(ctx, []string{"GL_HOST=" + u.releaseHost()}, "glab", args...)
	}
	return u.Runner.Run(ctx, "glab", args...)
}

func (u Updater) runGlabToFile(ctx context.Context, destination string, args ...string) error {
	if runner, ok := u.Runner.(EnvironmentFileRunner); ok {
		return runner.RunToFileWithEnv(ctx, []string{"GL_HOST=" + u.releaseHost()}, destination, "glab", args...)
	}
	runner, ok := u.Runner.(FileRunner)
	if !ok {
		return fmt.Errorf("authenticated glab asset download is unavailable")
	}
	return runner.RunToFile(ctx, destination, "glab", args...)
}

func (u Updater) releaseHost() string {
	return strings.TrimRight(strings.TrimSpace(u.gitLabSource().Origin), "/")
}

func (u Updater) releaseProject() string { return strings.TrimSpace(u.gitLabSource().Repository) }

func (u Updater) releaseProjectPath() string { return url.PathEscape(u.releaseProject()) }

func (u Updater) gitLabAPIURL(path string) string {
	return u.releaseHost() + "/api/v4/projects/" + u.releaseProjectPath() + "/" + path
}

func (u Updater) gitLabReleaseDownloadURL(tag, asset string) string {
	return u.releaseHost() + "/" + u.releaseProject() + "/-/releases/" + url.PathEscape(tag) + "/downloads/" + url.PathEscape(asset)
}

func gitLabToken() (string, error) {
	token := os.Getenv("GITLAB_TOKEN")
	if strings.ContainsAny(token, "\r\n") {
		return "", fmt.Errorf("GITLAB_TOKEN contains a control character")
	}
	if token == "" {
		return "", fmt.Errorf("latest private release requires authenticated glab or GITLAB_TOKEN")
	}
	return token, nil
}

func (u Updater) validateTokenFallbackHost() error {
	configuredHost := strings.TrimSpace(u.gitLabSource().Origin)
	if configuredHost == "" {
		return fmt.Errorf("GITLAB_TOKEN fallback requires explicit AIGW_GITLAB_RELEASE_ORIGIN with an HTTPS origin")
	}
	parsed, err := url.Parse(configuredHost)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("GITLAB_TOKEN fallback requires AIGW_GITLAB_RELEASE_ORIGIN to be an HTTPS origin without credentials, path, query, or fragment")
	}
	return nil
}

func (u Updater) gitLabHTTPClient() *http.Client {
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
			request.Header.Del("PRIVATE-TOKEN")
		}
		if len(previous) > 0 && previous[0].URL.Scheme == "https" && request.URL.Scheme != "https" {
			return fmt.Errorf("refusing GitLab update redirect from HTTPS to HTTP")
		}
		if defaultCheckRedirect != nil {
			return defaultCheckRedirect(request, previous)
		}
		return nil
	}
	return &client
}
