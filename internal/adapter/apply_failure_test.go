package adapter

import (
	"context"
	"errors"
	"os"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// restoringReceipt is a receipt that reports restoring its files.
type restoringReceipt struct{ restored platform.Restoration }

func (receipt restoringReceipt) Rollback() error                           { return receipt.restored.Err() }
func (receipt restoringReceipt) Restore() platform.Restoration             { return receipt.restored }
func (adapter restoringAdapter) Spec() domain.ClientSpec                   { return adapter.spec }
func (adapter restoringAdapter) ProjectionChanged(_, _ domain.Config) bool { return true }

// restoringAdapter is a client whose application writes files and reports
// restoring them, or fails with err.
type restoringAdapter struct {
	failingProjectionAdapter
	spec     domain.ClientSpec
	restored platform.Restoration
	err      error
}

func (adapter restoringAdapter) Apply(context.Context, Dependencies, domain.Config, domain.Config) (ProjectionReceipt, error) {
	if adapter.err != nil {
		return nil, adapter.err
	}
	return restoringReceipt{restored: adapter.restored}, nil
}

// An application that fails at a file reports the client, the file, and what
// undoing it did to each file, its own first and then the earlier clients'.
// A failure that names no file stays as it is.
func TestAnApplicationThatFailsAtAFileReportsEachFile(t *testing.T) {
	failed := &platform.FileError{Path: "/home/b/config.toml", Err: os.ErrPermission}
	own := platform.FailedWrite(failed, platform.Restoration{{Path: "/home/b/state"}})
	events := []string{}
	first := restoringAdapter{id: "first", events: &events, spec: domain.ClientSpec{ID: "first", Label: "First"}, restored: platform.Restoration{{Path: "/home/a/config.toml"}}}
	second := restoringAdapter{id: "second", events: &events, spec: domain.ClientSpec{ID: "second", Label: "Second"}, err: own}
	registry, err := NewRegistry(first, second)
	if err != nil {
		t.Fatal(err)
	}
	deps := Dependencies{}
	deps.Host.Home = "/home"
	_, _, err = registry.Apply(t.Context(), deps, domain.NewConfig(registry.Admission()), domain.NewConfig(registry.Admission()))
	failure, ok := errors.AsType[*ApplyError](err)
	if !ok || failure.Client != "Second" || failure.Home != "/home" || failure.Failed != failed || len(failure.Restored) != 2 ||
		failure.Restored[0].Path != "/home/b/state" || failure.Restored[1].Path != "/home/a/config.toml" || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Apply() = %#v, %v", failure, err)
	}

	second.err = errors.New("planning failed")
	registry, err = NewRegistry(first, second)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = registry.Apply(t.Context(), deps, domain.NewConfig(registry.Admission()), domain.NewConfig(registry.Admission()))
	if _, atFile := errors.AsType[*ApplyError](err); atFile || err == nil {
		t.Fatalf("a failure without a file = %v", err)
	}
}
