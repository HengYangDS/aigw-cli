package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (ExecRunner) RunToFile(ctx context.Context, destination, name string, args ...string) error {
	return ExecRunner{}.RunToFileWithEnv(ctx, nil, destination, name, args...)
}

func (ExecRunner) RunToFileWithEnv(ctx context.Context, environment []string, destination, name string, args ...string) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open command output %s: %w", filepath.Base(destination), err)
	}
	defer file.Close()
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergeEnvironment(os.Environ(), environment)
	cmd.Stdout = file
	cmd.Stderr = &limitedWriter{writer: &stderr, limit: 16 << 10}
	if err := cmd.Run(); err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(stderr.String()))
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

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (u Updater) installPortableArchive(archivePath, tag string) (string, bool, error) {
	archiveName := filepath.Base(archivePath)
	if err := verifyChecksum(archivePath, filepath.Join(filepath.Dir(archivePath), "checksums.txt"), archiveName); err != nil {
		return "", false, err
	}
	binaryName := "aigw"
	if u.GOOS == "windows" {
		binaryName = "aigw.exe"
	}
	binary, err := extractBinary(archivePath, expectedBinaryPath(normalizeVersion(tag), u.GOOS, u.GOARCH, binaryName))
	if err != nil {
		return "", false, err
	}
	if u.GOOS == "windows" && runtime.GOOS == "windows" {
		message, err := u.scheduleWindowsReplacement(binary, tag)
		return message, false, err
	}
	if err := u.replacePortableBinary(binary); err != nil {
		return "", false, err
	}
	return "updated to " + tag, false, nil
}

func (u Updater) replacePortableBinary(binary []byte) error {
	if err := preservePreviousBinary(u.Executable); err != nil {
		return err
	}
	if err := transaction.WriteFileAtomic(u.Executable, binary, 0o755); err != nil {
		return fmt.Errorf("replace AIGW executable: %w", err)
	}
	if err := os.Chmod(u.Executable, 0o755); err != nil {
		return fmt.Errorf("make updated AIGW executable runnable: %w", err)
	}
	return nil
}

func portableArchiveName(version, goos, goarch string) string {
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("aigw_%s_%s_%s%s", version, goos, goarch, extension)
}

func localCandidateVersion(directory, goos, goarch string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("read verified local candidate: %w", err)
	}
	prefix := "aigw_"
	suffix := "_" + goos + "_" + goarch
	if goos == "windows" {
		suffix += ".zip"
	} else {
		suffix += ".tar.gz"
	}
	versions := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		version := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), suffix)
		if _, err := parseVersion("v" + version); err == nil {
			versions = append(versions, version)
		}
	}
	if len(versions) != 1 {
		return "", fmt.Errorf("verified local candidate must contain exactly one portable archive for %s/%s", goos, goarch)
	}
	return versions[0], nil
}

func archiveVersion(name, goos, goarch string) (string, error) {
	prefix := "aigw_"
	suffix := "_" + goos + "_" + goarch
	if goos == "windows" {
		suffix += ".zip"
	} else {
		suffix += ".tar.gz"
	}
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return "", fmt.Errorf("verified local candidate archive must target %s/%s", goos, goarch)
	}
	version := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	if _, err := parseVersion("v" + version); err != nil {
		return "", err
	}
	return version, nil
}

func expectedBinaryPath(version, goos, goarch, binaryName string) string {
	return fmt.Sprintf("aigw_%s_%s_%s/%s", version, goos, goarch, binaryName)
}

// Rollback restores the immediately preceding portable AIGW executable without
// accessing the network. It swaps the current and previous binaries so the
// action itself remains reversible and never creates an unbounded chain.
func (u Updater) Rollback(_ context.Context) (string, error) {
	if u.Channel == "" {
		u.Channel = ChannelPortable
	}
	if u.Channel != ChannelPortable {
		return "", fmt.Errorf("program rollback is available only for a portable installation; use the native package manager for %s", u.Channel)
	}
	if strings.TrimSpace(u.Executable) == "" {
		return "", errors.New("AIGW executable path is empty")
	}
	if u.GOOS == "windows" && runtime.GOOS == "windows" {
		return u.scheduleWindowsRollback()
	}
	current, err := os.ReadFile(u.Executable)
	if err != nil {
		return "", fmt.Errorf("read current AIGW executable: %w", err)
	}
	backup := rollbackPath(u.Executable)
	previous, err := os.ReadFile(backup)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no previous portable AIGW binary is available")
		}
		return "", fmt.Errorf("read previous AIGW executable: %w", err)
	}
	info, err := os.Stat(u.Executable)
	if err != nil {
		return "", fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	if err := transaction.WriteFileAtomic(u.Executable, previous, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("restore previous AIGW executable: %w", err)
	}
	if err := os.Chmod(u.Executable, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("make restored AIGW executable runnable: %w", err)
	}
	if err := transaction.WriteFileAtomic(backup, current, info.Mode().Perm()); err != nil {
		rollbackErr := transaction.WriteFileAtomic(u.Executable, current, info.Mode().Perm())
		if rollbackErr != nil {
			return "", fmt.Errorf("save reversible AIGW rollback copy: %w; restore current binary also failed: %v", err, rollbackErr)
		}
		return "", fmt.Errorf("save reversible AIGW rollback copy failed and current binary was restored: %w", err)
	}
	if err := os.Chmod(backup, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("make reversible AIGW rollback copy runnable: %w", err)
	}
	return "restored the previous program version. If that older program does not support `aigw update --rollback`, download the current portable package and run its installer; it replaces only AIGW and retains one predecessor.", nil
}

func (u Updater) scheduleWindowsRollback() (string, error) {
	backup := rollbackPath(u.Executable)
	previous, err := os.ReadFile(backup)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no previous portable AIGW binary is available")
		}
		return "", fmt.Errorf("read previous AIGW executable: %w", err)
	}
	info, err := os.Stat(u.Executable)
	if err != nil {
		return "", fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	staged := windowsRollbackStagePath(u.Executable)
	if err := os.WriteFile(staged, previous, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("stage Windows AIGW rollback: %w", err)
	}
	if err := os.Chmod(staged, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("make staged Windows AIGW rollback executable: %w", err)
	}
	script := u.Executable + ".rollback.cmd"
	content, err := WindowsRollbackPlan(u.Executable)
	if err != nil {
		_ = os.Remove(staged)
		return "", err
	}
	if err := os.WriteFile(script, []byte(content), 0o600); err != nil {
		_ = os.Remove(staged)
		return "", fmt.Errorf("write Windows AIGW rollback helper: %w", err)
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/B", script)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(script)
		_ = os.Remove(staged)
		return "", fmt.Errorf("start Windows AIGW rollback helper: %w", err)
	}
	return "scheduled restoration of the previous program version; rollback completes after this command exits", nil
}

func preservePreviousBinary(executable string) error {
	previous, err := os.ReadFile(executable)
	if err != nil {
		return fmt.Errorf("read current AIGW executable: %w", err)
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	backup := rollbackPath(executable)
	if err := transaction.WriteFileAtomic(backup, previous, info.Mode().Perm()); err != nil {
		return fmt.Errorf("save previous AIGW executable: %w", err)
	}
	if err := os.Chmod(backup, info.Mode().Perm()); err != nil {
		return fmt.Errorf("make previous AIGW executable runnable: %w", err)
	}
	return nil
}

func rollbackPath(executable string) string {
	if strings.Contains(executable, `\`) && !strings.Contains(executable, "/") {
		directory := executable[:strings.LastIndex(executable, `\`)+1]
		if strings.EqualFold(filepath.Ext(executable), ".exe") {
			return directory + ".aigw.previous.exe"
		}
		return directory + ".aigw.previous"
	}
	directory := filepath.Dir(executable)
	if strings.EqualFold(filepath.Ext(executable), ".exe") {
		return filepath.Join(directory, ".aigw.previous.exe")
	}
	return filepath.Join(directory, ".aigw.previous")
}

func windowsRollbackStagePath(executable string) string {
	return executable + ".rollback"
}

func (u Updater) packageAssetName(version string) string {
	switch u.Channel {
	case ChannelPKG:
		if u.GOOS != "darwin" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_darwin_universal.pkg", version)
	case ChannelDeb:
		if u.GOOS != "linux" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_linux_%s.deb", version, u.GOARCH)
	case ChannelRPM:
		if u.GOOS != "linux" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_linux_%s.rpm", version, u.GOARCH)
	case ChannelMSI:
		if u.GOOS != "windows" {
			return ""
		}
		return fmt.Sprintf("aigw_%s_windows_%s.msi", version, u.GOARCH)
	default:
		return ""
	}
}

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
		release, err := u.githubRelease(ctx, source, "releases/latest")
		if err != nil {
			return "", isGitHubUnavailable(err), err
		}
		if release.TagName == "" {
			return "", false, fmt.Errorf("no AIGW release is available")
		}
		return release.TagName, false, nil
	default:
		return "", false, fmt.Errorf("unsupported release provider %q", source.Provider)
	}
}

func (u Updater) latestTagFromGitHub(ctx context.Context, source ReleaseSource) (string, error) {
	release, err := u.githubRelease(ctx, source, "releases/latest")
	if err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no AIGW release is available")
	}
	return release.TagName, nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
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
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
		return githubRelease{}, unavailable(fmt.Errorf("query GitHub release metadata: %s", response.Status))
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return githubRelease{}, fmt.Errorf("query GitHub release metadata: %s", response.Status)
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
		return err
	}
	if release.TagName != tag {
		return fmt.Errorf("GitHub release metadata tag %q does not match requested tag %q", release.TagName, tag)
	}
	return u.downloadGitHubReleaseAssets(ctx, release, directory, assets...)
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
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
		return unavailable(fmt.Errorf("%s", response.Status))
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s", response.Status)
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

func (u Updater) runPackageInstaller(ctx context.Context, path string) error {
	switch u.Channel {
	case ChannelPKG:
		_, err := u.Runner.Run(ctx, "open", path)
		if err != nil {
			return fmt.Errorf("open macOS installer: %w", err)
		}
	case ChannelDeb:
		_, err := u.Runner.Run(ctx, "sudo", "dpkg", "-i", path)
		if err != nil {
			return fmt.Errorf("install deb package: %w", err)
		}
	case ChannelRPM:
		_, err := u.Runner.Run(ctx, "sudo", "rpm", "-Uvh", path)
		if err != nil {
			return fmt.Errorf("install rpm package: %w", err)
		}
	case ChannelMSI:
		_, err := u.Runner.Run(ctx, "msiexec", "/i", path)
		if err != nil {
			return fmt.Errorf("start Windows installer: %w", err)
		}
	default:
		return fmt.Errorf("unknown installation channel %q", u.Channel)
	}
	return nil
}

func detectChannel(executable string) Channel {
	if channel, ok := parseChannel(InstallChannel); ok {
		return channel
	}
	if value := strings.TrimSpace(os.Getenv("AIGW_INSTALL_CHANNEL")); value != "" {
		if channel, ok := parseChannel(value); ok {
			return channel
		}
	}
	dir := filepath.Dir(executable)
	if runtime.GOOS == "darwin" && strings.HasPrefix(executable, "/usr/local/") {
		return ChannelPKG
	}
	if runtime.GOOS == "linux" && (dir == "/usr/bin" || dir == "/usr/local/bin") {
		return ChannelDeb
	}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(executable), `program files\aigw`) {
		return ChannelMSI
	}
	return ChannelPortable
}

func parseChannel(value string) (Channel, bool) {
	switch channel := Channel(strings.ToLower(strings.TrimSpace(value))); channel {
	case ChannelPortable, ChannelPKG, ChannelDeb, ChannelRPM, ChannelMSI:
		return channel, true
	default:
		return "", false
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
	tag := strings.TrimSpace(string(output))
	if tag == "" {
		return "", fmt.Errorf("no AIGW release is available")
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

func (u Updater) gitLabAPIURL(path string) string {
	return u.releaseHost() + "/api/v4/projects/" + u.releaseProjectPath() + "/" + path
}

func (u Updater) gitLabReleaseDownloadURL(tag, asset string) string {
	return u.releaseHost() + "/" + u.releaseProject() + "/-/releases/" + url.PathEscape(tag) + "/downloads/" + url.PathEscape(asset)
}

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

func normalizeVersion(value string) string { return strings.TrimPrefix(strings.TrimSpace(value), "v") }

func compareVersions(left, right string) (int, error) {
	leftParts, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseVersion(right)
	if err != nil {
		return 0, err
	}
	for index := 0; index < 3; index++ {
		if leftParts.core[index] < rightParts.core[index] {
			return -1, nil
		}
		if leftParts.core[index] > rightParts.core[index] {
			return 1, nil
		}
	}
	if leftParts.pre == rightParts.pre {
		return 0, nil
	}
	if leftParts.pre == "" {
		return 1, nil
	}
	if rightParts.pre == "" {
		return -1, nil
	}
	return comparePrerelease(leftParts.pre, rightParts.pre)
}

type parsedVersion struct {
	core [3]uint64
	pre  string
}

func parseVersion(value string) (parsedVersion, error) {
	value = normalizeVersion(value)
	if value == "" {
		return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
	}
	core, pre, _ := strings.Cut(value, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
	}
	parsed := parsedVersion{pre: pre}
	for index, part := range parts {
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
		}
		parsed.core[index] = number
	}
	if pre != "" && !validPrerelease(pre) {
		return parsedVersion{}, fmt.Errorf("invalid release version %q", value)
	}
	return parsed, nil
}

func comparePrerelease(left, right string) (int, error) {
	leftParts, rightParts := strings.Split(left, "."), strings.Split(right, ".")
	limit := len(leftParts)
	if len(rightParts) < limit {
		limit = len(rightParts)
	}
	for index := 0; index < limit; index++ {
		leftPart, rightPart := leftParts[index], rightParts[index]
		leftNumber, leftNumeric := prereleaseNumber(leftPart)
		rightNumber, rightNumeric := prereleaseNumber(rightPart)
		switch {
		case leftNumeric && rightNumeric:
			if leftNumber < rightNumber {
				return -1, nil
			}
			if leftNumber > rightNumber {
				return 1, nil
			}
		case leftNumeric:
			return -1, nil
		case rightNumeric:
			return 1, nil
		case leftPart < rightPart:
			return -1, nil
		case leftPart > rightPart:
			return 1, nil
		}
	}
	if len(leftParts) < len(rightParts) {
		return -1, nil
	}
	if len(leftParts) > len(rightParts) {
		return 1, nil
	}
	return 0, nil
}

func validPrerelease(value string) bool {
	for _, part := range strings.Split(value, ".") {
		if part == "" {
			return false
		}
		for _, character := range part {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
		if _, numeric := prereleaseNumber(part); numeric {
			continue
		}
		if allDigits(part) {
			return false
		}
	}
	return true
}

func prereleaseNumber(value string) (uint64, bool) {
	number, err := strconv.ParseUint(value, 10, 64)
	return number, err == nil
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func verifyChecksum(archivePath, checksumPath, archiveName string) error {
	if filepath.Base(archiveName) != archiveName || archiveName == "" {
		return fmt.Errorf("invalid release asset name %q", archiveName)
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	expected := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !isSHA256(fields[0]) {
			continue
		}
		name := strings.TrimPrefix(fields[1], "./")
		if name != archiveName {
			continue
		}
		if expected != "" {
			return fmt.Errorf("duplicate checksum entry for %s", archiveName)
		}
		expected = strings.ToLower(fields[0])
	}
	if expected == "" {
		return fmt.Errorf("checksum entry missing for %s", archiveName)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open update archive: %w", err)
	}
	defer archive.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return fmt.Errorf("hash update archive: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func extractBinary(path, expectedPath string) ([]byte, error) {
	if strings.HasSuffix(path, ".zip") {
		return extractZipBinary(path, expectedPath)
	}
	archive, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open update archive: %w", err)
	}
	defer archive.Close()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return nil, fmt.Errorf("open gzip archive: %w", err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	var binary []byte
	matches := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar archive: %w", err)
		}
		if filepath.Clean(header.Name) != expectedPath || header.Typeflag != tar.TypeReg {
			continue
		}
		matches++
		if matches > 1 {
			return nil, fmt.Errorf("update archive contains multiple expected AIGW binaries")
		}
		binary, err = io.ReadAll(io.LimitReader(reader, 128<<20))
		if err != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", err)
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("expected AIGW binary is missing from update archive")
	}
	return binary, nil
}

func extractZipBinary(path, expectedPath string) ([]byte, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	defer archive.Close()
	var binary []byte
	matches := 0
	for _, file := range archive.File {
		if filepath.Clean(file.Name) != expectedPath || file.FileInfo().IsDir() {
			continue
		}
		matches++
		if matches > 1 {
			return nil, fmt.Errorf("update archive contains multiple expected AIGW binaries")
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open AIGW binary in zip: %w", err)
		}
		binary, err = io.ReadAll(io.LimitReader(reader, 128<<20))
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close AIGW binary in zip: %w", closeErr)
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("expected AIGW binary is missing from update archive")
	}
	return binary, nil
}

func (u Updater) scheduleWindowsReplacement(binary []byte, tag string) (string, error) {
	staged := u.Executable + ".update"
	if err := os.WriteFile(staged, binary, 0o755); err != nil {
		return "", fmt.Errorf("stage Windows update: %w", err)
	}
	script := u.Executable + ".update.cmd"
	content, err := WindowsReplacementPlan(u.Executable)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(script, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write Windows update helper: %w", err)
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/B", script)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start Windows update helper: %w", err)
	}
	return "downloaded " + tag + "; the update completes after this command exits", nil
}

// WindowsReplacementPlan returns the delayed replacement script used when the
// running executable cannot be renamed immediately. It retains exactly one
// immediate predecessor beside the portable executable.
func WindowsReplacementPlan(executable string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", errors.New("Windows AIGW executable path is empty")
	}
	staged := executable + ".update"
	previous := rollbackPath(executable)
	return fmt.Sprintf("@echo off\r\nping 127.0.0.1 -n 3 > nul\r\nif exist \"%s\" move /Y \"%s\" \"%s\" > nul\r\nmove /Y \"%s\" \"%s\" > nul\r\ndel \"%%~f0\"\r\n", executable, executable, previous, staged, executable), nil
}

// WindowsRollbackPlan returns the delayed, reversible program-only rollback
// script. It runs after the invoking executable exits, swaps the current and
// previous portable binaries, and restores the original pair if activation of
// the staged predecessor fails. It deliberately has no network operations.
func WindowsRollbackPlan(executable string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", errors.New("Windows AIGW executable path is empty")
	}
	previous := rollbackPath(executable)
	staged := windowsRollbackStagePath(executable)
	return fmt.Sprintf("@echo off\r\nping 127.0.0.1 -n 3 > nul\r\nmove /Y \"%s\" \"%s\" > nul\r\nif errorlevel 1 goto :failed_before_swap\r\nmove /Y \"%s\" \"%s\" > nul\r\nif not errorlevel 1 goto :success\r\nmove /Y \"%s\" \"%s\" > nul\r\nif errorlevel 1 goto :failed\r\nmove /Y \"%s\" \"%s\" > nul\r\ngoto :failed\r\n:failed_before_swap\r\ndel \"%s\" > nul 2>&1\r\n:failed\r\ndel \"%%~f0\"\r\nexit /b 1\r\n:success\r\ndel \"%%~f0\"\r\nexit /b 0\r\n", executable, previous, staged, executable, previous, executable, staged, previous, staged), nil
}
