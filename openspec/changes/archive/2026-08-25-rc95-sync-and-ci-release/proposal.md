## Why

The verified post-rc.94 product completes progressive team setup, exact client
projection previews, current-user-protected Windows fallback credentials, and
provider-neutral CI lifecycle semantics. Those product bytes need a new,
immutable release identity; reusing rc.94 would make provenance ambiguous.

## What Changes

Publish the post-rc.94 product as the unique rc.95 release by advancing the
version source, recording its user-visible changes once, and validating the
existing release contract without adding provider, credential, client,
projection, or transport behavior in the release commit itself.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. Product behavior is owned by the already archived bounded Changes; this
Change owns release identity and chronology only.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified by
the release commit. One signed product object, annotated tag, and byte-identical
asset set are projected unchanged to both Forge peers.
