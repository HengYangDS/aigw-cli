package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const project = "dig/misc/agentic-third-party-api/aigw-cli"

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

type Updater struct {
	GOOS       string
	GOARCH     string
	Executable string
	Runner     CommandRunner
}

func Current(executable string) Updater {
	return Updater{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Executable: executable, Runner: ExecRunner{}}
}

func (u Updater) Update(ctx context.Context, currentVersion string) (string, error) {
	if u.Runner == nil {
		u.Runner = ExecRunner{}
	}
	tag, err := u.latestTag(ctx)
	if err != nil {
		return "", err
	}
	if normalizeVersion(tag) == normalizeVersion(currentVersion) {
		return "已经是最新版 " + tag + "。", nil
	}
	version := normalizeVersion(tag)
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
	for _, asset := range []string{archiveName, "checksums.txt"} {
		if _, err := u.Runner.Run(ctx, "glab", "release", "download", tag, "-R", project, "--asset-name", asset, "--dir", tmp); err != nil {
			return "", fmt.Errorf("download release asset %s: %w", asset, err)
		}
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

func (u Updater) latestTag(ctx context.Context) (string, error) {
	output, err := u.Runner.Run(ctx, "glab", "release", "list", "-R", project, "--per-page", "1", "--format", "json")
	if err != nil {
		return "", fmt.Errorf("query latest release: %w", err)
	}
	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(output, &releases); err != nil {
		return "", fmt.Errorf("parse latest release: %w", err)
	}
	if len(releases) == 0 || releases[0].TagName == "" {
		return "", fmt.Errorf("no AIGW release is available")
	}
	return releases[0].TagName, nil
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
