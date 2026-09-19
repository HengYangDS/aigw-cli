package main

import (
	"aigw-cli/tools/release/artifact"
	"aigw-cli/tools/release/construction"
	"aigw-cli/tools/release/publication"
	"aigw-cli/tools/release/readiness"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

func buildCommands(ctx context.Context) commandSet {
	return commandSet{
		"accept-native": func(args []string, _ io.Writer) error {
			flags := flag.NewFlagSet("accept-native", flag.ContinueOnError)
			clients := flags.Bool("clients", false, "Also verify real clients through the native lifecycle")
			artifacts := flags.String("artifacts", "", "Consume an existing signed release matrix instead of building")
			performance := flags.String("performance", "", "Retain Hyperfine measurements in this absolute output directory")
			if err := flags.Parse(args); err != nil {
				return err
			}
			if err := requireArguments(flags.Args(), 0, "usage: release accept-native [--artifacts <directory>] [--clients] [--performance <absolute-directory>]"); err != nil {
				return err
			}
			if *performance != "" && (*artifacts == "" || strings.TrimSpace(os.Getenv("AIGW_ACCEPTANCE_BASELINE")) == "") {
				return errors.New("performance acceptance requires an explicit published candidate and baseline")
			}
			if *artifacts != "" {
				if err := verifyArtifacts(ctx, *artifacts); err != nil {
					return err
				}
			}
			return construction.AcceptNative(ctx, *artifacts, *clients, *performance)
		},
		"build": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release build <output-directory>"); err != nil {
				return err
			}
			return construction.Build(ctx, args[0])
		},
		"build-ci": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 2, "usage: release build-ci <workspace> <output-directory>"); err != nil {
				return err
			}
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			return construction.BuildCI(ctx, root, args[0], args[1])
		},
	}
}

func policyCommands() commandSet {
	return commandSet{
		"validate-changelog": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 0, "usage: release validate-changelog"); err != nil {
				return err
			}
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			return readiness.ValidateChangelog(root, "CHANGELOG.md", readiness.SelectedReleaseTag())
		},
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
		"validate-version": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release validate-version <version>"); err != nil {
				return err
			}
			return readiness.ValidateVersion(args[0])
		},
		"validate-version-tag": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 0, "usage: release validate-version-tag"); err != nil {
				return err
			}
			tag := os.Getenv("CI_COMMIT_TAG")
			if !strings.HasPrefix(tag, "v") {
				return errors.New("CI_COMMIT_TAG must use v<semver>")
			}
			return readiness.ValidateVersion(strings.TrimPrefix(tag, "v"))
		},
	}
}

func artifactCommands(ctx context.Context) commandSet {
	return commandSet{
		"verify-macos-distribution": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 6, "usage: release verify-macos-distribution <artifact-directory> <version> <certificate-fingerprint> <uploaded-zip> <submission-id> <keychain-profile>"); err != nil {
				return err
			}
			return construction.VerifyMacOSDistribution(ctx, args[0], args[1], args[2], construction.Notarization{Archive: args[3], SubmissionID: args[4], KeychainProfile: args[5]})
		},
		"validate-artifacts": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 2, "usage: release validate-artifacts <directory> <version>"); err != nil {
				return err
			}
			return artifact.ValidateMatrix(ctx, args[0], args[1])
		},
		"compare-artifacts": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 3, "usage: release compare-artifacts <left-directory> <right-directory> <version>"); err != nil {
				return err
			}
			return artifact.CompareMatrices(args[0], args[1], args[2])
		},
	}
}

func publicationCommands(ctx context.Context) commandSet {
	trust := artifact.SignatureTrust{
		AllowedSigners: os.Getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"),
		Principal:      os.Getenv("AIGW_RELEASE_ARTIFACT_SIGNER"),
	}
	source := artifact.SourceTrust{Repository: ".", AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")}
	return commandSet{
		"verify-artifacts": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release verify-artifacts <artifact-directory>"); err != nil {
				return err
			}
			return verifyArtifacts(ctx, args[0])
		},
		"publish-github": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 1, "usage: release publish-github <artifact-directory>"); err != nil {
				return err
			}
			created, err := publication.PublishGitHub(ctx, http.DefaultClient, publication.GitHubConfig{
				APIBase: envDefault("GITHUB_API_URL", "https://api.github.com"), Repository: os.Getenv("GITHUB_REPOSITORY"),
				Tag: os.Getenv("CI_COMMIT_TAG"), Token: firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")), Artifacts: args[0],
				Trust:              trust,
				Source:             source,
				VerifyDistribution: verifyMacOSPublication,
			})
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "GitHub release verified (created=%t)\n", created); err != nil {
				return fmt.Errorf("GitHub release verified; report failed: %w", err)
			}
			return nil
		},
		"upload-gitlab": func(args []string, _ io.Writer) error {
			if err := requireArguments(args, 1, "usage: release upload-gitlab <artifact-directory>"); err != nil {
				return err
			}
			return publication.UploadGitLab(ctx, http.DefaultClient, publication.GitLabConfig{
				APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
				JobToken: os.Getenv("CI_JOB_TOKEN"), AccessToken: os.Getenv("GITLAB_TOKEN"), Artifacts: args[0],
				Trust:              trust,
				Source:             source,
				VerifyDistribution: verifyMacOSPublication,
			})
		},
		"publish-gitlab": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 1, "usage: release publish-gitlab <artifact-directory>"); err != nil {
				return err
			}
			created, err := publication.PublishGitLab(ctx, http.DefaultClient, publication.GitLabConfig{
				APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
				JobToken: os.Getenv("CI_JOB_TOKEN"), AccessToken: os.Getenv("GITLAB_TOKEN"), Artifacts: args[0],
				Trust:              trust,
				Source:             source,
				VerifyDistribution: verifyMacOSPublication,
			})
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "GitLab release verified (created=%t)\n", created); err != nil {
				return fmt.Errorf("GitLab release verified; report failed: %w", err)
			}
			return nil
		},
	}
}

func verifyArtifacts(ctx context.Context, directory string) error {
	version, err := readiness.ReadProductVersion(".")
	if err != nil {
		return err
	}
	trust := artifact.SignatureTrust{
		AllowedSigners: os.Getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"),
		Principal:      os.Getenv("AIGW_RELEASE_ARTIFACT_SIGNER"),
	}
	if err := artifact.VerifyMatrix(ctx, directory, version, trust); err != nil {
		return err
	}
	source := artifact.SourceTrust{Repository: ".", AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")}
	return artifact.VerifyProvenance(ctx, directory, os.Getenv("CI_COMMIT_TAG"), source)
}

func verifyMacOSPublication(ctx context.Context, directory, version string) error {
	return construction.VerifyMacOSDistribution(ctx, directory, version, os.Getenv("AIGW_MACOS_SIGNING_IDENTITY"), construction.Notarization{
		Archive: os.Getenv("AIGW_MACOS_NOTARY_ARCHIVE"), SubmissionID: os.Getenv("AIGW_MACOS_NOTARY_SUBMISSION"), KeychainProfile: os.Getenv("AIGW_MACOS_NOTARY_PROFILE"),
	})
}
