package synchronization

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

// projectedCodex projects gateway/gpt-test for Codex into a file, beside a
// second model of the same provider that Codex can choose, and returns the
// synchronizer, the stored configuration and the file.
func projectedCodex(t *testing.T) (Synchronizer, domain.Config, string) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(target)
	other := cfg.Models[domain.NewModelRef("gateway", "gpt-test")]
	other.ID, other.Family, other.Label = "gpt-other", "gpt-other", "Other"
	cfg.Models[other.Ref()] = other
	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}
	if err := syncer.CommitProjection(context.Background(), cfg, cfg, "sync"); err != nil {
		t.Fatal(err)
	}
	return syncer, cfg, target
}

// storedConfiguration loads what the synchronizer's store holds.
func storedConfiguration(t *testing.T, syncer Synchronizer) domain.Config {
	t.Helper()
	configStore, ok := syncer.Config.(store.Store)
	if !ok {
		t.Fatalf("the fixture's configuration store is %T", syncer.Config)
	}
	stored, err := configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

// chooseOther rewrites Codex's root model as Codex's own picker does.
func chooseOther(t *testing.T, target string) {
	t.Helper()
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	chosen := strings.Replace(string(data), `model = "gpt-test" # managed by AIGW`, `model = "gpt-other"`, 1)
	if chosen == string(data) {
		t.Fatalf("the file does not select gpt-test as AIGW wrote it:\n%s", data)
	}
	if err := os.WriteFile(target, []byte(chosen), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A command works from the configuration that it read. A model chosen in the
// client between that read and the commit changes what the configuration
// would be, so the commit refuses as for a stored configuration that changed,
// and neither the configuration nor the client's file is written.
func TestCommitRefusesAChoiceMadeAfterTheCommandReadTheConfiguration(t *testing.T) {
	syncer, cfg, target := projectedCodex(t)
	before, followed := syncer.Working(cfg)
	if len(followed) != 0 {
		t.Fatalf("the projected file is followed: %+v", followed)
	}
	chooseOther(t, target)
	chosen, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	provider := after.Providers["gateway"]
	provider.Label = "Renamed"
	after.Providers["gateway"] = provider
	err = syncer.Commit(context.Background(), before, after, "provider edit")
	if err == nil || !strings.Contains(err.Error(), "configuration preimage changed") {
		t.Fatalf("commit = %v, want the refusal of a preimage that changed", err)
	}
	if storedConfiguration(t, syncer).Providers["gateway"].Label == "Renamed" {
		t.Fatal("the refused commit saved its change")
	}
	if current, readErr := os.ReadFile(target); readErr != nil || string(current) != string(chosen) {
		t.Fatalf("the refused commit wrote the client's file: %v", readErr)
	}
}

// The working configuration follows the file's choice, and a transition that
// names nothing of the client records it with AIGW's record of the file: a
// second read follows nothing.
func TestATransitionRecordsAChoiceWithTheFilesRecord(t *testing.T) {
	syncer, cfg, target := projectedCodex(t)
	chooseOther(t, target)
	before, followed := syncer.Working(cfg)
	if len(followed) != 1 || before.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-other") {
		t.Fatalf("the working configuration follows %+v and selects %s", followed, before.SelectedModel(codex.ID))
	}
	if err := syncer.Commit(context.Background(), before, before.Clone(), "an unrelated command"); err != nil {
		t.Fatal(err)
	}
	stored := storedConfiguration(t, syncer)
	if stored.SelectedModel(codex.ID) != domain.NewModelRef("gateway", "gpt-other") {
		t.Fatalf("the choice was not saved: %s", stored.SelectedModel(codex.ID))
	}
	if _, again := syncer.Working(stored); len(again) != 0 {
		t.Fatalf("the record stayed behind: the saved choice is followed again: %+v", again)
	}
	if data, err := os.ReadFile(target); err != nil || !strings.Contains(string(data), `model = "gpt-other" # managed by AIGW`) {
		t.Fatalf("the file does not carry AIGW's record of the choice: %v\n%s", err, data)
	}
}

// A transition keeps what a choice that waits for its Token runs on: its
// model, the protocol and the endpoint. Each loss is refused with the client
// and the model named and the step that replaces the choice, whatever command
// would cause it, and a withdrawal of the client is not refused.
func TestATransitionKeepsWhatAWaitingChoiceRunsOn(t *testing.T) {
	syncer, cfg, target := projectedCodex(t)
	chooseOther(t, target)
	waits := func(domain.Config, string) bool { return true }
	working, followed := syncer.follow(cfg, false, waits)
	if len(followed) != 1 || !followed[0].Waiting || working.Clients[codex.ID].Model != cfg.Clients[codex.ID].Model {
		t.Fatalf("a choice that waits: followed %+v, working selection %s", followed, working.Clients[codex.ID].Model)
	}
	choice, chosen := followed[0], domain.NewModelRef("gateway", "gpt-other")
	if err := servesChoice(working, choice); err != nil {
		t.Fatalf("an unchanged configuration: %v", err)
	}
	for name, lose := range map[string]func(*domain.Config){
		"the model": func(after *domain.Config) { delete(after.Models, chosen) },
		"its protocol": func(after *domain.Config) {
			model := after.Models[chosen]
			model.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolChat: {}}
			after.Models[chosen] = model
		},
	} {
		after := working.Clone()
		lose(&after)
		_, _, _, err := syncer.recorded(working, after, cfg, followed, false, nil)
		refusal, refused := errors.AsType[*PreflightError](err)
		if !refused || !strings.Contains(refusal.Error(), "Codex uses gateway/gpt-other, chosen in Codex") {
			t.Fatalf("a transition that takes %s away = %v", name, err)
		}
		conflict, carried := errors.AsType[*adapter.ConflictError](err)
		if !carried || conflict.NextAction() != "aigw use codex <provider/model>" {
			t.Fatalf("the refusal of losing %s carries %v", name, err)
		}
	}
	withdrawn := working.Clone()
	withdrawn.SetClientActivation(codex.ID, false, "", nil)
	delete(withdrawn.Models, chosen)
	if _, named, reconcile, err := syncer.recorded(working, withdrawn, cfg, followed, false, nil); err != nil || !reconcile || len(named) != 1 || named[0] != codex.ID {
		t.Fatalf("a withdrawal = %v, %t, %v; want Codex's files reconciled", named, reconcile, err)
	}
}

// A setup that stores no Token and is refused before it writes fails as that
// refusal: it has no credential to take back, and reads as its preview does.
func TestSetupWithoutATokenFailsAsItsRefusal(t *testing.T) {
	target := t.TempDir()
	cfg := testConfig(target)
	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}
	_, err := syncer.Setup(context.Background(), cfg, cfg.Clone(), nil, codex.ID)
	if _, refused := errors.AsType[*PreflightError](err); !refused || strings.Contains(err.Error(), "credentials were rolled back") {
		t.Fatalf("setup = %v, want the refusal before any write, without a credential rollback", err)
	}
}
