## Why

Codex's independent acceptance of installed AIGW 0.4.10 on 2026-10-22
(product-readiness Attestation `1a752cff`) found a recovery gap. A
non-interactive `aigw use --for <client> <route>` on a Route with more than one
compatible protocol correctly refuses before mutation, and its message asks for
`--protocol`. Its JSON result, though, names `aigw check` as `next_action`,
because the refusal reaches the generic error renderer, which only recognizes a
quoted `aigw` command. `aigw check` cannot resolve the ambiguity. Direct
`aigw setup` with more than one endpoint URL has the same fallback in its
human Recommended action.

## What Changes

- The `aigw use` refusal for an ambiguous protocol carries its exact retry,
  `aigw use --for <client> --protocol <a|b> <route>`, as the structured
  recommended action, so JSON `next_action` and human output name it.
- The direct `aigw setup` refusal names the compatible protocols and asks for
  the same setup command with `--protocol`.
- Messages, refusal before mutation and unchanged Client Bindings stay as they
  are.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `route-client-selection`: the explicit protocol choice requirement gains a
  scenario for the recommended action of a non-interactive refusal.

## Impact

`internal/cli/selection` and `internal/cli/onboarding` and their tests change.
Configuration schema, credentials, client projections and the team manifest do
not. Agents that read JSON `next_action` get a command that resolves the
refusal.
