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

// Renamer owns identity migration, credential preparation, and verified finalization.
type Renamer struct {
	Config       configuration.Store
	Secrets      secrets.Store
	Accounts     secrets.DiagnosticCredentialStore
	HTTP         HTTPDoer
	Synchronizer synchronization.Synchronizer
}

// Resource identifies the configuration entity changed by a rename plan.
type Resource string

const (
	// ResourceAccount identifies an Account rename.
	ResourceAccount Resource = "account"
	// ResourceProfile identifies a Profile rename.
	ResourceProfile Resource = "profile"
)

// Status identifies the lifecycle state of a rename plan.
type Status string

const (
	// StatusPlanned identifies a mutation plan that has not been applied.
	StatusPlanned Status = "planned"
	// StatusBlocked identifies a plan that cannot proceed without an explicit prerequisite.
	StatusBlocked Status = "blocked"
	// StatusApplied identifies a completed rename whose old credential slots remain available.
	StatusApplied Status = "applied"
	// StatusAlreadyFinalized identifies an idempotent finalization with no remaining old state.
	StatusAlreadyFinalized Status = "already-finalized"
	// StatusFinalized identifies a completed removal of verified old credential slots.
	StatusFinalized Status = "finalized"
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
	Profile              configuration.Route   `json:"-"`
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
