//go:build !windows

package credential

func nativeShellPath(path string) (string, error) { return path, nil }

func nativeCanonicalPath(path string) (string, error) { return path, nil }
