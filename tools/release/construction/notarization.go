package construction

import (
	"aigw-cli/internal/upgrade/artifact"
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rogpeppe/go-internal/robustio"
)

// Notarization identifies an immutable upload and its authenticated Apple submission.
type Notarization struct {
	Archive         string
	SubmissionID    string
	KeychainProfile string
}

// VerifyMacOSDistribution verifies publisher identity and Apple's acceptance of the exact programs.
func VerifyMacOSDistribution(ctx context.Context, directory, version, identity string, submission Notarization) error {
	if identity == "" {
		return errors.New("macOS distribution verification requires an explicit signing identity")
	}
	if submission.Archive == "" || submission.SubmissionID == "" || submission.KeychainProfile == "" {
		return errors.New("macOS distribution requires the uploaded ZIP, Apple submission ID, and Keychain profile")
	}
	request := buildRequest{Version: version, Epoch: "0", MacOSSigningIdentity: identity}
	if err := validateRequest(request); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return verifyNotarizedArchives(request, directory, submission, executeTool(bounded))
}

func verifyNotarizedArchives(request buildRequest, directory string, submission Notarization, run toolRunner) (result error) {
	if err := verifySignedArchives(request, directory, run); err != nil {
		return err
	}
	scratch, err := os.MkdirTemp(filepath.Dir(directory), ".notarization-verification-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(scratch)) }()
	logPath := filepath.Join(scratch, "apple-log.json")
	if err := run(toolCall{Name: "xcrun", Args: []string{"notarytool", "log", submission.SubmissionID, "--keychain-profile", submission.KeychainProfile, logPath}}); err != nil {
		return fmt.Errorf("query Apple notarization: %w", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return err
	}
	var log struct {
		JobID  string `json:"jobId"`
		Status string `json:"status"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &log); err != nil {
		return fmt.Errorf("decode Apple notarization log: %w", err)
	}
	if log.JobID != submission.SubmissionID || log.Status != "Accepted" {
		return fmt.Errorf("Apple submission is not accepted: %s", log.Status)
	}
	upload, err := os.Open(submission.Archive)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, upload.Close()) }()
	hash := sha256.New()
	if _, err := io.Copy(hash, upload); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != log.SHA256 {
		return errors.New("uploaded ZIP differs from the archive accepted by Apple")
	}
	info, err := upload.Stat()
	if err != nil {
		return err
	}
	archive, err := zip.NewReader(upload, info.Size())
	if err != nil {
		return err
	}
	wanted := make(map[string]string)
	sizes := make(map[uint64]int64)
	for _, arch := range []string{"amd64", "arm64"} {
		target := artifact.Target{OS: "darwin", Arch: arch}
		program, err := target.ReadProgram(filepath.Join(directory, target.ArchiveName(request.Version)), filepath.Join(directory, "checksums.txt"), request.Version)
		if err != nil {
			return err
		}
		wanted[fmt.Sprintf("%x", sha256.Sum256(program))] = arch
		sizes[uint64(len(program))] = int64(len(program))
	}
	for _, entry := range archive.File {
		size, matches := sizes[entry.UncompressedSize64]
		if entry.FileInfo().IsDir() || !matches {
			continue
		}
		stream, err := entry.Open()
		if err != nil {
			return err
		}
		hash.Reset()
		count, readErr := io.CopyN(hash, stream, size+1)
		closeErr := stream.Close()
		if !errors.Is(readErr, io.EOF) || count != size || closeErr != nil {
			return fmt.Errorf("accepted upload entry has invalid size or checksum: %w", errors.Join(readErr, closeErr))
		}
		delete(wanted, fmt.Sprintf("%x", hash.Sum(nil)))
	}
	if len(wanted) != 0 {
		return errors.New("final macOS executables are absent from the accepted Apple upload")
	}
	return nil
}
