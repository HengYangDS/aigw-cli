package cli_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
)

func TestRouteListGroupsServicesAndPreservesStoredBindings(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	endpoints := configuration.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1", OpenAIChatCompletions: "https://team.test/v1"}
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Order: 10, Endpoints: endpoints}
	cfg.Models["strong"] = configuration.Model{Label: "Strong", Order: 10}
	cfg.Models["weak"] = configuration.Model{Label: "Weak", Order: 20}
	for _, row := range []struct {
		id, model string
		protocol  configuration.EndpointProtocol
	}{
		{"a-chat", "strong", configuration.ProtocolOpenAIChatCompletions},
		{"b-responses-weak", "weak", configuration.ProtocolOpenAIResponses},
		{"c-native-weak", "weak", configuration.ProtocolAnthropic},
		{"d-responses-strong", "strong", configuration.ProtocolOpenAIResponses},
		{"e-native-strong", "strong", configuration.ProtocolAnthropic},
	} {
		cfg.Routes[row.id] = qualifiedRoute(row.id, "team", row.model, row.protocol)
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "d-responses-strong", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"e-native-strong", "c-native-weak", "d-responses-strong", "b-responses-weak", "a-chat"}
	for _, jsonMode := range []bool{false, true} {
		out.Reset()
		args := []string{"route", "list"}
		if jsonMode {
			args = append(args, "--json")
		}
		if err := cli.Execute(app, args); err != nil {
			t.Fatal(err)
		}
		var ids []string
		if jsonMode {
			var result struct {
				Routes []struct {
					ID string `json:"id"`
				} `json:"routes"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			for _, row := range result.Routes {
				ids = append(ids, row.ID)
			}
		} else {
			for line := range strings.SplitSeq(out.String(), "\n") {
				if fields := strings.Fields(line); len(fields) == 3 && fields[1] == "Route" {
					ids = append(ids, fields[2])
				}
			}
		}
		if !slices.Equal(ids, want) {
			t.Fatalf("JSON=%v Route order = %v, want %v", jsonMode, ids, want)
		}
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("listing changed stored configuration or bindings: %v", err)
	}
}
