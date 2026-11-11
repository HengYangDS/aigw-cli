package credential

import (
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
)

// editConfiguration replaces each old text of the saved configuration with
// its new text, in order, and fails when one is missing.
func editConfiguration(t *testing.T, data []byte, replacements ...string) []byte {
	t.Helper()
	text := string(data)
	for index := 0; index+1 < len(replacements); index += 2 {
		if !strings.Contains(text, replacements[index]) {
			t.Fatalf("the configuration has no %q:\n%s", replacements[index], text)
		}
		text = strings.Replace(text, replacements[index], replacements[index+1], 1)
	}
	return []byte(text)
}

// The reader decodes only its view: a newer version, fields that it does not
// know, a field outside the view that the program would refuse, and a
// protocol that it does not know leave the Token that the fingerprint names,
// and standard output holds that Token alone.
func TestCredentialReaderIgnoresWhatItsViewLeavesOut(t *testing.T) {
	for name, replacements := range map[string][]string{
		"a newer version and unknown fields": {
			"version = 8", "version = 99\nfuture = 'unknown'",
			"label = 'Gateway'", "label = 'Gateway'\nregion = 'eu'",
			"id = 'codex-team'", "id = 'codex-team'\ncontext_window = 1000000",
			"enabled = true", "enabled = true\ntheme = 'dark'\n\n[telemetry]\nlevel = 2",
		},
		"malformed fields outside the view": {
			"label = 'Gateway'", "label = 5",
			"protocols = {responses = []}", "protocols = {responses = ['future_capability']}\ndeprecated = 'yes'\nfamily = 7",
			"protocol = 'responses'", "protocol = 'messages'\nretries = 'many'",
		},
		"protocols that the reader does not know": {
			"endpoints = {responses = 'https://gateway.test/v1'}", "endpoints = {responses = 'https://gateway.test/v1', messages = 'https://gateway.test/messages'}",
			"protocols = {responses = []}", "protocols = {messages = [], responses = []}",
		},
	} {
		t.Run(name, func(t *testing.T) {
			runtime, stdout := helperRuntime(t, codex.ID, true)
			args := helperArgs(t, runtime, codex.ID)
			if err := runtime.Secrets.Set("gateway", "gateway-token"); err != nil {
				t.Fatal(err)
			}
			path := runtime.Config.Path()
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, editConfiguration(t, data, replacements...), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.Config.Load(); err == nil {
				t.Fatal("the full program accepts the edited configuration, which proves nothing")
			}
			command := NewCommand(runtime)
			if err := command.RunE(command, args); err != nil || stdout.String() != "gateway-token\n" {
				t.Fatalf("reader = %q, %v", stdout.String(), err)
			}
		})
	}
}

// The reader refuses a client that it cannot serve, before it reads or
// observes any credential.
func TestCredentialReaderRefusesAClientItCannotServe(t *testing.T) {
	for _, test := range []struct {
		name, from, to, want string
		args                 func([]string) []string
	}{
		{name: "disabled", from: "enabled = true", to: "enabled = false", want: "adapter is not enabled"},
		{name: "authenticated by the client", from: "executable = 'codex'", to: "executable = 'codex'\nauthentication = 'client-native'", want: "client-owned authentication"},
		{name: "an unknown authentication", from: "executable = 'codex'", to: "executable = 'codex'\nauthentication = 'device-code'", want: `the authentication "device-code", which this credential reader does not know`},
		{name: "an endpoint that matches no fingerprint", from: "https://gateway.test/v1", to: "https://moved.test/v1", want: "no longer matches"},
		{name: "a fingerprint of another client", want: "no longer matches", args: func(args []string) []string {
			return []string{args[0], strings.Repeat("0", len(args[1]))}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, stdout := helperRuntime(t, codex.ID, true)
			args := helperArgs(t, runtime, codex.ID)
			if test.from != "" {
				path := runtime.Config.Path()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, editConfiguration(t, data, test.from, test.to), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.args != nil {
				args = test.args(args)
			}
			secrets := &refusingSecretStore{}
			runtime.Secrets = secrets
			command := NewCommand(runtime)
			err := command.RunE(command, args)
			if err == nil || !strings.Contains(err.Error(), test.want) || stdout.Len() != 0 || secrets.getCalls != 0 || secrets.existsCalls != 0 {
				t.Fatalf("reader = %v, stdout %q, reads %d/%d", err, stdout.String(), secrets.getCalls, secrets.existsCalls)
			}
		})
	}
}

// A session that a client left keeps its provider's Token while the provider
// and its endpoint exist, because the reader checks no selection, and gets
// none once either is gone.
func TestCredentialReaderServesALeftProviderWhileItExists(t *testing.T) {
	runtime, stdout := helperRuntime(t, codex.ID, true)
	session := helperArgs(t, runtime, codex.ID)
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Providers["other"] = domain.Provider{Label: "Other", Endpoints: domain.Endpoints{Responses: "https://other.test/v1"}}
	other := domain.Model{Provider: "other", ID: "gpt-other", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.Models[other.Ref()] = other
	cfg.SetSelectedModel(codex.ID, other.Ref(), "")
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("gateway", "gateway-token"); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	if err := command.RunE(command, session); err != nil || stdout.String() != "gateway-token\n" {
		t.Fatalf("the session that Codex left = %q, %v", stdout.String(), err)
	}
	for name, change := range map[string]func(*domain.Config){
		"the endpoint": func(cfg *domain.Config) {
			provider := cfg.Providers["gateway"]
			provider.Endpoints.Responses = "https://moved.test/v1"
			cfg.Providers["gateway"] = provider
		},
		"the provider": func(cfg *domain.Config) {
			delete(cfg.Models, clientModel(codex.ID))
			delete(cfg.Providers, "gateway")
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := cfg.Clone()
			change(&changed)
			if err := runtime.Config.Save(changed); err != nil {
				t.Fatal(err)
			}
			stdout.Reset()
			command := NewCommand(runtime)
			if err := command.RunE(command, session); err == nil || stdout.Len() != 0 {
				t.Fatalf("without %s the reader = %q, %v", name, stdout.String(), err)
			}
		})
	}
}
