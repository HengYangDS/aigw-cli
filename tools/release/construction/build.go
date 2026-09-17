// Package construction builds reproducible AIGW release artifacts.
package construction

import (
	"aigw-cli/internal/process"
	"aigw-cli/internal/upgrade"
	"aigw-cli/tools/release/artifact"
	"aigw-cli/tools/release/readiness"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rogpeppe/go-internal/robustio"
)

type buildRequest struct {
	Root, Output, Version, Epoch   string
	GitLabOrigin, GitLabRepository string
	GitHubOrigin, GitHubRepository string
	SigningKey                     string
	TargetOS                       string
}

type toolCall struct {
	Name, Directory string
	Args, Env       []string
	Stdout          io.Writer
}

type toolRunner func(toolCall) error

type releaseBuilder func(buildRequest) error
type releaseEpochResolver func(root, version string) (string, error)
type artifactComparator func(left, right, version string) error

func buildRelease(ctx context.Context, request buildRequest, run toolRunner) (result error) {
	if err := validateRequest(request); err != nil {
		return err
	}
	if strings.TrimSpace(request.SigningKey) == "" {
		return errors.New("release construction requires AIGW_RELEASE_SIGNING_KEY")
	}
	if _, err := nativeSigningEnvironment(); err != nil {
		return err
	}
	if err := ensureCleanSource(request.Root, run); err != nil {
		return err
	}
	instant, _ := readiness.ParseEpoch(request.Epoch)
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
	defer func() {
		if err := robustio.RemoveAll(workspace); err != nil {
			cleanup := fmt.Errorf("remove release workspace %s: %w", workspace, err)
			if result == nil {
				cleanup = fmt.Errorf("release output published; %w", cleanup)
			}
			result = errors.Join(result, cleanup)
		}
	}()
	candidate := filepath.Join(workspace, "artifacts")
	stage, err := buildArchives(request, workspace, run)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(candidate, 0o755); err != nil {
		return fmt.Errorf("create release candidate: %w", err)
	}
	for _, name := range artifact.Archives(request.Version) {
		if err := copyFile(filepath.Join(stage, name), filepath.Join(candidate, name)); err != nil {
			return err
		}
	}
	sbom := filepath.Join(candidate, "aigw_"+request.Version+".spdx.json")
	rawSBOM := filepath.Join(stage, "aigw.spdx.json")
	if err := run(toolCall{Name: "syft", Directory: request.Root, Args: []string{"scan", "dir:" + stage, "--config", filepath.Join(request.Root, ".config", "release", "syft.yaml"), "--override-default-catalogers", "go-module-binary-cataloger,file", "--source-name", "aigw", "--source-version", request.Version, "-o", "spdx-json=" + rawSBOM}}); err != nil {
		return fmt.Errorf("generate release SBOM: %w", err)
	}
	if err := normalizeSPDX(rawSBOM, sbom, request.Version, instant); err != nil {
		return err
	}
	rawDependencies := filepath.Join(stage, "aigw.dependencies.json")
	lockfiles := []string{filepath.Join(request.Root, "go.mod"), filepath.Join(request.Root, "package-lock.json")}
	if err := run(toolCall{
		Name: "osv-scanner", Directory: request.Root,
		Args: []string{
			"scan", "source",
			"--config", filepath.Join(request.Root, ".config", "checks", "dependencies", "policy.toml"),
			"--lockfile", lockfiles[0],
			"--lockfile", lockfiles[1],
			"--no-call-analysis=go", "--format", "json", "--all-packages", "--licenses=",
			"--output-file", rawDependencies,
		},
	}); err != nil {
		return fmt.Errorf("scan release dependencies: %w", err)
	}
	if err := normalizeDependencyEvidence(
		rawDependencies,
		filepath.Join(candidate, "aigw_"+request.Version+".vulnerabilities.json"),
		filepath.Join(candidate, "aigw_"+request.Version+".licenses.json"),
		lockfiles,
	); err != nil {
		return err
	}
	commit, err := resolveGitObject(request.Root, "HEAD^{commit}", run)
	if err != nil {
		return err
	}
	tree, err := resolveGitObject(request.Root, "HEAD^{tree}", run)
	if err != nil {
		return err
	}
	if err := artifact.WriteProvenance(
		request.Root,
		candidate,
		filepath.Join(candidate, "aigw_"+request.Version+".provenance.json"),
		request.Version,
		commit,
		tree,
	); err != nil {
		return err
	}
	if err := artifact.RewriteChecksums(candidate, request.Version); err != nil {
		return err
	}
	manifest := filepath.Join(candidate, "checksums.txt")
	if err := run(toolCall{
		Name: "ssh-keygen", Directory: candidate,
		Args: []string{"-Y", "sign", "-n", artifact.SignatureNamespace, "-f", request.SigningKey, manifest},
	}); err != nil {
		return fmt.Errorf("sign release checksums: %w", err)
	}
	if err := artifact.ValidateMatrix(ctx, candidate, request.Version); err != nil {
		return err
	}
	return replaceDirectory(candidate, output)
}

func buildArchives(request buildRequest, workspace string, run toolRunner) (string, error) {
	var signing []string
	if request.TargetOS == "" || request.TargetOS == "darwin" {
		var err error
		signing, err = nativeSigningEnvironment()
		if err != nil {
			return "", err
		}
	}
	stage := filepath.Join(workspace, "goreleaser")
	config, err := renderGoReleaserConfig(request.Root, workspace, stage)
	if err != nil {
		return "", err
	}
	instant, err := readiness.ParseEpoch(request.Epoch)
	if err != nil {
		return "", err
	}
	environment := []string{
		"AIGW_BUILD_OS=" + request.TargetOS,
		"AIGW_VERSION=" + request.Version,
		"AIGW_RELEASE_EPOCH=" + request.Epoch,
		"AIGW_RELEASE_TIMESTAMP=" + instant.Format(time.RFC3339),
		"AIGW_GITLAB_RELEASE_ORIGIN=" + request.GitLabOrigin,
		"AIGW_GITLAB_RELEASE_REPOSITORY=" + request.GitLabRepository,
		"AIGW_GITHUB_RELEASE_ORIGIN=" + request.GitHubOrigin,
		"AIGW_GITHUB_RELEASE_REPOSITORY=" + request.GitHubRepository,
	}
	environment = append(environment, signing...)
	if err := run(toolCall{Name: "goreleaser", Directory: request.Root, Args: []string{"release", "--snapshot", "--clean", "--skip=publish", "--config", config}, Env: environment}); err != nil {
		return "", fmt.Errorf("build portable release artifacts: %w", err)
	}
	return stage, nil
}

func nativeSigningEnvironment() ([]string, error) {
	var environment []string
	for _, name := range []string{"AIGW_MACOS_SIGNING_P12", "AIGW_MACOS_SIGNING_PASSWORD_FILE", "AIGW_MACOS_SIGNING_REQUIREMENTS"} {
		path := os.Getenv(name)
		if path == "" || !filepath.IsAbs(path) {
			return nil, fmt.Errorf("release construction requires an absolute %s file", name)
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("release construction requires a regular %s file", name)
		}
		environment = append(environment, name+"="+path)
	}
	return environment, nil
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

func validateRequest(request buildRequest) error {
	if _, err := semver.StrictNewVersion(request.Version); err != nil {
		return fmt.Errorf("invalid release version %q: %w", request.Version, err)
	}
	if _, err := readiness.ParseEpoch(request.Epoch); err != nil {
		return err
	}
	return request.validateSources()
}

// ValidateSources validates independently configured Forge tuples.
// Local builds may configure neither source; either Forge may configure itself.
func ValidateSources() error {
	request := buildRequest{
		GitLabOrigin:     os.Getenv("AIGW_GITLAB_RELEASE_ORIGIN"),
		GitLabRepository: os.Getenv("AIGW_GITLAB_RELEASE_REPOSITORY"),
		GitHubOrigin:     os.Getenv("AIGW_GITHUB_RELEASE_ORIGIN"),
		GitHubRepository: os.Getenv("AIGW_GITHUB_RELEASE_REPOSITORY"),
	}
	return request.validateSources()
}

func (request buildRequest) validateSources() error {
	for _, source := range []upgrade.ReleaseSource{
		{Provider: upgrade.ReleaseProviderGitLab, Origin: request.GitLabOrigin, Repository: request.GitLabRepository},
		{Provider: upgrade.ReleaseProviderGitHub, Origin: request.GitHubOrigin, Repository: request.GitHubRepository},
	} {
		if source.Origin == "" && source.Repository == "" {
			continue
		}
		if err := source.Validate(); err != nil {
			return err
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

func replaceDirectory(source, target string) (result error) {
	_, statErr := os.Lstat(target)
	if os.IsNotExist(statErr) {
		if err := robustio.Rename(source, target); err != nil {
			return fmt.Errorf("publish release output: %w", err)
		}
		return nil
	}
	if statErr != nil {
		return fmt.Errorf("inspect release output: %w", statErr)
	}
	workspace, err := os.MkdirTemp(filepath.Dir(target), ".aigw-release-backup-")
	if err != nil {
		return fmt.Errorf("prepare release backup: %w", err)
	}
	backup := filepath.Join(workspace, "previous")
	preserve := false
	defer func() {
		if preserve {
			return
		}
		if err := robustio.RemoveAll(workspace); err != nil {
			cleanup := fmt.Errorf("remove release backup %s: %w", workspace, err)
			if result == nil {
				cleanup = fmt.Errorf("release output published; %w", cleanup)
			}
			result = errors.Join(result, cleanup)
		}
	}()
	if err := robustio.Rename(target, backup); err != nil {
		return fmt.Errorf("stage previous release output: %w", err)
	}
	if err := robustio.Rename(source, target); err != nil {
		if restoreErr := robustio.Rename(backup, target); restoreErr != nil {
			preserve = true
			return fmt.Errorf("publish release output: %w; restore previous output retained at %s: %w", err, backup, restoreErr)
		}
		return fmt.Errorf("publish release output: %w", err)
	}
	return nil
}

func executeTool(ctx context.Context) toolRunner {
	return func(call toolCall) error {
		stdout := call.Stdout
		if stdout == nil {
			stdout = os.Stdout
		}
		return (process.Runner{}).RunStream(ctx, process.Plan{
			Executable: call.Name, Directory: call.Directory,
			Args: call.Args, Env: append(os.Environ(), call.Env...),
		}, stdout, os.Stderr)
	}
}

func resolveGitObject(root, revision string, run toolRunner) (string, error) {
	var output bytes.Buffer
	if err := run(toolCall{Name: "git", Directory: root, Args: []string{"rev-parse", "--verify", revision}, Stdout: &output}); err != nil {
		return "", fmt.Errorf("resolve release source %s: %w", revision, err)
	}
	object := strings.TrimSpace(output.String())
	if matched, _ := regexp.MatchString(`^[0-9a-f]{40}(?:[0-9a-f]{24})?$`, object); !matched {
		return "", fmt.Errorf("resolve release source %s: invalid Git object %q", revision, object)
	}
	return object, nil
}

func ensureCleanSource(root string, run toolRunner) error {
	var output bytes.Buffer
	if err := run(toolCall{
		Name: "git", Directory: root,
		Args: []string{"status", "--porcelain=v1", "--untracked-files=all"}, Stdout: &output,
	}); err != nil {
		return fmt.Errorf("inspect release source: %w", err)
	}
	if strings.TrimSpace(output.String()) != "" {
		return errors.New("release construction requires committed source")
	}
	return nil
}

// Build constructs the portable release matrix for the current repository.
func Build(ctx context.Context, output string) error {
	request, err := buildRequestFromEnvironment(ctx, output)
	if err != nil {
		return err
	}
	return buildRelease(ctx, request, executeTool(ctx))
}

// BuildCI constructs the release twice and admits only an identical matrix.
func BuildCI(ctx context.Context, root, workspace, output string) error {
	return buildCI(root, workspace, output, func(request buildRequest) error {
		return buildRelease(ctx, request, executeTool(ctx))
	}, func(root, version string) (string, error) {
		return resolveReleaseEpoch(ctx, root, version)
	}, artifact.CompareMatrices)
}

func buildRequestFromEnvironment(ctx context.Context, output string) (buildRequest, error) {
	root, err := os.Getwd()
	if err != nil {
		return buildRequest{}, err
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		return buildRequest{}, err
	}
	epoch, err := resolveReleaseEpoch(ctx, root, version)
	if err != nil {
		return buildRequest{}, err
	}
	return buildRequest{
		Root: root, Version: version, Epoch: epoch, Output: output,
		GitLabOrigin: os.Getenv("AIGW_GITLAB_RELEASE_ORIGIN"), GitLabRepository: os.Getenv("AIGW_GITLAB_RELEASE_REPOSITORY"),
		GitHubOrigin: os.Getenv("AIGW_GITHUB_RELEASE_ORIGIN"), GitHubRepository: os.Getenv("AIGW_GITHUB_RELEASE_REPOSITORY"),
		SigningKey: os.Getenv("AIGW_RELEASE_SIGNING_KEY"),
	}, nil
}

func buildCI(root, workspace, output string, build releaseBuilder, epoch releaseEpochResolver, compare artifactComparator) error {
	tag := strings.TrimSpace(os.Getenv("CI_COMMIT_TAG"))
	if tag == "" {
		return errors.New("CI build requires CI_COMMIT_TAG")
	}
	if !strings.HasPrefix(tag, "v") {
		return fmt.Errorf("invalid CI release tag %q", tag)
	}
	version := strings.TrimPrefix(tag, "v")
	if _, err := semver.StrictNewVersion(version); err != nil {
		return fmt.Errorf("invalid CI release version %q: %w", version, err)
	}
	carrier, err := readiness.ReadProductVersion(root)
	if err != nil {
		return err
	}
	if carrier != version {
		return fmt.Errorf("CI tag version %q disagrees with VERSION %q", version, carrier)
	}
	releaseEpoch, err := epoch(root, version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("prepare release output parent: %w", err)
	}
	first := filepath.Join(workspace, "first")
	second := filepath.Join(workspace, "second")
	request := buildRequest{
		Root: root, Version: version, Epoch: releaseEpoch,
		GitLabOrigin: os.Getenv("AIGW_GITLAB_RELEASE_ORIGIN"), GitLabRepository: os.Getenv("AIGW_GITLAB_RELEASE_REPOSITORY"),
		GitHubOrigin: os.Getenv("AIGW_GITHUB_RELEASE_ORIGIN"), GitHubRepository: os.Getenv("AIGW_GITHUB_RELEASE_REPOSITORY"),
		SigningKey: os.Getenv("AIGW_RELEASE_SIGNING_KEY"),
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

func resolveReleaseEpoch(ctx context.Context, root, version string) (string, error) {
	file, err := os.Open(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return "", fmt.Errorf("open CHANGELOG.md: %w", err)
	}
	defer func() { _ = file.Close() }()
	pattern := regexp.MustCompile(`^## \[([^]]+)] - (\d{4}-\d{2}-\d{2})$`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		match := pattern.FindStringSubmatch(scanner.Text())
		if len(match) < 3 || match[1] != version {
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
	if os.Getenv("CI_COMMIT_TAG") != "" || os.Getenv("GITHUB_REF_TYPE") == "tag" {
		return "", fmt.Errorf("release heading not found: %s", version)
	}
	var output bytes.Buffer
	if err := executeTool(ctx)(toolCall{Name: "git", Directory: root, Args: []string{"show", "-s", "--format=%ct", "HEAD"}, Stdout: &output}); err != nil {
		return "", fmt.Errorf("resolve candidate source epoch: %w", err)
	}
	epoch := strings.TrimSpace(output.String())
	if _, err := readiness.ParseEpoch(epoch); err != nil {
		return "", fmt.Errorf("candidate source epoch: %w", err)
	}
	return epoch, nil
}
