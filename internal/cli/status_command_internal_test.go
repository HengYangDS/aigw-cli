package cli

import (
	"aigw-cli/internal/account"
	"aigw-cli/internal/cli/readiness"
	configuration "aigw-cli/internal/configuration"
	"bytes"
	"strings"
	"testing"
)

func TestStatusHumanTransportAndProbeBranches(t *testing.T) {
	tests := []struct {
		name       string
		probe      *configuration.AccountProbe
		credential bool
		want       string
	}{
		{name: "none", want: "Provider does not expose a probe"},
		{name: "unsupported", probe: &configuration.AccountProbe{Kind: "future", BaseURL: "https://probe.test"}, want: "does not provide diagnostics"},
		{name: "supported missing", probe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.test"}, want: "Disabled"},
		{name: "supported present", probe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.test"}, credential: true, want: "Enabled"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := configuredCommandState()
			providerAccount := cfg.Accounts["one"]
			providerAccount.AccountProbe = test.probe
			cfg.Accounts["one"] = providerAccount
			app := configuredCommandApp(t, cfg)
			_ = app.Secrets.Set("one", "token")
			if test.credential {
				_ = app.Accounts.Set("one", account.Credential{SystemToken: "system", UserID: "user"})
			}
			if err := readiness.RunStatus(app.invocationContext(), false); err != nil {
				t.Fatal(err)
			}
			text := app.Out.(*bytes.Buffer).String()
			if !strings.Contains(text, test.want) || !strings.Contains(text, "External loopback compatibility layer") {
				t.Fatalf("output = %q", text)
			}
		})
	}
}
