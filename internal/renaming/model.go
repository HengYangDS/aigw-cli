// Package renaming owns account and profile identity migration.
package renaming

import (
	"io"
	"net/http"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
type Prompter interface {
	Secret(string) (string, error)
	Text(string) (string, error)
	Select(string, []prompt.Choice) (string, error)
}
type Dependencies struct {
	Config       configuration.Store
	Secrets      secrets.Store
	Accounts     account.Store
	Out          io.Writer
	Color        bool
	Width        int
	Interactive  bool
	Prompt       Prompter
	HTTP         HTTPDoer
	Synchronizer synchronization.Synchronizer
}
type Actions struct {
	Configuration  string `json:"configuration"`
	APIToken       string `json:"api_token"`
	AccountProbe   string `json:"account_probe"`
	Authentication string `json:"authentication"`
	Backup         string `json:"backup"`
}

type Plan struct {
	Resource           string   `json:"resource"`
	OldID              string   `json:"old_id"`
	NewID              string   `json:"new_id"`
	Status             string   `json:"status"`
	AffectedReferences []string `json:"affected_references"`
	Actions            Actions  `json:"actions"`
	ExternalTODOs      []string `json:"external_todos"`

	Config               configuration.Config  `json:"-"`
	Profile              configuration.Profile `json:"-"`
	Account              configuration.Account `json:"-"`
	tokenCopy            tokenCopy             `json:"-"`
	probeCopy            probeCopy             `json:"-"`
	blockedReason        string                `json:"-"`
	Finalize             bool                  `json:"-"`
	snapshot             configuration.VerifiedBackupSnapshot
	deleteToken          bool `json:"-"`
	deleteProbe          bool `json:"-"`
	verifyProbe          bool `json:"-"`
	externalTokenCleanup bool `json:"-"`
}

type tokenCopy struct {
	value string
	copy  bool
}

type probeCopy struct {
	value account.Credential
	copy  bool
}

type FinalizeOptions struct {
	ConfirmAPITokenRotation     bool
	ConfirmAccountProbeRotation bool
}
