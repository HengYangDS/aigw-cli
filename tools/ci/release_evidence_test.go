package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestReleaseEvidenceCommandRequiresExactInputs(t *testing.T) {
	for _, arguments := range [][]string{
		{"release-evidence"},
		{"release-evidence", "--repository", "example/aigw", "--tag", "v0.3.1", "--sha", strings.Repeat("a", 40), "--workflow", "verify.yml"},
		{"release-evidence", "--repository", "example/aigw", "--tag", "v0.3.1", "--sha", strings.Repeat("a", 40), "--workflow", "verify.yml", "--job", "Quality and governance", "extra"},
	} {
		if err := run(arguments, &bytes.Buffer{}, nil); err == nil || !strings.Contains(err.Error(), "usage: ci release-evidence") {
			t.Fatalf("arguments %q were admitted: %v", arguments, err)
		}
	}
}
