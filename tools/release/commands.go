package main

import (
	"aigw-cli/tools/release/artifact"
	"aigw-cli/tools/release/construction"
	"aigw-cli/tools/release/readiness"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

func buildCommands() commandSet {
	return commandSet{
		"build": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release build <output-directory>"); err != nil {
				return err
			}
			return construction.Build(args[0])
		},
		"build-ci": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 2, "usage: release build-ci <workspace> <output-directory>"); err != nil {
				return err
			}
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			return construction.BuildCI(root, args[0], args[1])
		},
	}
}

func policyCommands() commandSet {
	return commandSet{
		"validate-release-sources": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 0, "usage: release validate-release-sources"); err != nil {
				return err
			}
			return construction.ValidateSources()
		},
		"validate-toolchain": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release validate-toolchain <go.mod>"); err != nil {
				return err
			}
			return readiness.ValidateToolchain(args[0], runtime.Version())
		},
		"validate-readiness": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release validate-readiness <version>"); err != nil {
				return err
			}
			return readiness.ValidateVersion(args[0])
		},
		"validate-readiness-tag": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 0, "usage: release validate-readiness-tag"); err != nil {
				return err
			}
			tag := os.Getenv("CI_COMMIT_TAG")
			if !strings.HasPrefix(tag, "v") {
				return errors.New("CI_COMMIT_TAG must use v<semver>")
			}
			return readiness.ValidateVersion(strings.TrimPrefix(tag, "v"))
		},
		"validate-readiness-doc": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release validate-readiness-doc <document>"); err != nil {
				return err
			}
			return readiness.ValidateDocument(args[0])
		},
	}
}

func artifactCommands() commandSet {
	return commandSet{
		"validate-artifacts": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 2, "usage: release validate-artifacts <directory> <version>"); err != nil {
				return err
			}
			return artifact.ValidateMatrix(args[0], args[1])
		},
		"compare-artifacts": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 3, "usage: release compare-artifacts <left-directory> <right-directory> <version>"); err != nil {
				return err
			}
			return artifact.CompareMatrices(args[0], args[1], args[2])
		},
	}
}

func publicationCommands() commandSet {
	return commandSet{
		"publish-github": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 1, "usage: release publish-github <artifact-directory>"); err != nil {
				return err
			}
			created, err := publishGitHubRelease(context.Background(), http.DefaultClient, githubPublishConfig{
				APIBase: envDefault("GITHUB_API_URL", "https://api.github.com"), Repository: os.Getenv("GITHUB_REPOSITORY"),
				Tag: os.Getenv("CI_COMMIT_TAG"), Token: firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")), Artifacts: args[0],
			})
			if err == nil {
				_, _ = fmt.Fprintf(stdout, "GitHub release verified (created=%t)\n", created)
			}
			return err
		},
		"upload-gitlab": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release upload-gitlab <artifact-directory>"); err != nil {
				return err
			}
			return uploadGitLabArtifacts(context.Background(), http.DefaultClient, gitLabPublishConfig{
				APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
				Token: os.Getenv("CI_JOB_TOKEN"), Artifacts: args[0],
			})
		},
		"publish-gitlab": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 1, "usage: release publish-gitlab <artifact-directory>"); err != nil {
				return err
			}
			created, err := publishGitLabRelease(context.Background(), http.DefaultClient, gitLabPublishConfig{
				APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
				Token: os.Getenv("CI_JOB_TOKEN"), Artifacts: args[0],
			})
			if err == nil {
				_, _ = fmt.Fprintf(stdout, "GitLab release verified (created=%t)\n", created)
			}
			return err
		},
	}
}
