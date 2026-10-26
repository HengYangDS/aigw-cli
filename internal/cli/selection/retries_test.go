package selection

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestUseRetryPolicyPreviewIsExplicitAndCredentialFree(t *testing.T) {
	runtime, _, out := configuredRuntime(t)
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	command := NewUseCommand(runtime)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", "codex", "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--request-max-retries", "0", "--stream-max-retries", "0", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result useResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.RetryPolicy == nil || result.RetryPolicy.RequestMaxRetries == nil || result.RetryPolicy.StreamMaxRetries == nil ||
		*result.RetryPolicy.RequestMaxRetries != 0 || *result.RetryPolicy.StreamMaxRetries != 0 {
		t.Fatalf("explicit preview lost zero budgets: %s", out.Bytes())
	}
	after, err := runtime.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("preview changed configuration: %v", err)
	}
}

// The human preview states each declared budget, so an operator sees what the
// selection writes before confirming it.
func TestUseRetryPolicyHumanPreviewNamesEachDeclaredBudget(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []*regexp.Regexp
		omit string
	}{
		{"both budgets", []string{"--request-max-retries", "0", "--stream-max-retries", "3"},
			[]*regexp.Regexp{regexp.MustCompile(`Request retries\s+0\b`), regexp.MustCompile(`Stream retries\s+3\b`)}, "Client native defaults"},
		{"request budget only", []string{"--request-max-retries", "2"},
			[]*regexp.Regexp{regexp.MustCompile(`Request retries\s+2\b`)}, "Stream retries"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, _, out := configuredRuntime(t)
			command := NewUseCommand(runtime)
			command.SilenceErrors, command.SilenceUsage = true, true
			command.SetArgs(append([]string{"--for", "codex", "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run"}, test.args...))
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !want.Match(out.Bytes()) {
					t.Errorf("human preview lacks %q: %s", want, out.Bytes())
				}
			}
			if strings.Contains(out.String(), test.omit) {
				t.Errorf("human preview names %q: %s", test.omit, out.Bytes())
			}
		})
	}
}

func TestUseRetryPolicyRefusesInvalidIntentBeforeEffects(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"unsupported", []string{"--for", "hermes", "codex", "--request-max-retries", "0"}, "no admitted native retry"},
		{"overflow", []string{"--for", "codex", "codex", "--request-max-retries", "9223372036854775808"}, "maximum integer"},
		{"negative", []string{"--for", "codex", "codex", "--stream-max-retries", "-1"}, "invalid argument"},
		{"ambiguous", []string{"--for", "codex", "codex", "--native-retries", "--request-max-retries", "0"}, "mutually exclusive"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, _, _ := configuredRuntime(t)
			runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("invalid policy contacted an endpoint")
				return nil, errors.New("unexpected request")
			})
			before, err := runtime.Config.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			command := NewUseCommand(runtime)
			command.SilenceErrors, command.SilenceUsage = true, true
			command.SetArgs(test.args)
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("policy refusal = %v, want %q", err, test.want)
			}
			after, err := runtime.Config.CaptureSnapshot()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal mutated state: %v", err)
			}
		})
	}
}
