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

// schemaNineText is aigw.toml as AIGW writes it, with every field that
// schema 9 defines: %[1]s is the Codex program, %[2]s its target, %[3]s
// the credential command and %[4]s the upstream that forwarding recorded.
const schemaNineText = `version = 9

[[providers]]
id = 'dmxapi'
label = 'DMXAPI'
balance = { kind = 'dmxapi', base_url = 'https://www.dmxapi.cn' }
endpoints = { anthropic = 'https://www.dmxapi.cn', responses = 'https://www.dmxapi.cn/v1' }

[[providers.models]]
id = 'gpt-6.1-sol-cdx'
family = 'gpt-6.1-sol'
label = 'DMXAPI · GPT-6.1 Sol · CDX'
protocols = { responses = ['text', 'reasoning'] }

[[providers.models]]
id = 'claude-opus-5-5'
protocols = { anthropic = ['text', 'tool_search'] }

[[providers]]
id = 'aihubmix'
label = 'AIHubMix'
endpoints = { chat = 'https://aihubmix.com/v1' }

[[providers.models]]
id = 'gemini-3.8-flash'
protocols = { chat = [] }
deprecated = true

[[families]]
id = 'gpt-6.1-sol'
label = 'GPT-6.1 Sol'

[recommendations.claude]
primary = { model = 'dmxapi/claude-opus-5-5' }

[recommendations.codex]
primary = { model = 'dmxapi/gpt-6.1-sol-cdx', protocol = 'responses' }
alternatives = [{ model = 'dmxapi/gpt-6.1-sol-cdx' }]

[clients.claude]
enabled = false
model = 'dmxapi/claude-opus-5-5'
protocol = 'anthropic'
credential_command = '%[3]s'

[clients.codex]
enabled = true
model = 'dmxapi/gpt-6.1-sol-cdx'
protocol = 'responses'
program = '%[1]s'
targets = ['%[2]s']
retries = { request = 3, stream = 2 }
forwarding = { endpoint = 'http://127.0.0.1:8792/dmxapi/v1', upstream = '%[4]s', protocol = 'responses' }

[clients.hermes]
enabled = false
model = 'aihubmix/gemini-3.8-flash'
protocol = 'chat'
`

// schemaNineConfig builds, as commands do, the configuration that
// schemaNineText holds, and returns the text with its paths.
func schemaNineConfig(t *testing.T) (domain.Config, string) {
	t.Helper()
	root := t.TempDir()
	program, target, command := filepath.Join(root, "codex"), filepath.Join(root, ".codex", "config.toml"), filepath.Join(root, "aigw")
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
		Primary:      domain.ClientSelection{Model: sol.Ref(), Protocol: domain.ProtocolResponses},
		Alternatives: []domain.ClientSelection{{Model: sol.Ref()}},
	}
	cfg.SetSelectedModel(ClientClaude, opus.Ref(), "")
	claudeBinding := cfg.Clients[ClientClaude]
	claudeBinding.CredentialCommand = command
	cfg.Clients[ClientClaude] = claudeBinding
	cfg.SetSelectedModel(ClientCodex, sol.Ref(), domain.ProtocolResponses)
	cfg.SetClientActivation(ClientCodex, true, program, []string{target})
	request, stream := uint64(3), uint64(2)
	codexBinding := cfg.Clients[ClientCodex]
	codexBinding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &request, StreamMaxRetries: &stream}
	cfg.Clients[ClientCodex] = codexBinding
	if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/dmxapi/v1"); err != nil {
		t.Fatal(err)
	}
	cfg.SetSelectedModel(ClientHermes, flash.Ref(), domain.ProtocolChat)
	text := fmt.Sprintf(schemaNineText, program, target, command, cfg.Clients[ClientCodex].ForwardingUpstreamIdentity)
	return cfg, text
}

// A configuration with every schema 9 field is written in its display
// order, reads back as the same configuration, and a save without a change
// writes the same bytes and no backup.
func TestSchemaNineRoundTripsEveryField(t *testing.T) {
	cfg, want := schemaNineConfig(t)
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
		!slices.Equal(models, []domain.ModelRef{dmxapiModel("gpt-6.1-sol-cdx"), dmxapiModel("claude-opus-5-5")}) {
		t.Fatalf("providers %v and models %v do not follow the file", providers, models)
	}
}

// dmxapiModel is the reference of provider dmxapi's model id.
func dmxapiModel(id string) domain.ModelRef { return domain.NewModelRef("dmxapi", id) }

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A field that schema 9 does not define, among them the client-native keys of
// schema 8, is refused by its dotted name and its line, and another version
// by its number, before the configuration is used.
func TestSchemaNineRefusesAnUndefinedFieldOrAnotherVersion(t *testing.T) {
	_, valid := schemaNineConfig(t)
	for _, test := range []struct {
		name, from, to, field, line string
	}{
		{"a retired provider order", "id = 'aihubmix'\n", "id = 'aihubmix'\norder = 20\n", "providers.order", "order = 20"},
		{"a retired model purpose", "id = 'claude-opus-5-5'\n", "id = 'claude-opus-5-5'\npurpose = 'Default agent'\n", "providers.models.purpose", "purpose ="},
		{"a retired model lifecycle", "deprecated = true\n", "lifecycle = 'deprecated'\n", "providers.models.lifecycle", "lifecycle ="},
		{"a retired family order", "label = 'GPT-6.1 Sol'\n", "label = 'GPT-6.1 Sol'\norder = 10\n", "families.order", "order = 10"},
		{"a retired endpoint name", "endpoints = { chat = ", "endpoints = { openai_chat_completions = ", "providers.openai_chat_completions", "openai_chat_completions"},
		{"a retired selection field", "primary = { model = 'dmxapi/claude-opus-5-5' }", "primary = { route = 'claude' }", "recommendations.claude.route", "route ="},
		{"a retired retry policy", "retries = { request = 3, stream = 2 }", "retry_policy = { request_max_retries = 3 }", "clients.codex.retry_policy", "retry_policy ="},
		{"an unknown top-level table", "version = 9\n", "version = 9\n[accounts.team]\nlabel = 'Team'\n", "accounts.team", "[accounts.team]"},
		{"a client's executable", "program = ", "executable = ", "clients.codex.executable", "executable ="},
		{"a client's authentication", "protocol = 'responses'\nprogram", "protocol = 'responses'\nauthentication = 'client-native'\nprogram", "clients.codex.authentication", "authentication ="},
		{"a client's model provider", "protocol = 'responses'\nprogram", "protocol = 'responses'\nmodel_provider = 'amazon-bedrock'\nprogram", "clients.codex.model_provider", "model_provider ="},
		{"a recommendation's authentication", "primary = { model = 'dmxapi/claude-opus-5-5' }", "primary = { model = 'dmxapi/claude-opus-5-5', authentication = 'client-native' }", "recommendations.claude.authentication", "authentication ="},
		{"a recommendation's model provider", "alternatives = [{ model = 'dmxapi/gpt-6.1-sol-cdx' }]", "alternatives = [{ model = 'dmxapi/gpt-6.1-sol-cdx', model_provider = 'amazon-bedrock' }]", "recommendations.codex.model_provider", "model_provider ="},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := strings.Replace(valid, test.from, test.to, 1)
			if text == valid {
				t.Fatalf("the fixture has no %q", test.from)
			}
			line := slices.IndexFunc(strings.Split(text, "\n"), func(line string) bool { return strings.Contains(line, test.line) }) + 1
			_, err := decodeTOMLConfig([]byte(text), testAdmission())
			loadErr, isLoad := errors.AsType[*LoadError](err)
			undefined, isUndefined := errors.AsType[*UndefinedFieldError](err)
			if !isLoad || loadErr.Phase != LoadPhaseValidate || !isUndefined || undefined.Field != test.field ||
				!strings.Contains(err.Error(), fmt.Sprintf("schema %d does not define the field %s, on line %d", domain.ConfigVersion, test.field, line)) {
				t.Fatalf("load error = %v, want %s on line %d", err, test.field, line)
			}
		})
	}
	for _, version := range []int{0, 7, 10} {
		text := strings.Replace(valid, "version = 9\n", fmt.Sprintf("version = %d\n", version), 1)
		_, err := decodeTOMLConfig([]byte(text), testAdmission())
		unsupported, ok := errors.AsType[*domain.UnsupportedConfigVersionError](err)
		if !ok || unsupported.Version != version || unsupported.ExpectedVersion != domain.ConfigVersion || unsupported.Edits() != "" {
			t.Errorf("version %d = %v", version, err)
		}
	}
}

// A client's program declaration is checked when the file loads: one of
// program and extensions, each an absolute, clean path without a control
// character, and a refusal names the client and the field. Whether the path
// exists is not checked, so a declared program that is gone loads.
func TestAProgramDeclarationIsCheckedWhenTheFileLoads(t *testing.T) {
	_, valid := schemaNineConfig(t)
	_, rest, _ := strings.Cut(valid, "program = '")
	declared, _, _ := strings.Cut(rest, "'")
	for _, test := range []struct{ name, to, want string }{
		{"both declarations", "program = '" + declared + "'\nextensions = '/home/user/.vscode/extensions'", `client "codex" declares both program and extensions`},
		{"a relative program", "program = 'bin/codex'", `client "codex" program must be an absolute, clean path`},
		{"a program that is not clean", "program = '/opt/../codex'", `client "codex" program must be an absolute, clean path`},
		{"extensions with ..", "extensions = '/home/user/../.vscode/extensions'", `client "codex" extensions must be an absolute, clean path`},
		{"extensions with a control character", "extensions = \"/home/user/.vscode/ext\\u0007ensions\"", `client "codex" extensions must be an absolute, clean path`},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := strings.Replace(valid, "program = '"+declared+"'", test.to, 1)
			if text == valid {
				t.Fatal("the fixture declares no program")
			}
			if _, err := decodeTOMLConfig([]byte(text), testAdmission()); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v, want %q", err, test.want)
			}
		})
	}
	for _, test := range []struct{ name, to string }{
		{"a missing program", "program = '/nowhere/codex'"},
		{"a missing extensions directory", "extensions = '/nowhere/extensions'"},
		{"neither", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := decodeTOMLConfig([]byte(strings.Replace(valid, "program = '"+declared+"'\n", test.to+"\n", 1)), testAdmission())
			if err != nil {
				t.Fatalf("load error = %v", err)
			}
			if binding := cfg.Clients[ClientCodex]; test.to != "" && !binding.DeclaresProgram() || test.to == "" && binding.DeclaresProgram() {
				t.Fatalf("loaded %+v", binding)
			}
		})
	}
}

// A version 8 file is refused before any other field is read, whatever it
// holds, with the edits that schema 9 needs, and no file changes.
func TestAVersionEightFileIsRefusedWithItsEdits(t *testing.T) {
	const schemaEight = `version = 8

[[providers]]
id = 'team'
label = 'Team'
endpoints = { responses = 'https://team.test/v1' }

[[providers.models]]
id = 'gpt-team'
protocols = { responses = [] }

[recommendations.codex]
primary = { model = 'team/gpt-team', model_provider = 'amazon-bedrock', authentication = 'client-native' }

[clients.codex]
enabled = true
model = 'team/gpt-team'
model_provider = 'amazon-bedrock'
authentication = 'client-native'
executable = '/opt/codex'
retired = 'a field that no schema defines'
`
	path := filepath.Join(t.TempDir(), "aigw.toml")
	if err := os.WriteFile(path, []byte(schemaEight), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(path, testAdmission()).Load()
	loadErr, isLoad := errors.AsType[*LoadError](err)
	unsupported, isVersion := errors.AsType[*domain.UnsupportedConfigVersionError](err)
	if !isLoad || loadErr.Phase != LoadPhaseValidate || !isVersion || unsupported.Version != 8 {
		t.Fatalf("load error = %v", err)
	}
	for _, edit := range []string{"set `version = 9`", "rename each client's `executable` to `program`", "delete `authentication` and `model_provider` wherever they appear"} {
		if !strings.Contains(unsupported.Edits(), edit) || !strings.Contains(err.Error(), edit) {
			t.Errorf("the refusal %q does not name %q", err, edit)
		}
	}
	if kept := readTestFile(t, path); string(kept) != schemaEight {
		t.Fatalf("the refused file changed:\n%s", kept)
	}
	if entries, err := os.ReadDir(filepath.Dir(path)); err != nil || len(entries) != 1 {
		t.Fatalf("the refusal wrote beside the file: %v, %v", entries, err)
	}
}

// A repeated provider, model or family, a selection that is not
// provider/model and a value of the wrong type are refused at the stage
// that finds them.
func TestSchemaNineRefusesWhatItCannotHold(t *testing.T) {
	_, valid := schemaNineConfig(t)
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
		{"a recommendation without a provider", strings.Replace(valid, "primary = { model = 'dmxapi/claude-opus-5-5' }", "primary = { model = 'claude-opus-5-5' }", 1), LoadPhaseValidate, "the claude recommendation"},
		{"a value of the wrong type", strings.Replace(valid, "enabled = true", "enabled = 'yes'", 1), LoadPhaseParse, ""},
		{"a version of the wrong type", strings.Replace(valid, "version = 9", "version = 'nine'", 1), LoadPhaseParse, ""},
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

// A model that declares no protocol, absent or empty, is refused when read,
// naming the model: no client can use it.
func TestSchemaNineRefusesAModelWithoutProtocols(t *testing.T) {
	const head = "version = 9\n\n[[providers]]\nid = 'team'\nlabel = 'Team'\nendpoints = { anthropic = 'https://team.test' }\n\n[[providers.models]]\nid = 'bare'\n"
	for name, protocols := range map[string]string{"absent": "", "empty": "protocols = {}\n"} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeTOMLConfig([]byte(head+protocols), testAdmission())
			if err == nil || !strings.Contains(err.Error(), "model team/bare declares no protocol, so no client can use it") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

// A forwarding table names the endpoint that the client forwards to; one
// without an endpoint, empty or not, is refused by the field's name, rather
// than loaded as a client that forwards nothing and dropped on the next
// write.
func TestSchemaNineRefusesAForwardingTableWithoutAnEndpoint(t *testing.T) {
	_, valid := schemaNineConfig(t)
	lines := strings.Split(valid, "\n")
	index := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, "forwarding = ") })
	if index < 0 {
		t.Fatalf("the fixture has no forwarding table:\n%s", valid)
	}
	for name, table := range map[string]string{
		"an empty table":           "forwarding = {}",
		"a table without endpoint": "forwarding = { upstream = 'abc', protocol = 'responses' }",
	} {
		t.Run(name, func(t *testing.T) {
			edited := slices.Clone(lines)
			edited[index] = table
			_, err := decodeTOMLConfig([]byte(strings.Join(edited, "\n")), testAdmission())
			loadErr, isLoad := errors.AsType[*LoadError](err)
			missing, isMissing := errors.AsType[*MissingFieldError](err)
			if !isLoad || loadErr.Phase != LoadPhaseValidate || !isMissing || missing.Field != "clients.codex.forwarding.endpoint" ||
				!strings.Contains(err.Error(), fmt.Sprintf("schema %d requires the field clients.codex.forwarding.endpoint", domain.ConfigVersion)) {
				t.Fatalf("load error = %v", err)
			}
		})
	}
}

// An array on a line wider than taplo's column width is written one element
// to a line, as taplo writes it; a narrower one, an inline table and an
// unbalanced value stay, and commas inside strings and tables split nothing.
func TestEncoderExpandsAWideArrayAsTaploDoes(t *testing.T) {
	wide := "alternatives = [{ model = 'ucloud/gpt-6.2-sol', protocol = 'responses' }, { model = 'aihubmix/gpt-6.2-sol', label = 'a, b' }]"
	for line, want := range map[string]string{
		wide: "alternatives = [\n  { model = 'ucloud/gpt-6.2-sol', protocol = 'responses' },\n  { model = 'aihubmix/gpt-6.2-sol', label = 'a, b' },\n]",
		"alternatives = [{ model = 'ucloud/gpt-6.2-sol' }]":                              "alternatives = [{ model = 'ucloud/gpt-6.2-sol' }]",
		"endpoints = { anthropic = '" + strings.Repeat("x", 90) + "', responses = 'y' }": "endpoints = { anthropic = '" + strings.Repeat("x", 90) + "', responses = 'y' }",
		"capabilities = ['" + strings.Repeat("x", 90) + "', '[', \"]\"]":                 "capabilities = [\n  '" + strings.Repeat("x", 90) + "',\n  '[',\n  \"]\",\n]",
		"broken = [{ a = '" + strings.Repeat("x", 90) + "' ]":                            "broken = [{ a = '" + strings.Repeat("x", 90) + "' ]",
	} {
		if got := expandWideArray(line); got != want {
			t.Errorf("expandWideArray(%q) =\n%s\nwant\n%s", line, got, want)
		}
	}
}

// The encoder spaces the braces of each inline table as taplo does, and
// leaves braces and quotes inside strings alone, so a label keeps its text.
func TestSchemaNineSpacesInlineTablesOutsideStrings(t *testing.T) {
	for line, want := range map[string]string{
		"a = {b = 'c'}":               "a = { b = 'c' }",
		"a = {}":                      "a = {}",
		"a = [{b = 1}, {c = 2}]":      "a = [{ b = 1 }, { c = 2 }]",
		`a = {b = '{x}', c = "q\"{"}`: `a = { b = '{x}', c = "q\"{" }`,
		`label = "it's {here}"`:       `label = "it's {here}"`,
	} {
		if got := spaceInlineTables(line); got != want {
			t.Errorf("spaceInlineTables(%q) = %q, want %q", line, got, want)
		}
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Version = domain.ConfigVersion
	label := `Team {beta} "quoted" 'single'`
	if err := cfg.AppendProvider("team", domain.Provider{Label: label, Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}); err != nil {
		t.Fatal(err)
	}
	data, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := decodeTOMLConfig(data, testAdmission())
	if err != nil || loaded.Providers["team"].Label != label {
		t.Fatalf("the label read back as %q, %v:\n%s", loaded.Providers["team"].Label, err, data)
	}
}
