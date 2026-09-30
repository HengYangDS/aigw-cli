//go:build windows

package credential

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func nativeShellPath(path string) (string, error) {
	encoded, err := windows.UTF16FromString(path)
	if err != nil {
		return "", errors.New("credential executable has invalid native path encoding")
	}
	if len(encoded) <= windows.MAX_PATH && !strings.HasPrefix(path, `\\?\`) {
		return path, nil
	}
	var selected string
	for parent, atRoot := filepath.Dir(path), false; !atRoot; parent = filepath.Dir(parent) {
		atRoot = parent == filepath.Dir(parent)
		info, err := os.Stat(parent)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect credential path ancestor: %w", err)
		}
		short, err := nativePathName(parent, true)
		if err != nil {
			return "", err
		}
		shortInfo, err := os.Stat(short)
		if err != nil || !os.SameFile(info, shortInfo) {
			return "", errors.New("native credential path alias does not identify its owned ancestor")
		}
		relative, err := filepath.Rel(parent, path)
		if err != nil {
			return "", err
		}
		candidate := filepath.Join(short, relative)
		encoded, err := windows.UTF16FromString(candidate)
		if err == nil && len(encoded) <= windows.MAX_PATH && !strings.ContainsAny(candidate, "\"%!^&|<>()") {
			// The shallowest sufficient alias stays unchanged when reader
			// preparation creates its previously missing descendants.
			selected = candidate
		}
	}
	if selected == "" {
		return "", errors.New("credential executable exceeds the native Windows shell path limit and has no usable native alias")
	}
	return selected, nil
}

func nativeCanonicalPath(path string) (string, error) {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		if _, err := os.Stat(parent); err == nil {
			long, err := nativePathName(parent, false)
			if err != nil {
				return "", err
			}
			relative, err := filepath.Rel(parent, path)
			if err != nil {
				return "", err
			}
			return filepath.Join(long, relative), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect credential invocation ancestor: %w", err)
		}
		if parent == filepath.Dir(parent) {
			return "", errors.New("credential invocation has no native path ancestor")
		}
	}
}

func nativePathName(path string, short bool) (string, error) {
	native := path
	if !strings.HasPrefix(native, `\\?\`) {
		if unc, found := strings.CutPrefix(native, `\\`); found {
			native = `\\?\UNC\` + unc
		} else {
			native = `\\?\` + native
		}
	}
	input, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return "", err
	}
	const bufferSize = 32768
	buffer := make([]uint16, bufferSize)
	var length uint32
	if short {
		length, err = windows.GetShortPathName(input, &buffer[0], bufferSize)
	} else {
		length, err = windows.GetLongPathName(input, &buffer[0], bufferSize)
	}
	if err != nil {
		return "", fmt.Errorf("resolve native credential path name: %w", err)
	}
	if length == 0 || length >= bufferSize {
		return "", errors.New("native credential path name has an invalid length")
	}
	resolved := windows.UTF16ToString(buffer[:length])
	if unc, found := strings.CutPrefix(resolved, `\\?\UNC\`); found {
		return `\\` + unc, nil
	}
	return strings.TrimPrefix(resolved, `\\?\`), nil
}
