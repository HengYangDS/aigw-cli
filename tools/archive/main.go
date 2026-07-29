package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

type archiveFormat string

const (
	formatTarGz archiveFormat = "tar.gz"
	formatZip   archiveFormat = "zip"
)

type archiveEntry struct {
	Name   string
	Source string
}

type entryFlag []string

func (f *entryFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *entryFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("archive", flag.ContinueOnError)
	flags.SetOutput(stderr)
	formatFlag := flags.String("format", "", "archive format: tar.gz or zip")
	output := flags.String("output", "", "output archive path")
	root := flags.String("root", "", "archive root directory")
	epochFlag := flags.Int64("epoch", -1, "non-negative Unix epoch")
	var rawEntries entryFlag
	flags.Var(&rawEntries, "entry", "archive-path=source-path; repeat for every file")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *output == "" || *root == "" || *epochFlag < 0 || len(rawEntries) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: archive -format <tar.gz|zip> -output <path> -root <directory> -epoch <non-negative epoch> -entry <archive-path=source-path> [...]")
		return 2
	}
	entries, err := parseEntries(rawEntries)
	if err == nil {
		err = writeArchive(*output, archiveFormat(*formatFlag), *root, time.Unix(*epochFlag, 0).UTC(), entries)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func parseEntries(raw []string) ([]archiveEntry, error) {
	entries := make([]archiveEntry, 0, len(raw))
	for _, item := range raw {
		name, source, ok := strings.Cut(item, "=")
		if !ok || source == "" {
			return nil, fmt.Errorf("invalid archive entry %q: want archive-path=source-path", item)
		}
		entries = append(entries, archiveEntry{Name: name, Source: source})
	}
	return entries, nil
}

func writeArchive(output string, format archiveFormat, root string, epoch time.Time, entries []archiveEntry) error {
	if err := validateArchiveName(root); err != nil {
		return fmt.Errorf("invalid archive root: %w", err)
	}
	ordered, err := normalizeEntries(entries)
	if err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	return writeArchiveContents(file, format, root, epoch, ordered)
}

func writeArchiveContents(file io.WriteCloser, format archiveFormat, root string, epoch time.Time, entries []archiveEntry) error {
	var writeErr error
	switch format {
	case formatTarGz:
		writeErr = writeTarGz(file, root, epoch, entries)
	case formatZip:
		writeErr = writeZip(file, root, epoch, entries)
	default:
		writeErr = fmt.Errorf("unsupported archive format: %s", format)
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close output archive: %w", closeErr)
	}
	return nil
}

func validateArchiveName(value string) error {
	if value == "" || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") || strings.Contains(value, "\\") {
		return errors.New("must be a clean relative slash-separated path")
	}
	return nil
}

func normalizeEntries(entries []archiveEntry) ([]archiveEntry, error) {
	if len(entries) == 0 {
		return nil, errors.New("at least one archive entry is required")
	}
	ordered := append([]archiveEntry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	for index, entry := range ordered {
		if err := validateArchiveName(entry.Name); err != nil {
			return nil, fmt.Errorf("invalid archive entry %q: %w", entry.Name, err)
		}
		if index > 0 && entry.Name == ordered[index-1].Name {
			return nil, fmt.Errorf("duplicate archive entry: %s", entry.Name)
		}
		info, err := os.Stat(entry.Source)
		if err != nil {
			return nil, fmt.Errorf("stat archive entry %s: %w", entry.Name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("archive entry %s is not a regular file", entry.Name)
		}
	}
	return ordered, nil
}

func writeTarGz(output io.Writer, root string, epoch time.Time, entries []archiveEntry) error {
	gzipWriter := gzip.NewWriter(output)
	gzipWriter.Header.ModTime = epoch
	gzipWriter.Header.OS = 255
	if err := writeTarStream(gzipWriter, root, epoch, entries); err != nil {
		return err
	}
	return gzipWriter.Close()
}

func writeTarStream(output io.Writer, root string, epoch time.Time, entries []archiveEntry) error {
	tarWriter := tar.NewWriter(output)
	if err := tarWriter.WriteHeader(&tar.Header{Name: root + "/", Mode: 0o755, Typeflag: tar.TypeDir, ModTime: epoch, Format: tar.FormatUSTAR}); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := writeTarEntry(tarWriter, root, epoch, entry); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return nil
}

func writeTarEntry(writer *tar.Writer, root string, epoch time.Time, entry archiveEntry) error {
	info, err := os.Stat(entry.Source)
	if err != nil {
		return err
	}
	file, err := os.Open(entry.Source)
	if err != nil {
		return err
	}
	return writeTarEntryContents(writer, root, epoch, entry, info, file)
}

func writeTarEntryContents(writer *tar.Writer, root string, epoch time.Time, entry archiveEntry, info os.FileInfo, file io.ReadCloser) error {
	header := &tar.Header{
		Name:     root + "/" + entry.Name,
		Mode:     int64(info.Mode().Perm()),
		Size:     info.Size(),
		Typeflag: tar.TypeReg,
		ModTime:  epoch,
		Format:   tar.FormatUSTAR,
	}
	if err := writer.WriteHeader(header); err != nil {
		_ = file.Close()
		return err
	}
	_, copyErr := io.Copy(writer, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close archive entry %s: %w", entry.Name, closeErr)
	}
	return nil
}

func writeZip(output io.Writer, root string, epoch time.Time, entries []archiveEntry) error {
	writer := zip.NewWriter(output)
	directory := &zip.FileHeader{Name: root + "/", Method: zip.Store, Modified: epoch}
	directory.SetMode(os.ModeDir | 0o755)
	if _, err := writer.CreateHeader(directory); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := writeZipEntry(writer, root, epoch, entry); err != nil {
			return err
		}
	}
	return writer.Close()
}

func writeZipEntry(writer *zip.Writer, root string, epoch time.Time, entry archiveEntry) error {
	info, err := os.Stat(entry.Source)
	if err != nil {
		return err
	}
	file, err := os.Open(entry.Source)
	if err != nil {
		return err
	}
	return writeZipEntryContents(writer, root, epoch, entry, info, file)
}

func writeZipEntryContents(writer *zip.Writer, root string, epoch time.Time, entry archiveEntry, info os.FileInfo, file io.ReadCloser) error {
	header := &zip.FileHeader{Name: root + "/" + entry.Name, Method: zip.Store, Modified: epoch}
	header.SetMode(info.Mode().Perm())
	destination, err := writer.CreateHeader(header)
	if err != nil {
		_ = file.Close()
		return err
	}
	_, copyErr := io.Copy(destination, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close archive entry %s: %w", entry.Name, closeErr)
	}
	return nil
}
