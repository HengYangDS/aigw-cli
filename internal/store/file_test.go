package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// schemaEightText is aigw.toml as AIGW writes it, with every field that
// schema 8 defines: %[1]s is the Codex executable, %[2]s its target, %[3]s
// the credential command and %[4]s the upstream that forwarding recorded.
const schemaEightText = `version = 8

[[providers]]
id = 'dmxapi'
label = 'DMXAPI'
balance = {kind = 'dmxapi', base_url = 'https://www.dmxapi.cn'}
endpoints = {anthropic = 'https://www.dmxapi.cn', responses = 'https://www.dmxapi.cn/v1'}

[[providers.models]]
id = 'gpt-6.1-sol-cdx'
family = 'gpt-6.1-sol'
label = 'DMXAPI · GPT-6.1 Sol · CDX'
protocols = {responses = ['text', 'reasoning']}

[[providers.models]]
id = 'claude-opus-5-5'
protocols = {anthropic = ['text', 'tool_search']}

[[providers]]
id = 'aihubmix'
label = 'AIHubMix'
endpoints = {chat = 'https://aihubmix.com/v1'}

[[providers.models]]
id = 'gemini-3.8-flash'
protocols = {chat = []}
deprecated = true

[[families]]
id = 'gpt-6.1-sol'
label = 'GPT-6.1 Sol'

[recommendations.claude]
primary = {model = 'dmxapi/claude-opus-5-5'}

[recommendations.codex]
primary = {model = 'dmxapi/gpt-6.1-sol-cdx', protocol = 'responses'}
alternatives = [{model = 'dmxapi/gpt-6.1-sol-cdx', protocol = 'responses', model_provider = 'amazon-bedrock', authentication = 'client-native'}]

[clients.claude]
enabled = false
model = 'dmxapi/claude-opus-5-5'
protocol = 'anthropic'
credential_command = '%[3]s'

[clients.codex]
enabled = true
model = 'dmxapi/gpt-6.1-sol-cdx'
protocol = 'responses'
executable = '%[1]s'
targets = ['%[2]s']
retries = {request = 3, stream = 2}
forwarding = {endpoint = 'http://127.0.0.1:8792/dmxapi/v1', upstream = '%[4]s', protocol = 'responses'}

[clients.hermes]
enabled = false
model = 'aihubmix/gemini-3.8-flash'
protocol = 'chat'
`

// schemaEightConfig builds, as commands do, the configuration that
// schemaEightText holds, and returns the text with its paths.
func schemaEightConfig(t *testing.T) (domain.Config, string) {
	t.Helper()
	root := t.TempDir()
	executable, target, command := filepath.Join(root, "codex"), filepath.Join(root, ".codex", "config.toml"), filepath.Join(root, "aigw")
	cfg := domain.NewConfig(testAdmission())
	cfg.Version = domain.ConfigVersion
	if err := cfg.AppendProvider("dmxapi", domain.Provider{
		Label: "DMXAPI", BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"},
		Endpoints: domain.Endpoints{Anthropic: "https://www.dmxapi.cn", Responses: "https://www.dmxapi.cn/v1"},
	}); err != nil {
		t.Fatal(err)
	}
	sol := domain.Model{Provider: "dmxapi", ID: "gpt-6.1-sol-cdx", Family: "gpt-6.1-sol", Label: "DMXAPI · GPT-6.1 Sol · CDX",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {domain.CapabilityText, domain.CapabilityReasoning}}}
	opus := domain.Model{Provider: "dmxapi", ID: "claude-opus-5-5",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityText, domain.CapabilityToolSearch}}}
	if err := cfg.AppendProvider("aihubmix", domain.Provider{Label: "AIHubMix", Endpoints: domain.Endpoints{Chat: "https://aihubmix.com/v1"}}); err != nil {
		t.Fatal(err)
	}
	flash := domain.Model{Provider: "aihubmix", ID: "gemini-3.8-flash", Deprecated: true,
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolChat: {}}}
	for _, model := range []domain.Model{sol, opus, flash} {
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	if err := cfg.AppendFamily("gpt-6.1-sol", domain.Family{Label: "GPT-6.1 Sol"}); err != nil {
		t.Fatal(err)
	}
	cfg.Recommendations[ClientClaude] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: opus.Ref()}}
	cfg.Recommendations[ClientCodex] = domain.ClientRecommendation{
		Primary: domain.ClientSelection{Model: sol.Ref(), Protocol: domain.ProtocolResponses},
		Alternatives: []domain.ClientSelection{{Model: sol.Ref(), Protocol: domain.ProtocolResponses, ModelProvider: "amazon-bedrock",
			Authentication: domain.AuthenticationClientNative}},
	}
	cfg.SetSelectedModel(ClientClaude, opus.Ref(), "")
	claudeBinding := cfg.Clients[ClientClaude]
	claudeBinding.CredentialCommand = command
	cfg.Clients[ClientClaude] = claudeBinding
	cfg.SetSelectedModel(ClientCodex, sol.Ref(), domain.ProtocolResponses)
	cfg.SetClientActivation(ClientCodex, true, executable, []string{target})
	request, stream := uint64(3), uint64(2)
	codexBinding := cfg.Clients[ClientCodex]
	codexBinding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &request, StreamMaxRetries: &stream}
	cfg.Clients[ClientCodex] = codexBinding
	if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/dmxapi/v1"); err != nil {
		t.Fatal(err)
	}
	cfg.SetSelectedModel(ClientHermes, flash.Ref(), domain.ProtocolChat)
	text := fmt.Sprintf(schemaEightText, executable, target, command, cfg.Clients[ClientCodex].ForwardingUpstreamIdentity)
	return cfg, text
}

// A configuration with every schema 8 field is written in its display
// order, reads back as the same configuration, and a save without a change
// writes the same bytes and no backup.
func TestSchemaEightRoundTripsEveryField(t *testing.T) {
	cfg, want := schemaEightConfig(t)
	data, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("aigw.toml =\n%s\nwant\n%s", data, want)
	}
	path := filepath.Join(t.TempDir(), "aigw.toml")
	if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	configStore := NewStore(path, testAdmission())
	loaded, err := configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeConfig(loaded)
	if err != nil || string(again) != want {
		t.Fatalf("the loaded configuration encodes as\n%s\n%v", again, err)
	}
	if err := configStore.Save(loaded); err != nil {
		t.Fatal(err)
	}
	if saved := readTestFile(t, path); string(saved) != want {
		t.Fatalf("an unchanged save rewrote aigw.toml:\n%s", saved)
	}
	if _, err := os.Stat(path + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an unchanged save wrote a backup: %v", err)
	}
	if model := loaded.Models[domain.NewModelRef("aihubmix", "gemini-3.8-flash")]; !model.Deprecated || model.FamilyID() != "gemini-3.8-flash" {
		t.Fatalf("the deprecated model = %+v", model)
	}
	// The arrays' order is the display order, which no identifier sorts.
	if providers, models := loaded.ProviderIDs(), loaded.ProviderModelRefs("dmxapi"); !slices.Equal(providers, []string{"dmxapi", "aihubmix"}) ||
		!slices.Equal(models, []domain.ModelRef{domain.NewModelRef("dmxapi", "gpt-6.1-sol-cdx"), domain.NewModelRef("dmxapi", "claude-opus-5-5")}) {
		t.Fatalf("providers %v and models %v do not follow the file", providers, models)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A field that schema 8 does not define is refused by its dotted name and
// its line, and another version by its number, before the configuration is
// used.
func TestSchemaEightRefusesAnUndefinedFieldOrAnotherVersion(t *testing.T) {
	_, valid := schemaEightConfig(t)
	for _, test := range []struct {
		name, from, to, field, line string
	}{
		{"a retired provider order", "id = 'aihubmix'\n", "id = 'aihubmix'\norder = 20\n", "providers.order", "order = 20"},
		{"a retired model purpose", "id = 'claude-opus-5-5'\n", "id = 'claude-opus-5-5'\npurpose = 'Default agent'\n", "providers.models.purpose", "purpose ="},
		{"a retired model lifecycle", "deprecated = true\n", "lifecycle = 'deprecated'\n", "providers.models.lifecycle", "lifecycle ="},
		{"a retired family order", "label = 'GPT-6.1 Sol'\n", "label = 'GPT-6.1 Sol'\norder = 10\n", "families.order", "order = 10"},
		{"a retired endpoint name", "endpoints = {chat = ", "endpoints = {openai_chat_completions = ", "providers.openai_chat_completions", "openai_chat_completions"},
		{"a retired selection field", "primary = {model = 'dmxapi/claude-opus-5-5'}", "primary = {route = 'claude'}", "recommendations.claude.route", "route ="},
		{"a retired retry policy", "retries = {request = 3, stream = 2}", "retry_policy = {request_max_retries = 3}", "clients.codex.retry_policy", "retry_policy ="},
		{"an unknown top-level table", "version = 8\n", "version = 8\n[accounts.team]\nlabel = 'Team'\n", "accounts.team", "[accounts.team]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := strings.Replace(valid, test.from, test.to, 1)
			if text == valid {
				t.Fatalf("the fixture has no %q", test.from)
			}
			line := slices.IndexFunc(strings.Split(text, "\n"), func(line string) bool { return strings.Contains(line, test.line) }) + 1
			_, err := decodeTOMLConfig([]byte(text), testAdmission())
			loadErr, isLoad := errors.AsType[*LoadError](err)
			undefined, isUndefined := errors.AsType[*undefinedFieldError](err)
			if !isLoad || loadErr.Phase != LoadPhaseValidate || !isUndefined || undefined.Field != test.field ||
				!strings.Contains(err.Error(), fmt.Sprintf("schema 8 does not define the field %s, on line %d", test.field, line)) {
				t.Fatalf("load error = %v, want %s on line %d", err, test.field, line)
			}
		})
	}
	for _, version := range []int{0, 7, 9} {
		text := strings.Replace(valid, "version = 8\n", fmt.Sprintf("version = %d\n", version), 1)
		_, err := decodeTOMLConfig([]byte(text), testAdmission())
		unsupported, ok := errors.AsType[*domain.UnsupportedConfigVersionError](err)
		if !ok || unsupported.Version != version || unsupported.ExpectedVersion != domain.ConfigVersion {
			t.Errorf("version %d = %v", version, err)
		}
	}
}

// A repeated provider, model or family, a selection that is not
// provider/model and a value of the wrong type are refused at the stage
// that finds them.
func TestSchemaEightRefusesWhatItCannotHold(t *testing.T) {
	_, valid := schemaEightConfig(t)
	repeatedModel := strings.Replace(valid, "[[providers]]\nid = 'aihubmix'", "[[providers.models]]\nid = 'claude-opus-5-5'\nprotocols = {anthropic = []}\n\n[[providers]]\nid = 'aihubmix'", 1)
	for _, test := range []struct {
		name, text string
		phase      LoadPhase
		want       string
	}{
		{"a repeated provider", valid + "\n[[providers]]\nid = 'dmxapi'\nlabel = 'Again'\nendpoints = {}\n", LoadPhaseValidate, `provider "dmxapi" is defined twice`},
		{"a repeated model", repeatedModel, LoadPhaseValidate, `provider "dmxapi" defines model "claude-opus-5-5" twice`},
		{"a repeated family", valid + "\n[[families]]\nid = 'gpt-6.1-sol'\nlabel = 'Again'\n", LoadPhaseValidate, `family "gpt-6.1-sol" is defined twice`},
		{"a client selection without a provider", strings.Replace(valid, "model = 'aihubmix/gemini-3.8-flash'", "model = 'gemini-3.8-flash'", 1), LoadPhaseValidate, `client "hermes"`},
		{"a recommendation without a provider", strings.Replace(valid, "primary = {model = 'dmxapi/claude-opus-5-5'}", "primary = {model = 'claude-opus-5-5'}", 1), LoadPhaseValidate, "the claude recommendation"},
		{"a value of the wrong type", strings.Replace(valid, "enabled = true", "enabled = 'yes'", 1), LoadPhaseParse, ""},
		{"a version of the wrong type", strings.Replace(valid, "version = 8", "version = 'eight'", 1), LoadPhaseParse, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.text == valid {
				t.Fatal("the fixture did not change")
			}
			_, err := decodeTOMLConfig([]byte(test.text), testAdmission())
			loadErr, ok := errors.AsType[*LoadError](err)
			if !ok || loadErr.Phase != test.phase || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v", err)
			}
		})
	}
}

// A model that declares no protocol is written with an empty protocols
// table, which reads back as the same model.
func TestSchemaEightWritesAModelWithoutProtocols(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Version = domain.ConfigVersion
	if err := cfg.AppendProvider("team", domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AppendModel(domain.Model{Provider: "team", ID: "bare"}); err != nil {
		t.Fatal(err)
	}
	data, err := encodeConfig(cfg)
	if err != nil || !strings.Contains(string(data), "[[providers.models]]\nid = 'bare'\nprotocols = {}\n") {
		t.Fatalf("aigw.toml = %s, %v", data, err)
	}
	loaded, err := decodeTOMLConfig(data, testAdmission())
	if err != nil || len(loaded.Models[domain.NewModelRef("team", "bare")].Protocols) != 0 {
		t.Fatalf("the model without protocols read back as %+v, %v", loaded.Models, err)
	}
}
