// Package projection owns CUE rendering and reconciliation of Forge CI files.
package projection

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Reconcile renders the repository's CUE model and either verifies or updates
// its owned Forge files. Check mode never changes the checkout.
func Reconcile(root string, check bool) error {
	projections, err := renderProjections(root)
	if err != nil {
		return err
	}
	for _, item := range projections {
		path := filepath.Join(root, filepath.FromSlash(item.Path))
		if check {
			tracked, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read projection %s: %w", item.Path, err)
			}
			if !bytes.Equal(tracked, []byte(item.Content)) {
				return fmt.Errorf("projection drift: %s; run `mise exec --locked -- go run ./tools/ci project`", item.Path)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create projection directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(item.Content), 0o644); err != nil {
			return fmt.Errorf("write projection: %w", err)
		}
	}
	return nil
}

type projection struct {
	Path    string
	Content string
}

var projectionExpressions = []struct {
	path       string
	expression string
}{
	{path: ".gitlab-ci.yml", expression: "gitlab"},
	{path: ".github/workflows/verify.yml", expression: "githubVerify"},
	{path: ".github/workflows/release.yml", expression: "githubRelease"},
}

func renderProjections(root string) ([]projection, error) {
	projections := make([]projection, 0, len(projectionExpressions))
	for _, item := range projectionExpressions {
		process := projectionCommand(root, item.expression)
		output, err := process.Output()
		if err != nil {
			if exit, ok := errors.AsType[*exec.ExitError](err); ok {
				return nil, fmt.Errorf("render %s: %s", item.path, bytes.TrimSpace(exit.Stderr))
			}
			return nil, fmt.Errorf("render %s: %w", item.path, err)
		}
		projections = append(projections, projection{Path: item.path, Content: string(output)})
	}
	return projections, nil
}

func projectionCommand(root, expression string) *exec.Cmd {
	process := exec.Command(
		"cue",
		"export",
		filepath.FromSlash(".config/ci/pipeline.cue"),
		filepath.FromSlash(".ethos/workspace.toml"),
		"--expression",
		expression,
		"--out",
		"yaml",
	)
	process.Dir = root
	return process
}
