package check

import (
	"maps"
	"slices"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// The states of the listing of stored Tokens that no provider uses.
const (
	unusedListed     = "listed"
	unusedCannotList = "cannot_list"
)

// unusedReport is the listing of the stored Tokens that no provider uses:
// their providers' IDs, never a value, or that the backend cannot name its
// Tokens, which it says rather than asks for access.
type unusedReport struct {
	State     string        `json:"state"`
	Providers []unusedToken `json:"providers"`
}

// The fixes that reuse a stored Token. After an upgrade, the Tokens that no
// provider uses belong to the providers of the earlier release, and setup
// adds such a provider again and uses its Token without a prompt.
const (
	reuseEmpty      = "`aigw setup` adds the provider again and uses its stored Token without a prompt"
	reuseConfigured = "`aigw setup --from <file>` or `aigw setup` adds the provider again and uses its stored Token without a prompt"
)

// unusedToken is one stored Token that no provider uses. Its fix reuses the
// Token; the removal is a RemoveAction and never a next_action, because a tool
// that follows every next_action would otherwise delete the Token.
type unusedToken struct {
	Provider     string `json:"provider"`
	NextDetail   string `json:"next_detail"`
	RemoveAction string `json:"remove_action"`
}

// listUnused names the stored Tokens that no provider uses. It reads item
// names only. It runs only for a check that names no client, because a check
// of named clients observes no other provider's credentials, and what it
// finds changes no client's state, the exit status or the next step.
func listUnused(runtime invocation.Context, cfg domain.Config) *unusedReport {
	report := &unusedReport{State: unusedListed, Providers: []unusedToken{}}
	ids, err := secret.UnusedTokens(runtime.Secrets, slices.Collect(maps.Keys(cfg.Providers)))
	if err != nil {
		report.State = unusedCannotList
		return report
	}
	reuse := reuseConfigured
	if len(cfg.Providers) == 0 {
		reuse = reuseEmpty
	}
	for _, id := range ids {
		report.Providers = append(report.Providers, unusedToken{Provider: id, NextDetail: reuse, RemoveAction: "aigw provider remove " + id})
	}
	return report
}

// evidence says, for a check that stops before it has rows of its own, which
// stored Tokens no provider uses and how they are reused or removed, or nothing.
func (report *unusedReport) evidence() string {
	ids := make([]string, 0, len(report.Providers))
	for _, token := range report.Providers {
		ids = append(ids, token.Provider)
	}
	switch len(ids) {
	case 0:
		return ""
	case 1:
		return " A Token is stored for " + ids[0] + ", which no provider uses; " + report.Providers[0].NextDetail + ", or `" + report.Providers[0].RemoveAction + "` deletes it."
	default:
		return " Tokens are stored for " + joined(ids, "and") + ", which no provider uses; " + report.Providers[0].NextDetail + ", or `aigw provider remove <id>` deletes each."
	}
}

// render lists the unused Tokens by provider ID with the command that reuses
// each and the one that removes it, says when the backend cannot name them, and writes nothing when none
// is stored.
func (report *unusedReport) render(r *terminal.Renderer) {
	if report == nil || report.State == unusedListed && len(report.Providers) == 0 {
		return
	}
	r.Section("Stored Tokens that no provider uses")
	if report.State == unusedCannotList {
		r.Rows(terminal.Field{State: terminal.Info, Label: "Credential backend", Value: "Cannot list its Tokens"})
		return
	}
	rows := make([]terminal.Field, 0, len(report.Providers))
	for _, token := range report.Providers {
		rows = append(rows, terminal.Field{State: terminal.Info, Label: token.Provider, Value: token.NextDetail, Details: []string{"or `" + token.RemoveAction + "` deletes it"}})
	}
	r.Rows(rows...)
}
