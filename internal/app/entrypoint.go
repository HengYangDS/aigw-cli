package app

import (
	"aigw-cli/internal/domain"
	"aigw-cli/internal/synchronization"
)

// CredentialEntrypointPlan names the credential reader that a synchronization
// would install, and how.
type CredentialEntrypointPlan struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// PlanCredentialEntrypoint returns the credential reader that synchronizing to
// after would install, or nil when the installed reader is current.
func PlanCredentialEntrypoint(synchronizer synchronization.Synchronizer, after domain.Config) (*CredentialEntrypointPlan, error) {
	action, err := synchronizer.CredentialEntrypointPlan(after)
	if err != nil || action == synchronization.CredentialEntrypointUnchanged {
		return nil, err
	}
	path, err := synchronizer.CredentialEntrypointPath()
	if err != nil {
		return nil, err
	}
	return &CredentialEntrypointPlan{Path: path, Action: string(action)}, nil
}
