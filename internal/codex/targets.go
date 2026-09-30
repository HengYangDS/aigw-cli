package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"aigw-cli/internal/surface"
)

func codexTargetUnion(before, after []TargetRef) ([]codexReconciliationTarget, error) {
	normalizedBefore, err := normalizeCodexTargets(before)
	if err != nil {
		return nil, err
	}
	normalizedAfter, err := normalizeCodexTargets(after)
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

func normalizeCodexTargets(values []TargetRef) ([]TargetRef, error) {
	normalized := make([]TargetRef, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, target := range values {
		if target.Path == "" || target.SurfaceID == "" || target.Authority == "" || target.ProjectionMode == "" {
			return nil, fmt.Errorf("Codex target requires surface_id, authority, projection_mode, and path")
		}
		sourcePath, err := absoluteCodexTargetPath(target.Path)
		if err != nil {
			return nil, err
		}
		path, err := canonicalCodexTargetPath(sourcePath)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[path]; duplicate {
			return nil, fmt.Errorf("Codex config target %s is duplicated", path)
		}
		seen[path] = struct{}{}
		target.Path = path
		target.statePath = preferredCodexStatePath(sourcePath, path)
		normalized = append(normalized, target)
	}
	sort.Slice(normalized, func(left, right int) bool { return normalized[left].Path < normalized[right].Path })
	return normalized, nil
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
			return "", fmt.Errorf("resolve Codex target symlinks %s: %w", path, err)
		}
		if _, err := os.Lstat(absolute); err == nil {
			return "", fmt.Errorf("Codex target is a broken symlink: %s", absolute)
		}
		missing = append(missing, filepath.Base(absolute))
		absolute = filepath.Dir(absolute)
	}
}

func absoluteCodexTargetPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve Codex target %s: %w", path, err)
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

func validateDesiredCodexTarget(target TargetRef) error {
	surfaceID := surface.ID(target.SurfaceID)
	authority := surface.Authority(target.Authority)
	switch {
	case surfaceID.IsCodexHome() && surfaceID.HasAuthority(authority) && target.ProjectionMode == ProjectionFullSelection:
		return nil
	default:
		return fmt.Errorf("Codex target %s cannot use authority %s with projection mode %s", target.SurfaceID, target.Authority, target.ProjectionMode)
	}
}

func codexHomeTargets(paths []string) []TargetRef {
	targets := make([]TargetRef, 0, len(paths))
	for _, path := range paths {
		targets = append(targets, TargetRef{
			SurfaceID:      string(surface.CodexHomeDefault),
			Authority:      string(surface.AuthorityAIGW),
			ProjectionMode: ProjectionFullSelection,
			Path:           path,
		})
	}
	return targets
}
