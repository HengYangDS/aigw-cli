## Context

An active Change is authoring state. An accepted `dev` or `main` tree is a
publication state. Treating both as valid on the same tree made a structural
OpenSpec validation pass while lifecycle closure was incomplete.

The sole tracked manifest was intentionally fictitious, but AIGW is an internal
team control plane whose reviewed manifest is already token-free and suitable
for direct import.

## Decisions

### Check the positive accepted-tree shape

Source verification requires `openspec/changes/` to contain only `archive/`.
The gate consumes the filesystem shape directly and does not maintain a list of
forbidden Change names.

### Publish one real team manifest

`manifests/team.toml` is the single tracked manifest. It contains reviewed
Accounts, Profiles, Routes, and public endpoints, but no Tokens, client paths,
or host identity. Documentation snippets explain the format without creating a
parallel toy configuration.

## Non-Goals

- Do not put Tokens or workstation-specific adapter paths in source.
- Do not make AIGW depend on ETHOS or the Proxy runtime.
- Do not retain `example.toml` as a compatibility alias.
