//go:build darwin

package secrets

import (
	"errors"
	"os/exec"
	"testing"
)

func TestClassifyKeychainObservation(t *testing.T) {
	absent := exec.Command("/usr/bin/security", "find-generic-password", "-s", "aigw-test-absent", "-a", "aigw-test-absent")
	absentOutput, absentErr := absent.CombinedOutput()
	if absentErr == nil {
		t.Fatal("absent Keychain fixture unexpectedly exists")
	}
	tests := []struct {
		name    string
		output  []byte
		err     error
		present bool
		wantErr bool
	}{
		{name: "present", present: true},
		{name: "absent", output: absentOutput, err: absentErr},
		{name: "failure", err: errors.New("security failed"), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			present, err := classifyKeychainObservation(test.output, test.err)
			if present != test.present || (err != nil) != test.wantErr {
				t.Fatalf("classifyKeychainObservation() = %v, %v", present, err)
			}
		})
	}
}
