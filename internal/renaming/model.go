// Package renaming owns account and profile identity migration.
package renaming

import (
	"net/http"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
)

// HTTPDoer is the minimal transport required to verify a renamed account's diagnostic capability.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Service owns identity migration, credential preparation, and verified finalization.
type Service struct {
	Config       configuration.Store
	Secrets      secrets.Store
	Accounts     secrets.DiagnosticCredentialStore
	HTTP         HTTPDoer
	Synchronizer synchronization.Synchronizer
}

// Resource identifies the configuration entity changed by a rename plan.
type Resource string

const (
	ResourceAccount Resource = "account"
	ResourceProfile Resource = "profile"
)

// Status identifies the lifecycle state of a rename plan.
type Status string

const (
	StatusPlanned          Status = "planned"
	StatusBlocked          Status = "blocked"
	StatusApplied          Status = "applied"
	StatusAlreadyFinalized Status = "already-finalized"
	StatusFinalized        Status = "finalized"
)

// Actions describes every configuration, credential, and backup effect of a rename.
type Actions struct {
	Configuration string `json:"configuration"`
	APIToken      string `json:"api_token"`
	AccountProbe  string `json:"account_probe"`
	Backup        string `json:"backup"`
}

// Plan is the complete reviewable rename transaction, including affected references and deferred effects.
type Plan struct {
	Resource           Resource `json:"resource"`
	OldID              string   `json:"old_id"`
	NewID              string   `json:"new_id"`
	Status             Status   `json:"status"`
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
	snapshot             configuration.Snapshot
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
	value secrets.DiagnosticCredential
	copy  bool
}

// FinalizeOptions records explicit authorization for credential rotations discovered during planning.
type FinalizeOptions struct {
	ConfirmAPITokenRotation     bool
	ConfirmAccountProbeRotation bool
}
