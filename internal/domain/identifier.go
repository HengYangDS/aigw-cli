package domain

import "strings"

// SameIdentifier reports whether two identifiers differ at most in case. A
// file system that ignores case, as macOS and Windows use by default, keeps
// both in one credential slot, so neither may be taken for the other's
// leftover.
func SameIdentifier(left, right string) bool { return strings.EqualFold(left, right) }
