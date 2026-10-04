package construction

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/upgrade/artifact"
	releaseartifact "aigw-cli/tools/release/artifact"
	"aigw-cli/tools/release/readiness"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rogpeppe/go-internal/robustio"
)

// NativeAcceptance selects artifact identities and optional peer transport.
// Local paths never imply a network request; tags require an explicit peer to download.
type NativeAcceptance struct {
	Artifacts, Tag, BaselineArtifacts, BaselineTag string
	Peer, Repository                               string
	CandidateSource                                string
	Candidate, Clients                             bool
	Performance                                    string
	DiagnosticClient                               string
}

var nativeAcceptanceClients = []string{configuration.ClientClaude, configuration.ClientCodex, configuration.ClientHermes}

// ParseNativeAcceptance keeps native input and scope selection at the release owner.
func ParseNativeAcceptance(arguments []string) (NativeAcceptance, error) {
	input := NativeAcceptance{}
	flags := flag.NewFlagSet("accept-native", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&input.Artifacts, "artifacts", "", "Consume an existing signed release matrix")
	flags.StringVar(&input.Tag, "tag", "", "Select the published candidate tag")
	flags.StringVar(&input.BaselineArtifacts, "baseline-artifacts", "", "Consume a local published predecessor matrix")
	flags.StringVar(&input.BaselineTag, "baseline-tag", "", "Select the published predecessor tag")
	flags.StringVar(&input.Peer, "peer", "", "Download missing tagged inputs from github or gitlab")
	flags.StringVar(&input.Repository, "repository", "", "Explicit repository for the selected peer")
	flags.BoolVar(&input.Candidate, "candidate", false, "Bind untagged artifacts to signed source")
	flags.StringVar(&input.CandidateSource, "candidate-source", "", "Exact signed candidate commit; defaults to verifier HEAD")
	flags.BoolVar(&input.Clients, "clients", false, "Verify explicitly supplied native clients")
	flags.StringVar(&input.DiagnosticClient, "diagnostic-client", "", "Diagnose one native client; does not qualify complete product acceptance")
	flags.StringVar(&input.Performance, "performance", "", "Retain Hyperfine samples in this absolute directory")
	if err := flags.Parse(arguments); err != nil {
		return input, err
	}
	if flags.NArg() != 0 {
		return input, errors.New("native acceptance accepts flags only")
	}
	err := input.validate()
	return input, err
}

// UsesPrebuiltArtifacts distinguishes product acceptance from source qualification.
func (input *NativeAcceptance) UsesPrebuiltArtifacts() bool {
	return input.Artifacts != "" || input.Tag != ""
}

// AcceptNative proves one host lifecycle with admitted artifact inputs.
// It publishes nothing and owns every download and extracted predecessor.
func AcceptNative(ctx context.Context, input NativeAcceptance) error {
	if err := input.validate(); err != nil {
		return err
	}
	request, err := buildRequestFromEnvironment(ctx, "")
	if err != nil {
		return err
	}
	if input.Clients || input.DiagnosticClient != "" {
		if err := requireNativeClients(input.DiagnosticClient); err != nil {
			return err
		}
	}
	return acceptNativeInput(ctx, input, request, executeTool(ctx))
}

func acceptNativeInput(ctx context.Context, input NativeAcceptance, request buildRequest, run toolRunner) (result error) {
	baseline := os.Getenv("AIGW_ACCEPTANCE_BASELINE")
	if input.Artifacts == "" && input.Tag == "" && input.BaselineTag == "" {
		return acceptNative(request, "", baseline, input, run)
	}
	if err := ensureCleanSource(request.Root, run); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "aigw-native-inputs-*")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(workspace)) }()
	if err := input.prepareCandidate(ctx, request, workspace, run); err != nil {
		return err
	}
	if input.BaselineTag != "" {
		baseline, err = input.prepareBaseline(ctx, request.Root, workspace, run)
		if err != nil {
			return err
		}
	}
	return acceptNative(request, input.Artifacts, baseline, input, run)
}

func (input *NativeAcceptance) validate() error {
	if input.CandidateSource != "" && !input.Candidate {
		return errors.New("candidate source requires --candidate")
	}
	if input.Candidate && input.Artifacts == "" {
		return errors.New("candidate acceptance requires --artifacts")
	}
	if input.Candidate && (input.Tag != "" || readiness.SelectedReleaseTag() != "") {
		return errors.New("candidate acceptance cannot select a release tag")
	}
	if input.Peer != "" && (input.Peer != "github" && input.Peer != "gitlab" || input.Repository == "") {
		return errors.New("native release transport requires github or gitlab and an explicit repository")
	}
	if input.Peer == "" && (input.Tag != "" && input.Artifacts == "" || input.BaselineTag != "" && input.BaselineArtifacts == "") {
		return errors.New("native tagged acceptance requires local artifacts or an explicit peer")
	}
	if input.BaselineArtifacts != "" && input.BaselineTag == "" {
		return errors.New("baseline artifacts require the published baseline tag")
	}
	if (input.BaselineTag != "" || input.BaselineArtifacts != "") && os.Getenv("AIGW_ACCEPTANCE_BASELINE") != "" {
		return errors.New("published baseline inputs and an explicit baseline executable are mutually exclusive")
	}
	if err := input.validateOptionalScopes(); err != nil {
		return err
	}
	return nil
}

func (input *NativeAcceptance) validateOptionalScopes() error {
	if input.DiagnosticClient != "" && (!slices.Contains(nativeAcceptanceClients, input.DiagnosticClient) || !input.UsesPrebuiltArtifacts() || input.Clients || input.Performance != "") {
		return errors.New("native client diagnostics require one supported client, prebuilt artifacts, and no full-client or performance scope")
	}
	for _, tag := range []string{input.Tag, input.BaselineTag} {
		if tag == "" {
			continue
		}
		if _, err := semver.StrictNewVersion(strings.TrimPrefix(tag, "v")); err != nil || !strings.HasPrefix(tag, "v") {
			return errors.New("native release tags require v<semver>")
		}
	}
	if (input.Clients || input.DiagnosticClient != "" || runtime.GOOS == "darwin" && os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") == "1") && input.BaselineTag == "" && os.Getenv("AIGW_ACCEPTANCE_BASELINE") == "" {
		return errors.New("native succession requires a published predecessor")
	}
	if input.Performance != "" && (input.Artifacts == "" && input.Tag == "" || os.Getenv("AIGW_ACCEPTANCE_BASELINE") == "" && input.BaselineTag == "") {
		return errors.New("performance acceptance requires an explicit candidate artifact and published baseline")
	}
	return nil
}

func requireNativeClients(selected string) error {
	for _, client := range nativeAcceptanceClients {
		if selected != "" && selected != client {
			continue
		}
		key := "AIGW_ACCEPTANCE_" + strings.ToUpper(client)
		path := os.Getenv(key)
		info, err := os.Stat(path)
		if err != nil || !filepath.IsAbs(path) || !info.Mode().IsRegular() {
			return fmt.Errorf("real-client acceptance requires an explicit executable in %s", key)
		}
	}
	return nil
}

func (input *NativeAcceptance) prepareCandidate(ctx context.Context, request buildRequest, workspace string, run toolRunner) error {
	if input.Tag != "" && input.Artifacts == "" {
		input.Artifacts = filepath.Join(workspace, "candidate")
		if err := downloadNativeRelease(request.Root, *input, input.Tag, input.Artifacts, run); err != nil {
			return err
		}
	}
	if input.Artifacts == "" {
		return nil
	}
	trust := releaseartifact.SignatureTrust{AllowedSigners: os.Getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"), Principal: os.Getenv("AIGW_RELEASE_ARTIFACT_SIGNER")}
	if err := releaseartifact.VerifyMatrix(ctx, input.Artifacts, request.Version, trust); err != nil {
		return err
	}
	source := releaseartifact.SourceTrust{Repository: request.Root, AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE"), Commit: input.CandidateSource}
	if input.Candidate {
		return releaseartifact.VerifyCandidateProvenance(ctx, input.Artifacts, source)
	}
	tag := input.Tag
	if tag == "" {
		tag = readiness.SelectedReleaseTag()
	}
	return releaseartifact.VerifyProvenance(ctx, input.Artifacts, tag, source)
}

func (input *NativeAcceptance) prepareBaseline(ctx context.Context, root, workspace string, run toolRunner) (string, error) {
	if input.BaselineArtifacts == "" {
		input.BaselineArtifacts = filepath.Join(workspace, "baseline")
		if err := downloadNativeRelease(root, *input, input.BaselineTag, input.BaselineArtifacts, run); err != nil {
			return "", err
		}
	}
	return preparePublishedBaseline(ctx, root, input.BaselineArtifacts, input.BaselineTag, filepath.Join(workspace, "baseline-native"))
}

func downloadNativeRelease(root string, input NativeAcceptance, tag, directory string, run toolRunner) error {
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}
	call := toolCall{Name: "gh", Directory: root, Timeout: 2 * time.Minute, Env: []string{"GH_PROMPT_DISABLED=1", "GLAB_NO_PROMPT=1", "GLAB_ENABLE_CI_AUTOLOGIN=true"}, Args: []string{"release", "download", tag, "--repo", input.Repository, "--dir", directory}}
	if input.Peer == "gitlab" {
		call.Name = "glab"
		call.Args = append(call.Args, "--asset-name", "aigw_*", "--asset-name", "checksums.txt*")
	}
	if err := run(call); err != nil {
		return fmt.Errorf("download %s native release %s: %w", input.Peer, tag, err)
	}
	return nil
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

func acceptNative(request buildRequest, artifacts, baseline string, input NativeAcceptance, run toolRunner) (result error) {
	clients, performance := input.Clients, input.Performance
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
	defer func() {
		if err := robustio.RemoveAll(workspace); err != nil {
			result = errors.Join(result, fmt.Errorf("remove native acceptance workspace %s: %w", workspace, err))
		}
	}()
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
	commonEnvironment := append([]string{"AIGW_ACCEPTANCE_RELEASE=" + stage, "TMPDIR=" + workspace, "TMP=" + workspace, "TEMP=" + workspace}, forgeCredentialOverrides()...)
	currentEnvironment := append(append([]string{}, commonEnvironment...), "AIGW_ACCEPTANCE_BASELINE=")
	publishedEnvironment := append(append([]string{}, commonEnvironment...), "AIGW_ACCEPTANCE_BASELINE="+baseline)
	clientPattern := "^TestNativeClientJourney$"
	if input.DiagnosticClient != "" {
		clients = true
		clientPattern += "/^" + input.DiagnosticClient + "($|-)"
	}
	lifecycle := performance == "" && input.DiagnosticClient == ""
	for _, suite := range []struct {
		selected    bool
		args        []string
		environment []string
	}{
		{lifecycle, []string{"test", "-tags=native_resource_acceptance", "./tools/release", "-run", "^(TestNativeProductJourney|TestNativeRollbackConfigurationAdmission|TestNativeTeamManifestJourney|TestNativeVerificationResources)$", "-count=1", "-v"}, currentEnvironment},
		{lifecycle && baseline != "", []string{"test", "./tools/release", "-run", "^TestNativePublishedPredecessorJourney$", "-count=1", "-v"}, publishedEnvironment},
		{clients, []string{"test", "-tags=client_acceptance", "./tools/release", "-run", clientPattern, "-count=1", "-v"}, publishedEnvironment},
		{performance != "", []string{"test", "-tags=performance_acceptance", "./tools/release", "-run", "^TestNativePerformance$", "-count=1", "-v"}, append(publishedEnvironment, "AIGW_PERFORMANCE_OUTPUT="+performance)},
	} {
		if !suite.selected {
			continue
		}
		if err := run(toolCall{Name: "go", Directory: request.Root, Args: suite.args, Env: suite.environment}); err != nil {
			return err
		}
	}
	if performance != "" {
		data, err := os.ReadFile(filepath.Join(performance, "summary.json"))
		if err != nil || !json.Valid(data) {
			return errors.New("performance acceptance did not produce its result summary")
		}
	}
	return nil
}

func preparePublishedBaseline(ctx context.Context, root, directory, tag, stage string) (string, error) {
	version := strings.TrimPrefix(tag, "v")
	trust := releaseartifact.SignatureTrust{AllowedSigners: os.Getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE"), Principal: os.Getenv("AIGW_RELEASE_ARTIFACT_SIGNER")}
	if err := releaseartifact.VerifyMatrix(ctx, directory, version, trust); err != nil {
		return "", fmt.Errorf("verify published baseline: %w", err)
	}
	if err := releaseartifact.VerifyProvenance(ctx, directory, tag, releaseartifact.SourceTrust{Repository: root, AllowedSigners: os.Getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")}); err != nil {
		return "", err
	}
	if err := os.MkdirAll(stage, 0o700); err != nil {
		return "", err
	}
	target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	for _, name := range []string{target.ArchiveName(version), "checksums.txt"} {
		if err := copyFile(filepath.Join(directory, name), filepath.Join(stage, name)); err != nil {
			return "", err
		}
	}
	if err := prepareNativeBinary(stage, version); err != nil {
		return "", err
	}
	name := "aigw"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(stage, fmt.Sprintf("aigw_%s_%s_%s", version, target.OS, target.Arch), name), nil
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
