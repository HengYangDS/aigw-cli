package provenance

import (
	"aigw-cli/internal/program/artifact"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A published predecessor is verified as it was published, with a platform that the current matrix no
// longer lists; the current release's own checks refuse the same directory.
func TestVerifyPublishedReleaseAcceptsAPredecessorOfAnotherPlatformMatrix(t *testing.T) {
	const earlier = "aigw_1.2.3_windows_amd64.zip"
	directory, signature, source := publishedPredecessor(t, earlier, nil)
	if err := VerifyPublishedRelease(t.Context(), directory, "v1.2.3", signature, source); err != nil {
		t.Fatalf("a predecessor as it was published: %v", err)
	}
	if err := VerifyMatrix(t.Context(), directory, "1.2.3", signature); err == nil {
		t.Fatal("the current matrix authorized an earlier platform")
	}
	if err := VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
		t.Fatal("the current subjects rebuilt a predecessor's provenance")
	}
	for name, refusal := range map[string]struct {
		tag       string
		signature SignatureTrust
		want      string
	}{
		"untagged":            {"", signature, "v<semver> tag"},
		"no signer principal": {"v1.2.3", SignatureTrust{AllowedSigners: signature.AllowedSigners}, "signer principal"},
		"another release":     {"v1.2.4", signature, "must name the archives of release 1.2.4"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := VerifyPublishedRelease(t.Context(), directory, refusal.tag, refusal.signature, source); err == nil || !strings.Contains(err.Error(), refusal.want) {
				t.Fatalf("published release refusal = %v", err)
			}
		})
	}
	t.Run("subjects in another order", func(t *testing.T) {
		last := func(subjects []string) []string { return append(subjects[1:], subjects[0]) }
		directory, signature, source := publishedPredecessor(t, earlier, last)
		if err := VerifyPublishedRelease(t.Context(), directory, "v1.2.3", signature, source); err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("a provenance with the manifest's subjects reordered = %v", err)
		}
	})
	t.Run("manifest signed by an unauthorized key", func(t *testing.T) {
		directory, signature, source := publishedPredecessor(t, earlier, nil)
		signFixture(t, filepath.Join(directory, "checksums.txt"), signingKey(t))
		if err := VerifyPublishedRelease(t.Context(), directory, "v1.2.3", signature, source); err == nil {
			t.Fatal("a manifest signed by an unauthorized key verified")
		}
	})
}

// publishedPredecessor writes release 1.2.3 as a predecessor published it: a signed tag over its source,
// an earlier platform's archive before the current matrix, its provenance with the subjects in the
// manifest's order (or as reorder rearranges them) and a signed manifest.
func publishedPredecessor(t *testing.T, earlier string, reorder func([]string) []string) (string, SignatureTrust, SourceTrust) {
	t.Helper()
	root, directory := provenanceFixture(t)
	key := signingKey(t)
	publicKey, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	signature := SignatureTrust{AllowedSigners: filepath.Join(t.TempDir(), "allowed-signers"), Principal: "release@test.invalid"}
	allowed := signature.Principal + ` namespaces="git,` + artifact.SignatureNamespace + `" ` + string(publicKey)
	if err := os.WriteFile(signature.AllowedSigners, []byte(allowed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hooks := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		output, err := exec.Command("git", append([]string{"-C", root, "-c", "core.hooksPath=" + hooks,
			"-c", "user.name=Release Test", "-c", "user.email=" + signature.Principal, "-c", "gpg.format=ssh",
			"-c", "user.signingkey=" + key, "-c", "gpg.ssh.program=ssh-keygen"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q", "--template=", "-b", "main")
	git("add", ".")
	git("commit", "-q", "-S", "-m", "test: release source")
	git("tag", "-s", "-m", "Release 1.2.3", "v1.2.3")
	if err := os.WriteFile(filepath.Join(directory, earlier), []byte("fixture:"+earlier+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	subjects := append([]string{earlier}, artifact.ProvenanceSubjects("1.2.3")...)
	provenanceSubjects := slices.Clone(subjects)
	if reorder != nil {
		provenanceSubjects = reorder(provenanceSubjects)
	}
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(root, name)) }
	data, err := provenanceBytes(read, ".config/mise/config.toml", directory, "1.2.3", provenanceSubjects, git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	provenance := artifact.DocumentName("1.2.3", artifact.Provenance)
	if err := os.WriteFile(filepath.Join(directory, provenance), data, 0o600); err != nil {
		t.Fatal(err)
	}
	var manifest strings.Builder
	for _, name := range append(subjects, provenance) {
		content, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&manifest, "%x  %s\n", sha256.Sum256(content), name)
	}
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(manifest.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	signFixture(t, filepath.Join(directory, "checksums.txt"), key)
	return directory, signature, SourceTrust{Repository: root, AllowedSigners: signature.AllowedSigners}
}
