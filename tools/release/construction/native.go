package construction

import (
	"aigw-cli/internal/upgrade/artifact"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rogpeppe/go-internal/robustio"
)

// AcceptNative proves the current host lifecycle using built or supplied archives.
// It publishes nothing and owns the complete temporary build and test scope.
func AcceptNative(artifacts string, clients bool) error {
	request, err := buildRequestFromEnvironment("")
	if err != nil {
		return err
	}
	if artifacts == "" {
		return acceptNative(request, artifacts, clients, executeTool)
	}
	if err := ensureCleanSource(request.Root, executeTool); err != nil {
		return err
	}
	selected, err := resolveGitObject(request.Root, "refs/tags/"+os.Getenv("CI_COMMIT_TAG")+"^{commit}", executeTool)
	if err != nil {
		return err
	}
	head, err := resolveGitObject(request.Root, "HEAD^{commit}", executeTool)
	if err != nil {
		return err
	}
	if head != selected {
		return errors.New("native acceptance source must match the selected release tag")
	}
	return acceptNative(request, artifacts, clients, executeTool)
}

func acceptNative(request buildRequest, artifacts string, clients bool, run toolRunner) (result error) {
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
	stage := workspace
	if artifacts == "" {
		stage, err = buildArchives(request, workspace, run)
		if err != nil {
			return err
		}
	} else {
		target := artifact.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
		for _, name := range []string{target.ArchiveName(request.Version), "checksums.txt"} {
			if err := copyFile(filepath.Join(artifacts, name), filepath.Join(stage, name)); err != nil {
				return err
			}
		}
	}
	if err := prepareNativeBinary(stage, request.Version); err != nil {
		return err
	}
	call := toolCall{
		Name: "go", Directory: request.Root,
		Args: []string{"test", "./tools/release", "-run", "^TestNativeProductJourney$/(portable_artifact_lifecycle|system_credential_store)$", "-count=1", "-v"},
		Env:  []string{"AIGW_ACCEPTANCE_RELEASE=" + stage},
	}
	if err := run(call); err != nil {
		return err
	}
	if clients {
		call.Args = []string{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClientJourney$", "-count=1", "-v"}
		return run(call)
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
