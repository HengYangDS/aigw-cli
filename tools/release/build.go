package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type buildRequest struct {
	Root, Output, Version, Epoch   string
	GitLabOrigin, GitLabRepository string
	GitHubOrigin, GitHubRepository string
}

type toolCall struct {
	Name, Directory string
	Args, Env       []string
}

type toolRunner func(toolCall) error

type releaseBuilder func(buildRequest) error
type releaseEpochResolver func(root, version string) (string, error)
type artifactComparator func(left, right, version string) error

var releaseVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)

func buildRelease(request buildRequest, run toolRunner) error {
	if err := validateBuildRequest(request); err != nil {
		return err
	}
	instant, _ := parseEpoch(request.Epoch)
	output, err := filepath.Abs(request.Output)
	if err != nil {
		return fmt.Errorf("resolve release output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create release output parent: %w", err)
	}
	workspace, err := os.MkdirTemp(filepath.Dir(output), ".aigw-release-*")
	if err != nil {
		return fmt.Errorf("create release workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	stage := filepath.Join(workspace, "goreleaser")
	candidate := filepath.Join(workspace, "artifacts")
	config, err := renderGoReleaserConfig(request.Root, workspace, stage)
	if err != nil {
		return err
	}
	environment := []string{
		"AIGW_VERSION=" + request.Version,
		"AIGW_RELEASE_EPOCH=" + request.Epoch,
		"AIGW_RELEASE_TIMESTAMP=" + instant.Format(time.RFC3339),
		"AIGW_GITLAB_RELEASE_ORIGIN=" + request.GitLabOrigin,
		"AIGW_GITLAB_RELEASE_REPOSITORY=" + request.GitLabRepository,
		"AIGW_GITHUB_RELEASE_ORIGIN=" + request.GitHubOrigin,
		"AIGW_GITHUB_RELEASE_REPOSITORY=" + request.GitHubRepository,
	}
	if err := run(toolCall{Name: "goreleaser", Directory: request.Root, Args: []string{"release", "--snapshot", "--clean", "--skip=publish", "--config", config}, Env: environment}); err != nil {
		return fmt.Errorf("build portable release artifacts: %w", err)
	}
	if err := os.MkdirAll(candidate, 0o755); err != nil {
		return fmt.Errorf("create release candidate: %w", err)
	}
	for _, name := range portableArtifactNames(request.Version) {
		if name == "checksums.txt" || strings.HasSuffix(name, ".spdx.json") {
			continue
		}
		if err := copyFile(filepath.Join(stage, name), filepath.Join(candidate, name)); err != nil {
			return err
		}
	}
	binary, err := firstPortableBinary(stage)
	if err != nil {
		return err
	}
	sbom := filepath.Join(candidate, "aigw_"+request.Version+".spdx.json")
	rawSBOM := filepath.Join(stage, "aigw.spdx.json")
	if err := run(toolCall{Name: "syft", Directory: request.Root, Args: []string{"scan", "file:" + binary, "--source-name", "aigw", "--source-version", request.Version, "-o", "spdx-json=" + rawSBOM}, Env: environment}); err != nil {
		return fmt.Errorf("generate release SBOM: %w", err)
	}
	if err := normalizeSPDX(rawSBOM, sbom, request.Version, instant); err != nil {
		return err
	}
	if err := rewritePortableChecksums(candidate, request.Version); err != nil {
		return err
	}
	if err := validatePortableArtifactMatrix(candidate, request.Version); err != nil {
		return err
	}
	return replaceDirectory(candidate, output)
}

func renderGoReleaserConfig(root, workspace, stage string) (string, error) {
	source := filepath.Join(root, ".config", "release", "goreleaser.yaml")
	data, err := os.ReadFile(source)
	if err != nil {
		return "", fmt.Errorf("read GoReleaser config: %w", err)
	}
	config := filepath.Join(workspace, "goreleaser.yaml")
	content := append(data, []byte("\ndist: "+strconv.Quote(stage)+"\n")...)
	if err := os.WriteFile(config, content, 0o600); err != nil {
		return "", fmt.Errorf("write GoReleaser config: %w", err)
	}
	return config, nil
}

func validateBuildRequest(request buildRequest) error {
	if !releaseVersion.MatchString(request.Version) {
		return fmt.Errorf("invalid release version %q", request.Version)
	}
	if _, err := parseEpoch(request.Epoch); err != nil {
		return err
	}
	for name, tuple := range map[string][2]string{
		"GitLab": {request.GitLabOrigin, request.GitLabRepository},
		"GitHub": {request.GitHubOrigin, request.GitHubRepository},
	} {
		if (tuple[0] == "") != (tuple[1] == "") {
			return fmt.Errorf("%s release configuration requires origin and repository together", name)
		}
		if tuple[0] == "" {
			continue
		}
		parsed, err := url.Parse(tuple[0])
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return fmt.Errorf("%s release origin must be an HTTPS authority", name)
		}
		if strings.ContainsAny(tuple[1], " \t\r\n") {
			return fmt.Errorf("%s release repository contains whitespace", name)
		}
	}
	return nil
}

// validateBuildReleaseSources validates independently configured Forge tuples.
// Local builds may configure neither source; either Forge may configure itself.
func validateBuildReleaseSources() error {
	request := buildRequest{
		GitLabOrigin:     os.Getenv("AIGW_GITLAB_RELEASE_ORIGIN"),
		GitLabRepository: os.Getenv("AIGW_GITLAB_RELEASE_REPOSITORY"),
		GitHubOrigin:     os.Getenv("AIGW_GITHUB_RELEASE_ORIGIN"),
		GitHubRepository: os.Getenv("AIGW_GITHUB_RELEASE_REPOSITORY"),
	}
	for name, tuple := range map[string][2]string{
		"GitLab": {request.GitLabOrigin, request.GitLabRepository},
		"GitHub": {request.GitHubOrigin, request.GitHubRepository},
	} {
		if (tuple[0] == "") != (tuple[1] == "") {
			return fmt.Errorf("%s release source is incomplete; set origin and repository together", name)
		}
		if tuple[0] == "" {
			continue
		}
		parsed, err := url.Parse(tuple[0])
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return fmt.Errorf("%s release origin must be an HTTPS authority", name)
		}
		if strings.Trim(tuple[1], "/") != tuple[1] || strings.ContainsAny(tuple[1], " \\?#\t\r\n") {
			return fmt.Errorf("%s release repository must be a namespace/project path", name)
		}
		parts := strings.Split(tuple[1], "/")
		if name == "GitHub" && len(parts) != 2 {
			return errors.New("GitHub release repository must be an owner/repository path")
		}
		for _, part := range parts {
			if part == "" || part == "." || part == ".." {
				return fmt.Errorf("%s release repository must be a namespace/project path", name)
			}
		}
	}
	return nil
}

func copyFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read release artifact %s: %w", filepath.Base(source), err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		return fmt.Errorf("write release artifact %s: %w", filepath.Base(target), err)
	}
	return nil
}

func replaceDirectory(source, target string) error {
	backup := target + ".previous"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("clear prior release backup: %w", err)
	}
	_, statErr := os.Stat(target)
	hadTarget := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect release output: %w", statErr)
	}
	if hadTarget {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("stage previous release output: %w", err)
		}
	}
	if err := os.Rename(source, target); err != nil {
		if hadTarget {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("publish release output: %w", err)
	}
	if hadTarget {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous release output: %w", err)
		}
	}
	return nil
}

func execTool(call toolCall) error {
	command := exec.Command(call.Name, call.Args...)
	command.Dir = call.Directory
	command.Env = append(os.Environ(), call.Env...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func firstPortableBinary(stage string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(stage, "portable_*", "aigw*"))
	if err != nil || len(matches) == 0 {
		return "", errors.New("GoReleaser produced no portable binary for SBOM generation")
	}
	return matches[0], nil
}

func normalizeSPDX(source, target, version string, instant time.Time) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read Syft SPDX document: %w", err)
	}
	for _, forbidden := range []string{"/Users/", "/home/", "/private/tmp/", `:\\Users\\`} {
		if strings.Contains(string(data), forbidden) {
			return fmt.Errorf("Syft SPDX document contains a host-local path: %s", forbidden)
		}
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode Syft SPDX document: %w", err)
	}
	creation, _ := document["creationInfo"].(map[string]any)
	if creation == nil {
		creation = map[string]any{}
		document["creationInfo"] = creation
	}
	creation["created"] = instant.Format(time.RFC3339)
	digest := sha256.Sum256([]byte(version + "\x00" + instant.Format(time.RFC3339)))
	document["documentNamespace"] = fmt.Sprintf("urn:sha256:%x", digest)
	normalized, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode deterministic SPDX document: %w", err)
	}
	return os.WriteFile(target, append(normalized, '\n'), 0o600)
}

func parseBuildArguments(args []string) (buildRequest, error) {
	if len(args) != 3 {
		return buildRequest{}, errors.New("usage: release build <version> <source-date-epoch> <output-directory>")
	}
	root, err := os.Getwd()
	if err != nil {
		return buildRequest{}, err
	}
	return buildRequest{
		Root: root, Version: args[0], Epoch: args[1], Output: args[2],
		GitLabOrigin: os.Getenv("AIGW_GITLAB_RELEASE_ORIGIN"), GitLabRepository: os.Getenv("AIGW_GITLAB_RELEASE_REPOSITORY"),
		GitHubOrigin: os.Getenv("AIGW_GITHUB_RELEASE_ORIGIN"), GitHubRepository: os.Getenv("AIGW_GITHUB_RELEASE_REPOSITORY"),
	}, nil
}

func buildCI(root, workspace, output string, build releaseBuilder, epoch releaseEpochResolver, compare artifactComparator) error {
	tag := strings.TrimSpace(os.Getenv("CI_COMMIT_TAG"))
	version := strings.TrimPrefix(tag, "v")
	if version == "" {
		short := strings.TrimSpace(os.Getenv("CI_COMMIT_SHORT_SHA"))
		if short == "" {
			return errors.New("CI build requires CI_COMMIT_TAG or CI_COMMIT_SHORT_SHA")
		}
		version = "0.1.0-" + short
	}
	if !releaseVersion.MatchString(version) {
		return fmt.Errorf("invalid CI release version %q", version)
	}
	releaseEpoch, err := epoch(root, version)
	if err != nil {
		return err
	}
	first := filepath.Join(workspace, "first")
	second := filepath.Join(workspace, "second")
	request := buildRequest{
		Root: root, Version: version, Epoch: releaseEpoch,
		GitLabOrigin: os.Getenv("AIGW_GITLAB_RELEASE_ORIGIN"), GitLabRepository: os.Getenv("AIGW_GITLAB_RELEASE_REPOSITORY"),
		GitHubOrigin: os.Getenv("AIGW_GITHUB_RELEASE_ORIGIN"), GitHubRepository: os.Getenv("AIGW_GITHUB_RELEASE_REPOSITORY"),
	}
	request.Output = first
	if err := build(request); err != nil {
		return err
	}
	request.Output = second
	if err := build(request); err != nil {
		return err
	}
	if err := compare(first, second, version); err != nil {
		return err
	}
	return replaceDirectory(first, output)
}

func resolveReleaseEpoch(root, version string) (string, error) {
	if os.Getenv("CI_COMMIT_TAG") == "" {
		command := exec.Command("git", "-C", root, "log", "-1", "--format=%ct")
		output, err := command.Output()
		if err != nil {
			return "", fmt.Errorf("read source commit epoch: %w", err)
		}
		return strings.TrimSpace(string(output)), nil
	}
	file, err := os.Open(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return "", fmt.Errorf("open CHANGELOG.md: %w", err)
	}
	defer func() { _ = file.Close() }()
	pattern := regexp.MustCompile(`^## \[([^]]+)] - (\d{4}-\d{2}-\d{2})$`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		match := pattern.FindStringSubmatch(scanner.Text())
		if match == nil || match[1] != version {
			continue
		}
		date, parseErr := time.Parse("2006-01-02", match[2])
		if parseErr != nil {
			return "", parseErr
		}
		return fmt.Sprint(date.Unix()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("release heading not found: %s", version)
}
