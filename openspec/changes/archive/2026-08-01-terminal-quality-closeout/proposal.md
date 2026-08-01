## Why

The terminal candidate exposed two repeatable failure modes: semantic leaf
packages could omit an accurate package contract while the architecture gate
remained green, and handled CLI failures could regress into framework usage or
diagnostic noise. Both failures must become executable constraints rather than
post-hoc review notes.

## What Changes

- Extend the existing architecture policy with a package-documentation rule
  that is disabled in generic fixtures and enabled for the real repository.
- Give each previously undocumented CLI leaf one accurate package contract in
  its primary implementation file, without adding carrier-only files.
- Assert that handled synchronization failures emit no usage banner, warning,
  traceback, or false completion message.
- Restore `go.mod` as the only Go toolchain version owner and make workflow
  tests derive their expected projection from it.

## Capabilities

### Modified Capabilities

- `product-control-plane`: subject=aigw-cli:product-control-plane;
  strengthen its existing semantic-ownership and user-facing failure-output
  contract; reuse=extend; change=modify;
  facet:lifecycle=validation; facet:surface=source,tests,ci;
  facet:authority=architecture-policy,behavior-tests.

## Out of Scope

- New commands, compatibility layers, package aliases, or product behavior.
- Retrospective prose that is not enforced by an existing product mechanism.
- Publication, deployment, runtime acceptance, and lane retirement, which
  remain separate lifecycle transitions.

## Impact

The architecture checker, four CLI leaf package comments, one focused
failure-output test, and Go toolchain projections change. No public command
grammar or configuration schema changes.
