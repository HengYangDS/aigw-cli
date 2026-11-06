//go:build !windows

package main

import (
	"os"
	"testing"
)

// denyListing takes dir's read permission away until the test ends. A file
// inside dir stays readable, because dir keeps its search permission.
func denyListing(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}
