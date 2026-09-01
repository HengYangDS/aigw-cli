//go:build linux

package secrets

import (
	"errors"
	"testing"
)

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
