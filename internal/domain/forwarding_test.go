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
		"https://api.example.com:0443/v1":   root + "/https/api.example.com/v1",
		"https://api.example.com:/v1":       root + "/https/api.example.com/v1",
		"https://[2001:DB8::1]:8443/v1":     root + "/https/[2001:db8::1]:8443/v1",
		"https://api_1.example.com/a-b/c~d": root + "/https/api_1.example.com/a-b/c~d",
	} {
		got, err := forwardingDestination(root+"/", upstream)
		if err != nil || got != want {
			t.Errorf("forwardingDestination(%q) = %q, %v, want %q", upstream, got, err, want)
		}
	}
}

// An upstream that the grammar cannot name is refused, and the refusal names
// no part of the URL. As openai-responses-proxy 8.0.0 does, the grammar takes
// a host and path segments only in their URL-safe form, ASCII letters,
// digits and "-._~", so a non-ASCII host is refused whether the URL writes it
// raw or percent-encoded; it refuses an empty segment anywhere in the path;
// and it admits plain HTTP only to 127.0.0.1, localhost and ::1 as spelled.
func TestForwardingDestinationRefusesWhatTheGrammarCannotName(t *testing.T) {
	for name, upstream := range map[string]string{
		"user information":            "https://user:secret@api.example.com/v1",
		"a query":                     "https://api.example.com/v1?region=east",
		"an empty query":              "https://api.example.com/v1?",
		"a fragment":                  "https://api.example.com/v1#part",
		"an empty fragment":           "https://api.example.com/v1#",
		"percent-encoding":            "https://api.example.com/v%31",
		"an encoded slash":            "https://api.example.com/a%2Fb/v1",
		"an empty segment":            "https://api.example.com//v1",
		"an empty segment inside":     "https://api.example.com/v1//responses",
		"a dot segment":               "https://api.example.com/./v1",
		"a dot-dot segment":           "https://api.example.com/v1/../v2",
		"plain HTTP to a remote":      "http://api.example.com/v1",
		"plain HTTP to 127.0.0.2":     "http://127.0.0.2:8000/v1",
		"another scheme":              "ftp://api.example.com/v1",
		"no host":                     "https:///v1",
		"not a URL":                   "api.example.com/v1",
		"an IPv6 zone":                "http://[::1%25lo0]:9000/v1",
		"a percent-encoded hostname":  "https://api%2Eexample.com/v1",
		"a non-ASCII host":            "https://api.exämple.com/v1",
		"a percent-encoded non-ASCII": "https://api.ex%C3%A4mple.com/v1",
		"a reserved host character":   "https://api!.example.com/v1",
		"an empty host":               "https://:8443/v1",
		"a port above 65535":          "https://api.example.com:65536/v1",
		"plain HTTP to another ::1":   "http://[0:0:0:0:0:0:0:1]:9000/v1",
		"an empty path":               "https://api.example.com//",
		"an empty segment at the end": "https://api.example.com/v1//",
		"a reserved path character":   "https://api.example.com/v1;x",
		"a colon in a segment":        "https://api.example.com/v1:x",
		"a non-ASCII segment":         "https://api.example.com/vä",
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

// The rules above are about the selection. Another provider of a forwarded
// client, which Hermes lists beside the selection and which a check may
// name, is reached through the root when the grammar names its Responses
// endpoint and directly otherwise, and refuses nothing: validation passes,
// and resolving its model, as Hermes's catalog and a check do, succeeds.
func TestForwardingReachesAnotherProviderDirectlyWhenTheGrammarCannotNameIt(t *testing.T) {
	cfg := validConfig()
	if err := cfg.SetForwardingRoot(ClientCodex, "http://127.0.0.1:8792"); err != nil {
		t.Fatal(err)
	}
	const outside = "https://azure.test/openai/v1?api-version=preview"
	cfg.Providers["azure"] = Provider{Label: "Azure", Endpoints: Endpoints{Responses: outside}}
	cfg.Providers["named"] = Provider{Label: "Named", Endpoints: Endpoints{Responses: "https://named.test/v1"}}
	cfg.Models[NewModelRef("azure", "gpt-azure")] = testModel("Azure", "azure", "gpt-azure", ProtocolResponses)
	cfg.Models[NewModelRef("named", "gpt-named")] = testModel("Named", "named", "gpt-named", ProtocolResponses)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("another provider outside the grammar refused the configuration: %v", err)
	}
	for ref, want := range map[ModelRef]string{
		NewModelRef("azure", "gpt-azure"): outside,
		NewModelRef("named", "gpt-named"): "http://127.0.0.1:8792/https/named.test/v1",
		NewModelRef("backup", "gpt-test"): "http://127.0.0.1:8792/https/backup.test/v1",
	} {
		listed, err := cfg.ResolveModelProtocol(ClientCodex, ref, ProtocolResponses)
		if err != nil || listed.Endpoint != want || listed.UpstreamEndpoint != cfg.Providers[ref.Provider()].Endpoints.Responses {
			t.Errorf("the listed entry of %s = %q (upstream %q), %v; want %q", ref, listed.Endpoint, listed.UpstreamEndpoint, err, want)
		}
		checked, err := cfg.ResolveRuntime(ClientCodex, ref)
		if err != nil || checked.Endpoint != want {
			t.Errorf("a check of %s reaches %q, %v; want %q", ref, checked.Endpoint, err, want)
		}
	}
}
