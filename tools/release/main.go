// Command release owns deterministic package metadata and GitLab release validation.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type releaseLink struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	DirectAssetPath string `json:"direct_asset_path"`
}

type releaseAssets struct {
	Links []releaseLink `json:"links"`
}

type releasePayload struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Assets      releaseAssets `json:"assets"`
}

type remoteRelease struct {
	TagName string `json:"tag_name"`
	Assets  struct {
		Links []struct {
			URL string `json:"url"`
		} `json:"links"`
	} `json:"assets"`
}

func main() { os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr)) }

func execute(args []string, stdout, stderr io.Writer) int {
	if err := run(args, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: release <build|build-ci|validate-release-sources|validate-toolchain|validate-readiness|validate-readiness-tag|validate-readiness-doc|validate-artifacts|compare-artifacts|upload-gitlab|publish-github|publish-gitlab|write-gitlab-release|same-authority|resolve-redirect|verify-gitlab-release|project-gitlab-response|validate-gitlab-release>")
	}
	switch args[0] {
	case "build":
		request, err := parseBuildArguments(args[1:])
		if err != nil {
			return err
		}
		return buildRelease(request, execTool)
	case "build-ci":
		if len(args) != 3 {
			return errors.New("usage: release build-ci <workspace> <output-directory>")
		}
		root, err := os.Getwd()
		if err != nil {
			return err
		}
		return buildCI(root, args[1], args[2], func(request buildRequest) error { return buildRelease(request, execTool) }, resolveReleaseEpoch, compareArtifactMatrices)
	case "validate-release-sources":
		if len(args) != 1 {
			return errors.New("usage: release validate-release-sources")
		}
		return validateBuildReleaseSources()
	case "validate-toolchain":
		if len(args) != 2 {
			return errors.New("usage: release validate-toolchain <go.mod>")
		}
		return validateToolchain(args[1], runtime.Version())
	case "validate-readiness":
		if len(args) != 2 {
			return errors.New("usage: release validate-readiness <version>")
		}
		return validateReleaseReadiness(args[1])
	case "validate-readiness-tag":
		if len(args) != 1 {
			return errors.New("usage: release validate-readiness-tag")
		}
		tag := os.Getenv("CI_COMMIT_TAG")
		if !strings.HasPrefix(tag, "v") {
			return errors.New("CI_COMMIT_TAG must use v<semver>")
		}
		return validateReleaseReadiness(strings.TrimPrefix(tag, "v"))
	case "validate-readiness-doc":
		if len(args) != 2 {
			return errors.New("usage: release validate-readiness-doc <document>")
		}
		return validateReleaseReadinessDocument(args[1])
	case "validate-artifacts":
		if len(args) != 3 {
			return errors.New("usage: release validate-artifacts <directory> <version>")
		}
		return validateArtifactMatrix(args[1], args[2])
	case "compare-artifacts":
		if len(args) != 4 {
			return errors.New("usage: release compare-artifacts <left-directory> <right-directory> <version>")
		}
		return compareArtifactMatrices(args[1], args[2], args[3])
	case "publish-github":
		if len(args) != 2 {
			return errors.New("usage: release publish-github <artifact-directory>")
		}
		created, err := publishGitHubRelease(context.Background(), http.DefaultClient, githubPublishConfig{
			APIBase: envDefault("GITHUB_API_URL", "https://api.github.com"), Repository: os.Getenv("GITHUB_REPOSITORY"),
			Tag: os.Getenv("CI_COMMIT_TAG"), Token: firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")), Artifacts: args[1],
		})
		if err == nil {
			_, _ = fmt.Fprintf(stdout, "GitHub release verified (created=%t)\n", created)
		}
		return err
	case "upload-gitlab":
		if len(args) != 2 {
			return errors.New("usage: release upload-gitlab <artifact-directory>")
		}
		return uploadGitLabArtifacts(context.Background(), http.DefaultClient, gitLabPublishConfig{
			APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
			Token: os.Getenv("CI_JOB_TOKEN"), Artifacts: args[1],
		})
	case "publish-gitlab":
		if len(args) != 2 {
			return errors.New("usage: release publish-gitlab <artifact-directory>")
		}
		created, err := publishGitLabRelease(context.Background(), http.DefaultClient, gitLabPublishConfig{
			APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
			Token: os.Getenv("CI_JOB_TOKEN"), Artifacts: args[1],
		})
		if err == nil {
			_, _ = fmt.Fprintf(stdout, "GitLab release verified (created=%t)\n", created)
		}
		return err
	case "write-gitlab-release":
		if len(args) != 4 {
			return errors.New("usage: release write-gitlab-release <tag> <base-url> <output>")
		}
		return writeJSON(args[3], releaseDocument(args[1], args[2]))
	case "same-authority":
		if len(args) != 3 {
			return errors.New("usage: release same-authority <left-url> <right-url>")
		}
		same, err := sameAuthority(args[1], args[2])
		if err != nil {
			return err
		}
		if same {
			_, _ = fmt.Fprintln(stdout, "yes")
		} else {
			_, _ = fmt.Fprintln(stdout, "no")
		}
		return nil
	case "resolve-redirect":
		if len(args) != 3 {
			return errors.New("usage: release resolve-redirect <current-url> <headers-file>")
		}
		location, err := readLocation(args[2])
		if err != nil {
			return err
		}
		resolved, err := resolveRedirect(args[1], location)
		if err == nil {
			_, _ = fmt.Fprintln(stdout, resolved)
		}
		return err
	case "verify-gitlab-release":
		if len(args) != 5 {
			return errors.New("usage: release verify-gitlab-release <expected-json> <actual-json> <asset-list> <tag>")
		}
		return verifyGitLabRelease(args[1], args[2], args[3], args[4])
	case "project-gitlab-response":
		if len(args) != 4 {
			return errors.New("usage: release project-gitlab-response <release-json> <mode> <output>")
		}
		var expected releasePayload
		if err := readJSON(args[1], &expected); err != nil {
			return err
		}
		projected, err := projectGitLabResponse(expected, args[2])
		if err != nil {
			return err
		}
		return writeJSON(args[3], projected)
	case "validate-gitlab-release":
		if len(args) != 3 {
			return errors.New("usage: release validate-gitlab-release <release-json> <tag>")
		}
		var payload releasePayload
		if err := readJSON(args[1], &payload); err != nil {
			return err
		}
		return validateReleaseDocument(payload, args[2])
	default:
		return fmt.Errorf("unknown release command: %s", args[0])
	}
}
