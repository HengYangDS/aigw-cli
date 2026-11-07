package model

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// TestEveryCommandReportsAConfigurationThatDoesNotLoad runs each subcommand on
// a configuration that does not parse: each returns the load failure before
// it reads a catalog or writes anything.
func TestEveryCommandReportsAConfigurationThatDoesNotLoad(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(f.runtime.Config.Path(), []byte("version = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list"}, {"show", "team/team-model"}, {"add", "team/new"}, {"edit", "team/team-model", "--label", "x"}, {"remove", "team/team-model"},
	} {
		if err := f.run(args...); err == nil || !strings.Contains(err.Error(), "parse") {
			t.Errorf("model %q on a broken configuration = %v", args, err)
		}
	}
	if f.catalog.requests != 0 {
		t.Fatalf("%d catalog requests on a broken configuration", f.catalog.requests)
	}
}

// TestAWriteThatCannotCommitChangesNothing blocks the configuration's backup
// path, so that each write fails at commit and leaves the configuration as it
// was.
func TestAWriteThatCannotCommitChangesNothing(t *testing.T) {
	f := newFixture(t)
	backup := f.runtime.Config.Path() + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "team/spare-model"}, {"edit", "team/team-model", "--label", "x"}, {"remove", "mixed/Chat-Only"},
	} {
		if err := f.run(args...); err == nil {
			t.Errorf("model %q committed through a blocked backup", args)
		}
	}
	if after, err := os.ReadFile(f.runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("a failed write changed the configuration: %v", err)
	}
}

// TestAPreviewRefusesWhatTheWriteRefuses applies a change whose result fails
// validation, a model of a provider that the configuration lacks: the
// preview and the write both refuse it, before anything is written.
func TestAPreviewRefusesWhatTheWriteRefuses(t *testing.T) {
	f := newFixture(t)
	before, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	cfg := f.load(t)
	after := cfg.Clone()
	after.Models["orphan"] = domain.Model{Provider: "missing", Family: "team-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	command := NewCommand(f.runtime)
	command.SetArgs([]string{"add", "--help"})
	add, _, _ := command.Find([]string{"add"})
	for _, dryRun := range []bool{true, false} {
		err := change{before: cfg, after: after, subject: "model add", version: addSchemaVersion}.apply(add, f.runtime, []string{"missing/orphan"}, dryRun, false)
		if err == nil || !strings.Contains(err.Error(), "refuse invalid config") {
			t.Fatalf("a change to an invalid configuration with dryRun %t = %v", dryRun, err)
		}
	}
	if after, err := os.ReadFile(f.runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("a refused change wrote the configuration: %v", err)
	}
}

// TestAChangeNamesTheClientFilesThatItWrites adds a model to the provider of
// Hermes's selection, whose configuration lists every model of that provider:
// the preview names Hermes's file, which the addition would change.
func TestAChangeNamesTheClientFilesThatItWrites(t *testing.T) {
	f := newFixture(t)
	home := t.TempDir()
	f.runtime.Host = discovery.Host{Home: home, GOOS: "linux"}
	// The program and its credential reader lie in the test's own directory.
	f.runtime.Executable = filepath.Join(home, "bin", "aigw")
	reader := filepath.Join(home, "data", "credential", "aigw")
	f.runtime.ResolveCredentialPath = func() (string, error) { return reader, nil }
	target := filepath.Join(home, ".hermes", "config.yaml")
	cfg := f.load(t)
	cfg.SetSelectedModel("hermes", "mixed-chat", domain.ProtocolOpenAIChatCompletions)
	cfg.SetClientActivation("hermes", true, "", []string{target})
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.Secrets.Set("mixed", "mixed-token"); err != nil {
		t.Fatal(err)
	}
	if err := f.run("add", "mixed/spare", "--protocol", "chat", "--unlisted", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "Client files Hermes ~/.hermes/config.yaml") {
		t.Fatalf("the preview does not name Hermes's file:\n%s", f.out)
	}
	if err := f.run("add", "mixed/spare", "--protocol", "chat", "--unlisted", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	if files := f.document(t).ClientFiles; len(files) != 1 || files[0].Client != "hermes" || files[0].File != "~/.hermes/config.yaml" {
		t.Fatalf("client files = %+v", files)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("a preview wrote Hermes's file: %v", err)
	}
}
