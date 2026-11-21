package adapter

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// selectionRecordingAdapter records a selection: it notes each call and the
// configuration that it was given, and its receipt notes a rollback.
type selectionRecordingAdapter struct {
	failingProjectionAdapter
	followed *[]domain.Config
}

func (adapter selectionRecordingAdapter) RecordPending(Dependencies, domain.Config) bool {
	return true
}

func (adapter selectionRecordingAdapter) PlanRecord(_ Dependencies, followed domain.Config) ([]ProjectionPlan, error) {
	*adapter.events = append(*adapter.events, "plan-record:"+adapter.id)
	*adapter.followed = append(*adapter.followed, followed)
	return []ProjectionPlan{{Client: adapter.id, Target: "file", Action: ActionRecord, Effects: []string{"file.record"}, ChangesState: true}}, nil
}

func (adapter selectionRecordingAdapter) ApplyRecord(_ context.Context, _ Dependencies, followed domain.Config) (ProjectionReceipt, error) {
	*adapter.events = append(*adapter.events, "record:"+adapter.id)
	*adapter.followed = append(*adapter.followed, followed)
	return rollbackFunc(func() error {
		*adapter.events = append(*adapter.events, "rollback-record:"+adapter.id)
		return nil
	}), nil
}

// A client that the dependencies mark is planned and applied through its
// record of the selection alone, from the configuration that the transition
// starts from, and its projection is neither planned nor applied; another
// client of the same transition is projected as before, and a failure of that
// one rolls the record back.
func TestTheRegistryWritesOnlyTheRecordOfAMarkedClient(t *testing.T) {
	var events []string
	var followed []domain.Config
	recording := selectionRecordingAdapter{failingProjectionAdapter{id: "first", events: &events}, &followed}
	projected := failingProjectionAdapter{id: "second", events: &events}
	registry, err := NewRegistry(recording, projected)
	if err != nil {
		t.Fatal(err)
	}
	before := domain.NewConfig(registry.Admission())
	before.Providers["starts"] = domain.Provider{Label: "The transition starts here"}
	after := domain.NewConfig(registry.Admission())
	deps := Dependencies{RecordOnly: []string{"first"}}

	plans, err := registry.Plan(deps, before, after)
	if err != nil || len(plans) != 1 || plans[0].Action != ActionRecord || !reflect.DeepEqual(plans[0].Effects, []string{"file.record"}) {
		t.Fatalf("plans = %+v, %v; want the record's plan alone", plans, err)
	}
	if want := []string{"plan-record:first", "plan:second"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("planning called %v, want %v", events, want)
	}
	events = nil
	if _, changed, err := registry.Apply(t.Context(), deps, before, after); err != nil || !changed {
		t.Fatalf("apply = %t, %v; want a record that changed", changed, err)
	}
	if want := []string{"plan-record:first", "plan:second", "record:first", "apply:second"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("applying called %v, want %v", events, want)
	}
	for _, given := range followed {
		if _, starts := given.Providers["starts"]; !starts {
			t.Fatalf("the record was given %+v, not the configuration that the transition starts from", given.Providers)
		}
	}

	events = nil
	failure := errors.New("the second client's file cannot be written")
	failing := failingProjectionAdapter{id: "second", events: &events, applyErr: failure}
	registry, err = NewRegistry(recording, failing)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.Apply(t.Context(), deps, before, after); !errors.Is(err, failure) {
		t.Fatalf("apply = %v, want the second client's failure", err)
	}
	if last := events[len(events)-1]; last != "rollback-record:first" {
		t.Fatalf("the failure left the record written: %v", events)
	}
}

// A client that records no selection cannot be marked: the registry refuses
// instead of projecting it after all.
func TestAMarkedClientThatRecordsNoSelectionIsRefused(t *testing.T) {
	var events []string
	registry, err := NewRegistry(failingProjectionAdapter{id: "plain", events: &events})
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(registry.Admission())
	deps := Dependencies{RecordOnly: []string{"plain"}}
	if _, err := registry.Plan(deps, cfg, cfg); err == nil || !strings.Contains(err.Error(), "keeps no record of a selection") {
		t.Fatalf("plan = %v, want the refusal", err)
	}
	if _, _, err := registry.Apply(t.Context(), deps, cfg, cfg); err == nil || len(events) != 0 {
		t.Fatalf("apply = %v after %v, want the refusal before any call", err, events)
	}
}
