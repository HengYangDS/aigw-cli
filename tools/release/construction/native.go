package construction

import (
	"aigw-cli/internal/upgrade/artifact"
	releaseartifact "aigw-cli/tools/release/artifact"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rogpeppe/go-internal/robustio"
)

// NativeAcceptance selects exact artifacts and optional native verification scopes.
type NativeAcceptance struct {
	Artifacts, CandidateSource, BaselineArtifacts, BaselineTag              string
	InputPackage, InputRelease, InputArchive, InputSHA256, Peer, Repository string
	Tag                                                                     string
	Candidate, Clients                                                      bool
	Performance                                                             string
}

// ParseNativeAcceptance owns the native artifact and test input contract.
func ParseNativeAcceptance(arguments []string) (NativeAcceptance, error) {
	input := NativeAcceptance{}
	flags := flag.NewFlagSet("accept-native", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&input.Artifacts, "artifacts", "", "Consume an existing signed release matrix")
	flags.StringVar(&input.Tag, "tag", "", "Download a published candidate from the selected peer")
	flags.BoolVar(&input.Candidate, "candidate", false, "Bind untagged artifacts to signed source")
	flags.StringVar(&input.CandidateSource, "candidate-source", "", "Exact signed candidate commit")
	flags.StringVar(&input.BaselineArtifacts, "baseline-artifacts", "", "Consume an existing signed published predecessor matrix")
	flags.StringVar(&input.BaselineTag, "baseline-tag", "", "Select the published predecessor tag")
	flags.StringVar(&input.InputPackage, "input-package", "", "Download signed matrices from a GitLab generic package")
	flags.StringVar(&input.InputRelease, "input-release", "", "Download signed matrices from a signed GitHub input carrier")
	flags.StringVar(&input.InputArchive, "input-archive", "", "Consume a caller-owned native input archive")
	flags.StringVar(&input.InputSHA256, "input-sha256", "", "Exact SHA256 of the native input archive")
	flags.StringVar(&input.Peer, "peer", "", "Explicit package transport")
	flags.StringVar(&input.Repository, "repository", "", "Repository for the package transport")
	flags.BoolVar(&input.Clients, "clients", false, "Verify real clients through the native lifecycle")
	flags.StringVar(&input.Performance, "performance", "", "Retain Hyperfine measurements in this absolute directory")
	if err := flags.Parse(arguments); err != nil {
		return input, err
	}
	if len(flags.Args()) != 0 {
		return input, errors.New("native acceptance does not take positional arguments")
	}
	err := input.validate()
	return input, err
}

func (input *NativeAcceptance) validate() error {
	if err := input.validatePackage(); err != nil {
		return err
	}
	if input.Peer != "" && (input.Peer != "github" && input.Peer != "gitlab" || input.Repository == "") {
		return errors.New("native transport requires an explicit github or gitlab peer and repository")
	}
	if input.CandidateSource != "" && !input.Candidate || input.Candidate && (input.Artifacts == "" && !input.hasPackage() || input.CandidateSource == "") {
		return errors.New("candidate acceptance requires explicit artifacts and an exact candidate source")
	}
	if input.BaselineArtifacts != "" && input.BaselineTag == "" {
		return errors.New("native predecessor artifacts require their published tag")
	}
	return input.validatePublished()
}

func (input *NativeAcceptance) validatePublished() error {
	if input.Tag != "" && (input.Candidate || input.Artifacts == "" && input.Peer == "") {
		return errors.New("published candidate requires explicit artifact or peer inputs and cannot be untagged")
	}
	for _, tag := range []string{input.Tag, input.BaselineTag} {
		if tag != "" {
			if _, err := semver.StrictNewVersion(strings.TrimPrefix(tag, "v")); err != nil || !strings.HasPrefix(tag, "v") {
				return errors.New("native published inputs require v<semver> tags")
			}
		}
	}
	if input.BaselineTag != "" && !input.hasPackage() && input.BaselineArtifacts == "" && input.Peer == "" {
		return errors.New("native predecessor tag requires explicit artifact or peer inputs")
	}
	if input.Performance != "" && (input.Artifacts == "" && input.Tag == "" && !input.hasPackage() || strings.TrimSpace(os.Getenv("AIGW_ACCEPTANCE_BASELINE")) == "" && input.BaselineTag == "") {
		return errors.New("performance acceptance requires an explicit published candidate and baseline")
	}
	return nil
}

// AcceptNative proves the current host lifecycle using built or supplied archives.
// It publishes nothing and owns the complete temporary build and test scope.
func AcceptNative(ctx context.Context, input NativeAcceptance) error {
	if err := input.validate(); err != nil {
		return err
	}
	request, err := buildRequestFromEnvironment(ctx, "")
	if err != nil {
		return err
	}
	run := executeTool(ctx)
	if input.Artifacts != "" || input.Tag != "" || input.hasPackage() {
		if err := ensureCleanSource(request.Root, run); err != nil {
			return err
		}
	}
	return acceptNativeInput(ctx, input, request, run)
}

func acceptNativeInput(ctx context.Context, input NativeAcceptance, request buildRequest, run toolRunner) (result error) {
	if input.Artifacts == "" && input.Tag == "" && !input.hasPackage() && input.BaselineTag == "" {
		return acceptNative(request, input.Artifacts, os.Getenv("AIGW_ACCEPTANCE_BASELINE"), input.Clients, input.Performance, run)
	}
	workspace, err := os.MkdirTemp("", "aigw-native-inputs-*")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(workspace)) }()
	if err := input.preparePackage(ctx, request, workspace, run); err != nil {
		return err
	}
	if input.Tag != "" && input.Artifacts == "" {
		input.Artifacts = filepath.Join(workspace, "candidate")
		if err := downloadNativeRelease(request.Root, input, input.Tag, input.Artifacts, run); err != nil {
			return err
		}
	}
	trust := releaseartifact.SignatureTrust{AllowedSigners: os.Getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"), Principal: os.Getenv("AIGW_RELEASE_ARTIFACT_SIGNER")}
	if err := releaseartifact.VerifyMatrix(ctx, input.Artifacts, request.Version, trust); err != nil {
		return err
	}
	source := releaseartifact.SourceTrust{Repository: request.Root, AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE"), Commit: input.CandidateSource}
	if input.Candidate {
		err = releaseartifact.VerifyCandidateProvenance(ctx, input.Artifacts, source)
	} else {
		tag := input.Tag
		if tag == "" {
			tag = os.Getenv("CI_COMMIT_TAG")
		}
		err = releaseartifact.VerifyProvenance(ctx, input.Artifacts, tag, source)
	}
	if err != nil {
		return err
	}
	baseline := os.Getenv("AIGW_ACCEPTANCE_BASELINE")
	if input.BaselineTag == "" {
		return acceptNative(request, input.Artifacts, baseline, input.Clients, input.Performance, run)
	}
	version := strings.TrimPrefix(input.BaselineTag, "v")
	if input.BaselineArtifacts == "" {
		input.BaselineArtifacts = filepath.Join(workspace, "baseline")
		if err := downloadNativeRelease(request.Root, input, input.BaselineTag, input.BaselineArtifacts, run); err != nil {
			return err
		}
	}
	if err := releaseartifact.VerifyMatrix(ctx, input.BaselineArtifacts, version, trust); err != nil {
		return fmt.Errorf("verify published baseline: %w", err)
	}
	if err := releaseartifact.VerifyProvenance(ctx, input.BaselineArtifacts, input.BaselineTag, releaseartifact.SourceTrust{Repository: request.Root, AllowedSigners: source.AllowedSigners}); err != nil {
		return err
	}
	stage := filepath.Join(workspace, "baseline-native")
	if err := os.Mkdir(stage, 0o700); err != nil {
		return err
	}
	target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	for _, name := range []string{target.ArchiveName(version), "checksums.txt"} {
		if err := copyFile(filepath.Join(input.BaselineArtifacts, name), filepath.Join(stage, name)); err != nil {
			return err
		}
	}
	if err := prepareNativeBinary(stage, version); err != nil {
		return err
	}
	name := "aigw"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	baseline = filepath.Join(stage, fmt.Sprintf("aigw_%s_%s_%s", version, target.OS, target.Arch), name)
	return acceptNative(request, input.Artifacts, baseline, input.Clients, input.Performance, run)
}

// BuildNative constructs and extracts the current host's archive through the release owner.
// The caller owns workspace and its cleanup; this neither publishes nor installs.
func BuildNative(ctx context.Context, root, workspace, version string) (string, error) {
	epoch, err := resolveReleaseEpoch(ctx, root, version)
	if err != nil {
		return "", err
	}
	request := buildRequest{Root: root, Version: version, Epoch: epoch, TargetOS: runtime.GOOS}
	if err := validateRequest(request); err != nil {
		return "", err
	}
	stage, err := buildArchives(request, workspace, executeTool(ctx))
	if err != nil {
		return "", err
	}
	return stage, prepareNativeBinary(stage, version)
}

func acceptNative(request buildRequest, artifacts, baseline string, clients bool, performance string, run toolRunner) (result error) {
	if performance != "" && !filepath.IsAbs(performance) {
		return errors.New("performance output must be an absolute directory")
	}
	if performance != "" {
		if _, err := os.Stat(performance); !os.IsNotExist(err) {
			return errors.New("performance output must be a new directory")
		}
	}
	if err := validateRequest(request); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "aigw-native-release-*")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(workspace)) }()
	stage := ""
	if artifacts == "" && clients {
		request.TargetOS = runtime.GOOS
		stage, err = buildArchives(request, workspace, run)
		if err != nil {
			return err
		}
		if err := prepareNativeBinary(stage, request.Version); err != nil {
			return err
		}
	}
	if artifacts != "" {
		stage = workspace
		target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
		for _, name := range []string{target.ArchiveName(request.Version), "checksums.txt"} {
			if err := copyFile(filepath.Join(artifacts, name), filepath.Join(stage, name)); err != nil {
				return err
			}
		}
		if err := prepareNativeBinary(stage, request.Version); err != nil {
			return err
		}
	}
	lifecycleBaseline := ""
	if stage != "" {
		lifecycleBaseline = baseline
	}
	commonEnvironment := []string{"AIGW_ACCEPTANCE_RELEASE=" + stage, "TMPDIR=" + workspace, "TMP=" + workspace, "TEMP=" + workspace}
	currentEnvironment := append(append([]string{}, commonEnvironment...), "AIGW_ACCEPTANCE_BASELINE="+lifecycleBaseline)
	publishedEnvironment := append(append([]string{}, commonEnvironment...), "AIGW_ACCEPTANCE_BASELINE="+baseline)
	call := toolCall{
		Name: "go", Directory: request.Root,
		Timeout: 15 * time.Minute,
		Args:    []string{"test", "./tools/release", "-run", "^(TestNativeProductJourney|TestNativeRollbackConfigurationAdmission|TestNativeTeamManifestJourney)$", "-count=1", "-v"},
		Env:     currentEnvironment,
	}
	if err := run(call); err != nil {
		return err
	}
	if baseline != "" {
		call.Args = []string{"test", "./tools/release", "-run", "^TestNativePublishedPredecessorJourney$", "-count=1", "-v"}
		call.Env = publishedEnvironment
		if err := run(call); err != nil {
			return err
		}
	}
	if clients {
		call.Args = []string{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClientJourney$", "-count=1", "-v"}
		call.Env = publishedEnvironment
		if err := run(call); err != nil {
			return err
		}
	}
	if performance != "" {
		call.Args = []string{"test", "-tags=performance_acceptance", "./tools/release", "-run", "^TestNativePerformance$", "-count=1", "-v"}
		call.Env = append(publishedEnvironment, "AIGW_PERFORMANCE_OUTPUT="+performance)
		if err := run(call); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(performance, "summary.json"))
		if err != nil || !json.Valid(data) {
			return errors.New("performance acceptance did not produce its result summary")
		}
	}
	return nil
}

func prepareNativeBinary(stage, version string) error {
	target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	program, err := target.ReadProgram(filepath.Join(stage, target.ArchiveName(version)), filepath.Join(stage, "checksums.txt"), version)
	if err != nil {
		return err
	}
	directory := filepath.Join(stage, fmt.Sprintf("aigw_%s_%s_%s", version, target.OS, target.Arch))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	name := "aigw"
	if target.OS == "windows" {
		name += ".exe"
	}
	return os.WriteFile(filepath.Join(directory, name), program, 0o700)
}

func verifySignedArchives(request buildRequest, stage string, run toolRunner) (result error) {
	if request.MacOSSigningIdentity == "" {
		return nil
	}
	scratch, err := os.MkdirTemp(filepath.Dir(stage), ".signature-verification-")
	if err != nil {
		return fmt.Errorf("prepare signed archive verification: %w", err)
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(scratch)) }()
	for _, arch := range []string{"amd64", "arm64"} {
		target := artifact.Target{OS: "darwin", Arch: arch}
		program, err := target.ReadProgram(filepath.Join(stage, target.ArchiveName(request.Version)), filepath.Join(stage, "checksums.txt"), request.Version)
		if err != nil {
			return err
		}
		path := filepath.Join(scratch, arch)
		if err := os.WriteFile(path, program, 0o700); err != nil {
			return err
		}
		requirement := `-R=anchor apple generic and certificate leaf = H"` + request.MacOSSigningIdentity + `"`
		if err := run(toolCall{Name: "/usr/bin/codesign", Directory: request.Root, Args: []string{"--verify", "--strict", requirement, path}}); err != nil {
			return fmt.Errorf("verify signed macOS %s archive: %w", arch, err)
		}
	}
	return nil
}
