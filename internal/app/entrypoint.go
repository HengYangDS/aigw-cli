package app

import (
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/domain"
)

// CredentialEntrypointPlan names the credential reader that a synchronization
// would install, and how.
type CredentialEntrypointPlan struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// PlanCredentialEntrypoint returns the credential reader that synchronizing to
// after would install, or nil when the installed reader is current. Named
// clients limit the plan to their Accounts without checking their Tokens;
// without any, every client whose Token is stored counts.
func PlanCredentialEntrypoint(synchronizer synchronization.Synchronizer, after domain.Config, clientIDs ...string) (*CredentialEntrypointPlan, error) {
	action, err := synchronizer.CredentialEntrypointPlan(after, clientIDs...)
	if err != nil || action == synchronization.CredentialEntrypointUnchanged {
		return nil, err
	}
	path, err := synchronizer.CredentialEntrypointPath()
	if err != nil {
		return nil, err
	}
	return &CredentialEntrypointPlan{Path: path, Action: string(action)}, nil
}
