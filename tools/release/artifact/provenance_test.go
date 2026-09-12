package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteProvenanceBindsSourceLocksToolsAndUnsignedArtifacts(t *testing.T) {
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
