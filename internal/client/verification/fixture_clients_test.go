package verification

// The built-in client IDs as fixtures: this package's tests cannot import the
// adapters that own them, because the adapters import this package.
const (
	clientCodex  = "codex"
	clientHermes = "hermes"
)
