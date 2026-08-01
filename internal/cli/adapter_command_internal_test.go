package cli

import (
	"aigw-cli/internal/cli/adapter"
	configuration "aigw-cli/internal/configuration"
	"bytes"
	"strings"
	"testing"
)

func TestCommandBoundaryMissingExecutables(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	app := configuredCommandApp(t, configuredCommandState())
	cmd := adapter.NewCommand(app.invocationContext())
	cmd.SetArgs([]string{"discover"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := app.Out.(*bytes.Buffer)
	if strings.Count(output.String(), "Not found") != len(configuration.AdmittedClientIDs()) {
		t.Fatalf("output = %q", output.String())
	}
}
