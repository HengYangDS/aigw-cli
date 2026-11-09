package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

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
	union := make([]codexReconciliationTarget, 0, len(normalizedBefore)+len(normalizedAfter))
	// A kept target replaces the dropped one that names the same file, however
	// each is spelled, so one file is never both restored and projected.
	merge := func(target TargetRef, desired bool) {
		for index, existing := range union {
			if existing.ref.Path == target.Path || sameTargetFile(existing.ref.configured, target.configured) {
				union[index] = codexReconciliationTarget{ref: target, desired: desired}
				return
			}
		}
		union = append(union, codexReconciliationTarget{ref: target, desired: desired})
	}
	for _, target := range normalizedBefore {
		merge(target, false)
	}
	for _, target := range normalizedAfter {
		if err := validateDesiredCodexTarget(target); err != nil {
			return nil, err
		}
		merge(target, true)
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
		if slices.ContainsFunc(normalized, func(kept TargetRef) bool { return kept.Path == path || sameTargetFile(kept.configured, sourcePath) }) {
			// A configuration from before sync converged one file's names, which
			// the change drops, has nothing to plan twice.
			if !desired {
				continue
			}
			return nil, &adapter.TargetError{Path: sourcePath, Desired: desired, Err: duplicateTargetError{}}
		}
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

// statTarget reads the identity that sameTargetFile compares. A test replaces
// it to stand for a volume that keeps or ignores case.
var statTarget = os.Stat

// sameTargetFile reports whether two configured paths name one file. Paths
// that both exist are one file only when the operating system says so, which
// covers a volume that ignores case and a symbolic link, never a guess from
// the platform. Otherwise they are one file only when their cleaned spellings
// are equal, which makes / and \ alike on Windows and folds no case.
func sameTargetFile(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := statTarget(left)
	rightInfo, rightErr := statTarget(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
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

// codexRecordPath is where AIGW keeps its record of the Codex file that path
// names, for every surface that reads or writes it. A path through a symbolic
// link keeps the record beside the resolved file, so two targets that resolve
// to one file share it; a record that an earlier release left beside the link
// is used until the next write moves it.
func codexRecordPath(path string) string {
	sourcePath, err := absoluteCodexTargetPath(path)
	if err != nil {
		return codexStatePath(path)
	}
	canonicalPath, err := canonicalCodexTargetPath(sourcePath)
	if err != nil {
		return codexStatePath(sourcePath)
	}
	return preferredCodexStatePath(sourcePath, canonicalPath)
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
