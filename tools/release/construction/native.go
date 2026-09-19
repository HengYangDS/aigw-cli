package construction

import (
	"aigw-cli/internal/upgrade/artifact"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rogpeppe/go-internal/robustio"
)

// AcceptNative proves the current host lifecycle using built or supplied archives.
// It publishes nothing and owns the complete temporary build and test scope.
func AcceptNative(ctx context.Context, artifacts string, clients bool, performance string) error {
	request, err := buildRequestFromEnvironment(ctx, "")
	if err != nil {
		return err
	}
	if artifacts == "" {
		return acceptNative(request, artifacts, clients, performance, executeTool(ctx))
	}
	if err := ensureCleanSource(request.Root, executeTool(ctx)); err != nil {
		return err
	}
	return acceptNative(request, artifacts, clients, performance, executeTool(ctx))
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

func acceptNative(request buildRequest, artifacts string, clients bool, performance string, run toolRunner) (result error) {
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
	call := toolCall{
		Name: "go", Directory: request.Root,
		Args: []string{"test", "./tools/release", "-run", "^(TestNativeProductJourney|TestNativeRollbackConfigurationAdmission|TestNativeTeamManifestJourney)$", "-count=1", "-v"},
		Env:  []string{"AIGW_ACCEPTANCE_RELEASE=" + stage, "TMPDIR=" + workspace, "TMP=" + workspace, "TEMP=" + workspace},
	}
	if err := run(call); err != nil {
		return err
	}
	if clients {
		call.Args = []string{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClientJourney$", "-count=1", "-v"}
		if err := run(call); err != nil {
			return err
		}
	}
	if performance != "" {
		call.Args = []string{"test", "-tags=performance_acceptance", "./tools/release", "-run", "^TestNativePerformance$", "-count=1", "-v"}
		call.Env = append(call.Env, "AIGW_PERFORMANCE_OUTPUT="+performance)
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

// VerifyMacOSDistribution checks the selected publisher and Gatekeeper admission for both macOS archives.
func VerifyMacOSDistribution(ctx context.Context, directory, version, identity string) error {
	if identity == "" {
		return errors.New("macOS distribution verification requires an explicit signing identity")
	}
	request := buildRequest{Version: version, Epoch: "0", MacOSSigningIdentity: identity}
	if err := validateRequest(request); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return verifyMacOSArchives(request, directory, executeTool(bounded), true)
}

func verifySignedArchives(request buildRequest, stage string, run toolRunner) error {
	return verifyMacOSArchives(request, stage, run, false)
}

func verifyMacOSArchives(request buildRequest, stage string, run toolRunner, distribution bool) (result error) {
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
		if distribution {
			if err := run(toolCall{Name: "/usr/sbin/spctl", Directory: request.Root, Args: []string{"--assess", "--type", "execute", "--verbose=4", path}}); err != nil {
				return fmt.Errorf("macOS %s distribution is not approved by Gatekeeper: %w", arch, err)
			}
		}
	}
	return nil
}
