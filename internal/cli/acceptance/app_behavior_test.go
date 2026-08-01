package cli_test

import (
	"strings"
	"testing"
)

func TestFailureSuggestionUsesCommandNamedInEnglishGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := app.Config.Save(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "setup", "--profile", "new-profile")
	if err == nil || !strings.Contains(out.String(), "AIGW is already configured") || !strings.Contains(out.String(), "aigw add") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}
