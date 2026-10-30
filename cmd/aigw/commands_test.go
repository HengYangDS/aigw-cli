package main

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestEveryCommandHasItsPackageAndScript holds the CLI to its command tree:
// each top-level command runs only handlers from the internal/cli package
// named after it, and the acceptance script named after it runs it.
func TestEveryCommandHasItsPackageAndScript(t *testing.T) {
	root := cli.NewRoot(&cli.App{Clients: clients.Registry(), Out: io.Discard, Err: io.Discard})
	if len(root.Commands()) == 0 {
		t.Fatal("the root registers no command")
	}
	for _, command := range root.Commands() {
		name := command.Name()
		owner := "aigw-cli/internal/cli/" + name
		handlers := commandHandlers(command)
		if len(handlers) == 0 {
			t.Errorf("command %q has no handler", name)
		}
		for _, handler := range handlers {
			if got := handlerPackage(handler); got != owner {
				t.Errorf("command %q runs a handler from %s instead of its package %s", name, got, owner)
			}
		}
		script, err := os.ReadFile(filepath.Join("testdata", "script", name+".txtar"))
		if err != nil {
			t.Errorf("command %q has no acceptance script: %v", name, err)
			continue
		}
		runs := regexp.MustCompile(`(?m)\bexec aigw ` + regexp.QuoteMeta(name) + `(?:[ \t]|$)`)
		if !runs.Match(script) {
			t.Errorf("the acceptance script of command %q does not run it", name)
		}
	}
}

// commandHandlers returns the run functions of command and its subcommands.
func commandHandlers(command *cobra.Command) []any {
	var handlers []any
	if command.RunE != nil {
		handlers = append(handlers, command.RunE)
	}
	if command.Run != nil {
		handlers = append(handlers, command.Run)
	}
	for _, child := range command.Commands() {
		handlers = append(handlers, commandHandlers(child)...)
	}
	return handlers
}

// handlerPackage returns the import path of the package that defines handler.
func handlerPackage(handler any) string {
	name := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	slash := strings.LastIndex(name, "/") + 1
	if dot := strings.Index(name[slash:], "."); dot >= 0 {
		return name[:slash+dot]
	}
	return name
}
