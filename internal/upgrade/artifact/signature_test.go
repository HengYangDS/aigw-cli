package artifact

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

func TestVerifySignatureRequiresIndependentSignerAndExactManifest(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte("release checksum manifest\n")
	public := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
	for name, test := range map[string]struct {
		key, namespace string
		content        []byte
		valid          bool
	}{
		"authorized":       {public, SignatureNamespace, manifest, true},
		"no signer":        {"", SignatureNamespace, manifest, false},
		"invalid signer":   {"invalid", SignatureNamespace, manifest, false},
		"multiple signers": {public + public, SignatureNamespace, manifest, false},
		"signer options":   {"restrict " + public, SignatureNamespace, manifest, false},
		"wrong namespace":  {public, "git", manifest, false},
		"changed manifest": {public, SignatureNamespace, []byte("replacement manifest\n"), false},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "checksums.txt")
			signature, err := sshsig.Sign(bytes.NewReader(manifest), signer, sshsig.HashSHA512, test.namespace)
			if err != nil {
				t.Fatal(err)
			}
			for file, data := range map[string][]byte{path: test.content, path + ".sig": sshsig.Armor(signature)} {
				if err := os.WriteFile(file, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := VerifySignature(path, path+".sig", test.key); (err == nil) != test.valid {
				t.Fatalf("signature validity = %v, want %t", err, test.valid)
			}
		})
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "checksums.txt")
	if err := VerifySignature(path, path+".sig", public); err == nil {
		t.Fatal("missing manifest was accepted")
	}
	if err := os.WriteFile(path, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(path, path+".sig", public); err == nil {
		t.Fatal("missing signature was accepted")
	}
	if err := os.WriteFile(path+".sig", []byte("invalid signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(path, path+".sig", public); err == nil {
		t.Fatal("invalid signature was accepted")
	}
}

func TestVerifySignatureAcceptsNativeOpenSSHWithoutVerificationExecutable(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	key := filepath.Join(directory, "signer")
	block, err := ssh.MarshalPrivateKey(private, "synthetic release fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(manifest, []byte("native release manifest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(t.Context(), "ssh-keygen", "-Y", "sign", "-f", key, "-n", SignatureNamespace, manifest).CombinedOutput(); err != nil {
		t.Fatalf("native signing: %v: %s", err, output)
	}
	public, err := ssh.NewPublicKey(private.Public())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	if err := VerifySignature(manifest, manifest+".sig", string(ssh.MarshalAuthorizedKey(public))); err != nil {
		t.Fatal(err)
	}
}
