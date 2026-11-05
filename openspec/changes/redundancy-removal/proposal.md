## Why

A read-only review of the repository found code that duplicates an owner or
has no consumer: branch and tag publication commands that duplicate
`ethos publish`, repository subcommands that only their tests call, a reader
for a Claude Desktop profile that no release wrote, fields and hooks that only
tests set, hand-written code that the Go standard library ships, and test
fixtures copied between packages. Each copy is code to keep correct twice.

## What Changes

- Remove `tools/forge project` and `tools/forge publish-tag`; ETHOS publishes
  branches and tags, and `tools/forge` keeps its verification commands.
- Remove repository subcommands, options, fields, hooks and legacy readers
  that have no production, CI, documentation or operator consumer.
- Replace hand-written helpers with the Go standard library.
- Keep one copy of each duplicated test fixture in the package that already
  owns its subject.

Non-goals: product behavior, configuration formats, released document
fields and credential handling do not change, and no shared test-support API
is introduced.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: Forge publication has one owner, and a
  repository command without a consumer is removed.

## Impact

Repository tools, tests and their documentation change. The released
programs behave the same, so no release follows from this Change alone.
