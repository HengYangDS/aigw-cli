package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

// BuildGitLabRelease* and BuildGitHubRelease* identify independently configured
// release peers embedded in official artifacts. A source build leaves them
// empty, so remote updates require explicit source configuration.
var (
	BuildGitLabReleaseOrigin     string
	BuildGitLabReleaseRepository string
	BuildGitHubReleaseOrigin     string
	BuildGitHubReleaseRepository string
)

// ReleaseProvider identifies the Forge selected as the release metadata source.
type ReleaseProvider string

const (
	// ReleaseProviderGitLab selects GitLab as the release metadata source.
	ReleaseProviderGitLab ReleaseProvider = "gitlab"
	// ReleaseProviderGitHub selects GitHub as the release metadata source.
	ReleaseProviderGitHub ReleaseProvider = "github"
)

// ReleaseSource identifies one release namespace. It contains no credential.
// Configured sources are equal peers; neither is subordinate to the other.
type ReleaseSource struct {
	Provider   ReleaseProvider
	Origin     string
	Repository string
}

func (u Updater) forgeSources() []ReleaseSource {
	return []ReleaseSource{
		u.GitLab.withEnvironment(ReleaseProviderGitLab),
		u.GitHub.withEnvironment(ReleaseProviderGitHub),
	}
}

func (s ReleaseSource) withEnvironment(provider ReleaseProvider) ReleaseSource {
	s.Provider = provider
	prefix := "AIGW_" + strings.ToUpper(string(provider)) + "_RELEASE_"
	if origin := strings.TrimSpace(os.Getenv(prefix + "ORIGIN")); origin != "" {
		s.Origin = origin
	}
	if repository := strings.TrimSpace(os.Getenv(prefix + "REPOSITORY")); repository != "" {
		s.Repository = repository
	}
	return s
}

func (s ReleaseSource) empty() bool {
	return strings.TrimSpace(s.Origin) == "" && strings.TrimSpace(s.Repository) == ""
}

func validateReleaseSources(sources ...ReleaseSource) error {
	configured := false
	for _, source := range sources {
		if source.empty() {
			continue
		}
		configured = true
		if err := validateReleaseSource(source); err != nil {
			return err
		}
	}
	if !configured {
		return fmt.Errorf("release source is not configured; install an official release or use `aigw update --candidate ARCHIVE --checksums MANIFEST`")
	}
	return nil
}

func validateReleaseSource(source ReleaseSource) error {
	origin, repository := strings.TrimSpace(source.Origin), strings.TrimSpace(source.Repository)
	if source.Provider != ReleaseProviderGitLab && source.Provider != ReleaseProviderGitHub {
		return fmt.Errorf("unsupported release provider %q", source.Provider)
	}
	if origin == "" || repository == "" {
		return fmt.Errorf("%s release source is incomplete; set provider, origin, and repository together", source.Provider)
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || strings.TrimSuffix(origin, "/") != (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String() {
		return fmt.Errorf("%s release origin must be an HTTP(S) origin without credentials, path, query, or fragment", source.Provider)
	}
	if parsed.Scheme != "https" && !plainHTTPReleaseOriginAllowed(parsed.Hostname()) {
		return fmt.Errorf("%s release origin must use HTTPS", source.Provider)
	}
	if source.Provider == ReleaseProviderGitHub && strings.Count(repository, "/") != 1 {
		return fmt.Errorf("GitHub release repository must be an owner/repository path")
	}
	if !fs.ValidPath(repository) || !strings.Contains(repository, "/") || (&url.URL{Path: repository}).EscapedPath() != repository {
		return fmt.Errorf("%s release repository must be a valid namespace/project path", source.Provider)
	}
	return nil
}

func plainHTTPReleaseOriginAllowed(host string) bool {
	if strings.HasSuffix(host, ".test") || host == "localhost" {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && (address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast())
}

func (u Updater) latestTagFromSource(ctx context.Context, source ReleaseSource) (string, bool, error) {
	switch source.Provider {
	case ReleaseProviderGitLab:
		u.GitLab = source
		tag, err := u.latestTag(ctx)
		return tag, isGlabUnavailable(err), err
	case ReleaseProviderGitHub:
		tag, err := u.latestTagFromGitHubRelease(ctx, source)
		return tag, isSourceUnavailable(err), err
	default:
		return "", false, fmt.Errorf("unsupported release provider %q", source.Provider)
	}
}

func (u Updater) downloadReleaseAssetsFromExactSource(ctx context.Context, source ReleaseSource, tag, directory string, assets ...string) (bool, error) {
	switch source.Provider {
	case ReleaseProviderGitLab:
		u.GitLab = source
		err := u.downloadReleaseAssets(ctx, tag, directory, assets...)
		return isGlabUnavailable(err), err
	case ReleaseProviderGitHub:
		err := u.downloadReleaseAssetsFromGitHub(ctx, source, tag, directory, assets...)
		return isSourceUnavailable(err), err
	default:
		return false, fmt.Errorf("unsupported release provider %q", source.Provider)
	}
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
