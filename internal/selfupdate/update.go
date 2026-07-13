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
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const project = "dig/misc/agentic-third-party-api/aigw-cli"

const defaultGitLabHost = "http://192.168.64.101:18086"

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

func (ExecRunner) RunWithEnv(ctx context.Context, environment []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergeEnvironment(os.Environ(), environment)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return output, nil
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
	tag, err := u.latestTag(ctx)
	if err != nil {
		return "", err
	}
	if normalizeVersion(tag) == normalizeVersion(currentVersion) {
		return "已经是最新版 " + tag + "。", nil
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
	if err := transaction.WriteFileAtomic(u.Executable, binary, 0o755); err != nil {
		return "", fmt.Errorf("replace AIGW executable: %w", err)
	}
	if err := os.Chmod(u.Executable, 0o755); err != nil {
		return "", fmt.Errorf("make updated AIGW executable runnable: %w", err)
	}
	return "已更新到 " + tag + "。", nil
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
		if _, err := u.runGlab(ctx, "release", "download", tag, "-R", project, "--asset-name", asset, "--dir", directory); err != nil {
			if !isGlabUnavailable(err) {
				return fmt.Errorf("download release asset %s: %w", asset, err)
			}
			for _, remaining := range assets[index:] {
				if err := u.downloadReleaseAssetFromGitLabAPI(ctx, tag, remaining, directory); err != nil {
					return err
				}
			}
			return nil
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
	response, err := gitLabHTTPClient().Do(request)
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
	response, err := gitLabHTTPClient().Do(request)
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

func (u Updater) gitLabHost() string {
	if host := strings.TrimRight(strings.TrimSpace(os.Getenv("AIGW_GL_HOST")), "/"); host != "" {
		return host
	}
	return defaultGitLabHost
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

func gitLabHTTPClient() *http.Client {
	client := *http.DefaultClient
	defaultCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, previous []*http.Request) error {
		if len(previous) > 0 && !strings.EqualFold(request.URL.Host, previous[0].URL.Host) {
			request.Header.Del("PRIVATE-TOKEN")
		}
		if defaultCheckRedirect != nil {
			return defaultCheckRedirect(request, previous)
		}
		return nil
	}
	return &client
}

func normalizeVersion(value string) string { return strings.TrimPrefix(strings.TrimSpace(value), "v") }

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
	content := fmt.Sprintf("@echo off\r\nping 127.0.0.1 -n 3 > nul\r\nmove /Y \"%s\" \"%s\" > nul\r\ndel \"%%~f0\"\r\n", staged, u.Executable)
	if err := os.WriteFile(script, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write Windows update helper: %w", err)
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/B", script)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start Windows update helper: %w", err)
	}
	return "已下载 " + tag + "；退出本次命令后将完成更新。", nil
}
