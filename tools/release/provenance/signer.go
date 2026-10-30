package provenance

import (
	"aigw-cli/internal/program/artifact"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SignatureTrust identifies the independently supplied authorization for artifact signatures.
type SignatureTrust struct {
	AllowedSigners string
	Principal      string
}

// VerifyMatrix checks a complete artifact matrix and requires that its manifest
// signer is the principal that the allowed-signers file authorizes in the
// release namespace. ssh-keygen evaluates that file, the operator's trust input.
func VerifyMatrix(ctx context.Context, directory, version string, trust SignatureTrust) error {
	if trust.AllowedSigners == "" || trust.Principal == "" {
		return errors.New("release artifact authorization requires allowed-signers file and signer principal")
	}
	if err := artifact.ValidateMatrix(directory, version); err != nil {
		return err
	}
	manifest := filepath.Join(directory, "checksums.txt")
	file, err := os.Open(manifest)
	if err != nil {
		return fmt.Errorf("release artifact matrix: read checksum manifest for signature verification: %w", err)
	}
	defer func() { _ = file.Close() }()
	command := exec.CommandContext(ctx, "ssh-keygen", "-Y", "verify",
		"-n", artifact.SignatureNamespace,
		"-s", manifest+".sig",
		"-f", trust.AllowedSigners, "-I", trust.Principal,
	)
	command.Stdin = file
	if output, runErr := command.CombinedOutput(); runErr != nil {
		return fmt.Errorf("release artifact matrix: invalid detached signature: %w: %s", runErr, strings.TrimSpace(string(output)))
	}
	return nil
}
