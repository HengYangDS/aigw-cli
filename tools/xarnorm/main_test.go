package main

import (
	"bytes"
	"compress/zlib"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeXARRewritesCreationTimeWithoutChangingHeap(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	heap := []byte("deterministic heap")
	epoch := time.Unix(1784246400, 0).UTC()
	firstInput := filepath.Join(tmp, "first-input.xar")
	secondInput := filepath.Join(tmp, "second-input.xar")
	firstOutput := filepath.Join(tmp, "first-output.xar")
	secondOutput := filepath.Join(tmp, "second-output.xar")
	firstTOC := []byte(`<?xml version="1.0"?><xar><toc><creation-time>2026-07-17T12:34:56</creation-time><file><data><offset>0</offset><length>18</length></data></file></toc></xar>`)
	secondTOC := []byte(`<?xml version="1.0"?><xar><toc><creation-time>2026-07-17T23:45:01</creation-time><file><data><offset>0</offset><length>18</length></data></file></toc></xar>`)
	if err := writeFixtureXAR(firstInput, firstTOC, heap); err != nil {
		t.Fatal(err)
	}
	if err := writeFixtureXAR(secondInput, secondTOC, heap); err != nil {
		t.Fatal(err)
	}
	if err := normalizeXAR(firstInput, firstOutput, epoch); err != nil {
		t.Fatalf("normalizeXAR(first) error = %v", err)
	}
	if err := normalizeXAR(secondInput, secondOutput, epoch); err != nil {
		t.Fatalf("normalizeXAR(second) error = %v", err)
	}
	firstBytes, err := os.ReadFile(firstOutput)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(secondOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("normalized XAR outputs differ only because their source creation-time differed")
	}
	gotTOC, gotHeap, err := readXAR(firstOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotHeap, heap) {
		t.Fatalf("heap changed: %q != %q", gotHeap, heap)
	}
	if !bytes.Contains(gotTOC, []byte("2026-07-17T00:00:00")) {
		t.Fatalf("normalized TOC did not contain release time: %s", gotTOC)
	}
}

func TestNormalizeXARRejectsVolatileTOC(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	input := filepath.Join(tmp, "input.xar")
	toc := []byte(`<xar><toc><creation-time>2026-07-17T12:34:56</creation-time><file><inode>42</inode></file></toc></xar>`)
	if err := writeFixtureXAR(input, toc, []byte("heap")); err != nil {
		t.Fatal(err)
	}
	if err := normalizeXAR(input, filepath.Join(tmp, "output.xar"), time.Unix(1, 0).UTC()); err == nil {
		t.Fatal("normalizeXAR() error = nil for a volatile TOC")
	}
}

func writeFixtureXAR(path string, toc, heap []byte) error {
	compressed := compressTOC(toc)
	return writeXAR(path, compressed, len(toc), heap)
}

func TestCompressTOCUsesZlib(t *testing.T) {
	t.Parallel()

	compressed := compressTOC([]byte("toc"))
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("compressTOC() did not produce a zlib stream: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}
