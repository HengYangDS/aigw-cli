// Package artifact owns the release artifact format: the platform matrix, the
// names of a release's artifacts, the checksum manifest and its signature, and
// the archives that program update verifies and reads.
package artifact

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
)

func extractBinary(archivePath, expectedPath string) ([]byte, error) {
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
		// Tar entry names always use "/", so the comparison uses the
		// slash-based "path" package.
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
