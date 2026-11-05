## Why

`aigw verify --for hermes` copies the operator's Hermes configuration into a
private home. The copy drops plugins and the memory provider, but it keeps
`mcp_servers`, `hooks` and `hooks_auto_accept`. Hermes then starts every
enabled operator MCP server and registers every operator shell hook inside the
verification turn.

On 2026-11-05 that turn started four MCP servers and waited for their tools.
The work counts against the 60-second turn bound. When the host was busy, the
verification timed out on both 0.6.0 and 0.6.1. The route-client-selection
specification already forbids this: verification runs "without starting the
operator's MCP servers, plugins, hooks, notifications or memory providers".

## What Changes

- The isolated Hermes verification projection names no MCP servers and no
  shell hooks, and it carries no hook consent.
- A regression test proves that operator MCP servers, shell hooks and hook
  consent never reach the projection, and that the operator's configuration
  stays unchanged.

Non-goals: the turn bound, the gateway Hermes starts for a chat, and
verification of other clients do not change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `route-client-selection`: the extension-isolation scenario names what the
  Hermes projection leaves out.

## Impact

`internal/adapter/hermes` changes, and the next release ships the fix. The
operator's Hermes configuration, credentials and the native installation do
not change.
