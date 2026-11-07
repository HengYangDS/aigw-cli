package provider

import (
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
)

// The JSON documents of the provider subcommands. Each one that replaces the
// documents of earlier commands continues from their highest schema_version,
// 1, plus one: list replaces `aigw account list`, add `aigw add`, edit
// `aigw account edit` and `aigw account diagnostics`, rotate `aigw rotate` and
// balance `aigw balance`. show and remove have no predecessor.
const (
	listSchemaVersion    = 2
	showSchemaVersion    = 1
	addSchemaVersion     = 2
	editSchemaVersion    = 2
	rotateSchemaVersion  = 2
	balanceSchemaVersion = 2
	removeSchemaVersion  = 1
)

// The states of a credential slot, which a command observes without reading
// the credential: the backend holds one, holds none, or could not tell.
const (
	credentialPresent    = "present"
	credentialAbsent     = "absent"
	credentialUnobserved = "unobserved"
)

// listDocument is `aigw provider list`'s JSON document: the providers in
// configuration order.
type listDocument struct {
	Providers []providerItem `json:"providers"`
}

// providerItem is one provider as list and show report it.
type providerItem struct {
	ID        string         `json:"id"`
	Label     string         `json:"label"`
	Endpoints []endpointItem `json:"endpoints"`
	// BalanceProbe names the kind of the provider's balance probe.
	BalanceProbe string   `json:"balance_probe,omitempty"`
	Models       []string `json:"models"`
}

// endpointItem is the provider's endpoint for one protocol.
type endpointItem struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
}

// showDocument is `aigw provider show`'s JSON document: the provider, the
// clients that select one of its models, and the state of its Token and, when
// it has a balance probe, of its balance credentials.
type showDocument struct {
	providerItem
	SelectedBy        []string          `json:"selected_by"`
	Token             string            `json:"token"`
	BalanceCredential string            `json:"balance_credential,omitempty"`
	CredentialBackend credentialBackend `json:"credential_backend"`
}

// credentialBackend is the credential backend that this invocation selected,
// as it reports itself without a credential being read: its kind, whether it
// answers, and whether it stores credentials or only reads them.
type credentialBackend struct {
	Kind         string `json:"kind"`
	Availability string `json:"availability"`
	Mutability   string `json:"mutability"`
}

// changeDocument is the JSON document of `aigw provider add` and `edit`: the
// provider after the change, what the change does beyond the configuration,
// and in a preview that it was not applied.
type changeDocument struct {
	DryRun bool `json:"dry_run,omitempty"`
	providerItem
	// TokenValidated and TokenStored report add's Token: a preview reads
	// none, and the environment backend supplies one that add validates and
	// does not store.
	TokenValidated *bool `json:"token_validated,omitempty"`
	TokenStored    *bool `json:"token_stored,omitempty"`
	// ClearedCapabilities and ClearedForwarding report what an edit of an
	// endpoint clears, because it was qualified or set up for the old one,
	// and KeptRetryBudgets the clients whose retry budgets stay, because they
	// belong to the provider.
	ClearedCapabilities []clearedCapability `json:"cleared_capabilities,omitempty"`
	ClearedForwarding   []string            `json:"cleared_forwarding,omitempty"`
	KeptRetryBudgets    []string            `json:"kept_retry_budgets,omitempty"`
	// BalanceCredential is stored or removed when an edit changes it.
	BalanceCredential string                        `json:"balance_credential,omitempty"`
	ClientFiles       []invocation.ClientFile       `json:"client_files"`
	CredentialReader  *app.CredentialEntrypointPlan `json:"credential_reader,omitempty"`
	NextAction        string                        `json:"next_action,omitempty"`
	NextDetail        string                        `json:"next_detail,omitempty"`
}

// clearedCapability is the capabilities of one model on one protocol that an
// endpoint change cleared.
type clearedCapability struct {
	Model        string   `json:"model"`
	Protocol     string   `json:"protocol"`
	Capabilities []string `json:"capabilities"`
}

// rotateDocument is `aigw provider rotate`'s JSON document.
type rotateDocument struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	TokenValidated bool   `json:"token_validated"`
	TokenStored    bool   `json:"token_stored"`
	NextAction     string `json:"next_action,omitempty"`
	NextDetail     string `json:"next_detail,omitempty"`
}

// balanceDocument is `aigw provider balance`'s JSON document: each provider
// that it reports, in the order asked for, and the first failure's fix.
type balanceDocument struct {
	Providers  []balanceItem `json:"providers"`
	NextAction string        `json:"next_action,omitempty"`
	NextDetail string        `json:"next_detail,omitempty"`
}

// balanceItem is one provider's balance report: reported, no_probe or failed.
type balanceItem struct {
	ID      string        `json:"id"`
	Label   string        `json:"label"`
	State   string        `json:"state"`
	Detail  string        `json:"detail,omitempty"`
	Balance *float64      `json:"balance,omitempty"`
	Token   *balanceToken `json:"token,omitempty"`
	// Problem and the next step say why a probe failed and what fixes it.
	Problem    string `json:"problem,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// balanceToken is the provider's quota report for the provider's Token,
// which names the Token only by its name at the provider.
type balanceToken struct {
	Name              string  `json:"name"`
	State             string  `json:"state"`
	Used              float64 `json:"used"`
	Remaining         float64 `json:"remaining"`
	UnlimitedQuota    bool    `json:"unlimited_quota"`
	RemainingRequests int64   `json:"remaining_requests"`
	UnlimitedRequests bool    `json:"unlimited_requests"`
}

// removeDocument is `aigw provider remove`'s JSON document: the provider and
// everything that its removal deletes or drops, as planned, done or blocked.
type removeDocument struct {
	DryRun bool     `json:"dry_run,omitempty"`
	Status string   `json:"status"`
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Models []string `json:"models"`
	// Token and BalanceCredential are the states of the provider's credential
	// slots before the removal, which deletes a present one. With the
	// read-only environment backend, Unset names each of the provider's
	// variables that is still set, which the user removes.
	Token             string                        `json:"token"`
	BalanceCredential string                        `json:"balance_credential"`
	Unset             []string                      `json:"unset,omitempty"`
	Recommendations   []string                      `json:"recommendations"`
	ClientFiles       []invocation.ClientFile       `json:"client_files"`
	CredentialReader  *app.CredentialEntrypointPlan `json:"credential_reader,omitempty"`
	Problem           string                        `json:"problem,omitempty"`
	NextAction        string                        `json:"next_action,omitempty"`
	NextDetail        string                        `json:"next_detail,omitempty"`
}
