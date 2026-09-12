package artifact

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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
	for _, namespace := range []string{"git", SignatureNamespace} {
		if err := os.WriteFile(trust.AllowedSigners, []byte(trust.Principal+` namespaces="`+namespace+`" `+string(publicKey)), 0o600); err != nil {
			t.Fatal(err)
		}
		err := VerifyMatrix(context.Background(), directory, "1.2.3", trust)
		if (err == nil) != (namespace == SignatureNamespace) {
			t.Fatalf("namespace %q: %v", namespace, err)
		}
	}
	if err := os.Remove(filepath.Join(directory, Archives("1.2.3")[0])); err != nil {
		t.Fatal(err)
	}
	if err := VerifyMatrix(context.Background(), directory, "1.2.3", trust); err == nil {
		t.Fatal("authorized signature must cover a complete artifact matrix")
	}
}

func TestNamesIncludeCompleteReleaseEvidence(t *testing.T) {
	want := []string{
		"aigw_1.2.3.spdx.json",
		"aigw_1.2.3.vulnerabilities.json",
		"aigw_1.2.3.licenses.json",
		"aigw_1.2.3.provenance.json",
		"checksums.txt",
		"checksums.txt.sig",
	}
	got := Names("1.2.3")
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("release matrix does not contain %q: %v", name, got)
		}
	}
}

func TestValidateMatrixVerifiesDetachedSignature(t *testing.T) {
	version := "1.2.3"
	key := signingKey(t)
	directory := writeFixtureWithKey(t, version, key)
	if err := ValidateMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt.sig"), []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("invalid detached signature=%v", err)
	}
}

func TestMatrixRejectsMissingExtraAndCorruptFiles(t *testing.T) {
	version := "1.2.3"
	if err := ValidateMatrix(filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing matrix accepted")
	}
	directory := writeFixtureWithKey(t, version, signingKey(t))
	if err := ValidateMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, Names(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("missing artifact=%v", err)
	}

	directory = writeFixtureWithKey(t, version, signingKey(t))
	if err := os.WriteFile(filepath.Join(directory, Names(version)[0]), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("empty artifact=%v", err)
	}

	directory = writeFixtureWithKey(t, version, signingKey(t))
	if err := os.WriteFile(filepath.Join(directory, "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("extra artifact=%v", err)
	}

	directory = writeFixtureWithKey(t, version, signingKey(t))
	if err := os.WriteFile(filepath.Join(directory, Names(version)[0]), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt artifact=%v", err)
	}

	directory = writeFixtureWithKey(t, version, signingKey(t))
	checksumPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumPath, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "invalid checksum") {
		t.Fatalf("invalid checksum manifest=%v", err)
	}

	directory = writeFixtureWithKey(t, version, signingKey(t))
	content, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(content), "\n")
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(first+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "duplicate checksum") {
		t.Fatalf("duplicate checksum=%v", err)
	}

	directory = writeFixtureWithKey(t, version, signingKey(t))
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(strings.Repeat("0", 64)+"  unknown\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected entries") {
		t.Fatalf("unexpected checksum=%v", err)
	}
}

func TestCompareMatrices(t *testing.T) {
	version := "1.2.3"
	key := signingKey(t)
	left, right := writeFixtureWithKey(t, version, key), writeFixtureWithKey(t, version, key)
	if err := CompareMatrices(left, right, version); err != nil {
		t.Fatal(err)
	}
	if err := CompareMatrices(filepath.Join(t.TempDir(), "missing"), right, version); err == nil {
		t.Fatal("missing left matrix accepted")
	}
	if err := os.WriteFile(filepath.Join(right, Names(version)[0]), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RewriteChecksums(right, version); err != nil {
		t.Fatal(err)
	}
	signFixture(t, filepath.Join(right, "checksums.txt"), key)
	if err := CompareMatrices(left, right, version); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different matrix=%v", err)
	}
	if err := CompareMatrices(left, filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing right matrix accepted")
	}
}

func TestRewriteChecksumsReportsInputAndOutputFailures(t *testing.T) {
	version := "1.2.3"
	directory := writeFixtureWithKey(t, version, signingKey(t))
	if err := os.Remove(filepath.Join(directory, Names(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := RewriteChecksums(directory, version); err == nil {
		t.Fatal("missing checksum input accepted")
	}

	directory = writeFixtureWithKey(t, version, signingKey(t))
	manifest := filepath.Join(directory, "checksums.txt")
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(manifest, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RewriteChecksums(directory, version); err == nil {
		t.Fatal("unwritable checksum destination accepted")
	}
}

func writeFixtureWithKey(t *testing.T, version, key string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range Names(version) {
		if name == "checksums.txt" || name == "checksums.txt.sig" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := RewriteChecksums(directory, version); err != nil {
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
	if output, err := exec.Command("ssh-keygen", "-Y", "sign", "-n", SignatureNamespace, "-f", key, manifest).CombinedOutput(); err != nil {
		t.Fatalf("sign checksum manifest: %v: %s", err, output)
	}
}
