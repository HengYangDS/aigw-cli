package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// checkProtectedLifecycle keeps accepted publication refs free of active
// OpenSpec Changes. Implementation Changes belong to work/candidate lanes;
// an accepted tree carries only archived history and current specifications.
func checkProtectedLifecycle(root string) error {
	branch := repositoryBranch(root)
	if branch != "" && (strings.HasPrefix(branch, "work/") || strings.HasPrefix(branch, "proposal/") || branch == "candidate/dev") {
		return nil
	}
	changes := filepath.Join(root, "openspec", "changes")
	entries, err := os.ReadDir(changes)
	if err != nil {
		return fmt.Errorf("read OpenSpec changes: %w", err)
	}
	active := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "archive" || entry.Name()[0] == '.' {
			continue
		}
		active = append(active, entry.Name())
	}
	if len(active) != 0 {
		sort.Strings(active)
		return fmt.Errorf("accepted publication tree contains active OpenSpec Changes: %v; archive completed Changes before publication", active)
	}
	return nil
}

func repositoryBranch(root string) string {
	command := exec.Command("git", "-C", root, "symbolic-ref", "--quiet", "--short", "HEAD")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
