//go:build !windows

package winjob

import "testing"

func TestNonWindowsPackageSurface(t *testing.T) {
	if Supported() {
		t.Fatal("non-Windows build reported Windows Job Object support")
	}
}
