package construction

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rogpeppe/go-internal/robustio"
)

// AcceptNative builds the release archives and proves the current host lifecycle.
// It publishes nothing and owns the complete temporary build and test scope.
func AcceptNative(clients bool) error {
	request, err := buildRequestFromEnvironment("")
	if err != nil {
		return err
	}
	return acceptNative(request, clients, executeTool)
}

func acceptNative(request buildRequest, clients bool, run toolRunner) (result error) {
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
	stage, err := buildArchives(request, workspace, run)
	if err != nil {
		return err
	}
	if err := prepareNativeBinary(request.Root, stage, request.Version); err != nil {
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

func prepareNativeBinary(root, stage, version string) error {
	data, err := os.ReadFile(filepath.Join(stage, "artifacts.json"))
	if err != nil {
		return err
	}
	var artifacts []struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Goos   string `json:"goos"`
		Goarch string `json:"goarch"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(data, &artifacts); err != nil {
		return fmt.Errorf("read GoReleaser artifact inventory: %w", err)
	}
	var selected string
	for _, item := range artifacts {
		if item.Type != "Binary" || item.Goos != runtime.GOOS || item.Goarch != runtime.GOARCH {
			continue
		}
		if selected != "" {
			return errors.New("GoReleaser produced more than one native executable")
		}
		path := item.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		relative, err := filepath.Rel(stage, path)
		if err != nil || !filepath.IsLocal(relative) || filepath.Base(item.Name) != item.Name {
			return errors.New("GoReleaser native executable is outside its owned build scope")
		}
		selected = path
		directory := filepath.Join(stage, fmt.Sprintf("aigw_%s_%s_%s", version, runtime.GOOS, runtime.GOARCH))
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
		target := filepath.Join(directory, item.Name)
		if err := copyFile(path, target); err != nil {
			return err
		}
		if err := os.Chmod(target, 0o700); err != nil {
			return err
		}
	}
	if selected == "" {
		return errors.New("GoReleaser did not produce an executable for this host")
	}
	return nil
}
