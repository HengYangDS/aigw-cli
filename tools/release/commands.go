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

func buildCommands() commandSet {
	return commandSet{
		"accept-native": func(args []string, _ io.Writer) error {
			flags := flag.NewFlagSet("accept-native", flag.ContinueOnError)
			clients := flags.Bool("clients", false, "Also verify real clients through the native lifecycle")
			local := flags.Bool("local", false, "Consume exact-source local artifacts without public distribution claims")
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
				if err := verifyArtifacts(*artifacts, *local); err != nil {
					return err
				}
			}
			return construction.AcceptNative(*artifacts, *clients, *performance, *local)
		},
		"build": func(args []string, _ io.Writer) error {
			flags := flag.NewFlagSet("build", flag.ContinueOnError)
			local := flags.Bool("local", false, "Build exact-source local artifacts; never publish them")
			if err := flags.Parse(args); err != nil {
				return err
			}
			if err := requireArguments(flags.Args(), 1, "usage: release build [--local] <output-directory>"); err != nil {
				return err
			}
			return construction.Build(flags.Arg(0), *local)
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
	trust := artifact.SignatureTrust{
		AllowedSigners: os.Getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"),
		Principal:      os.Getenv("AIGW_RELEASE_ARTIFACT_SIGNER"),
	}
	source := artifact.SourceTrust{Repository: ".", AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")}
	return commandSet{
		"verify-artifacts": func(args []string, _ io.Writer) error {
			flags := flag.NewFlagSet("verify-artifacts", flag.ContinueOnError)
			local := flags.Bool("local", false, "Verify local artifacts against signed source rather than a release tag")
			if err := flags.Parse(args); err != nil {
				return err
			}
			if err := requireArguments(flags.Args(), 1, "usage: release verify-artifacts [--local] <artifact-directory>"); err != nil {
				return err
			}
			return verifyArtifacts(flags.Arg(0), *local)
		},
		"publish-github": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 1, "usage: release publish-github <artifact-directory>"); err != nil {
				return err
			}
			created, err := publication.PublishGitHub(context.Background(), http.DefaultClient, publication.GitHubConfig{
				APIBase: envDefault("GITHUB_API_URL", "https://api.github.com"), Repository: os.Getenv("GITHUB_REPOSITORY"),
				Tag: os.Getenv("CI_COMMIT_TAG"), Token: firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")), Artifacts: args[0],
				Trust:  trust,
				Source: source,
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
			return publication.UploadGitLab(context.Background(), http.DefaultClient, publication.GitLabConfig{
				APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
				JobToken: os.Getenv("CI_JOB_TOKEN"), AccessToken: os.Getenv("GITLAB_TOKEN"), Artifacts: args[0],
				Trust:  trust,
				Source: source,
			})
		},
		"publish-gitlab": func(args []string, stdout io.Writer) error {
			if err := requireArguments(args, 1, "usage: release publish-gitlab <artifact-directory>"); err != nil {
				return err
			}
			created, err := publication.PublishGitLab(context.Background(), http.DefaultClient, publication.GitLabConfig{
				APIBase: os.Getenv("CI_API_V4_URL"), ProjectID: os.Getenv("CI_PROJECT_ID"), Tag: os.Getenv("CI_COMMIT_TAG"),
				JobToken: os.Getenv("CI_JOB_TOKEN"), AccessToken: os.Getenv("GITLAB_TOKEN"), Artifacts: args[0],
				Trust:  trust,
				Source: source,
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

func verifyArtifacts(directory string, local bool) error {
	version, err := readiness.ReadDeliveryVersion(".", local)
	if err != nil {
		return err
	}
	trust := artifact.SignatureTrust{
		AllowedSigners: os.Getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"),
		Principal:      os.Getenv("AIGW_RELEASE_ARTIFACT_SIGNER"),
	}
	if err := artifact.VerifyMatrix(context.Background(), directory, version, trust); err != nil {
		return err
	}
	source := artifact.SourceTrust{Repository: ".", AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")}
	if local {
		return artifact.VerifyLocalProvenance(context.Background(), directory, version, source)
	}
	return artifact.VerifyProvenance(context.Background(), directory, os.Getenv("CI_COMMIT_TAG"), source)
}
