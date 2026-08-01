package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunXARNorm(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.Join(tmp, "input.xar")
	output := filepath.Join(tmp, "output.xar")
	toc := []byte(`<xar><toc><creation-time>2026-07-17T12:34:56</creation-time></toc></xar>`)
	if err := writeFixtureXAR(input, toc, []byte("heap")); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{"-input", input, "-output", output, "-epoch", "1"}, &stderr); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	normalized, heap, err := readXAR(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(normalized, []byte("1970-01-01T00:00:01")) || string(heap) != "heap" {
		t.Fatalf("normalized TOC = %q, heap = %q", normalized, heap)
	}
}

func TestRunXARNormRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "usage", want: "usage: xarnorm"},
		{name: "unknown flag", args: []string{"-unknown"}, want: "flag provided but not defined"},
		{name: "invalid input", args: []string{"-input", "missing.xar", "-output", "out.xar", "-epoch", "1"}, want: "missing.xar"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(test.args, &stderr); code != 2 {
				t.Fatalf("run() code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("run() stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestNormalizeTOCRejectsVolatileAndAmbiguousMetadata(t *testing.T) {
	epoch := time.Unix(1, 0).UTC()
	for _, token := range []string{"<inode>", "<deviceno>", "<atime>", "<mtime>", "<ctime>", "<FinderCreateTime>", "<ea "} {
		toc := []byte(`<xar><creation-time>old</creation-time>` + token + `value</xar>`)
		if _, err := normalizeTOC(toc, epoch); err == nil || !strings.Contains(err.Error(), token) {
			t.Errorf("normalizeTOC(%q) error = %v", token, err)
		}
	}
	for _, toc := range [][]byte{
		[]byte(`<xar></xar>`),
		[]byte(`<xar><creation-time>one</creation-time><creation-time>two</creation-time></xar>`),
	} {
		if _, err := normalizeTOC(toc, epoch); err == nil {
			t.Fatalf("normalizeTOC(%q) error = nil", toc)
		}
	}
}

func TestReadXARRejectsMalformedFixtures(t *testing.T) {
	tmp := t.TempDir()
	validCompressed := compressTOC([]byte("toc"))
	truncated := append([]byte(nil), validCompressed[:len(validCompressed)-2]...)
	badChecksum := rawXAR(validCompressed, 3, nil)
	badChecksum[xarHeaderSize+len(validCompressed)] ^= 0xff
	tests := []struct {
		name string
		data []byte
	}{
		{name: "short", data: []byte("xar")},
		{name: "unsupported header", data: make([]byte, xarHeaderSize)},
		{name: "length exceeds input", data: xarWithLengths(100, 1)},
		{name: "invalid compression", data: rawXAR([]byte{0}, 1, nil)},
		{name: "truncated compression", data: rawXAR(truncated, 3, nil)},
		{name: "uncompressed length mismatch", data: rawXAR(validCompressed, 4, nil)},
		{name: "checksum mismatch", data: badChecksum},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(tmp, strings.ReplaceAll(test.name, " ", "-")+".xar")
			if err := os.WriteFile(path, test.data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readXAR(path); err == nil {
				t.Fatal("readXAR() error = nil")
			}
		})
	}
	if _, _, err := readXAR(filepath.Join(tmp, "missing.xar")); err == nil {
		t.Fatal("readXAR(missing) error = nil")
	}
}

func TestWriteXARRejectsInvalidInputs(t *testing.T) {
	tmp := t.TempDir()
	compressed := compressTOC([]byte("toc"))
	if err := writeXAR(filepath.Join(tmp, "negative.xar"), compressed, -1, nil); err == nil {
		t.Fatal("writeXAR(negative) error = nil")
	}
	if err := writeXAR(filepath.Join(tmp, "invalid.xar"), []byte("invalid"), 3, nil); err == nil {
		t.Fatal("writeXAR(invalid compression) error = nil")
	}
	if err := writeXAR(filepath.Join(tmp, "truncated.xar"), compressed[:len(compressed)-2], 3, nil); err == nil {
		t.Fatal("writeXAR(truncated compression) error = nil")
	}
	if err := writeXAR(filepath.Join(tmp, "length.xar"), compressed, 4, nil); err == nil {
		t.Fatal("writeXAR(length mismatch) error = nil")
	}
	if err := writeXAR(tmp, compressed, 3, nil); err == nil {
		t.Fatal("writeXAR(directory output) error = nil")
	}
}

func TestNormalizeXARReportsReadNormalizeAndWriteFailures(t *testing.T) {
	tmp := t.TempDir()
	epoch := time.Unix(1, 0).UTC()
	if err := normalizeXAR(filepath.Join(tmp, "missing.xar"), filepath.Join(tmp, "out.xar"), epoch); err == nil {
		t.Fatal("normalizeXAR(missing) error = nil")
	}
	volatile := filepath.Join(tmp, "volatile.xar")
	if err := writeFixtureXAR(volatile, []byte(`<xar><creation-time>old</creation-time><inode>1</inode></xar>`), nil); err != nil {
		t.Fatal(err)
	}
	if err := normalizeXAR(volatile, filepath.Join(tmp, "out.xar"), epoch); err == nil {
		t.Fatal("normalizeXAR(volatile) error = nil")
	}
	valid := filepath.Join(tmp, "valid.xar")
	if err := writeFixtureXAR(valid, []byte(`<xar><creation-time>old</creation-time></xar>`), nil); err != nil {
		t.Fatal(err)
	}
	if err := normalizeXAR(valid, tmp, epoch); err == nil {
		t.Fatal("normalizeXAR(directory output) error = nil")
	}
}

func rawXAR(compressed []byte, uncompressedLength uint64, heap []byte) []byte {
	header := make([]byte, xarHeaderSize)
	binary.BigEndian.PutUint32(header[0:4], xarMagic)
	binary.BigEndian.PutUint16(header[4:6], xarHeaderSize)
	binary.BigEndian.PutUint16(header[6:8], xarVersion)
	binary.BigEndian.PutUint64(header[8:16], uint64(len(compressed)))
	binary.BigEndian.PutUint64(header[16:24], uncompressedLength)
	binary.BigEndian.PutUint32(header[24:28], xarSHA1)
	sum := sha1.Sum(compressed)
	result := append(header, compressed...)
	result = append(result, sum[:]...)
	return append(result, heap...)
}

func xarWithLengths(compressedLength, uncompressedLength uint64) []byte {
	header := make([]byte, xarHeaderSize)
	binary.BigEndian.PutUint32(header[0:4], xarMagic)
	binary.BigEndian.PutUint16(header[4:6], xarHeaderSize)
	binary.BigEndian.PutUint16(header[6:8], xarVersion)
	binary.BigEndian.PutUint64(header[8:16], compressedLength)
	binary.BigEndian.PutUint64(header[16:24], uncompressedLength)
	binary.BigEndian.PutUint32(header[24:28], xarSHA1)
	return header
}
