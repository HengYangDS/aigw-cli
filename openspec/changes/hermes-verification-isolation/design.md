## Context

`PrepareVerification` in `internal/adapter/hermes/document.go` is the one owner
of the isolated Hermes verification projection. It already overrides
installation, update, launcher, gateway idle-exit, plugin and memory-provider
settings, and it keeps the native model settings. The proposal names the gap.

## Goals / Non-Goals

**Goals:**

- The verification turn starts no operator MCP server and runs no operator
  shell hook.
- The projection keeps one owner and every native setting that verification
  needs.

**Non-Goals:**

- Changing the turn bound, the gateway Hermes starts for a chat, or other
  clients' verification.

## Decisions

- **Explicit empty mappings.** The projection names `mcp_servers: {}` and
  `hooks: {}` rather than deleting the keys. An absent key falls back to the
  native defaults; the empty mapping states that the turn starts nothing.
- **Hook consent is removed, not set false.** `hooks_auto_accept` only grants
  consent for configured hooks. With no hooks it has nothing to grant, so the
  projection drops the key and keeps the native default.
- **One owner.** The settings join the existing projection, which stays the
  only place that decides what a verification home contains.
- **Not `--safe-mode`.** Hermes's `chat --safe-mode` disables every
  customization, including the model and provider settings that verification
  must keep, so it cannot keep the Route while dropping the extensions.

## Risks / Trade-offs

- The fix removes the extensions' startup work, but a turn on a saturated host
  can still exceed 60 s. On 2026-11-06, with a 1-minute load above 40, a lane
  build started no MCP server and still exceeded the bound. The same Hermes
  completed in 26 to 27 s at a load of about 5. The remaining cost is
  Hermes's own gateway boot under contention, outside this Change.
- A longer turn bound would hide that cost instead of reporting it, so the
  bound stays.
