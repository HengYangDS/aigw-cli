// Package artifact verifies and reads platform-specific portable release archives.
package artifact

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func extractBinary(archivePath, expectedPath string) ([]byte, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZipBinary(archivePath, expectedPath)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open update archive: %w", err)
	}
	defer func() { _ = archive.Close() }()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return nil, fmt.Errorf("open gzip archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	reader := tar.NewReader(gz)
	var binary []byte
	matches := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar archive: %w", err)
		}
		// Tar entry names always use "/" regardless of the host OS, so the
		// comparison must use the slash-based "path" package rather than
		// "path/filepath", which would normalize to "\\" on Windows.
		if path.Clean(header.Name) != expectedPath || header.Typeflag != tar.TypeReg {
			continue
		}
		matches++
		if matches > 1 {
			return nil, fmt.Errorf("update archive contains multiple expected AIGW binaries")
		}
		binary, err = io.ReadAll(io.LimitReader(reader, 128<<20))
		if err != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", err)
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("expected AIGW binary is missing from update archive")
	}
	return binary, nil
}

func extractZipBinary(archivePath, expectedPath string) ([]byte, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	defer func() { _ = archive.Close() }()
	var binary []byte
	matches := 0
	for _, file := range archive.File {
		// Zip entry names always use "/" regardless of the host OS, so the
		// comparison must use the slash-based "path" package rather than
		// "path/filepath", which would normalize to "\\" on Windows.
		if path.Clean(file.Name) != expectedPath || file.FileInfo().IsDir() {
			continue
		}
		matches++
		if matches > 1 {
			return nil, fmt.Errorf("update archive contains multiple expected AIGW binaries")
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open AIGW binary in zip: %w", err)
		}
		binary, err = io.ReadAll(io.LimitReader(reader, 128<<20))
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("extract AIGW binary: %w", err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close AIGW binary in zip: %w", closeErr)
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("expected AIGW binary is missing from update archive")
	}
	return binary, nil
}
