// Package surface owns stable host-surface identities and their authorities.
package surface

import "strings"

// ID identifies a host integration surface.
type ID string

// Authority identifies the system that owns a surface.
type Authority string

const (
	CodexCLIStandalone ID = "codex-cli-standalone"

	AuthorityAIGW Authority = "aigw"

	codexCLIExplicitPrefix = "codex-cli-explicit-"
)

// CodexCLIExplicit returns the stable identity for an explicitly selected
// Codex CLI configuration.
func CodexCLIExplicit(identifier string) ID {
	return ID(codexCLIExplicitPrefix + identifier)
}

// IsCodexCLI reports whether the identity is a standalone or explicit Codex
// CLI surface.
func (id ID) IsCodexCLI() bool {
	return id == CodexCLIStandalone || strings.HasPrefix(string(id), codexCLIExplicitPrefix)
}

// Authority returns the owner of a known surface identity.
func (id ID) Authority() (Authority, bool) {
	if id.IsCodexCLI() {
		return AuthorityAIGW, true
	}
	return "", false
}

// HasAuthority reports whether authority is the owner of this surface.
func (id ID) HasAuthority(authority Authority) bool {
	want, known := id.Authority()
	return known && authority == want
}
