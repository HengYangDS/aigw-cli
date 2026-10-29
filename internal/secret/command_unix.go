//go:build !windows

package secret

func nativeShellPath(path string) (string, error) { return path, nil }
