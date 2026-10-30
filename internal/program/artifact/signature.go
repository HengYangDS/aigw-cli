package artifact

import (
	"bytes"
	_ "crypto/sha256" // Register both hash algorithms supported by SSHSIG.
	_ "crypto/sha512"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

// SignatureNamespace separates release signatures from Git and authentication.
const SignatureNamespace = "aigw-release"

// BuildReleasePublicKey is bound by the release producer, never a download peer.
var BuildReleasePublicKey string

// CanonicalPublicKey admits one option-free key and removes its comment.
func CanonicalPublicKey(publicKey string) (string, error) {
	key, err := parseReleasePublicKey(publicKey)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))), nil
}

func parseReleasePublicKey(publicKey string) (ssh.PublicKey, error) {
	key, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return nil, fmt.Errorf("release signer must be a valid SSH public key: %w", err)
	}
	if len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("release signer must contain exactly one SSH public key without options")
	}
	return key, nil
}

// VerifySignature authenticates a checksum manifest against independent trust.
func VerifySignature(manifestPath, signaturePath, publicKey string) error {
	key, err := parseReleasePublicKey(publicKey)
	if err != nil {
		return err
	}
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read signed release manifest: %w", err)
	}
	armored, err := os.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("read release signature: %w", err)
	}
	signature, err := sshsig.Unarmor(armored)
	if err != nil {
		return fmt.Errorf("decode release signature: %w", err)
	}
	if err := sshsig.Verify(bytes.NewReader(manifest), signature, key, signature.HashAlgorithm, SignatureNamespace); err != nil {
		return fmt.Errorf("authenticate release manifest: %w", err)
	}
	return nil
}
