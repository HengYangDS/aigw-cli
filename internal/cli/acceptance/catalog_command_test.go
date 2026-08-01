package cli_test

import (
	"strings"
	"testing"
)

func TestCatalogUnconfiguredPointsToSetup(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "catalog")
	if err == nil {
		t.Fatal("catalog succeeded without configuration")
	}
	text := out.String() + "\n" + err.Error()
	if !strings.Contains(text, "aigw setup") || strings.Contains(text, "aigw profile add") {
		t.Fatalf("catalog should direct first use to setup:\n%s", text)
	}
}

func TestCatalogJSONUnconfiguredIsEmptyAndMachineReadable(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "catalog", "--json"); err != nil {
		t.Fatalf("catalog --json error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "{\n  \"accounts\": []\n}" {
		t.Fatalf("catalog --json = %q", out.String())
	}
}
