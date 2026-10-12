package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func provenanceFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":            "module example.invalid/aigw\n",
		"go.sum":            "sum\n",
		"package-lock.json": "{}\n",
		"mise.lock":         "lockfile_version = 1\n",
		"mise.toml":         "[tools]\ngo = \"1.27.1\"\nnode = \"26.8.1\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	candidate := t.TempDir()
	for _, name := range provenanceSubjects("1.2.3") {
		if err := os.WriteFile(filepath.Join(candidate, name), []byte("fixture:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, candidate
}

func TestWriteProvenanceBindsSourceLocksToolsAndUnsignedArtifacts(t *testing.T) {
	root, candidate := provenanceFixture(t)
	target := filepath.Join(candidate, "aigw_1.2.3.provenance.json")
	if err := WriteProvenance(root, candidate, target, "1.2.3", strings.Repeat("a", 40), strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		`"_type": "https://in-toto.io/Statement/v1"`,
		`"predicateType": "https://slsa.dev/provenance/v1"`,
		`"version": "1.2.3"`,
		`"go": "1.27.1"`,
		strings.Repeat("a", 40),
		strings.Repeat("b", 40),
		`"uri": "file:go.mod"`,
		`"aigw_1.2.3_linux_amd64.tar.gz"`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("provenance missing %q: %s", required, text)
		}
	}
	for _, forbidden := range []string{root, candidate, "timestamp", "credential", "installer"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("provenance contains forbidden metadata %q: %s", forbidden, text)
		}
	}
}

func TestWriteProvenancePreservesPriorEvidenceWhenNativeInputsFail(t *testing.T) {
	for _, scenario := range []struct {
		name, path, content, diagnostic string
		remove                          bool
	}{
		{"missing toolchain", "mise.toml", "", "read mise toolchain", true},
		{"invalid toolchain", "mise.toml", "[tools\n", "decode mise toolchain", false},
		{"empty toolchain", "mise.toml", "[tools]\n", "contains no tools", false},
		{"missing dependency lock", "go.sum", "", "digest release input go.sum", true},
		{"missing artifact", "", "", "digest release subject", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root, candidate := provenanceFixture(t)
			target := filepath.Join(candidate, "aigw_1.2.3.provenance.json")
			prior := []byte("retained original provenance\n")
			if err := os.WriteFile(target, prior, 0o600); err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(root, scenario.path)
			if scenario.path == "" {
				input = filepath.Join(candidate, provenanceSubjects("1.2.3")[0])
			}
			if scenario.remove {
				if err := os.Remove(input); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(input, []byte(scenario.content), 0o600); err != nil {
				t.Fatal(err)
			}
			err := WriteProvenance(root, candidate, target, "1.2.3", strings.Repeat("a", 40), strings.Repeat("b", 40))
			if err == nil || !strings.Contains(err.Error(), scenario.diagnostic) {
				t.Fatalf("incomplete native provenance inputs accepted: %v", err)
			}
			if actual, err := os.ReadFile(target); err != nil || string(actual) != string(prior) {
				t.Fatalf("failed provenance construction changed prior evidence: %v", err)
			}
		})
	}
}

func TestCandidateProvenanceRequiresExactSourceAuthorization(t *testing.T) {
	for _, commit := range []string{"HEAD", strings.Repeat("A", 40)} {
		if err := VerifyCandidateProvenance(t.Context(), t.TempDir(), SourceTrust{Commit: commit}); err == nil || !strings.Contains(err.Error(), "exact lowercase Git commit ID") {
			t.Fatalf("non-exact candidate source was accepted: %v", err)
		}
	}
	if err := VerifyCandidateProvenance(t.Context(), t.TempDir(), SourceTrust{Commit: strings.Repeat("a", 40)}); err == nil || !strings.Contains(err.Error(), "authorization requires") {
		t.Fatalf("candidate without source trust was accepted: %v", err)
	}
	if err := VerifyProvenance(t.Context(), t.TempDir(), "latest", SourceTrust{}); err == nil || !strings.Contains(err.Error(), "v<semver>") {
		t.Fatalf("unbound release reference was accepted: %v", err)
	}
}
