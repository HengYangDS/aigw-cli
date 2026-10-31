package domain

import (
	"strings"
	"testing"
)

func TestValidateAllowsExplicitLoopbackDevelopmentAccount(t *testing.T) {
	cfg := validConfig()
	a := cfg.Accounts["dmx"]
	a.Endpoints.OpenAIResponses = "http://127.0.0.1:18765/v1"
	cfg.Accounts["dmx"] = a
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEndpointHTTPAdmissionUsesLoopbackIdentity(t *testing.T) {
	for _, test := range []struct {
		host    string
		allowed bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"127.0.0.1", true},
		{"127.1.2.3", true},
		{"[::1]", true},
		{"[::ffff:127.0.0.2]", true},
		{"192.168.1.1", false},
		{"0.0.0.0", false},
		{"[::]", false},
		{"localhost.example.test", false},
		{"[::ffff:192.168.1.1]", false},
	} {
		t.Run(test.host, func(t *testing.T) {
			if err := validateEndpoint("http://" + test.host + ":4567/v1"); (err == nil) != test.allowed {
				t.Errorf("endpoint admission: error=%v, allowed=%t", err, test.allowed)
			}
		})
	}
}

func TestValidateEndpointRejectsMalformedURL(t *testing.T) {
	cfg := validConfig()
	a := cfg.Accounts["dmx"]
	a.Endpoints.Anthropic = "://not-a-valid-url"
	cfg.Accounts["dmx"] = a
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "must use http or https") {
		t.Fatalf("malformed URL error = %v", err)
	}
}
