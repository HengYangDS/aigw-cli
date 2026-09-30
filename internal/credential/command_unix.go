//go:build !windows

package credential

func nativeShellPath(path string) (string, error) { return path, nil }
