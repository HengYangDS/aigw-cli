package manifest_test

import (
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
)

func TestMigrateLegacyV2PreservesProxyEndpointAndCompactsRoutes(t *testing.T) {
	raw := []byte(`{
  "version": 2,
  "profiles": {
    "dmx": {
      "label": "DMXAPI",
      "base_url": "https://www.dmxapi.cn/v1",
      "adapters": {
        "claude": {"base_url": "https://www.dmxapi.cn"},
        "codex": {"base_url": "https://www.dmxapi.cn/v1"}
      },
      "proxy": {"codex_responses": true, "url": "http://127.0.0.1:8791/v1"}
    }
  },
  "routes": {"default": "dmx", "claude": "dmx", "codex": "dmx"}
}`)
	cfg, err := manifest.MigrateLegacyV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Profiles["dmx"]
	if p.Endpoints.OpenAIResponses != "http://127.0.0.1:8791/v1" || p.Endpoints.Anthropic != "https://www.dmxapi.cn" {
		t.Fatalf("profile = %#v", p)
	}
	if cfg.Routes.Default != "dmx" || len(cfg.Routes.Overrides) != 0 {
		t.Fatalf("routes = %#v", cfg.Routes)
	}
	if _, ok := cfg.Adapters[domain.ClientClaude]; ok {
		t.Fatalf("legacy adapter state must not be trusted: %#v", cfg.Adapters)
	}
}
