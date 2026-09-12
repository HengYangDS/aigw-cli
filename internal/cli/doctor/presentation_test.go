package doctor

import (
	configuration "aigw-cli/internal/configuration"
	domainreadiness "aigw-cli/internal/readiness"
	"strings"
	"testing"
)

func TestHumanProjectionFailsClosedForFutureChecks(t *testing.T) {
	for _, check := range []Check{
		{Name: "future:internal", OK: true, Detail: "internal success detail"},
		{Name: "future:internal", Detail: "internal failure detail", Fix: "internal repair instruction"},
	} {
		if got := Label(check.Name); got != "Other check" {
			t.Fatalf("label = %q, want Other check", got)
		}
		want := "Check failed"
		if check.OK {
			want = "Healthy"
		}
		if got := Detail(check); got != want {
			t.Fatalf("detail = %q, want %q", got, want)
		}
		if !check.OK && Fix(check) != "aigw doctor --json" {
			t.Fatalf("fix = %q", Fix(check))
		}
	}
}

func TestNextActionFallsBackForMixedOrUnclassifiedFailures(t *testing.T) {
	for _, checks := range [][]Check{
		{
			{Name: "codex:target-1", Fix: "run `aigw sync` to reconcile this target"},
			{Name: "launcher:claude", Fix: "run `aigw repair`"},
		},
		{{Name: "config", Detail: "unexpected"}},
	} {
		if got := NextAction(checks); got != "aigw repair" {
			t.Fatalf("next action = %q", got)
		}
	}
}

func TestHumanFormattingBranches(t *testing.T) {
	labels := map[string]string{
		"environment:client-token": "Client token environment",
		"config":                   "Local configuration",
		"credential:backend":       "Credential backend",
		"secret:team":              "System secret",
		"adapter:claude":           "Claude adapter",
		"adapter:codex":            "Codex adapter",
		"projection:codex":         "Codex route",
		"codex:target-7":           "Codex configuration target 7",
	}
	for name, want := range labels {
		if got := Label(name); got != want {
			t.Errorf("Label(%q) = %q, want %q", name, got, want)
		}
	}
	details := []struct {
		check Check
		want  string
	}{
		{Check{Name: "environment:client-token", OK: true}, "No global client token environment variables detected"},
		{Check{Name: "environment:client-token", Detail: "global client token environment variables are set: OPENAI_API_KEY"}, "Global client token environment variables detected: OPENAI_API_KEY"},
		{Check{Name: "environment:client-token"}, "Global client token environment variables detected"},
		{Check{Name: "config", Detail: "valid"}, "Configuration is valid"},
		{Check{Name: "config", Detail: "not configured"}, "First-time setup is incomplete"},
		{Check{Name: "config", Detail: "read config: denied"}, "Cannot read or validate configuration"},
		{Check{Name: "credential:backend"}, "Credential storage is unavailable"},
		{Check{Name: "secret:team", OK: true}, "team · available"},
		{Check{Name: "secret:team"}, "team · missing"},
		{Check{Name: "adapter:claude", OK: true, Detail: "enabled"}, "Enabled"},
		{Check{Name: "adapter:claude", Detail: "Claude executable is not configured"}, "Enabled, but no executable is configured"},
		{Check{Name: "adapter:codex", Detail: "Codex executable is not configured"}, "Enabled, but no executable is configured"},
		{Check{Name: "adapter:codex", Detail: "Codex configuration target is missing"}, "Enabled, but no Codex configuration file is configured"},
		{Check{Name: "projection:codex", Detail: "unavailable"}, "Current Codex route cannot be resolved"},
		{Check{Name: "codex:target-1", OK: true}, "Matches the current route"},
		{Check{Name: "codex:target-1"}, "Does not match the current route"},
	}
	for _, test := range details {
		if got := Detail(test.check); got != test.want {
			t.Errorf("Detail(%+v) = %q, want %q", test.check, got, test.want)
		}
	}
	if got := Fix(Check{Fix: "run `aigw rotate team`"}); got != "aigw rotate team" {
		t.Fatalf("generic run fix = %q", got)
	}
	for raw, want := range map[string]string{
		"aigw doctor":       "aigw doctor",
		"run `aigw setup`":  "aigw setup",
		"run `aigw repair`": "aigw repair",
		"remove them from the parent environment; now": "Remove the variables above from the parent environment that launched this terminal",
		"inspect or restore /private/configuration":    "Inspect or restore the local configuration file",
	} {
		if got := Fix(Check{Fix: raw}); got != want {
			t.Errorf("Fix(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := doctorTitle(""); got != "" {
		t.Fatalf("empty title = %q", got)
	}
	if got := strings.Join(ForbiddenClientTokenEnvironment([]string{"malformed", "OPENAI_API_KEY=one", "OPENAI_API_KEY=two"}), ","); got != "OPENAI_API_KEY" {
		t.Fatalf("forbidden environment names = %q", got)
	}
}

func TestResultClassifiesObservedClientStates(t *testing.T) {
	for _, test := range []struct {
		state domainreadiness.State
		ok    bool
	}{
		{domainreadiness.Configured, true},
		{domainreadiness.Deferred, true},
		{domainreadiness.EndpointChecked, true},
		{domainreadiness.Invalid, false},
		{domainreadiness.Degraded, false},
		{domainreadiness.Unavailable, false},
		{domainreadiness.State("unclassified"), false},
	} {
		t.Run(string(test.state), func(t *testing.T) {
			deps, out, secretStore := doctorDependencies(t, validDoctorConfig())
			if err := secretStore.Set("team", "token"); err != nil {
				t.Fatal(err)
			}
			deps.Inspect = func(configuration.Config) map[string]domainreadiness.Client {
				return map[string]domainreadiness.Client{configuration.ClientClaude: {State: test.state}}
			}
			result := executeJSON(t, deps, out)
			if result.OK != test.ok || (!test.ok && result.NextAction != "aigw doctor --json") || (test.ok && result.NextAction != "") {
				t.Fatalf("doctor result=%#v", result)
			}
		})
	}
}

func TestAllOKAndNextActionBranches(t *testing.T) {
	if !AllOK(nil) || AllOK([]Check{{Name: "bad"}}) {
		t.Fatal("AllOK result mismatch")
	}
	for _, test := range []struct {
		checks []Check
		want   string
	}{
		{[]Check{{Name: "config", Detail: "not configured"}}, "aigw setup"},
		{[]Check{{Name: "healthy", OK: true}, {Name: "codex:target-1", Fix: "run `aigw sync`"}}, "aigw sync"},
		{nil, "aigw repair"},
	} {
		if got := NextAction(test.checks); got != test.want {
			t.Errorf("NextAction(%#v) = %q, want %q", test.checks, got, test.want)
		}
	}
}

func TestUnknownCheckProjectionFailsClosed(t *testing.T) {
	check := Check{Name: "future:internal", Detail: "private implementation detail", Fix: "private repair"}
	if got := Label(check.Name); got != "Other check" {
		t.Fatalf("Label() = %q, want Other check", got)
	}
	if got := Detail(check); got != "Check failed" {
		t.Fatalf("Detail() = %q, want Check failed", got)
	}
	if got := Fix(check); got != "aigw doctor --json" {
		t.Fatalf("Fix() = %q, want aigw doctor --json", got)
	}
}
