package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

const (
	xarMagic      = 0x78617221
	xarHeaderSize = 28
	xarVersion    = 1
	xarSHA1       = 1
)

var creationTime = regexp.MustCompile(`<creation-time>[^<]+</creation-time>`)

func main() {
	input := flag.String("input", "", "input XAR package")
	output := flag.String("output", "", "normalized XAR package")
	epoch := flag.Int64("epoch", -1, "non-negative Unix epoch")
	flag.Parse()
	if *input == "" || *output == "" || *epoch < 0 {
		fmt.Fprintln(os.Stderr, "usage: xarnorm -input <package.xar> -output <package.xar> -epoch <non-negative epoch>")
		os.Exit(2)
	}
	if err := normalizeXAR(*input, *output, time.Unix(*epoch, 0).UTC()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func normalizeXAR(input, output string, epoch time.Time) error {
	toc, heap, err := readXAR(input)
	if err != nil {
		return err
	}
	normalized, err := normalizeTOC(toc, epoch)
	if err != nil {
		return err
	}
	compressed, err := compressTOC(normalized)
	if err != nil {
		return err
	}
	return writeXAR(output, compressed, len(normalized), heap)
}

func normalizeTOC(toc []byte, epoch time.Time) ([]byte, error) {
	for _, token := range [][]byte{
		[]byte("<inode>"),
		[]byte("<deviceno>"),
		[]byte("<atime>"),
		[]byte("<mtime>"),
		[]byte("<ctime>"),
		[]byte("<FinderCreateTime>"),
		[]byte("<ea "),
	} {
		if bytes.Contains(toc, token) {
			return nil, fmt.Errorf("XAR TOC contains volatile metadata: %s", token)
		}
	}
	matches := creationTime.FindAllIndex(toc, -1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("XAR TOC must contain exactly one creation-time element")
	}
	replacement := []byte("<creation-time>" + epoch.UTC().Format("2006-01-02T15:04:05") + "</creation-time>")
	return creationTime.ReplaceAll(toc, replacement), nil
}

func readXAR(path string) ([]byte, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if len(data) < xarHeaderSize {
		return nil, nil, errors.New("XAR is shorter than its header")
	}
	magic := binary.BigEndian.Uint32(data[0:4])
	headerSize := binary.BigEndian.Uint16(data[4:6])
	version := binary.BigEndian.Uint16(data[6:8])
	compressedLength := binary.BigEndian.Uint64(data[8:16])
	uncompressedLength := binary.BigEndian.Uint64(data[16:24])
	checksum := binary.BigEndian.Uint32(data[24:28])
	if magic != xarMagic || headerSize != xarHeaderSize || version != xarVersion || checksum != xarSHA1 {
		return nil, nil, errors.New("unsupported XAR header")
	}
	end := uint64(headerSize) + compressedLength
	if end > uint64(len(data)) || end+sha1.Size > uint64(len(data)) {
		return nil, nil, errors.New("XAR lengths exceed input size")
	}
	reader, err := zlib.NewReader(bytes.NewReader(data[headerSize:end]))
	if err != nil {
		return nil, nil, fmt.Errorf("decompress XAR TOC: %w", err)
	}
	toc, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("read XAR TOC: %w", err)
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("close XAR TOC: %w", closeErr)
	}
	if uint64(len(toc)) != uncompressedLength {
		return nil, nil, errors.New("XAR TOC length does not match its header")
	}
	sum := sha1.Sum(data[headerSize:end])
	heapStart := end + sha1.Size
	if !bytes.Equal(sum[:], data[end:heapStart]) {
		return nil, nil, errors.New("XAR compressed TOC checksum does not match its heap prefix")
	}
	return toc, append([]byte(nil), data[heapStart:]...), nil
}

func compressTOC(toc []byte) ([]byte, error) {
	var output bytes.Buffer
	writer, err := zlib.NewWriterLevel(&output, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(toc); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writeXAR(path string, compressed []byte, uncompressedLength int, heap []byte) error {
	if uncompressedLength < 0 {
		return errors.New("negative XAR TOC length")
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("re-read compressed XAR TOC: %w", err)
	}
	toc, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if len(toc) != uncompressedLength {
		return errors.New("compressed XAR TOC does not match the requested length")
	}
	header := make([]byte, xarHeaderSize)
	binary.BigEndian.PutUint32(header[0:4], xarMagic)
	binary.BigEndian.PutUint16(header[4:6], xarHeaderSize)
	binary.BigEndian.PutUint16(header[6:8], xarVersion)
	binary.BigEndian.PutUint64(header[8:16], uint64(len(compressed)))
	binary.BigEndian.PutUint64(header[16:24], uint64(uncompressedLength))
	binary.BigEndian.PutUint32(header[24:28], xarSHA1)
	sum := sha1.Sum(compressed)
	result := make([]byte, 0, len(header)+len(compressed)+len(sum)+len(heap))
	result = append(result, header...)
	result = append(result, compressed...)
	result = append(result, sum[:]...)
	result = append(result, heap...)
	return os.WriteFile(path, result, 0o644)
}
