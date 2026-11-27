package cli_test

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// The forwarding refuses only the selection that it cannot carry. A forwarded
// Hermes lists the models of every provider, and another provider whose
// Responses endpoint the grammar cannot name, here one with an `api-version`
// query, is reached directly: forwarding Hermes succeeds and writes its file,
// a model added to that provider while Hermes forwards is listed, and a check
// of that provider's model probes its own endpoint, where a check of a model
// of a provider that the grammar names goes through the root.
func TestAForwardedHermesReachesAnotherProviderOutsideTheGrammarDirectly(t *testing.T) {
	t.Parallel()
	const (
		root    = "http://127.0.0.1:8792"
		outside = "https://azure.test/openai/v1?api-version=preview"
	)
	app, output, httpClient, target := shippedHermesApp(t)
	app.In = strings.NewReader("fixture-token\n")
	for _, args := range [][]string{
		{"provider", "add", "azure", "--endpoint", "responses=" + outside, "--token-stdin"},
		{"model", "add", "azure/gpt-azure", "--unlisted", "--protocol", "responses"},
		{"client", "edit", "hermes", "--via", root},
		{"model", "add", "azure/gpt-azure-next", "--unlisted", "--protocol", "responses"},
	} {
		if err := selRun(t, app, output, args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	entries := hermesProviderEntries(t, target)
	if entry := entries["aigw-dmxapi-responses"]; entry.BaseURL != root+"/https/www.dmxapi.cn/v1" {
		t.Errorf("the selected provider is listed at %q, not through the root", entry.BaseURL)
	}
	entry := entries["aigw-azure-responses"]
	if _, listed := entry.Models["gpt-azure-next"]; entry.BaseURL != outside || !listed {
		t.Errorf("the provider outside the grammar is listed at %q with %v; want its own endpoint and the added model", entry.BaseURL, entry.Models)
	}

	for model, want := range map[string]string{"azure/gpt-azure": "https://azure.test", "ucloud/gpt-6.2-astra": root + "/https/api.modelverse.cn/v1"} {
		var probed []string
		httpClient.handler = func(req *http.Request) (*http.Response, error) {
			probed = append(probed, req.URL.String())
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(fixtureResponseBody(req))), Request: req}, nil
		}
		if err := selRun(t, app, output, "check", "hermes", "--model", model, "--depth", "endpoint"); err != nil {
			t.Fatalf("check of %s: %v\n%s", model, err, output)
		}
		if len(probed) == 0 || slices.ContainsFunc(probed, func(address string) bool { return !strings.HasPrefix(address, want+"/") }) {
			t.Errorf("the check of %s probed %q; want only addresses under %s", model, probed, want)
		}
	}
}

// hermesEntry is one provider entry of Hermes's file: its address and models.
type hermesEntry struct {
	BaseURL string         `yaml:"base_url"`
	Models  map[string]any `yaml:"models"`
}

// hermesProviderEntries reads the provider entries of Hermes's file.
func hermesProviderEntries(t *testing.T, target string) map[string]hermesEntry {
	t.Helper()
	var document struct {
		Providers map[string]hermesEntry `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(t, target), &document); err != nil {
		t.Fatal(err)
	}
	return document.Providers
}
