package synchronization

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestServiceCreationAdmitsStateBeforeTokenAcquisition(t *testing.T) {
	for _, phase := range []string{"cancelled", "profile collision", "account collision", "invalid configuration"} {
		t.Run(phase, func(t *testing.T) {
			before := setupConfiguration()
			original := before.Clone()
			account := configuration.Account{Label: "New", Endpoints: configuration.Endpoints{Anthropic: "https://new.test"}}
			profile := configuration.Profile{Label: "New", Client: configuration.ClientClaude, Model: "new-model"}
			name := "new"
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch phase {
			case "cancelled":
				cancel()
			case "profile collision":
				name = "claude"
			case "account collision":
				name = "team"
			case "invalid configuration":
				account.Endpoints.Anthropic = "invalid-endpoint"
			}
			acquired := false
			err := (Synchronizer{}).CreateService(ctx, before, name, account, profile, func() (string, error) {
				acquired = true
				return "token", nil
			})
			if err == nil || acquired || !reflect.DeepEqual(before, original) {
				t.Fatalf("creation admission = %v, token acquired = %v", err, acquired)
			}
		})
	}
}

func TestServiceCreationPreservesStateWhenTokenAcquisitionFails(t *testing.T) {
	for _, phase := range []string{"input failure", "cancelled during input", "empty token"} {
		t.Run(phase, func(t *testing.T) {
			before := setupConfiguration()
			store := &configStoreStub{}
			credentials := &setupCredentials{Store: secrets.NewMemoryStore()}
			syncer := Synchronizer{Config: store, Secrets: credentials, Discovery: setupDiscovery(nil)}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			failure := errors.New("input unavailable")
			err := syncer.CreateService(ctx, before, "new", before.Accounts["team"], before.Profiles["claude"], func() (string, error) {
				switch phase {
				case "input failure":
					return "", failure
				case "cancelled during input":
					cancel()
					return "token", nil
				default:
					return "", nil
				}
			})
			if err == nil || store.commits != 0 || credentials.writes != 0 {
				t.Fatalf("failed acquisition = %v, commits = %d, credential writes = %d", err, store.commits, credentials.writes)
			}
			if phase == "input failure" && !errors.Is(err, failure) {
				t.Fatalf("input failure lost: %v", err)
			}
		})
	}
}
