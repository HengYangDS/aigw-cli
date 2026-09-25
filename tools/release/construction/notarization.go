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
	APIKeyFile      string
	APIKeyID        string
	APIIssuerID     string
}

func (submission Notarization) authArguments() ([]string, error) {
	apiKeySelected := submission.APIKeyFile != "" || submission.APIKeyID != "" || submission.APIIssuerID != ""
	if submission.KeychainProfile != "" {
		if apiKeySelected {
			return nil, errors.New("macOS notarization requires exactly one authentication mode")
		}
		return []string{"--keychain-profile", submission.KeychainProfile}, nil
	}
	if submission.APIKeyFile == "" || submission.APIKeyID == "" {
		return nil, errors.New("macOS notarization requires a Keychain profile or complete API key identity")
	}
	arguments := []string{"--key", submission.APIKeyFile, "--key-id", submission.APIKeyID}
	if submission.APIIssuerID != "" {
		arguments = append(arguments, "--issuer", submission.APIIssuerID)
	}
	return arguments, nil
}

// VerifyMacOSDistribution verifies publisher identity and Apple's acceptance of the exact programs.
func VerifyMacOSDistribution(ctx context.Context, directory, version, identity string, submission Notarization) error {
	if identity == "" {
		return errors.New("macOS distribution verification requires an explicit signing identity")
	}
	if submission.Archive == "" || submission.SubmissionID == "" {
		return errors.New("macOS distribution requires the uploaded ZIP and Apple submission ID")
	}
	if _, err := submission.authArguments(); err != nil {
		return err
	}
	request := buildRequest{Version: version, Epoch: "0", MacOSSigningIdentity: identity}
	if err := validateRequest(request); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return verifyNotarizedArchives(request, directory, submission, executeTool(bounded))
}

// acceptedUploadDigest observes Apple's verdict through the selected native credential mode.
func (submission Notarization) acceptedUploadDigest(logPath string, run toolRunner) (string, error) {
	authArguments, err := submission.authArguments()
	if err != nil {
		return "", err
	}
	arguments := append([]string{"notarytool", "log", submission.SubmissionID}, authArguments...)
	arguments = append(arguments, logPath)
	if err := run(toolCall{Name: "xcrun", Args: arguments}); err != nil {
		return "", fmt.Errorf("query Apple notarization: %w", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", err
	}
	var log struct {
		JobID  string `json:"jobId"`
		Status string `json:"status"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &log); err != nil {
		return "", fmt.Errorf("decode Apple notarization log: %w", err)
	}
	if log.JobID != submission.SubmissionID || log.Status != "Accepted" {
		return "", fmt.Errorf("Apple submission is not accepted: %s", log.Status)
	}
	return log.SHA256, nil
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
	digest, err := submission.acceptedUploadDigest(filepath.Join(scratch, "apple-log.json"), run)
	if err != nil {
		return err
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
	if fmt.Sprintf("%x", hash.Sum(nil)) != digest {
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
