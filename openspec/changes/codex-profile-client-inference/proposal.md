## Why

An explicitly selected client-scoped profile already declares whether it is
for Codex or Claude Code. Requiring the operator to repeat that fact with
`--for` adds input without adding authority and currently makes `aigw test`
probe the wrong client first.

## What Changes

- Infer the client when `--profile` names a profile with one declared client
  and `--for` is omitted.
- Keep explicit `--for` authoritative and reject profile/client conflicts.
- Require `--for` when the selected profile is not client-scoped.
- Preserve existing behavior when no profile is selected.

## Impact

- **UX:** one explicit profile is sufficient for `test` and `verify`.
- **Authority:** the profile's canonical `client` field remains the sole owner
  of its client scope.
- **Compatibility:** no alias, fallback parser, or legacy path is introduced.

## Non-goals

- Inferring a client from model names, endpoints, routes, or profile IDs.
- Changing default route selection or multi-client verification semantics.
