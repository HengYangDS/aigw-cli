package synchronization

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// codexWithReader projects gateway/gpt-test for Codex as projectedCodex does,
// from a synchronizer that has a credential reader and a Token store, as in
// production: Codex's files are written only while the gateway has a Token
// that AIGW can use, which the returned store holds.
func codexWithReader(t *testing.T) (Synchronizer, domain.Config, string, secret.Store) {
	t.Helper()
	root := t.TempDir()
	source, target := filepath.Join(root, "aigw"), filepath.Join(root, "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{source: "executable fixture", target: "model_provider = \"native\"\n"} {
		if err := os.WriteFile(path, []byte(data), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	reader, err := versionedReaderPath(filepath.Join(root, "data"), source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(target)
	other := cfg.Models[domain.NewModelRef("gateway", "gpt-test")]
	other.ID, other.Family, other.Label = "gpt-other", "gpt-other", "Other"
	cfg.Models[other.Ref()] = other
	configStore := store.NewStore(filepath.Join(root, "aigw.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	tokens := secret.NewMemoryStore()
	if err := tokens.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{
		Registry: builtinClients, Config: configStore, Secrets: tokens, Discovery: targetDiscovery(target),
		AIGWExecutable: source, ResolveCredentialPath: readerAt(reader), Follows: &Follows{},
	}
	if err := syncer.CommitProjection(context.Background(), cfg, cfg, "sync"); err != nil {
		t.Fatal(err)
	}
	return syncer, cfg, target, tokens
}

// leftAs reports whether the file at path is still the file that info names,
// with the bytes in data: a file that was written again is another file,
// whatever was written.
func leftAs(t *testing.T, path string, info os.FileInfo, data []byte) bool {
	t.Helper()
	now, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(info, now) && bytes.Equal(current, data)
}

// chosenInCodex chooses the other model in Codex's file and returns the file
// as the choice left it.
func chosenInCodex(t *testing.T, target string) (os.FileInfo, []byte) {
	t.Helper()
	chooseOther(t, target)
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	return info, data
}

// A choice whose provider has no Token that AIGW can use is recorded by a
// transition that names nothing of the client: the stored selection and
// AIGW's record of the file move to it, the file is left as it is, and no
// credential reader is prepared. So nothing is left to follow, and the
// earlier model, chosen again, is a choice. A preview plans the same and
// writes nothing. Holds the cell chosen/other of the lane's state table.
func TestATransitionRecordsAChoiceWithoutItsToken(t *testing.T) {
	syncer, cfg, target, tokens := codexWithReader(t)
	if err := tokens.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	// The reader that the first projection installed goes too, to show that
	// a record prepares none.
	reader, err := syncer.ResolveCredentialPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(reader)); err != nil {
		t.Fatal(err)
	}
	info, chosen := chosenInCodex(t, target)
	other := domain.NewModelRef("gateway", "gpt-other")
	before, followed := syncer.Working(cfg)
	if len(followed) != 1 || before.SelectedModel(codex.ID) != other {
		t.Fatalf("the working configuration follows %+v and selects %s", followed, before.SelectedModel(codex.ID))
	}

	projection, err := syncer.Preview(before, before.Clone())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(projection.Recorded, []string{codex.ID}) || len(projection.Clients) != 0 || len(projection.Targets) != 1 || !projection.Targets[0].RecordOnly() || !projection.Configuration {
		t.Fatalf("the preview plans %+v; want Codex's record alone and the configuration", projection)
	}
	if kept, settled := syncer.Follows.Recorded(); !settled || len(kept) != 1 || kept[0].Now.Model != other {
		t.Fatalf("the preview records %+v", kept)
	}
	if unsaved := storedConfiguration(t, syncer); unsaved.SelectedModel(codex.ID) == other || !leftAs(t, target, info, chosen) {
		t.Fatal("the preview saved the choice or wrote Codex's file")
	}

	if err := syncer.Commit(context.Background(), before, before.Clone(), "an unrelated command"); err != nil {
		t.Fatal(err)
	}
	stored := storedConfiguration(t, syncer)
	if stored.SelectedModel(codex.ID) != other {
		t.Fatalf("the choice was not saved: %s", stored.SelectedModel(codex.ID))
	}
	if !leftAs(t, target, info, chosen) {
		t.Fatal("the transition wrote the file of a client whose provider has no Token")
	}
	if _, err := os.Stat(reader); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a credential reader was prepared for a record: %v", err)
	}
	if _, again := syncer.Working(stored); len(again) != 0 {
		t.Fatalf("the record stayed behind: the saved choice is followed again: %+v", again)
	}
	// The earlier model, chosen in Codex again, is a choice like any other.
	if err := os.WriteFile(target, bytes.Replace(chosen, []byte(`model = "gpt-other"`), []byte(`model = "gpt-test"`), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if back, returned := syncer.Working(stored); len(returned) != 1 || back.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-test") {
		t.Fatalf("the earlier model, chosen again, is followed as %+v", returned)
	}
}

// With a Token that AIGW can use, the same transition writes the client's
// file with AIGW's record, as before. Holds the cell chosen/other.
func TestATransitionWritesAChoiceWithItsToken(t *testing.T) {
	syncer, cfg, target, _ := codexWithReader(t)
	chosenInCodex(t, target)
	before, _ := syncer.Working(cfg)
	projection, err := syncer.Preview(before, before.Clone())
	if err != nil || !slices.Equal(projection.Clients, []string{codex.ID}) || len(projection.Recorded) != 0 {
		t.Fatalf("the preview plans %+v, %v; want Codex's files", projection, err)
	}
	if err := syncer.Commit(context.Background(), before, before.Clone(), "an unrelated command"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || !strings.Contains(string(data), `model = "gpt-other" # managed by AIGW`) {
		t.Fatalf("the file does not carry AIGW's projection of the choice: %v\n%s", err, data)
	}
	if _, again := syncer.Working(storedConfiguration(t, syncer)); len(again) != 0 {
		t.Fatalf("the saved choice is followed again: %+v", again)
	}
}

// failingClient is a client whose application fails.
type failingClient struct {
	adapter.Adapter
	err error
}

func (client failingClient) Apply(context.Context, adapter.Dependencies, domain.Config, domain.Config) (adapter.ProjectionReceipt, error) {
	return nil, client.err
}

// A transition that fails after AIGW's record was written puts the record
// back with the stored configuration: the choice is one to follow again, and
// the client's file was never written.
func TestAFailureAfterTheRecordRollsItBack(t *testing.T) {
	syncer, cfg, target, tokens := codexWithReader(t)
	if err := tokens.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	// Codex is applied first, then the client that fails.
	failure := errors.New("the next client's file cannot be written")
	registry, err := adapter.NewRegistry(codex.New(), failingClient{claude.New(), failure})
	if err != nil {
		t.Fatal(err)
	}
	syncer.Registry = registry
	info, chosen := chosenInCodex(t, target)
	record, err := os.ReadFile(target + adapter.StateSuffix)
	if err != nil {
		t.Fatal(err)
	}
	before, followed := syncer.Working(cfg)
	if len(followed) != 1 {
		t.Fatalf("the working configuration follows %+v", followed)
	}
	if err := syncer.CommitProjection(context.Background(), before, before.Clone(), "sync"); !errors.Is(err, failure) {
		t.Fatalf("commit = %v, want the next client's failure", err)
	}
	if after, err := os.ReadFile(target + adapter.StateSuffix); err != nil || !bytes.Equal(after, record) {
		t.Fatalf("the failed transition left its record: %v", err)
	}
	stored := storedConfiguration(t, syncer)
	if stored.SelectedModel(codex.ID) != cfg.SelectedModel(codex.ID) || !leftAs(t, target, info, chosen) {
		t.Fatalf("the failed transition saved %s or wrote Codex's file", stored.SelectedModel(codex.ID))
	}
	if _, again := syncer.Working(stored); len(again) != 1 {
		t.Fatalf("after the failure the choice is followed as %+v", again)
	}
}

// What a transition does with a choice, beside saving it: a client that it
// withdraws is reconciled, whose adapter removes AIGW's part of its file; the
// model that was stored before is free to go, because nothing names it once
// the choice is recorded; and a selection that names no protocol is the
// choice when it resolves to the chosen model's. Holds the cells
// chosen/withdraws and chosen/removes, and the comparison of selections that
// chosen/restores rests on.
func TestATransitionSettlesAChoiceByWhatItSaves(t *testing.T) {
	syncer, cfg, target, tokens := codexWithReader(t)
	if err := tokens.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	chosenInCodex(t, target)
	working, followed := syncer.Working(cfg)

	withdrawn := working.Clone()
	withdrawn.SetClientActivation(codex.ID, false, "", nil)
	if _, named, reconcile, err := syncer.recorded(working, withdrawn, cfg, followed, false, nil); err != nil || !reconcile || !slices.Equal(named, []string{codex.ID}) {
		t.Fatalf("a withdrawal reconciles %v, %t: %v", named, reconcile, err)
	}
	if kept := syncer.keeps(followed, working, withdrawn); len(kept) != 0 {
		t.Fatalf("a withdrawal keeps %+v as a selection", kept)
	}

	removed := working.Clone()
	delete(removed.Models, cfg.SelectedModel(codex.ID))
	saved, named, reconcile, err := syncer.recorded(working, removed, cfg, followed, false, nil)
	if err != nil || !reconcile || !slices.Equal(named, []string{codex.ID}) || saved.SelectedModel(codex.ID) != working.SelectedModel(codex.ID) {
		t.Fatalf("removing the earlier model saves %s and reconciles %v: %v", saved.SelectedModel(codex.ID), named, err)
	}
	if kept := syncer.keeps(followed, working, removed); len(kept) != 1 || kept[0].Returns != "" {
		t.Fatalf("removing the earlier model keeps %+v; want the choice, with no step back to a model that is gone", kept)
	}

	unpinned := working.Clone()
	binding := unpinned.Clients[codex.ID]
	binding.Protocol = ""
	unpinned.Clients[codex.ID] = binding
	if working.Clients[codex.ID].Protocol == "" {
		t.Fatal("the followed selection names no protocol, so the comparison compares nothing")
	}
	if !SameSelection(working, unpinned, codex.ID) || len(syncer.keeps(followed, working, unpinned)) != 1 {
		t.Fatal("a selection that names no protocol is not the chosen model's, though it resolves to it")
	}
	if SameSelection(working, cfg, codex.ID) || SameSelection(working, withdrawnSelection(working), codex.ID) {
		t.Fatal("another model, or no model, compares as the chosen one")
	}
}

// withdrawnSelection is cfg in which Codex selects nothing.
func withdrawnSelection(cfg domain.Config) domain.Config {
	cleared := cfg.Clone()
	delete(cleared.Clients, codex.ID)
	return cleared
}
