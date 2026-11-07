package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

const forwardingEndpoint = "http://127.0.0.1:8792/v1"

// countingSecretStore is a credential store that holds the gateway's Token and
// counts reads of a Token value, which no edit makes.
type countingSecretStore struct{ reads int }

func (s *countingSecretStore) Get(string) (string, error) { s.reads++; return "token", nil }
func (*countingSecretStore) Set(string, string) error     { return errors.New("an edit stores no Token") }
func (*countingSecretStore) Delete(string) error          { return errors.New("an edit deletes no Token") }
func (*countingSecretStore) Exists(account string) (bool, error) {
	return account == "gateway", nil
}

// enabledCodex enables Codex on a configuration file below a private
// directory, whose retry budgets are request 3 and stream 1, and returns the
// runtime, its output, the file and the credential store.
func enabledCodex(t *testing.T) (invocation.Context, *bytes.Buffer, string, *countingSecretStore) {
	t.Helper()
	cfg := adapterConfig()
	three, one := uint64(3), uint64(1)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &three, StreamMaxRetries: &one}
	cfg.Clients[codex.ID] = binding
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model = \"original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(codex.HomeDefault), ConfigPath: target, Present: true}}}}
	if err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", target); err != nil {
		t.Fatal(err)
	}
	counting := &countingSecretStore{}
	runtime.Secrets = counting
	out.Reset()
	return runtime, out, target, counting
}

// edited runs `aigw client edit` with --json and returns its document.
func edited(t *testing.T, runtime invocation.Context, out *bytes.Buffer, args ...string) editDocument {
	t.Helper()
	out.Reset()
	if err := executeAdapter(t, runtime, append(append([]string{"edit"}, args...), "--json")...); err != nil {
		t.Fatal(err)
	}
	var document editDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("edit document: %v\n%s", err, out)
	}
	return document
}

func readTarget(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Forwarding sends Codex to another endpoint and back while its provider,
// model and retry budgets stay; Codex's file follows each change, and no edit
// reads a Token.
func TestEditForwardsAClientAndKeepsItsRetryBudgets(t *testing.T) {
	runtime, out, target, secrets := enabledCodex(t)
	document := edited(t, runtime, out, codex.ID, "--via", forwardingEndpoint+"/")
	if !document.Changed || document.Forwarding != forwardingEndpoint || document.Model != "gateway/codex-test" ||
		document.Retries == nil || *document.Retries.Request != 3 || *document.Retries.Stream != 1 || document.NextAction != "aigw check" {
		t.Fatalf("forwarding document = %#v", document)
	}
	if projected := readTarget(t, target); !strings.Contains(projected, `base_url = "`+forwardingEndpoint+`"`) || !strings.Contains(projected, "request_max_retries = 3") {
		t.Fatalf("Codex's file does not forward with its budgets:\n%s", projected)
	}
	if len(document.ClientFiles) != 1 || document.ClientFiles[0].Client != codex.ID || document.ClientFiles[0].Action == "" {
		t.Fatalf("client files = %#v", document.ClientFiles)
	}
	document = edited(t, runtime, out, codex.ID, "--direct")
	if !document.Changed || document.Forwarding != "" || document.Retries == nil || *document.Retries.Request != 3 {
		t.Fatalf("direct document = %#v", document)
	}
	if projected := readTarget(t, target); !strings.Contains(projected, `base_url = "https://gateway.test/v1"`) {
		t.Fatalf("Codex's file does not reach its provider again:\n%s", projected)
	}
	if secrets.reads != 0 {
		t.Fatalf("an edit read %d Token values", secrets.reads)
	}
}

// Budgets that an edit names replace those budgets, the others stay, and
// --native-retries removes both; Codex's file follows each change.
func TestEditDeclaresAndRemovesRetryBudgets(t *testing.T) {
	runtime, out, target, _ := enabledCodex(t)
	document := edited(t, runtime, out, codex.ID, "--request-retries", "0")
	if document.Retries == nil || *document.Retries.Request != 0 || *document.Retries.Stream != 1 {
		t.Fatalf("partial budgets = %#v", document.Retries)
	}
	if projected := readTarget(t, target); !strings.Contains(projected, "request_max_retries = 0") || !strings.Contains(projected, "stream_max_retries = 1") {
		t.Fatalf("Codex's file lacks the budgets:\n%s", projected)
	}
	document = edited(t, runtime, out, codex.ID, "--native-retries")
	if document.Retries != nil || !document.Changed {
		t.Fatalf("native budgets = %#v", document)
	}
	if projected := readTarget(t, target); strings.Contains(projected, "max_retries") {
		t.Fatalf("Codex's file keeps a budget:\n%s", projected)
	}
	saved, err := runtime.Config.Load()
	if err != nil || saved.Clients[codex.ID].RetryPolicy != nil {
		t.Fatalf("saved budgets = %#v, %v", saved.Clients[codex.ID].RetryPolicy, err)
	}
}

// A preview reports the edit, the file that it changes and the command that
// applies it, and writes nothing; an edit that changes nothing writes nothing
// either and says so.
func TestEditPreviewAndUnchangedEditWriteNothing(t *testing.T) {
	runtime, out, target, _ := enabledCodex(t)
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	file := readTarget(t, target)
	document := edited(t, runtime, out, codex.ID, "--via", forwardingEndpoint, "--dry-run")
	if !document.DryRun || !document.Changed || len(document.ClientFiles) != 1 || document.NextAction != "aigw client edit "+codex.ID+" --via "+forwardingEndpoint {
		t.Fatalf("preview document = %#v", document)
	}
	document = edited(t, runtime, out, codex.ID, "--direct")
	if document.Changed || len(document.ClientFiles) != 0 || document.NextAction != "" {
		t.Fatalf("unchanged document = %#v", document)
	}
	out.Reset()
	if err := executeAdapter(t, runtime, "edit", codex.ID, "--request-retries", "3"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Client unchanged") || !strings.Contains(out.String(), "Nothing to change: Codex already has these settings") {
		t.Fatalf("unchanged output:\n%s", out)
	}
	if after, err := runtime.Config.CaptureSnapshot(); err != nil || !reflect.DeepEqual(before, after) || readTarget(t, target) != file {
		t.Fatalf("a preview or an unchanged edit wrote something: %v", err)
	}
}

// A new configuration file replaces the old one: AIGW's settings leave the old
// file and reach the new one, and the executable is kept as an absolute path.
func TestEditMovesTheConfigurationFileAndTheExecutable(t *testing.T) {
	runtime, out, target, _ := enabledCodex(t)
	moved := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(moved, []byte("model = \"other\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	document := edited(t, runtime, out, codex.ID, "--target", moved, "--executable", "codex-next")
	if want, _ := filepath.Abs("codex-next"); document.Executable != want || len(document.Targets) != 1 || document.Targets[0] != moved {
		t.Fatalf("moved document = %#v", document)
	}
	if old := readTarget(t, target); old != "model = \"original\"\n" {
		t.Fatalf("the old file keeps AIGW's settings:\n%s", old)
	}
	if projected := readTarget(t, moved); !strings.Contains(projected, "AIGW managed provider") {
		t.Fatalf("the new file lacks AIGW's settings:\n%s", projected)
	}
}

// Forwarding and retry budgets belong to a client's connection to its model's
// provider, so a client without a model refuses them before anything is
// written, naming the selection that gives it one.
func TestEditRefusesAConnectionWithoutAModel(t *testing.T) {
	cfg := adapterConfig()
	delete(cfg.Clients, codex.ID)
	runtime, _, _, _ := adapterRuntime(t, cfg)
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"edit", hermes.ID, "--via", forwardingEndpoint}, {"edit", codex.ID, "--request-retries", "0"}} {
		err := executeAdapter(t, runtime, args...)
		problem := terminal.ProblemFor(err)
		if !strings.Contains(problem.Title, "selects no model") || problem.Fix != "aigw use "+args[1]+" <provider/model>" {
			t.Fatalf("%q = %q, fix %q", args, problem.Title, problem.Fix)
		}
		if after, err := runtime.Config.CaptureSnapshot(); err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("a refused edit wrote the configuration: %v", err)
		}
	}
}

// A disabled client keeps its settings in the configuration, and no client
// file changes until it is enabled.
func TestEditOfADisabledClientChangesOnlyTheConfiguration(t *testing.T) {
	runtime, out, _, _ := adapterRuntime(t, adapterConfig())
	document := edited(t, runtime, out, claude.ID, "--via", forwardingEndpoint)
	if !document.Changed || document.Enabled || document.Forwarding != forwardingEndpoint || len(document.ClientFiles) != 0 || document.NextAction != "" {
		t.Fatalf("disabled document = %#v", document)
	}
	saved, err := runtime.Config.Load()
	if err != nil || saved.Clients[claude.ID].ForwardingEndpoint != forwardingEndpoint {
		t.Fatalf("saved forwarding = %#v, %v", saved.Clients[claude.ID], err)
	}
}

// The edit is refused before anything is written when the adapter refuses a
// configuration file, or when the client's file cannot be planned.
func TestEditRefusesWhatTheClientCannotUse(t *testing.T) {
	runtime, _, target, _ := enabledCodex(t)
	executable := filepath.Join(t.TempDir(), "codex")
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{
		{ID: string(codex.HomeDefault), ConfigPath: target, Present: true}, {ID: string(codex.HomeDefault), Executable: executable},
	}}}
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "edit", codex.ID, "--target", executable); err == nil || !strings.Contains(err.Error(), "executable") || terminal.ProblemFor(err).Fix != "aigw client edit --help" {
		t.Fatalf("an executable as a configuration file = %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing", "config.toml")
	err = executeAdapter(t, runtime, "edit", codex.ID, "--target", missing)
	if problem := terminal.ProblemFor(err); !strings.Contains(problem.Title, "does not exist") || problem.Fix != "aigw client edit "+codex.ID+" --target <file>" {
		t.Fatalf("a missing configuration file = %q, fix %q", problem.Title, problem.Fix)
	}
	if after, err := runtime.Config.CaptureSnapshot(); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("a refused edit wrote the configuration: %v", err)
	}
	if file := readTarget(t, target); !strings.Contains(file, "AIGW managed provider") {
		t.Fatalf("a refused edit changed Codex's file:\n%s", file)
	}
}

func TestEditShowsTheClientsSettings(t *testing.T) {
	runtime, out, target, _ := enabledCodex(t)
	if err := executeAdapter(t, runtime, "edit", codex.ID, "--stream-retries", "2"); err != nil {
		t.Fatal(err)
	}
	executable, _ := filepath.Abs("/opt/codex")
	for _, want := range []string{"Client edited", "Codex", "gateway/codex-test", "Enabled", executable, target,
		"None; the client reaches its provider's endpoint", "Request retries", "Stream retries", "Client files", "aigw check"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("edit output lacks %q:\n%s", want, out)
		}
	}
	out.Reset()
	if err := executeAdapter(t, runtime, "edit", codex.ID, "--native-retries", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Client edited (preview)", "The client's own defaults", "Nothing was written", "aigw client edit " + codex.ID + " --native-retries"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("preview output lacks %q:\n%s", want, out)
		}
	}
}

func TestEditExamplesAndHelpNameTheClientsThatAcceptEachFlag(t *testing.T) {
	runtime, _, _, _ := adapterRuntime(t, adapterConfig())
	command := newEditCommand(runtime)
	if examples := strings.Split(command.Example, "\n"); len(examples) != 3 || !strings.HasPrefix(examples[0], "aigw client edit "+codex.ID+" ") {
		t.Fatalf("examples = %q", examples)
	}
	if usage := command.Flags().Lookup("target").Usage; !strings.Contains(usage, "for Claude Desktop, Codex and Hermes;") || !strings.Contains(usage, "repeat for multiple Codex homes") {
		t.Fatalf("--target help = %q", usage)
	}
	if got := clientsWith(runtime.Clients.Admission(), func(domain.ClientSpec) bool { return false }); got != "no client" {
		t.Fatalf("clientsWith(none) = %q", got)
	}
	if got := clientsWith(runtime.Clients.Admission(), func(spec domain.ClientSpec) bool { return spec.ID != hermes.ID }); got != "Claude, Claude Desktop and Codex" {
		t.Fatalf("clientsWith(three) = %q", got)
	}
}

// Each refusal of an edit's arguments comes before the configuration is read,
// names what is wrong and names the command's help, or the client list for an
// unknown client.
func TestEditRefusesItsArgumentsBeforeReadingTheConfiguration(t *testing.T) {
	runtime, _, _, _ := adapterRuntime(t, adapterConfig())
	// A configuration that cannot load shows that each refusal comes first.
	runtime.Config = store.NewStore(t.TempDir(), clients.Admission())
	for _, test := range []struct {
		args       []string
		want, next string
	}{
		{[]string{}, "needs a <client> argument: claude, claude-desktop, codex, or hermes", "aigw client edit --help"},
		{[]string{codex.ID, hermes.ID, "--direct"}, "takes one <client> argument, not 2", "aigw client edit --help"},
		{[]string{"codx", "--direct"}, `unknown client "codx"; did you mean codex?`, "aigw client list"},
		{[]string{codex.ID}, "client edit needs --executable, --target, --via, --direct", "aigw client edit --help"},
		{[]string{codex.ID, "--executable", ""}, "--executable requires a non-empty path", "aigw client edit --help"},
		{[]string{codex.ID, "--target", " "}, "--target requires a non-empty path", "aigw client edit --help"},
		{[]string{codex.ID, "--target", "a.toml", "--target", "./a.toml"}, "twice", "aigw client edit --help"},
		{[]string{claude.ID, "--target", "settings.json"}, "Claude has no configuration file to name; --target is for Claude Desktop, Codex and Hermes", "aigw client edit --help"},
		{[]string{codex.ID, "--via", forwardingEndpoint, "--direct"}, "--via and --direct are mutually exclusive", "aigw client edit --help"},
		{[]string{codex.ID, "--via", ""}, "--via requires an endpoint URL", "aigw client edit --help"},
		{[]string{codex.ID, "--via", "https://user:secret@proxy.test/v1"}, "--via: URL userinfo is forbidden", "aigw client edit --help"},
		{[]string{codex.ID, "--native-retries", "--request-retries", "1"}, "mutually exclusive", "aigw client edit --help"},
		{[]string{hermes.ID, "--stream-retries", "1"}, "Hermes has no native retry budgets; --request-retries, --stream-retries and --native-retries are for Codex", "aigw client edit --help"},
		{[]string{codex.ID, "--stream-retries", "1", "--stream-retries", "2"}, "this flag takes one value", ""},
		{[]string{codex.ID, "--request-retries", "9223372036854775808"}, "expected a whole number of retries", ""},
	} {
		err := executeAdapter(t, runtime, append([]string{"edit"}, test.args...)...)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%q = %v, want %q", test.args, err, test.want)
			continue
		}
		if fix := terminal.ProblemFor(err).Fix; test.next != "" && fix != test.next {
			t.Errorf("%q fix = %q, want %q", test.args, fix, test.next)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("%q repeats the URL's credential: %v", test.args, err)
		}
	}
}

func TestClientCommandsCompleteOneClient(t *testing.T) {
	runtime, _, _, _ := adapterRuntime(t, adapterConfig())
	complete := completeClient(runtime)
	completions, directive := complete(nil, nil, "")
	if len(completions) != 4 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completions = %q, %v", completions, directive)
	}
	if completions, _ := complete(nil, []string{codex.ID}, ""); len(completions) != 0 {
		t.Fatalf("a second client was offered: %q", completions)
	}
	var unset budget
	if unset.String() != "" || unset.Type() != "count" {
		t.Fatalf("an unset budget = %q, %q", unset.String(), unset.Type())
	}
}

// Enabling an enabled client again names the edit that sets its executable and
// configuration files, exactly when every word runs as written.
func TestEnableOfAnEnabledClientNamesTheEdit(t *testing.T) {
	runtime, _, target, _ := enabledCodex(t)
	next := filepath.Join(t.TempDir(), "codex-next")
	err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", next, "--target", target)
	if problem := terminal.ProblemFor(err); problem.Title != "Codex is already enabled, so its executable and configuration files change through client edit" ||
		problem.Fix != "aigw client edit "+codex.ID+" --executable "+next+" --target "+target {
		t.Fatalf("enable of an enabled client = %q, fix %q", problem.Title, problem.Fix)
	}
	codexSpec, _ := clients.Admission().Spec(codex.ID)
	if fix := editInstead(codexSpec, "/opt/codex next", nil); fix != "aigw client edit "+codex.ID+" --executable <path>" {
		t.Fatalf("a path with a space = %q", fix)
	}
}

// Enable stores the executable and configuration files as absolute paths, so
// they name the same files from any directory, and refuses a configuration
// file for a client that reads none, as edit does; Claude Desktop and Hermes
// read one.
func TestEnableStoresAbsolutePathsAndRefusesTargetsThatMeanNothing(t *testing.T) {
	runtime, _, secretStore, _ := adapterRuntime(t, adapterConfig())
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model = \"original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(codex.HomeDefault), ConfigPath: target, Present: true}}}}
	t.Chdir(filepath.Dir(target))
	if err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", filepath.Join("bin", "codex"), "--target", "config.toml"); err != nil {
		t.Fatal(err)
	}
	saved, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	executable, _ := filepath.Abs(filepath.Join("bin", "codex"))
	if binding := saved.Clients[codex.ID]; binding.Executable != executable || len(binding.Targets) != 1 || !filepath.IsAbs(binding.Targets[0]) ||
		filepath.Base(binding.Targets[0]) != "config.toml" {
		t.Fatalf("saved Codex = %#v, want %s and an absolute config.toml", binding, executable)
	}
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	err = executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/opt/claude", "--target", "settings.json")
	if problem := terminal.ProblemFor(err); problem.Title != "Claude has no configuration file to name; --target is for Claude Desktop, Codex and Hermes; run `aigw client enable --help`" ||
		problem.Fix != "aigw client enable --help" {
		t.Fatalf("enable of Claude Code with a target = %q, fix %q", problem.Title, problem.Fix)
	}
	if err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", "a.toml", "--target", "./a.toml"); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("a file named twice = %v", err)
	}
	if after, err := runtime.Config.CaptureSnapshot(); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("a refused enable wrote the configuration: %v", err)
	}
}

// Claude Desktop reads its configuration library and Hermes its configuration
// file from a target, so edit sets one for them.
func TestEditSetsTheTargetOfClaudeDesktopAndHermes(t *testing.T) {
	runtime, out, _, _ := adapterRuntime(t, adapterConfig())
	for _, client := range []string{claudedesktop.ID, hermes.ID} {
		target := filepath.Join(t.TempDir(), client)
		document := edited(t, runtime, out, client, "--target", target)
		if !document.Changed || len(document.Targets) != 1 || document.Targets[0] != target {
			t.Fatalf("%s edit = %#v", client, document)
		}
	}
}

// With the read-only environment backend, whose variable supplies the Token,
// an edit writes Codex's file and reads no Token value.
func TestEditWithTheEnvironmentBackend(t *testing.T) {
	runtime, out, target, _ := enabledCodex(t)
	runtime.Secrets = secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("gateway") {
			return "token"
		}
		return ""
	})
	document := edited(t, runtime, out, codex.ID, "--via", forwardingEndpoint)
	if !document.Changed || len(document.ClientFiles) != 1 {
		t.Fatalf("environment-backed edit = %#v", document)
	}
	if projected := readTarget(t, target); !strings.Contains(projected, `base_url = "`+forwardingEndpoint+`"`) {
		t.Fatalf("Codex's file does not forward:\n%s", projected)
	}
}

// An enabled client's configuration files do not move while its provider's
// Token is missing, because the files that the edit drops would keep AIGW's
// settings; the fix stores the Token, or sets the environment backend's
// variable.
func TestEditKeepsTheConfigurationFilesWhileTheTokenIsMissing(t *testing.T) {
	runtime, _, target, _ := enabledCodex(t)
	moved := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(moved, []byte("model = \"other\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		store secret.Store
		fix   string
	}{
		{secret.NewMemoryStore(), "aigw provider rotate gateway"},
		{secret.NewEnvironmentStore(func(string) string { return "" }), "Set " + secret.EnvironmentKey("gateway") + ", then run the same command again."},
		{unavailableAdapterSecretStore{}, "aigw provider show gateway"},
	} {
		runtime.Secrets = test.store
		err := executeAdapter(t, runtime, "edit", codex.ID, "--target", moved)
		if problem := terminal.ProblemFor(err); !strings.Contains(problem.Title, "Codex's configuration files cannot change") || problem.Impact != "Nothing changed." || problem.Fix != test.fix {
			t.Fatalf("%T: %q, impact %q, fix %q", test.store, problem.Title, problem.Impact, problem.Fix)
		}
	}
	if after, err := runtime.Config.CaptureSnapshot(); err != nil || !reflect.DeepEqual(before, after) || !strings.Contains(readTarget(t, target), "AIGW managed provider") {
		t.Fatalf("a refused move changed the configuration or the file: %v", err)
	}
}
