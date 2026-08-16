## Decision

The configuration domain resolves a profile name to its declared client. CLI
commands consume that single operation only when `--profile` is present and
`--for` is absent. A profile without an explicit client fails closed with a
clear instruction to provide `--for`.

`test` keeps its current all-client behavior without `--profile`. `verify`
keeps requiring `--for` without `--profile`. An explicit `--for` is never
overridden; the existing runtime resolver continues to reject conflicts.

## Dependency direction

```text
test / verify -> configuration profile selection -> profile.client
```

No command inspects models, endpoints, route names, or provider-specific
identifiers to guess a client.

## Verification

1. Prove configuration selection for scoped, unscoped, and unknown profiles.
2. Prove `test --profile <codex>` performs only the Codex endpoint probe.
3. Prove `verify --profile <codex>` performs one bounded Codex request.
4. Prove explicit conflicts and no-profile behavior remain unchanged.
5. Pass the complete source graph and exact-HEAD proof before landing.
