package domain

import (
	"errors"
	"strings"
	"testing"
)

// The destination names the upstream as openai-responses-proxy 8.0.0's
// grammar does, with one spelling for each upstream: the four providers in
// use, a port, a default port, an upper-case host and a trailing slash.
func TestForwardingDestinationComposesTheUpstream(t *testing.T) {
	const root = "http://127.0.0.1:8792"
	for upstream, want := range map[string]string{
		"https://www.dmxapi.cn/v1":          root + "/https/www.dmxapi.cn/v1",
		"https://suheai.cn/v1":              root + "/https/suheai.cn/v1",
		"https://api.modelverse.cn/v1":      root + "/https/api.modelverse.cn/v1",
		"https://api.inferera.com/v1":       root + "/https/api.inferera.com/v1",
		"https://api.example.com:8443/v1":   root + "/https/api.example.com:8443/v1",
		"https://API.Example.com:443/v1/":   root + "/https/api.example.com/v1",
		"https://api.example.com/v1/":       root + "/https/api.example.com/v1",
		"https://api.example.com":           root + "/https/api.example.com",
		"https://api.example.com/":          root + "/https/api.example.com",
		"https://api.example.com/openai/v1": root + "/https/api.example.com/openai/v1",
		"http://127.0.0.1:48721/v1":         root + "/http/127.0.0.1:48721/v1",
		"http://LOCALHOST:80/v1":            root + "/http/localhost/v1",
		"http://[::1]:9000/v1":              root + "/http/[::1]:9000/v1",
		"HTTPS://Gateway.Example.com/x/v1/": root + "/https/gateway.example.com/x/v1",
	} {
		got, err := forwardingDestination(root+"/", upstream)
		if err != nil || got != want {
			t.Errorf("forwardingDestination(%q) = %q, %v, want %q", upstream, got, err, want)
		}
	}
}

// An upstream that the grammar cannot name is refused, and the refusal names
// no part of the URL.
func TestForwardingDestinationRefusesWhatTheGrammarCannotName(t *testing.T) {
	for name, upstream := range map[string]string{
		"user information":           "https://user:secret@api.example.com/v1",
		"a query":                    "https://api.example.com/v1?region=east",
		"an empty query":             "https://api.example.com/v1?",
		"a fragment":                 "https://api.example.com/v1#part",
		"an empty fragment":          "https://api.example.com/v1#",
		"percent-encoding":           "https://api.example.com/v%31",
		"an encoded slash":           "https://api.example.com/a%2Fb/v1",
		"an empty segment":           "https://api.example.com//v1",
		"an empty segment inside":    "https://api.example.com/v1//responses",
		"a dot segment":              "https://api.example.com/./v1",
		"a dot-dot segment":          "https://api.example.com/v1/../v2",
		"plain HTTP to a remote":     "http://api.example.com/v1",
		"plain HTTP to 127.0.0.2":    "http://127.0.0.2:8000/v1",
		"another scheme":             "ftp://api.example.com/v1",
		"no host":                    "https:///v1",
		"not a URL":                  "api.example.com/v1",
		"an IPv6 zone":               "http://[::1%25lo0]:9000/v1",
		"a percent-encoded hostname": "https://api%2Eexample.com/v1",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := forwardingDestination("http://127.0.0.1:8792", upstream)
			if err == nil || got != "" {
				t.Fatalf("forwardingDestination(%q) = %q, %v", upstream, got, err)
			}
			for _, part := range []string{"secret", "region", "part", "example"} {
				if strings.Contains(err.Error(), part) {
					t.Fatalf("the refusal %q names %q", err, part)
				}
			}
		})
	}
}

// Resolution, validation and a followed choice use the one composition: a
// forwarded client whose provider's endpoint the grammar cannot name does
// not resolve, is refused with the step that connects it directly, and a
// model chosen in the client is followed without the forwarding.
func TestForwardingRefusesAnUpstreamOutsideTheGrammar(t *testing.T) {
	cfg := validConfig()
	if err := cfg.SetForwardingRoot(ClientCodex, "http://127.0.0.1:8792"); err != nil {
		t.Fatal(err)
	}
	backup := cfg.Providers["backup"]
	backup.Endpoints.Responses = "https://backup.test/v1?region=east"
	cfg.Providers["backup"] = backup
	if _, err := cfg.ResolveRuntime(ClientCodex, ModelRef{}); err == nil {
		t.Fatal("a forwarded upstream outside the grammar resolved")
	}
	err := cfg.Validate()
	refused, ok := errors.AsType[*ForwardingUpstreamError](err)
	if !ok || refused.Client != ClientCodex || refused.Provider != "backup" || !strings.Contains(err.Error(), "carries a query") ||
		!strings.Contains(err.Error(), "`aigw client edit codex --direct`") || strings.Contains(err.Error(), "region") {
		t.Fatalf("validation error = %v", err)
	}
	cfg.FollowSelectedModel(ClientCodex, NewModelRef("backup", "gpt-test"), "")
	if cfg.Clients[ClientCodex].ForwardingRoot != "" || cfg.Validate() != nil {
		t.Fatalf("a followed choice kept a forwarding that cannot carry it: %#v, %v", cfg.Clients[ClientCodex], cfg.Validate())
	}
}
