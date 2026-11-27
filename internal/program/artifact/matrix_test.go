package artifact

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

func TestArchivesFollowTheTargets(t *testing.T) {
	var want []string
	for _, target := range Targets() {
		want = append(want, target.ArchiveName("1.2.3"))
	}
	if len(want) == 0 {
		t.Fatal("the release matrix has no target")
	}
	if got := Archives("1.2.3"); !slices.Equal(got, want) {
		t.Fatalf("archives = %v, want %v", got, want)
	}
	targets := Targets()
	targets[0].OS = "plan9"
	if got := Archives("1.2.3"); !slices.Equal(got, want) {
		t.Fatalf("a caller changed the platform matrix: %v", got)
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
	if subjects := ProvenanceSubjects("1.2.3"); slices.Contains(subjects, "aigw_1.2.3.provenance.json") || slices.Contains(subjects, "checksums.txt") {
		t.Fatalf("provenance names itself or the manifest as a subject: %v", subjects)
	}
}

func TestValidateMatrixVerifiesTheSignatureEnvelope(t *testing.T) {
	version := "1.2.3"
	signer := matrixSigner(t)
	directory := writeMatrix(t, version, signer)
	if err := ValidateMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	// The envelope names its own key; whether that signer is authorized is each consumer's decision.
	signMatrix(t, directory, matrixSigner(t), SignatureNamespace)
	if err := ValidateMatrix(directory, version); err != nil {
		t.Fatalf("another signer's valid envelope: %v", err)
	}
	signMatrix(t, directory, signer, "git")
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("signature in another namespace = %v", err)
	}
	signMatrix(t, directory, signer, SignatureNamespace)
	if err := os.WriteFile(filepath.Join(directory, Names(version)[0]), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("signature over an earlier manifest = %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt.sig"), []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("invalid detached signature = %v", err)
	}
}

// The envelope check accepts a signature only as ssh-keygen writes it.
func TestValidateMatrixRefusesAnInexactSignatureEnvelope(t *testing.T) {
	version := "1.2.3"
	directory := writeMatrix(t, version, matrixSigner(t))
	path := filepath.Join(directory, "checksums.txt.sig")
	armored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(armored)
	for name, variant := range map[string][]byte{
		"leading text":   append([]byte("note\n"), armored...),
		"trailing text":  append(slices.Clone(armored), []byte("note\n")...),
		"header":         pem.EncodeToMemory(&pem.Block{Type: block.Type, Headers: map[string]string{"Comment": "release"}, Bytes: block.Bytes}),
		"trailing bytes": pem.EncodeToMemory(&pem.Block{Type: block.Type, Bytes: append(slices.Clone(block.Bytes), 0)}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, variant, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "signature") {
				t.Fatalf("inexact envelope = %v", err)
			}
		})
	}
	if err := os.WriteFile(path, armored, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err != nil {
		t.Fatalf("the exact envelope was refused: %v", err)
	}
}

func TestMatrixRejectsMissingExtraAndCorruptFiles(t *testing.T) {
	version := "1.2.3"
	signer := matrixSigner(t)
	if err := ValidateMatrix(filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing matrix accepted")
	}
	directory := writeMatrix(t, version, signer)
	if err := os.Remove(filepath.Join(directory, Names(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("missing artifact=%v", err)
	}

	directory = writeMatrix(t, version, signer)
	if err := os.WriteFile(filepath.Join(directory, Names(version)[0]), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("empty artifact=%v", err)
	}

	directory = writeMatrix(t, version, signer)
	if err := os.WriteFile(filepath.Join(directory, "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("extra artifact=%v", err)
	}

	directory = writeMatrix(t, version, signer)
	if err := os.WriteFile(filepath.Join(directory, Names(version)[0]), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt artifact=%v", err)
	}

	directory = writeMatrix(t, version, signer)
	checksumPath := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksumPath, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "invalid checksum") {
		t.Fatalf("invalid checksum manifest=%v", err)
	}

	directory = writeMatrix(t, version, signer)
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

	directory = writeMatrix(t, version, signer)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), append(content, []byte(strings.Repeat("0", 64)+"  unknown\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected entries") {
		t.Fatalf("unexpected checksum=%v", err)
	}
}

func TestCompareMatrices(t *testing.T) {
	version := "1.2.3"
	signer := matrixSigner(t)
	left, right := writeMatrix(t, version, signer), writeMatrix(t, version, signer)
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
	signMatrix(t, right, signer, SignatureNamespace)
	if err := CompareMatrices(left, right, version); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different matrix=%v", err)
	}
	if err := CompareMatrices(left, filepath.Join(t.TempDir(), "missing"), version); err == nil {
		t.Fatal("missing right matrix accepted")
	}
}

func TestCompareMatricesComparesRawChecksumManifest(t *testing.T) {
	version := "1.2.3"
	signer := matrixSigner(t)
	left, right := writeMatrix(t, version, signer), writeMatrix(t, version, signer)
	manifest := filepath.Join(right, "checksums.txt")
	content, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	// Keep identical payloads and signature bytes to isolate the manifest comparison.
	content = []byte(strings.ReplaceAll(string(content), "  ", "   "))
	if err := os.WriteFile(manifest, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifiedDigests(right, Names(version)); err != nil {
		t.Fatalf("equivalent checksum entries changed matrix validity: %v", err)
	}
	if err := CompareMatrices(left, right, version); err == nil || !strings.Contains(err.Error(), "differs across forge stages: checksums.txt") {
		t.Fatalf("different checksum manifest bytes: %v", err)
	}
}

func TestRewriteChecksumsReportsInputAndOutputFailures(t *testing.T) {
	version := "1.2.3"
	directory := writeMatrix(t, version, matrixSigner(t))
	if err := os.Remove(filepath.Join(directory, Names(version)[0])); err != nil {
		t.Fatal(err)
	}
	if err := RewriteChecksums(directory, version); err == nil {
		t.Fatal("missing checksum input accepted")
	}

	directory = writeMatrix(t, version, matrixSigner(t))
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

// writeMatrix writes a complete matrix of fixture artifacts, its canonical
// manifest and the manifest's signature by signer.
func writeMatrix(t *testing.T, version string, signer ssh.Signer) string {
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
	signMatrix(t, directory, signer, SignatureNamespace)
	return directory
}

func matrixSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func signMatrix(t *testing.T, directory string, signer ssh.Signer, namespace string) {
	t.Helper()
	manifest, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	signature, err := sshsig.Sign(bytes.NewReader(manifest), signer, sshsig.HashSHA512, namespace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt.sig"), sshsig.Armor(signature), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A predecessor is verified by the matrix that its own signed manifest names, which may list
// platforms that the current matrix no longer publishes.
func TestPublishedMatrixIsTheOneItsOwnManifestNames(t *testing.T) {
	version := "1.2.3"
	signer := matrixSigner(t)
	current := writeMatrix(t, version, signer)
	if names, err := PublishedNames(current, version); err != nil || !slices.Equal(names, Names(version)) {
		t.Fatalf("a current matrix's published names = %v, %v", names, err)
	}
	if err := ValidatePublishedMatrix(current, version); err != nil {
		t.Fatal(err)
	}
	earlier := "aigw_" + version + "_windows_amd64.zip"
	names := append([]string{earlier}, Names(version)[:len(Names(version))-2]...)
	predecessor := writePublishedMatrix(t, names, signer, map[string]string{})
	if err := ValidateMatrix(predecessor, version); err == nil {
		t.Fatal("the current matrix accepted an earlier platform")
	}
	if err := ValidatePublishedMatrix(predecessor, version); err != nil {
		t.Fatalf("a predecessor with an earlier platform: %v", err)
	}
	subjects, err := PublishedSubjects(predecessor, version)
	if err != nil || !slices.Equal(subjects, names[:len(names)-1]) {
		t.Fatalf("published subjects = %v, %v", subjects, err)
	}
	remove := func(name string) func(string) error {
		return func(directory string) error { return os.Remove(filepath.Join(directory, name)) }
	}
	replace := func(name string) func(string) error {
		return func(directory string) error {
			return os.WriteFile(filepath.Join(directory, name), []byte("changed\n"), 0o600)
		}
	}
	for _, refusal := range []struct {
		name  string
		names []string
		extra map[string]string
		after func(string) error
		want  string
	}{
		{"unnamed file beside the matrix", names, map[string]string{"aigw_1.2.3_plan9_amd64.tar.gz": "x"}, nil, "unexpected or missing"},
		{"named file absent", names, nil, remove(earlier), "missing or empty artifact: " + earlier},
		{"digest mismatch", names, nil, replace(earlier), "checksum mismatch for " + earlier},
		{"duplicate name", append([]string{earlier}, names...), nil, nil, "duplicate checksum entry"},
		{"empty manifest", nil, nil, nil, "must name the archives of release 1.2.3"},
		{"dependency report absent", slices.DeleteFunc(slices.Clone(names), func(name string) bool { return name == DocumentName(version, SBOM) }), nil, nil, "must name the archives of release 1.2.3"},
		{"no archive", names[len(names)-4:], nil, nil, "must name the archives of release 1.2.3"},
		{"provenance not last", append(slices.Clone(names[1:]), earlier), nil, nil, "must name the archives of release 1.2.3"},
		{"file of another version", append([]string{"aigw_1.2.2_linux_arm64.tar.gz"}, names...), nil, nil, "not an archive of release 1.2.3"},
		{"name that only begins with the version", append([]string{"aigw_1.2.3.1_linux_arm64.tar.gz"}, names...), nil, nil, "not an archive of release 1.2.3"},
		{"document among the archives", append([]string{DocumentName(version, Licenses) + ".zip"}, names...), nil, nil, "not an archive of release 1.2.3"},
		{"path in a name", append([]string{"nested/aigw_1.2.3_linux_arm64.tar.gz"}, names...), nil, nil, "not an archive of release 1.2.3"},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			directory := writePublishedMatrix(t, refusal.names, signer, refusal.extra)
			if refusal.after != nil {
				if err := refusal.after(directory); err != nil {
					t.Fatal(err)
				}
			}
			if err := ValidatePublishedMatrix(directory, version); err == nil || !strings.Contains(err.Error(), refusal.want) {
				t.Fatalf("published matrix refusal = %v", err)
			}
		})
	}
	for _, name := range []string{earlier, Archives(version)[0], DocumentName(version, Provenance), "checksums.txt.sig"} {
		if !PublishedFile(name, version) {
			t.Errorf("%s is not a published file of %s", name, version)
		}
	}
	for _, other := range []struct{ name, version string }{
		{"aigw_1.2.3.1_linux_arm64.tar.gz", version}, {"aigw_1.2.3_linux.tar.gz", version},
		{"aigw_1.2.3_linux_arm64.tar", version}, {"aigw_1.2.30_linux_arm64.tar.gz", version}, {"notes.txt", version},
		{"aigw_1.0.0-alpha.1_linux_arm64.tar.gz", "1.0.0-alpha"},
	} {
		if PublishedFile(other.name, other.version) {
			t.Errorf("%s is a published file of %s", other.name, other.version)
		}
	}
	unsigned := writePublishedMatrix(t, names, signer, nil)
	if err := os.WriteFile(filepath.Join(unsigned, "checksums.txt.sig"), []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublishedMatrix(unsigned, version); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("a published matrix with an invalid signature = %v", err)
	}
}

// writePublishedMatrix writes the files that names lists, a manifest that names them in that order and
// its signature, and the extra files that no manifest names.
func writePublishedMatrix(t *testing.T, names []string, signer ssh.Signer, extra map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	var manifest strings.Builder
	for _, name := range names {
		content := []byte("fixture:" + name + "\n")
		path := filepath.Join(directory, filepath.Base(name))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&manifest, "%x  %s\n", sha256.Sum256(content), name)
	}
	for name, content := range extra {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(manifest.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	signMatrix(t, directory, signer, SignatureNamespace)
	return directory
}
