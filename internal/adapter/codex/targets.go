package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/surface"
)

func codexTargetUnion(before, after []TargetRef) ([]codexReconciliationTarget, error) {
	normalizedBefore, err := normalizeCodexTargets(before, false)
	if err != nil {
		return nil, err
	}
	normalizedAfter, err := normalizeCodexTargets(after, true)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]codexReconciliationTarget, len(normalizedBefore)+len(normalizedAfter))
	for _, target := range normalizedBefore {
		byPath[target.Path] = codexReconciliationTarget{ref: target}
	}
	for _, target := range normalizedAfter {
		if err := validateDesiredCodexTarget(target); err != nil {
			return nil, err
		}
		byPath[target.Path] = codexReconciliationTarget{ref: target, desired: true}
	}
	union := make([]codexReconciliationTarget, 0, len(byPath))
	for _, target := range byPath {
		union = append(union, target)
	}
	sort.Slice(union, func(left, right int) bool { return union[left].ref.Path < union[right].ref.Path })
	return union, nil
}

// normalizeCodexTargets resolves each target's path, keeping it as
// configured for reports. A target that cannot be resolved, or that names the
// same file as another, fails as that target, which desired says the change
// keeps.
func normalizeCodexTargets(values []TargetRef, desired bool) ([]TargetRef, error) {
	normalized := make([]TargetRef, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, target := range values {
		if target.Path == "" || target.SurfaceID == "" {
			return nil, fmt.Errorf("Codex target requires surface_id and path")
		}
		sourcePath, err := absoluteCodexTargetPath(target.Path)
		if err != nil {
			return nil, &adapter.TargetError{Path: target.Path, Desired: desired, Err: err}
		}
		path, err := canonicalCodexTargetPath(sourcePath)
		if err != nil {
			return nil, &adapter.TargetError{Path: sourcePath, Desired: desired, Err: err}
		}
		if _, duplicate := seen[path]; duplicate {
			return nil, &adapter.TargetError{Path: sourcePath, Desired: desired, Err: duplicateTargetError{}}
		}
		seen[path] = struct{}{}
		folder, err := canonicalCodexTargetPath(filepath.Dir(sourcePath))
		if err != nil {
			return nil, &adapter.TargetError{Path: sourcePath, Desired: desired, Err: err}
		}
		target.Path = path
		target.statePath = preferredCodexStatePath(sourcePath, path)
		target.configured, target.configuredFolder = sourcePath, folder
		normalized = append(normalized, target)
	}
	sort.Slice(normalized, func(left, right int) bool { return normalized[left].Path < normalized[right].Path })
	return normalized, nil
}

// duplicateTargetError refuses two configured targets that name one file. A
// sync converges them into one, so a refusal here came from an explicit edit,
// whose own command line names the targets again.
type duplicateTargetError struct{}

func (duplicateTargetError) Error() string {
	return "Codex config target is duplicated: another target names the same file"
}

// NextAction names each file once with client edit.
func (duplicateTargetError) NextAction() string {
	return "aigw client edit codex --target <file>, naming each Codex configuration file once"
}

// sameFileSpelling is path cleaned, with the separators and, where the file
// system ignores case by default, the case that two spellings of one file can
// differ by, so that they compare equal.
func sameFileSpelling(path string) string {
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

func canonicalCodexTargetPath(path string) (string, error) {
	absolute, err := absoluteCodexTargetPath(path)
	if err != nil {
		return "", err
	}
	missing := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(absolute)
		if err == nil {
			for _, segment := range slices.Backward(missing) {
				resolved = filepath.Join(resolved, segment)
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve Codex target symlinks: %w", err)
		}
		if _, err := os.Lstat(absolute); err == nil {
			return "", errors.New("Codex target is a broken symlink")
		}
		missing = append(missing, filepath.Base(absolute))
		absolute = filepath.Dir(absolute)
	}
}

func absoluteCodexTargetPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve Codex target: %w", err)
	}
	return absolute, nil
}

func preferredCodexStatePath(sourcePath, canonicalPath string) string {
	canonicalStatePath := codexStatePath(canonicalPath)
	if sourcePath == canonicalPath {
		return canonicalStatePath
	}
	if info, err := os.Lstat(canonicalStatePath); err == nil && !info.IsDir() {
		return canonicalStatePath
	}
	sourceStatePath := codexStatePath(sourcePath)
	if info, err := os.Lstat(sourceStatePath); err == nil && !info.IsDir() {
		return sourceStatePath
	}
	return canonicalStatePath
}

func targetCodexStatePath(target TargetRef) string {
	if target.statePath != "" {
		return target.statePath
	}
	return codexStatePath(target.Path)
}

// validateDesiredCodexTarget refuses a surface other than a Codex home. The
// refusal keeps the text that released versions print.
func validateDesiredCodexTarget(target TargetRef) error {
	if !IsHome(surface.ID(target.SurfaceID)) {
		return fmt.Errorf("Codex target %s cannot use authority aigw with projection mode %s", target.SurfaceID, ProjectionFullSelection)
	}
	return nil
}

// HomeTarget names the existing configuration at path as the default Codex
// Home.
func HomeTarget(path string) TargetRef {
	return TargetRef{SurfaceID: string(HomeDefault), Path: path}
}
