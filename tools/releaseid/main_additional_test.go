package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunReleaseID(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-namespace", "6ba7b814-9dad-11d1-80b4-00c04fd430c8", "-name", "aigw/release"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "a4cb5eb8-eb05-5fd6-8a6e-9725fc567ca2" {
		t.Fatalf("run() UUID = %q", got)
	}
}

func TestRunReleaseIDRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "usage", want: "usage: releaseid"},
		{name: "unknown flag", args: []string{"-unknown"}, want: "flag provided but not defined"},
		{name: "invalid namespace", args: []string{"-namespace", "not-a-uuid", "-name", "release"}, want: "invalid UUID namespace"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := run(test.args, &stdout, &stderr); code != 2 {
				t.Fatalf("run() code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("run() stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestParseUUIDRejectsNonHexadecimalInput(t *testing.T) {
	if _, err := parseUUID("zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"); err == nil {
		t.Fatal("parseUUID() error = nil")
	}
}
