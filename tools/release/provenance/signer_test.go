package provenance

import (
	"aigw-cli/internal/program/artifact"
	"context"
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
