package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
)

func TestHermesSetupDeferredSyncCredentialCheckAndWithdrawal(t *testing.T) {
	app, output, credentials, runner, httpClient := testApp(t, "")
	root := t.TempDir()
	manifest := filepath.Join(root, "team.toml")
	writeFile(t, manifest, []byte(`version = 7
[recommendations.hermes.primary]
route = "team-model"
protocol = "anthropic"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://provider.test"
[models.model-test]
label = "Model Test"
[routes.team-model]
label = "Team Model"
account = "team"
model = "model-test"
upstream_model = "model-test"
interfaces = { anthropic = ["text", "streaming", "tools"] }
`), 0o600)
	if err := credentials.Set("team", "public-fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"setup", "--from", manifest}); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Clients[configuration.ClientHermes].Enabled || cfg.SelectedRoute(configuration.ClientHermes) != "team-model" {
		t.Fatal("deferred setup lost selected or enabled intent")
	}
	target := filepath.Join(root, "hermes", "config.yaml")
	executable := executableFixture(t, "hermes")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientHermes: executable},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", Product: "Hermes", Authority: "aigw", ConfigPath: target}},
	}}
	for _, args := range [][]string{{"sync"}, {"status", "--json"}, {"check"}, {"test", "--for", "hermes"}, {"verify", "--for", "hermes"}} {
		output.Reset()
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output.String())
		}
		if strings.Contains(output.String(), "public-fixture-token") {
			t.Fatal("public command exposed its fixture credential")
		}
	}
	if len(runner.plans) != 2 {
		t.Fatalf("native verification invocation count = %d", len(runner.plans))
	}
	if httpClient.headers.Get("X-Api-Key") != "public-fixture-token" || httpClient.headers.Get("Authorization") != "" {
		t.Fatalf("wrong selected protocol headers: %v", httpClient.headers)
	}
	before := readFile(t, target)
	if !bytes.Contains(before, []byte("key_cmd:")) || bytes.Contains(before, []byte("public-fixture-token")) {
		t.Fatal("Hermes did not receive a secret-free native helper projection")
	}
	if err := cli.Execute(app, []string{"client", "disable", "hermes"}); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{target, target + ".aigw-state.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("withdrawal left %s: %v", path, err)
		}
	}
}
