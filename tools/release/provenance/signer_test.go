package provenance

import (
	"aigw-cli/internal/program/artifact"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestVerifyMatrixUsesExplicitNamespaceScopedAuthorization(t *testing.T) {
	key := signingKey(t)
	directory := writeFixtureWithKey(t, "1.2.3", key)
	publicKey, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	trust := SignatureTrust{AllowedSigners: filepath.Join(t.TempDir(), "allowed-signers"), Principal: "release@test.invalid"}
	if err := VerifyMatrix(context.Background(), directory, "1.2.3", SignatureTrust{AllowedSigners: trust.AllowedSigners}); err == nil {
		t.Fatal("a matrix verified without a signer principal")
	}
	for _, namespace := range []string{"git", artifact.SignatureNamespace} {
		if err := os.WriteFile(trust.AllowedSigners, []byte(trust.Principal+` namespaces="`+namespace+`" `+string(publicKey)), 0o600); err != nil {
			t.Fatal(err)
		}
		err := VerifyMatrix(context.Background(), directory, "1.2.3", trust)
		if (err == nil) != (namespace == artifact.SignatureNamespace) {
			t.Fatalf("namespace %q: %v", namespace, err)
		}
	}
	other := writeFixtureWithKey(t, "1.2.3", signingKey(t))
	if err := VerifyMatrix(context.Background(), other, "1.2.3", trust); err == nil {
		t.Fatal("a matrix signed by an unauthorized key verified")
	}
	if err := os.Remove(filepath.Join(directory, artifact.Archives("1.2.3")[0])); err != nil {
		t.Fatal(err)
	}
	if err := VerifyMatrix(context.Background(), directory, "1.2.3", trust); err == nil {
		t.Fatal("authorized signature must cover a complete artifact matrix")
	}
}

func writeFixtureWithKey(t *testing.T, version, key string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range artifact.Names(version) {
		if name == "checksums.txt" || name == "checksums.txt.sig" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := artifact.RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	signFixture(t, filepath.Join(directory, "checksums.txt"), key)
	return directory
}

func signingKey(t *testing.T) string {
	t.Helper()
	key := filepath.Join(t.TempDir(), "release-signing-key")
	if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("generate signing key: %v: %s", err, output)
	}
	return key
}

func signFixture(t *testing.T, manifest, key string) {
	t.Helper()
	if err := os.Remove(manifest + ".sig"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if output, err := exec.Command("ssh-keygen", "-Y", "sign", "-n", artifact.SignatureNamespace, "-f", key, manifest).CombinedOutput(); err != nil {
		t.Fatalf("sign checksum manifest: %v: %s", err, output)
	}
}

func TestVerifyPublishedMatrixAuthorizesAPredecessorOfAnotherPlatformMatrix(t *testing.T) {
	key := signingKey(t)
	directory := writeFixtureWithKey(t, "1.2.3", key)
	prependEarlierPlatform(t, directory, "aigw_1.2.3_windows_amd64.zip")
	signFixture(t, filepath.Join(directory, "checksums.txt"), key)
	publicKey, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	trust := SignatureTrust{AllowedSigners: filepath.Join(t.TempDir(), "allowed-signers"), Principal: "release@test.invalid"}
	if err := os.WriteFile(trust.AllowedSigners, []byte(trust.Principal+` namespaces="`+artifact.SignatureNamespace+`" `+string(publicKey)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyMatrix(context.Background(), directory, "1.2.3", trust); err == nil {
		t.Fatal("the current matrix authorized an earlier platform")
	}
	if err := VerifyPublishedMatrix(context.Background(), directory, "1.2.3", trust); err != nil {
		t.Fatalf("a predecessor's own signed matrix: %v", err)
	}
	if err := VerifyPublishedMatrix(context.Background(), directory, "1.2.3", SignatureTrust{AllowedSigners: trust.AllowedSigners}); err == nil {
		t.Fatal("a published matrix verified without a signer principal")
	}
	other := writeFixtureWithKey(t, "1.2.3", signingKey(t))
	if err := VerifyPublishedMatrix(context.Background(), other, "1.2.3", trust); err == nil {
		t.Fatal("a published matrix signed by an unauthorized key verified")
	}
}

// prependEarlierPlatform adds an archive that the current matrix does not name and lists it first in the
// manifest, as an earlier release with that platform wrote it.
func prependEarlierPlatform(t *testing.T, directory, name string) {
	t.Helper()
	content := []byte("fixture:" + name + "\n")
	if err := os.WriteFile(filepath.Join(directory, name), content, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, "checksums.txt")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf("%x  %s\n", sha256.Sum256(content), name)
	if err := os.WriteFile(manifest, append([]byte(line), data...), 0o600); err != nil {
		t.Fatal(err)
	}
}
