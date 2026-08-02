// Package surface owns stable host-surface identities and their authorities.
package surface

import "strings"

// ID identifies a host integration surface.
type ID string

// Authority identifies the system that owns a surface.
type Authority string

const (
	CodexHomeDefault ID = "codex-home-default"

	AuthorityAIGW Authority = "aigw"

	codexHomeExplicitPrefix = "codex-home-explicit-"
)

// CodexHomeExplicit returns the stable identity for an explicitly selected
// Codex home.
func CodexHomeExplicit(identifier string) ID {
	return ID(codexHomeExplicitPrefix + identifier)
}

// IsCodexHome reports whether the identity is the default or an explicit Codex
// home shared by Codex CLI and Codex Desktop.
func (id ID) IsCodexHome() bool {
	return id == CodexHomeDefault || strings.HasPrefix(string(id), codexHomeExplicitPrefix)
}

// Authority returns the owner of a known surface identity.
func (id ID) Authority() (Authority, bool) {
	if id.IsCodexHome() {
		return AuthorityAIGW, true
	}
	return "", false
}

// HasAuthority reports whether authority is the owner of this surface.
func (id ID) HasAuthority(authority Authority) bool {
	want, known := id.Authority()
	return known && authority == want
}
