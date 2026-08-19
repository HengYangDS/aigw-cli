## Why

Claude Code can launch outside the shell that installed AIGW. A helper value that
relies on PATH therefore makes a valid client projection fail in GUI and service
contexts; the installed executable is the only stable invocation authority.

## What Changes

- Project Claude Code `apiKeyHelper` as an absolute, shell-safe invocation of the
  installed AIGW executable.
- Validate the executable path before writing the owned projection.
- Preserve the existing boundary: AIGW owns only its marked Claude settings and
  credential lookup; it does not intercept Claude or store a token in settings.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Claude Code projection remains client-owned but its
  credential helper is bound to the installed executable and fails closed for an
  invalid path.

## Impact

The change is limited to Claude projection, its transaction wiring, focused
regression coverage, and this OpenSpec record. Provider routing, Codex
projection, credentials, sessions, and Proxy lifecycle remain unchanged.
