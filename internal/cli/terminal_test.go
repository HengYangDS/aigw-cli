package cli

import (
	"os"
	"testing"
)

func TestDevNullIsNotInteractiveTerminal(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if isTerminal(file) {
		t.Fatal("os.DevNull must not trigger the interactive wizard")
	}
}
