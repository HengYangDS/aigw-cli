// Package selfupdate owns dual-Forge release resolution, verified artifact
// retrieval, channel-aware installation, and portable rollback.
package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const releaseRequestTimeout = 30 * time.Second

// BuildGitLabRelease* and BuildGitHubRelease* identify independently built
// release planes embedded by their respective publishing pipelines. A source
// build leaves them empty so self-update fails closed.
var (
	BuildGitLabReleaseOrigin     string
	BuildGitLabReleaseRepository string
	BuildGitHubReleaseOrigin     string
	BuildGitHubReleaseRepository string
)

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return ExecRunner{}.RunWithEnv(ctx, nil, name, args...)
}

type EnvironmentRunner interface {
	RunWithEnv(context.Context, []string, string, ...string) ([]byte, error)
}

type FileRunner interface {
	RunToFile(context.Context, string, string, ...string) error
}

type EnvironmentFileRunner interface {
	RunToFileWithEnv(context.Context, []string, string, string, ...string) error
}

func (ExecRunner) RunWithEnv(ctx context.Context, environment []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergeEnvironment(os.Environ(), environment)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &limitedWriter{writer: &stderr, limit: 16 << 10}
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func (ExecRunner) RunToFile(ctx context.Context, destination, name string, args ...string) error {
	return ExecRunner{}.RunToFileWithEnv(ctx, nil, destination, name, args...)
}

func (ExecRunner) RunToFileWithEnv(ctx context.Context, environment []string, destination, name string, args ...string) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open command output %s: %w", filepath.Base(destination), err)
	}
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergeEnvironment(os.Environ(), environment)
	cmd.Stdout = file
	cmd.Stderr = &limitedWriter{writer: &stderr, limit: 16 << 10}
	runErr := cmd.Run()
	closeErr := file.Close()
	if runErr != nil {
		_ = os.Remove(destination)
		if closeErr != nil {
			return fmt.Errorf("%s failed: %w: %s; close command output %s: %v", name, runErr, strings.TrimSpace(stderr.String()), filepath.Base(destination), closeErr)
		}
		return fmt.Errorf("%s failed: %w: %s", name, runErr, strings.TrimSpace(stderr.String()))
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("close command output %s: %w", filepath.Base(destination), closeErr)
	}
	return nil
}

type limitedWriter struct {
	writer  io.Writer
	limit   int
	written int
}

func (w *limitedWriter) Write(value []byte) (int, error) {
	originalLength := len(value)
	remaining := w.limit - w.written
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		count, err := w.writer.Write(value)
		w.written += count
		if err != nil {
			return count, err
		}
	}
	return originalLength, nil
}

func mergeEnvironment(base, overrides []string) []string {
	result := append([]string(nil), base...)
	for _, override := range overrides {
		name, _, ok := strings.Cut(override, "=")
		if !ok || name == "" {
			continue
		}
		prefix := name + "="
		filtered := result[:0]
		for _, value := range result {
			if !strings.HasPrefix(value, prefix) {
				filtered = append(filtered, value)
			}
		}
		result = append(filtered, override)
	}
	return result
}

type Updater struct {
	GOOS       string
	GOARCH     string
	Channel    Channel
	Executable string
	Runner     CommandRunner
	HTTPClient *http.Client
	GitLab     ReleaseSource
	GitHub     ReleaseSource
}

type ReleaseProvider string

const (
	ReleaseProviderGitLab ReleaseProvider = "gitlab"
	ReleaseProviderGitHub ReleaseProvider = "github"
)

// ReleaseSource identifies one release namespace. It contains no credential.
// Configured sources are equal peers; neither is subordinate to the other.
type ReleaseSource struct {
	Provider   ReleaseProvider
	Origin     string
	Repository string
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

type Channel string

const (
	ChannelPortable Channel = "portable"
	ChannelPKG      Channel = "pkg"
	ChannelDeb      Channel = "deb"
	ChannelRPM      Channel = "rpm"
	ChannelMSI      Channel = "msi"
)

var InstallChannel = string(ChannelPortable)

func Current(executable string) Updater {
	return Updater{
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Channel:    detectChannel(executable),
		Executable: executable,
		Runner:     ExecRunner{},
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

func (u Updater) Update(ctx context.Context, currentVersion string) (string, error) {
	if u.Runner == nil {
		u.Runner = ExecRunner{}
	}
	if u.Channel == "" {
		u.Channel = ChannelPortable
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
		return nil, fmt.Errorf("no configured release source is reachable: %s", joinErrors(unavailableSources))
	}
	for _, candidate := range resolved[1:] {
		if candidate.Tag != resolved[0].Tag {
			return nil, fmt.Errorf("configured release sources disagree on latest tag: %s=%s, %s=%s", resolved[0].Source.Provider, resolved[0].Tag, candidate.Source.Provider, candidate.Tag)
		}
	}
	return resolved, nil
}

func joinErrors(errors []error) string {
	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	return strings.Join(parts, "; ")
}

func (u Updater) updateFromResolvedPeers(ctx context.Context, releases []resolvedRelease, currentVersion string) (string, error) {
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
	asset := portableArchiveName(normalizeVersion(selected.Tag), u.GOOS, u.GOARCH)
	if u.Channel != ChannelPortable {
		asset = u.packageAssetName(normalizeVersion(selected.Tag))
		if asset == "" {
			return "", fmt.Errorf("installation channel %q is not supported on %s/%s", u.Channel, u.GOOS, u.GOARCH)
		}
	}
	downloads, cleanup, err := u.downloadPeerAssets(ctx, releases, asset)
	if err != nil {
		return "", err
	}
	defer cleanup()
	for _, candidate := range downloads[1:] {
		if candidate.Digest != downloads[0].Digest {
			return "", fmt.Errorf("reachable release sources disagree on %s asset bytes: %s != %s", asset, downloads[0].Source.Provider, candidate.Source.Provider)
		}
	}
	if u.Channel == ChannelPortable {
		message, _, err := u.installPortableArchive(downloads[0].Asset, selected.Tag)
		if err != nil {
			return "", err
		}
		return message + " verified from " + releaseProviders(downloads), nil
	}
	if err := u.runPackageInstaller(ctx, downloads[0].Asset); err != nil {
		return "", err
	}
	switch u.Channel {
	case ChannelPKG, ChannelMSI:
		return "downloaded the " + selected.Tag + " installer verified from " + releaseProviders(downloads) + "; complete the update through the installer", nil
	case ChannelDeb, ChannelRPM:
		return "updated to " + selected.Tag + " through the system package manager (verified from " + releaseProviders(downloads) + ")", nil
	default:
		return "prepared the " + selected.Tag + " update verified from " + releaseProviders(downloads), nil
	}
}

func (u Updater) downloadPeerAssets(ctx context.Context, releases []resolvedRelease, asset string) ([]downloadedRelease, func(), error) {
	downloads := make([]downloadedRelease, 0, len(releases))
	cleanupDirectories := make([]string, 0, len(releases))
	cleanup := func() {
		for _, directory := range cleanupDirectories {
			_ = os.RemoveAll(directory)
		}
	}
	unavailableSources := make([]error, 0, len(releases))
	for _, release := range releases {
		directory, err := os.MkdirTemp("", "aigw-update-*")
		if err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("create update workspace: %w", err)
		}
		cleanupDirectories = append(cleanupDirectories, directory)
		unavailable, err := u.downloadReleaseAssetsFromExactSource(ctx, release.Source, release.Tag, directory, asset, "checksums.txt")
		if err != nil {
			if unavailable {
				_ = os.RemoveAll(directory)
				cleanupDirectories = cleanupDirectories[:len(cleanupDirectories)-1]
				unavailableSources = append(unavailableSources, fmt.Errorf("%s: %w", release.Source.Provider, err))
				continue
			}
			cleanup()
			return nil, func() {}, fmt.Errorf("%s release assets failed: %w", release.Source.Provider, err)
		}
		path := filepath.Join(directory, asset)
		if err := verifyChecksum(path, filepath.Join(directory, "checksums.txt"), asset); err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("%s release checksum failed: %w", release.Source.Provider, err)
		}
		digest, err := fileSHA256(path)
		if err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("hash %s release asset: %w", release.Source.Provider, err)
		}
		downloads = append(downloads, downloadedRelease{Source: release.Source, Asset: path, Digest: digest})
	}
	if len(downloads) == 0 {
		cleanup()
		return nil, func() {}, fmt.Errorf("all reachable release sources failed while downloading %s: %s", asset, joinErrors(unavailableSources))
	}
	return downloads, cleanup, nil
}

func releaseProviders(downloads []downloadedRelease) string {
	providers := make([]string, 0, len(downloads))
	for _, download := range downloads {
		providers = append(providers, string(download.Source.Provider))
	}
	return strings.Join(providers, " and ")
}

func (u Updater) gitLabSource() ReleaseSource {
	result := u.GitLab
	result.Provider = ReleaseProviderGitLab
	if origin := strings.TrimSpace(os.Getenv("AIGW_GITLAB_RELEASE_ORIGIN")); origin != "" {
		result.Origin = origin
	}
	if repository := strings.TrimSpace(os.Getenv("AIGW_GITLAB_RELEASE_REPOSITORY")); repository != "" {
		result.Repository = repository
	}
	return result
}

func (u Updater) gitHubSource() ReleaseSource {
	result := u.GitHub
	result.Provider = ReleaseProviderGitHub
	if origin := strings.TrimSpace(os.Getenv("AIGW_GITHUB_RELEASE_ORIGIN")); origin != "" {
		result.Origin = origin
	}
	if repository := strings.TrimSpace(os.Getenv("AIGW_GITHUB_RELEASE_REPOSITORY")); repository != "" {
		result.Repository = repository
	}
	return result
}

func (u Updater) forgeSources() []ReleaseSource {
	return []ReleaseSource{u.gitLabSource(), u.gitHubSource()}
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
	origin, repository := strings.TrimRight(strings.TrimSpace(source.Origin), "/"), strings.TrimSpace(source.Repository)
	if source.Provider != ReleaseProviderGitLab && source.Provider != ReleaseProviderGitHub {
		return fmt.Errorf("unsupported release provider %q", source.Provider)
	}
	if origin == "" || repository == "" {
		return fmt.Errorf("%s release source is incomplete; set provider, origin, and repository together", source.Provider)
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("%s release origin must be an HTTP(S) origin without credentials, path, query, or fragment", source.Provider)
	}
	if source.Provider == ReleaseProviderGitHub && parsed.Scheme != "https" && !strings.HasSuffix(parsed.Hostname(), ".test") && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return fmt.Errorf("GitHub release origin must use HTTPS")
	}
	if strings.Trim(repository, "/") != repository || strings.ContainsAny(repository, "?#\r\n") {
		return fmt.Errorf("%s release repository must be a valid namespace/project path", source.Provider)
	}
	parts := strings.Split(repository, "/")
	if source.Provider == ReleaseProviderGitHub && len(parts) != 2 {
		return fmt.Errorf("GitHub release repository must be an owner/repository path")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.Contains(part, "\\") {
			return fmt.Errorf("%s release repository must be a valid namespace/project path", source.Provider)
		}
	}
	return nil
}
