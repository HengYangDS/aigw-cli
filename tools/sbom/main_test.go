package main

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadModulesRetainsGoListDiagnostics(t *testing.T) {
	original := runGoList
	t.Cleanup(func() { runGoList = original })
	runGoList = func() ([]byte, error) {
		return []byte("go: example.invalid/module: unavailable\n"), errors.New("exit status 1")
	}

	_, err := loadModules()
	if err == nil {
		t.Fatal("loadModules() error = nil, want diagnostic failure")
	}
	for _, want := range []string{"run go list -m -json all", "exit status 1", "example.invalid/module"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("loadModules() error = %q, want %q", err, want)
		}
	}
}

func TestLoadModulesDecodesAllModules(t *testing.T) {
	original := runGoList
	t.Cleanup(func() { runGoList = original })
	runGoList = func() ([]byte, error) {
		return []byte("{\"Path\":\"example.com/aigw\"}\n{\"Path\":\"example.com/dependency\",\"Version\":\"v1.2.3\"}\n"), nil
	}

	modules, err := loadModules()
	if err != nil {
		t.Fatalf("loadModules() error = %v", err)
	}
	if len(modules) != 2 {
		t.Fatalf("loadModules() returned %d modules, want 2", len(modules))
	}
	if modules[1].Path != "example.com/dependency" || modules[1].Version != "v1.2.3" {
		t.Errorf("dependency = %#v, want path and version", modules[1])
	}
}
