package doctor

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/presentation"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestHumanProjectionFailsClosedForFutureChecks(t *testing.T) {
	for _, check := range []Check{
		{Name: "future:internal", OK: true, Detail: "internal success detail"},
		{Name: "future:internal", Detail: "internal failure detail", Fix: "internal repair instruction"},
	} {
		if got := Label(clients.Admission(), check); got != "Other check" {
			t.Fatalf("label = %q, want Other check", got)
		}
		want := "Check failed"
		if check.OK {
			want = "Healthy"
		}
		if got := Detail(clients.Admission(), check); got != want {
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
		"secret:team":              "Account Token",
		"adapter:claude":           "Claude adapter",
		"adapter:claude-desktop":   "Claude Desktop adapter",
		"adapter:codex":            "Codex adapter",
		"adapter:hermes":           "Hermes adapter",
		"projection:codex":         "Codex route",
		"projection:hermes":        "Hermes route",
		"codex:target-7":           "Codex configuration target 7",
	}
	for name, want := range labels {
		if got := Label(clients.Admission(), Check{Name: name}); got != want {
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
		{Check{Name: "secret:team", Detail: "credential backend is unavailable"}, "team · credential backend unavailable"},
		{Check{Name: "adapter:claude", OK: true, Detail: "enabled"}, "Enabled"},
		{Check{Name: "adapter:claude", Detail: "Claude executable is not configured"}, "Enabled, but no executable is configured"},
		{Check{Name: "adapter:codex", Detail: "Codex executable is not configured"}, "Enabled, but no executable is configured"},
		{Check{Name: "adapter:codex", Detail: "Codex configuration target is missing"}, "Enabled, but no Codex configuration file is configured"},
		{Check{Name: "projection:codex", Detail: "unavailable"}, "Current Codex binding cannot be resolved"},
		{Check{Name: "codex:target-1", OK: true}, "Matches the current binding"},
		{Check{Name: "codex:target-1"}, "Does not match the current binding"},
	}
	for _, test := range details {
		if got := Detail(clients.Admission(), test.check); got != test.want {
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

func TestCodexTargetLabelNamesOnlyFilesUnderHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	inside := filepath.Join(home, ".codex", "config.toml")
	outside := filepath.Join(root, "work", "codex", "config.toml")
	homeRelative := filepath.Join("~", ".codex", "config.toml")
	for _, test := range []struct {
		path, home, target string
		outside            bool
	}{
		{inside, home, homeRelative, false},
		{outside, home, "", true},
		{inside, "", "", false},
		{"relative.toml", home, "", false},
	} {
		target, outside := publicTarget(test.path, test.home)
		if target != test.target || outside != test.outside {
			t.Errorf("publicTarget(%q, %q) = %q, %v; want %q, %v", test.path, test.home, target, outside, test.target, test.outside)
		}
	}
	for _, test := range []struct {
		check Check
		want  string
	}{
		{Check{Name: "codex:target-1", Target: homeRelative}, "Codex configuration " + homeRelative},
		{Check{Name: "codex:target-2", outsideHome: true}, "Codex configuration target 2 (outside the home directory)"},
		{Check{Name: "codex:target-3"}, "Codex configuration target 3"},
	} {
		if got := Label(clients.Admission(), test.check); got != test.want {
			t.Errorf("Label(%+v) = %q, want %q", test.check, got, test.want)
		}
	}
	for env, want := range map[string]string{
		"PATH=/bin\x00USERPROFILE=" + home:   home,
		"HOME=" + home + "\x00USERPROFILE=x": home,
		"HOME=\x00PATH=/bin":                 "",
	} {
		if got := homeDirectory(strings.Split(env, "\x00")); got != want {
			t.Errorf("homeDirectory(%q) = %q, want %q", env, got, want)
		}
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
		{domainreadiness.InferenceChecked, true},
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
			deps.Inspect = func(domain.Config, secrets.Store) map[string]domainreadiness.Client {
				return map[string]domainreadiness.Client{claude.ID: {State: test.state}}
			}
			result := executeJSON(t, deps, out)
			if result.OK != test.ok || (!test.ok && result.NextAction != "aigw doctor --json") || (test.ok && result.NextAction != "aigw use --for claude claude") {
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
	if got := Label(clients.Admission(), check); got != "Other check" {
		t.Fatalf("Label() = %q, want Other check", got)
	}
	if got := Detail(clients.Admission(), check); got != "Check failed" {
		t.Fatalf("Detail() = %q, want Check failed", got)
	}
	if got := Fix(check); got != "aigw doctor --json" {
		t.Fatalf("Fix() = %q, want aigw doctor --json", got)
	}
}

func TestHumanChecksShareOneValueColumn(t *testing.T) {
	deps, out, secretStore := doctorDependencies(t, validDoctorConfig())
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	deps.Env = []string{"OPENAI_API_KEY=secret"}
	checks := executeJSON(t, deps, out).Checks
	longest := 0
	for _, check := range checks {
		longest = max(longest, presentation.DisplayWidth("✓ "+Label(clients.Admission(), check)))
	}
	for _, width := range []int{0, 72, 100} {
		out.Reset()
		deps.Width = width
		if err := executeDoctorCommand(NewCommand(deps)); err == nil {
			t.Fatal("doctor accepted a global client token environment variable")
		}
		_, section, _ := strings.Cut(out.String(), "\nChecks\n")
		section, _, _ = strings.Cut(section, "\n\n")
		column := -1
		for line := range strings.SplitSeq(section, "\n") {
			text := strings.TrimLeft(line, " ")
			if symbol, rest, classified := strings.Cut(text, " "); classified && (symbol == "✓" || symbol == "✗") {
				for _, check := range checks {
					if label := Label(clients.Admission(), check); strings.HasPrefix(rest, label+" ") {
						text = strings.TrimLeft(strings.TrimPrefix(rest, label), " ")
						break
					}
				}
			}
			start := presentation.DisplayWidth(line) - presentation.DisplayWidth(text)
			if column < 0 {
				column = start
			}
			if start != column || column < 2+longest+2 {
				t.Fatalf("width=%d: check text starts at column %d, want one column of at least %d:\n%s", width, start, 2+longest+2, section)
			}
		}
		if !strings.Contains(section, "Fix: Remove the variables above") {
			t.Fatalf("width=%d: the failing check lost its fix:\n%s", width, section)
		}
	}
}

func TestHumanDoctorNamesNativeClaudeModelPreferenceAndVerification(t *testing.T) {
	var output bytes.Buffer
	renderer := presentation.NewWithWidth(&output, false, 100)
	renderClients(renderer, clients.Admission(), map[string]domainreadiness.Client{
		claude.ID: {
			State:               domainreadiness.Configured,
			Route:               "fable",
			NativeModelOverride: true,
			Detail:              "Claude Code uses a native model preference",
			NextAction:          "aigw verify --for claude",
		},
	})
	if err := renderer.Err(); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	if !strings.Contains(rendered, "native model preference") || !strings.Contains(rendered, "aigw verify --for claude") {
		t.Fatalf("doctor output = %q", rendered)
	}
}
