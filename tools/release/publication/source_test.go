package publication

import (
	"aigw-cli/internal/program/artifact"
	"aigw-cli/tools/release/provenance"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvenanceUsesTaggedInputsAndCompleteArtifactSubjects(t *testing.T) {
	directory := releaseFixture(t, "1.2.3")
	source := fixtureSource(directory)
	commit := releaseGit(t, source.Repository, "rev-parse", "refs/tags/v1.2.3^{commit}")
	tree := releaseGit(t, source.Repository, "rev-parse", commit+"^{tree}")
	for _, name := range []string{"go.mod", "go.sum", "package-lock.json", ".config/mise/mise.lock", ".config/mise/config.toml"} {
		t.Run(name, func(t *testing.T) {
			original, err := os.ReadFile(filepath.Join(source.Repository, name))
			if err != nil {
				t.Fatal(err)
			}
			changed := []byte("changed input\n")
			if name == ".config/mise/config.toml" {
				changed = []byte("[tools]\ngo = \"0.0.0\"\n")
			}
			if err := os.WriteFile(filepath.Join(source.Repository, name), changed, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := provenance.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err != nil {
				t.Fatalf("mutable checkout must not replace tagged inputs: %v", err)
			}
			target := filepath.Join(directory, "aigw_1.2.3.provenance.json")
			if err := provenance.WriteProvenance(source.Repository, directory, target, "1.2.3", commit, tree); err != nil {
				t.Fatal(err)
			}
			if err := provenance.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
				t.Fatal("untagged dependency or toolchain input accepted")
			}
			if err := os.WriteFile(filepath.Join(source.Repository, name), original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := provenance.WriteProvenance(source.Repository, directory, target, "1.2.3", commit, tree); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(directory, artifact.Archives("1.2.3")[0]), []byte("different binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provenance.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
		t.Fatal("artifact absent from signed provenance subjects accepted")
	}
}

func TestProvenanceUsesTheSignedNativeToolchainLocation(t *testing.T) {
	directory := releaseFixture(t, "1.2.3")
	source := fixtureSource(directory)
	previousCommit := releaseGit(t, source.Repository, "rev-parse", "refs/tags/v1.2.3^{commit}")
	previousTree := releaseGit(t, source.Repository, "rev-parse", previousCommit+"^{tree}")
	for name, destination := range map[string]string{"config.toml": "mise.toml", "mise.lock": "mise.lock"} {
		if err := os.Rename(filepath.Join(source.Repository, ".config", "mise", name), filepath.Join(source.Repository, destination)); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, source.Repository, "add", "-A")
	releaseGit(t, source.Repository, "commit", "-q", "-S", "-m", "test: native root toolchain inputs")
	releaseGit(t, source.Repository, "tag", "-f", "-s", "-m", "Release", "v1.2.3")
	commit := releaseGit(t, source.Repository, "rev-parse", "refs/tags/v1.2.3^{commit}")
	tree := releaseGit(t, source.Repository, "rev-parse", commit+"^{tree}")
	target := filepath.Join(directory, "aigw_1.2.3.provenance.json")
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	declared := strings.NewReplacer(previousCommit, commit, previousTree, tree, "file:.config/mise/mise.lock", "file:mise.lock").Replace(string(original))
	if err := os.WriteFile(target, []byte(declared), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provenance.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err != nil {
		t.Fatalf("signed predecessor native inputs were not preserved: %v", err)
	}
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provenance.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
		t.Fatal("a different signed source and lock location replaced the selected provenance")
	}
}

func TestProvenanceRequiresApprovedSignedSourceAndVersion(t *testing.T) {
	for name, mutate := range map[string]func(*testing.T, provenance.SourceTrust){
		"missing tag": func(t *testing.T, source provenance.SourceTrust) {
			releaseGit(t, source.Repository, "tag", "-d", "v1.2.3")
		},
		"lightweight tag": func(t *testing.T, source provenance.SourceTrust) {
			releaseGit(t, source.Repository, "tag", "-d", "v1.2.3")
			releaseGit(t, source.Repository, "-c", "tag.gpgsign=false", "tag", "v1.2.3")
		},
		"unsigned source": func(t *testing.T, source provenance.SourceTrust) {
			releaseGit(t, source.Repository, "commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", "test: unsigned source")
			releaseGit(t, source.Repository, "tag", "-f", "-s", "-m", "Release", "v1.2.3")
		},
		"different version": func(t *testing.T, source provenance.SourceTrust) {
			if err := os.WriteFile(filepath.Join(source.Repository, "VERSION"), []byte("2.0.0\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			releaseGit(t, source.Repository, "add", "VERSION")
			releaseGit(t, source.Repository, "commit", "-q", "-S", "-m", "test: different version")
			releaseGit(t, source.Repository, "tag", "-f", "-s", "-m", "Release", "v1.2.3")
		},
		"unapproved signer": func(t *testing.T, source provenance.SourceTrust) {
			if err := os.WriteFile(source.AllowedSigners, []byte("# No approved signers\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := releaseFixture(t, "1.2.3")
			source := fixtureSource(directory)
			if err := provenance.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err != nil {
				t.Fatal(err)
			}
			mutate(t, source)
			if err := provenance.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
				t.Fatal("source admission accepted a source without its required signed identity")
			}
		})
	}
}
