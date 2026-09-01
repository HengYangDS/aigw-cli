//go:build linux

package secrets

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveKeyringItemReportsSecretServiceConnectionFailure(t *testing.T) {
	t.Setenv(
		"DBUS_SESSION_BUS_ADDRESS",
		"unix:path="+filepath.Join(t.TempDir(), "missing-session-bus.sock"),
	)

	_, err := observeKeyringItem("aigw-test", "missing")
	if err == nil || !strings.Contains(err.Error(), "connect to Secret Service") {
		t.Fatalf("observeKeyringItem() error = %v", err)
	}
}

func TestClassifySecretServiceObservation(t *testing.T) {
	want := errors.New("search failed")
	for _, test := range []struct {
		name    string
		matches int
		err     error
		present bool
		wantErr error
	}{
		{name: "absent"},
		{name: "present", matches: 1, present: true},
		{name: "failure", err: want, wantErr: want},
	} {
		t.Run(test.name, func(t *testing.T) {
			present, err := classifySecretServiceObservation(test.matches, test.err)
			if present != test.present || !errors.Is(err, test.wantErr) {
				t.Fatalf("classifySecretServiceObservation() = %v, %v", present, err)
			}
		})
	}
}

func TestSecretServiceCloseErrorPreservesPrimaryFailure(t *testing.T) {
	primary := errors.New("search failed")
	closeFailure := errors.New("close failed")
	if err := secretServiceCloseError(primary, closeFailure); !errors.Is(err, primary) {
		t.Fatalf("primary error = %v", err)
	}
	if err := secretServiceCloseError(nil, closeFailure); !errors.Is(err, closeFailure) {
		t.Fatalf("close error = %v", err)
	}
	if err := secretServiceCloseError(nil, nil); err != nil {
		t.Fatalf("successful close error = %v", err)
	}
}
