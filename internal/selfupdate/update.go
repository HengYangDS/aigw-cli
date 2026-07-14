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

const project = "dig/misc/agentic-third-party-api/aigw-cli"

const projectID = "456"

const defaultGitLabHost = "http://192.168.64.101:18086"

const gitLabRequestTimeout = 30 * time.Second

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
	return Updater{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Channel: detectChannel(executable), Executable: executable, Runner: ExecRunner{}}
}

func (u Updater) Update(ctx context.Context, currentVersion string) (string, error) {
	if u.Runner == nil {
		u.Runner = ExecRunner{}
	}
	if u.Channel == "" {
		u.Channel = ChannelPortable
	}
	if err := u.validateGitLabHost(); err != nil {
		return "", err
	}
	tag, err := u.latestTag(ctx)
	if err != nil {
		return "", err
	}
	comparison, err := compareVersions(tag, currentVersion)
	if err != nil {
		return "", err
	}
	if comparison == 0 {
		return "已经是最新版 " + tag + "。", nil
	}
	if comparison < 0 {
		return "", fmt.Errorf("refusing to replace %s with older release %s", currentVersion, tag)
	}
	version := normalizeVersion(tag)
	if u.Channel != ChannelPortable {
		return u.updatePackage(ctx, tag, version)
	}
	archiveName := fmt.Sprintf("aigw_%s_%s_%s", version, u.GOOS, u.GOARCH)
	extension := ".tar.gz"
	if u.GOOS == "windows" {
		extension = ".zip"
	}
	archiveName += extension
	tmp, err := os.MkdirTemp("", "aigw-update-*")
	if err != nil {
		return "", fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := u.downloadReleaseAssets(ctx, tag, tmp, archiveName, "checksums.txt"); err != nil {
		return "", err
	}
	archivePath := filepath.Join(tmp, archiveName)
	if err := verifyChecksum(archivePath, filepath.Join(tmp, "checksums.txt"), archiveName); err != nil {
		return "", err
	}
	binaryName := "aigw"
	if u.GOOS == "windows" {
		binaryName = "aigw.exe"
	}
	binary, err := extractBinary(archivePath, binaryName)
	if err != nil {
		return "", err
	}
	if u.GOOS == "windows" && runtime.GOOS == "windows" {
		return u.scheduleWindowsReplacement(binary, tag)
	}
	if err := preservePreviousBinary(u.Executable); err != nil {
		return "", err
	}
	if err := transaction.WriteFileAtomic(u.Executable, binary, 0o755); err != nil {
		return "", fmt.Errorf("replace AIGW executable: %w", err)
	}
	if err := os.Chmod(u.Executable, 0o755); err != nil {
		return "", fmt.Errorf("make updated AIGW executable runnable: %w", err)
	}
	return "已更新到 " + tag + "。", nil
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
	return "已恢复上一程序版本。若该旧版本不支持 `aigw update --rollback`，请从团队发布页重新下载当前便携包并运行其中的安装脚本即可恢复当前程序；该安装仅替换 AIGW 程序并保留旧程序副本。", nil
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
	return "已安排恢复上一程序版本；退出本次命令后将完成回退。", nil
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

func (u Updater) updatePackage(ctx context.Context, tag, version string) (string, error) {
	asset := u.packageAssetName(version)
	if asset == "" {
		return "", fmt.Errorf("installation channel %q is not supported on %s/%s", u.Channel, u.GOOS, u.GOARCH)
	}
	tmp, err := os.MkdirTemp("", "aigw-update-*")
	if err != nil {
		return "", fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := u.downloadReleaseAssets(ctx, tag, tmp, asset, "checksums.txt"); err != nil {
		return "", err
	}
	path := filepath.Join(tmp, asset)
	if err := verifyChecksum(path, filepath.Join(tmp, "checksums.txt"), asset); err != nil {
		return "", err
	}
	if err := u.runPackageInstaller(ctx, path); err != nil {
		return "", err
	}
	switch u.Channel {
	case ChannelPKG, ChannelMSI:
		return "已下载 " + tag + " 安装包；请按安装器提示完成更新。", nil
	case ChannelDeb, ChannelRPM:
		return "已通过系统包管理器更新到 " + tag + "。", nil
	default:
		return "已准备 " + tag + " 更新。", nil
	}
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

func (u Updater) downloadReleaseAssets(ctx context.Context, tag, directory string, assets ...string) error {
	for index, asset := range assets {
		path := filepath.Join(directory, asset)
		_, err := u.runGlab(ctx, "release", "download", tag, "-R", project, "--asset-name", asset, "--dir", directory)
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
	output, err := u.runGlab(ctx, "api", "projects/"+projectID+"/releases/"+url.PathEscape(tag))
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
		if filepath.Base(link.Name) == link.Name && link.URL != "" {
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
	output, err := u.runGlab(ctx, "release", "list", "-R", project, "--per-page", "1", "-F", "json", "--jq", ".[0].tag_name")
	if err != nil {
		if isGlabUnavailable(err) {
			if err := u.validateTokenFallbackHost(); err != nil {
				return "", err
			}
			return u.latestTagFromGitLabAPI(ctx)
		}
		return "", fmt.Errorf("query latest release: %w", err)
	}
	tag := strings.TrimSpace(string(output))
	if tag == "" {
		return "", fmt.Errorf("no AIGW release is available")
	}
	return tag, nil
}

func isGlabUnavailable(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
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
		return "", fmt.Errorf("query GitLab latest release: %w", err)
	}
	defer response.Body.Close()
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
		return runner.RunWithEnv(ctx, []string{"GL_HOST=" + u.gitLabHost()}, "glab", args...)
	}
	return u.Runner.Run(ctx, "glab", args...)
}

func (u Updater) runGlabToFile(ctx context.Context, destination string, args ...string) error {
	if runner, ok := u.Runner.(EnvironmentFileRunner); ok {
		return runner.RunToFileWithEnv(ctx, []string{"GL_HOST=" + u.gitLabHost()}, destination, "glab", args...)
	}
	runner, ok := u.Runner.(FileRunner)
	if !ok {
		return fmt.Errorf("authenticated glab asset download is unavailable")
	}
	return runner.RunToFile(ctx, destination, "glab", args...)
}

func (u Updater) gitLabHost() string {
	host := strings.TrimRight(strings.TrimSpace(os.Getenv("AIGW_GL_HOST")), "/")
	if host == "" {
		return defaultGitLabHost
	}
	parsed, err := url.Parse(host)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return ""
	}
	return host
}

func (u Updater) gitLabAPIURL(path string) string {
	return u.gitLabHost() + "/api/v4/projects/" + url.PathEscape(project) + "/" + path
}

func (u Updater) gitLabReleaseDownloadURL(tag, asset string) string {
	return u.gitLabHost() + "/" + project + "/-/releases/" + url.PathEscape(tag) + "/downloads/" + url.PathEscape(asset)
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

func (u Updater) validateGitLabHost() error {
	if u.gitLabHost() == "" {
		return fmt.Errorf("AIGW_GL_HOST must be an HTTP(S) origin without credentials, path, query, or fragment")
	}
	return nil
}

func (u Updater) validateTokenFallbackHost() error {
	configuredHost := strings.TrimSpace(os.Getenv("AIGW_GL_HOST"))
	if configuredHost == "" {
		return fmt.Errorf("GITLAB_TOKEN fallback requires explicit AIGW_GL_HOST with an HTTPS origin")
	}
	parsed, err := url.Parse(configuredHost)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("GITLAB_TOKEN fallback requires AIGW_GL_HOST to be an HTTPS origin without credentials, path, query, or fragment")
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
		client.Timeout = gitLabRequestTimeout
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
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	expected := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "./") == archiveName {
			expected = strings.ToLower(fields[0])
			break
		}
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

func extractBinary(path, binaryName string) ([]byte, error) {
	if strings.HasSuffix(path, ".zip") {
		return extractZipBinary(path, binaryName)
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
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar archive: %w", err)
		}
		if filepath.Base(filepath.Clean(header.Name)) != binaryName || header.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(reader, 128<<20))
		if err != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("AIGW binary is missing from update archive")
}

func extractZipBinary(path, binaryName string) ([]byte, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if filepath.Base(filepath.Clean(file.Name)) != binaryName {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open AIGW binary in zip: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, 128<<20))
		reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", readErr)
		}
		return data, nil
	}
	return nil, fmt.Errorf("AIGW binary is missing from update archive")
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
	return "已下载 " + tag + "；退出本次命令后将完成更新。", nil
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
