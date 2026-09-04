package main

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"testing"
)

type miseTask struct {
	Name string   `json:"name"`
	Run  []string `json:"run"`
}

func TestMiseTasksDelegateToCanonicalOwners(t *testing.T) {
	root := repositoryRoot(t)
	command := exec.Command("mise", "-C", root, "tasks", "ls", "--json")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list mise tasks: %v", err)
	}
	var listed []miseTask
	if err := json.Unmarshal(output, &listed); err != nil {
		t.Fatalf("decode mise tasks: %v", err)
	}
	got := make(map[string][]string, len(listed))
	for _, task := range listed {
		got[task.Name] = task.Run
	}
	want := map[string][]string{
		"bootstrap": {"npm ci --ignore-scripts"},
		"check":     {"go run ./tools/ci source"},
		"native":    {"go run ./tools/ci native"},
		"release":   {"go run ./tools/release build dist"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mise tasks = %#v, want %#v", got, want)
	}
}
