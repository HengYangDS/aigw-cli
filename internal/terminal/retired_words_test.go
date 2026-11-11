package terminal

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// retired matches the implementation's nouns that provider, model and client
// replaced, which no failure text names.
var retired = regexp.MustCompile(`(?i)\b(accounts?|routes?|projections?|projected|bindings?)\b`)

// Every explanation that terminal gives of a domain error, and of the other
// failures that it recognizes by type, speaks the product's vocabulary:
// Problem, Evidence, Impact and Fix name no retired word.
func TestEveryExplanationOfADomainErrorSpeaksTheProductVocabulary(t *testing.T) {
	ref := domain.NewModelRef("team", "m")
	for name, err := range map[string]error{
		"an unselected client":     &domain.RuntimeBindingUnselectedError{Client: "codex"},
		"an unsupported version":   &domain.UnsupportedConfigVersionError{Version: 3, ExpectedVersion: 8},
		"an unknown provider":      &domain.RuntimeModelUnknownProviderError{Model: ref, ProviderID: "ghost"},
		"a missing endpoint":       &domain.RuntimeMissingEndpointError{ProviderID: "team", Protocol: domain.ProtocolChat},
		"a team file version":      &domain.UnsupportedTeamFileVersionError{Version: 7},
		"an entry":                 domain.NewEntryError("provider `team`", errors.New("provider \"team\" has an empty label")),
		"an unverified reader":     fmt.Errorf("read: %w", secret.ErrNativeReaderUnverified),
		"a model without protocol": validationError(t),
	} {
		t.Run(name, func(t *testing.T) {
			problem := ProblemFor(err)
			text := strings.Join([]string{problem.Title, problem.Evidence, problem.Impact, problem.Fix, err.Error()}, "\n")
			if word := retired.FindString(text); word != "" {
				t.Fatalf("the explanation names %q:\n%s", word, text)
			}
		})
	}
}

// validationError is the refusal of a configuration with every kind of
// provider, model and client fault that Validate reports.
func validationError(t *testing.T) error {
	t.Helper()
	cfg := domain.NewConfig(domain.NewAdmission())
	cfg.Version = domain.ConfigVersion
	if err := cfg.AppendProvider("team", domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Responses: "https://team.test/v1"}}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AppendModel(domain.Model{Provider: "team", ID: "bare"}); err != nil {
		t.Fatal(err)
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("the configuration was accepted")
	}
	return err
}
