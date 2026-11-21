package hermes

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v4"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// fileAt is the file at path as the file system identifies it and the bytes
// that it holds, so that a file that was written again is told from one that
// was left, whatever was written and whenever.
type fileAt struct {
	info os.FileInfo
	data []byte
}

func readFileAt(t *testing.T, path string) fileAt {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileAt{info: info, data: data}
}

// left reports whether the file at path is still the file that was read, with
// the bytes that it held.
func (file fileAt) left(t *testing.T, path string) bool {
	t.Helper()
	now := readFileAt(t, path)
	return os.SameFile(file.info, now.info) && bytes.Equal(file.data, now.data)
}

// The record of a choice moves alone: Hermes' file is the file that it was,
// the record holds what the file shows, so that nothing is left to follow,
// and the receipt puts the earlier record back.
func TestARecordOfAChoiceMovesTheRecordAndLeavesHermesFile(t *testing.T) {
	chosen := Selection{Provider: "other", Model: "other-model"}
	path := projectedSelection(t, twoProviders(), chosen.Provider, chosen.Model)
	record := path + adapter.StateSuffix
	config, recorded := readFileAt(t, path), readFileAt(t, record)

	plan, err := PrepareRecord(path, chosen)
	if err != nil || !plan.ChangesState() || !slices.Equal(plan.Effects(), []string{record}) {
		t.Fatalf("the plan changes state %t with effects %v: %v", plan.ChangesState(), plan.Effects(), err)
	}
	if !config.left(t, path) || !recorded.left(t, record) {
		t.Fatal("preparing the record wrote a file")
	}
	receipt, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !config.left(t, path) {
		t.Fatal("recording the choice wrote Hermes' file")
	}
	if recorded.left(t, record) {
		t.Fatal("the record did not move")
	}
	if _, changed, err := ObserveSelection(path); err != nil || changed {
		t.Fatalf("the file's selection is still a choice to follow: %t, %v", changed, err)
	}
	if again, err := PrepareRecord(path, chosen); err != nil || again.ChangesState() {
		t.Fatalf("a record that holds what the file shows is planned again: %v", err)
	}
	// The recorded file is one that a projection of the choice plans over.
	desired := twoProviders()
	desired.SelectedProvider, desired.SelectedModel = chosen.Provider, chosen.Model
	if _, err := PrepareTransition(path, &desired, nil, false); err != nil {
		t.Fatalf("a projection after the record: %v", err)
	}

	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFileAt(t, record).data, recorded.data) || !config.left(t, path) {
		t.Fatal("the receipt did not restore the record alone")
	}
	if selection, changed, err := ObserveSelection(path); err != nil || !changed || selection != chosen {
		t.Fatalf("after the rollback the choice is %+v, %t, %v", selection, changed, err)
	}
}

// Nothing is recorded, and nothing written, for a file that holds more than
// a choice of the model that the configuration follows: another setting that
// AIGW owns changed beside it, the file selects a model that its entry does
// not list, or it selects another listed model.
func TestARecordIsRefusedForAnythingButTheFollowedChoice(t *testing.T) {
	followed := Selection{Provider: "other", Model: "other-model"}
	for name, test := range map[string]struct {
		selected Selection
		edit     func(root *yaml.Node)
		shown    Selection
	}{
		"an edited provider entry beside the choice": {followed, func(root *yaml.Node) {
			setString(field(field(root, "providers"), "aigw"), "base_url", "https://edited.test/v1")
		}, followed},
		"a model that the entry does not list": {Selection{Provider: "other", Model: "unlisted"}, nil, followed},
		"another listed model":                 {followed, nil, Selection{Provider: "aigw", Model: "model-test"}},
	} {
		t.Run(name, func(t *testing.T) {
			path := projectedSelection(t, twoProviders(), test.selected.Provider, test.selected.Model)
			if test.edit != nil {
				editRoot(t, path, test.edit)
			}
			record := path + adapter.StateSuffix
			config, recorded := readFileAt(t, path), readFileAt(t, record)
			plan, err := PrepareRecord(path, test.shown)
			if err == nil || plan.ChangesState() {
				t.Fatalf("the record was planned: %v", err)
			}
			if _, applyErr := plan.Apply(); applyErr != nil {
				t.Fatalf("a refused plan applies as %v", applyErr)
			}
			if !config.left(t, path) || !recorded.left(t, record) {
				t.Fatal("the refusal wrote a file")
			}
		})
	}
}

// editRoot rewrites the file at path through edit.
func editRoot(t *testing.T, path string, edit func(root *yaml.Node)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	edit(root)
	edited, err := encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A record that changed after the plan read it is not written over, and
// Hermes' file is left.
func TestARecordIsNotWrittenOverOneThatChangedSinceItWasRead(t *testing.T) {
	chosen := Selection{Provider: "other", Model: "other-model"}
	path := projectedSelection(t, twoProviders(), chosen.Provider, chosen.Model)
	record := path + adapter.StateSuffix
	plan, err := PrepareRecord(path, chosen)
	if err != nil {
		t.Fatal(err)
	}
	newer := append(readFileAt(t, record).data, '\n')
	if err := os.WriteFile(record, newer, 0o600); err != nil {
		t.Fatal(err)
	}
	config := readFileAt(t, path)
	if _, err := plan.Apply(); err == nil {
		t.Fatal("the record was written over a newer one")
	}
	if !bytes.Equal(readFileAt(t, record).data, newer) || !config.left(t, path) {
		t.Fatal("the refused write changed a file")
	}
}

// recordedChoiceFixture projects model-test for Hermes, chooses model-other
// in Hermes's file, and returns that file, the configuration that follows
// the choice, and the stored configuration.
func recordedChoiceFixture(t *testing.T) (string, domain.Config, domain.Config) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	chosen := domain.NewModelRef("team", "model-other")
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://provider.test"}}
	cfg.Models[domain.NewModelRef("team", "model-test")] = qualifiedModel("Test", "team", "model-test", domain.ProtocolAnthropic)
	cfg.Models[chosen] = qualifiedModel("Other", "team", "model-other", domain.ProtocolAnthropic)
	cfg.Clients[ID] = domain.ClientBinding{Model: domain.NewModelRef("team", "model-test"), Enabled: true, Protocol: domain.ProtocolAnthropic, Executable: "/opt/hermes", Targets: []string{path}}
	tokens := secret.NewMemoryStore()
	if err := tokens.Set("team", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	projecting := adapter.Dependencies{Secrets: tokens, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}
	if _, _, err := hermesRegistry().Apply(t.Context(), projecting, domain.NewConfig(testAdmission()), cfg, ID); err != nil {
		t.Fatal(err)
	}
	editModel(t, path, func(model *yaml.Node) *yaml.Node {
		setString(model, "default", chosen.ID())
		return model
	})
	followed := cfg.Clone()
	if followedRef, err := (hermesAdapter{}).AdoptSelection(adapter.Dependencies{}, &followed); err != nil || followedRef != chosen {
		t.Fatalf("the choice is followed as %s: %v", followedRef, err)
	}
	return path, cfg, followed
}

// Through the registry, a Hermes that the dependencies mark is recorded
// without a Token and without a credential reader: the plan names the record
// alone, the file is left, and the next read follows nothing.
func TestTheAdapterRecordsAChoiceWithoutAToken(t *testing.T) {
	path, cfg, followed := recordedChoiceFixture(t)
	registry := hermesRegistry()
	record := path + adapter.StateSuffix
	config, recorded := readFileAt(t, path), readFileAt(t, record)
	// No Token and no reader: the record needs neither.
	recording := adapter.Dependencies{RecordOnly: []string{ID}}
	plans, err := registry.Plan(recording, followed, followed, ID)
	if err != nil || len(plans) != 1 {
		t.Fatalf("plans = %+v, %v", plans, err)
	}
	if plan := plans[0]; plan.Action != adapter.ActionRecord || plan.Target != path || !slices.Equal(plan.Effects, []string{record}) || !plan.ChangesState {
		t.Fatalf("the record's plan is %+v", plan)
	}
	if !config.left(t, path) || !recorded.left(t, record) {
		t.Fatal("planning the record wrote a file")
	}
	receipt, changed, err := registry.Apply(t.Context(), recording, followed, followed, ID)
	if err != nil || !changed {
		t.Fatalf("apply = %t, %v", changed, err)
	}
	if !config.left(t, path) || recorded.left(t, record) {
		t.Fatal("recording the choice wrote Hermes' file, or left the record")
	}
	again := followed.Clone()
	if chosen, err := (hermesAdapter{}).AdoptSelection(adapter.Dependencies{}, &again); err != nil || !chosen.IsZero() {
		t.Fatalf("the recorded choice is followed again as %s: %v", chosen, err)
	}
	if plans, err := registry.Plan(recording, followed, followed, ID); err != nil || len(plans) != 0 {
		t.Fatalf("a record that holds the choice is planned again: %+v, %v", plans, err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFileAt(t, record).data, recorded.data) || !config.left(t, path) {
		t.Fatal("the receipt did not restore the record alone")
	}
	// A record is of the model that the configuration follows; the stored
	// configuration, which names the earlier model, has none to write.
	if _, err := registry.Plan(recording, cfg, cfg, ID); err == nil {
		t.Fatal("a configuration that does not follow the file's model recorded it")
	}
	var conflict *adapter.ConflictError
	if _, err := PrepareRecord(path, Selection{Provider: "aigw-team-anthropic", Model: "model-test"}); !errors.As(err, &conflict) {
		t.Fatalf("the refusal is %v, want a conflict with its recovery", err)
	}
}
