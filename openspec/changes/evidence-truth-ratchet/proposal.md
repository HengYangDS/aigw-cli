## Why

The candidate exposed two ways a green gate could overstate truth: a unit test
silently contacted the public GitHub API, and a coverage observation was copied
without its source revision, tree, or raw counts. Verification must be isolated
and quantitative claims must remain reproducible.

## What Changes

- Make the GitHub private-release fallback test use an explicit in-memory HTTP
  transport and prove that it never reaches the network.
- Cover the enabled Codex adapter-without-target diagnostic through an explicit
  fixture instead of an accidental worktree state.
- Require dated quantitative evidence to name its source revision, source tree,
  numerator, denominator, and derived percentage.
- Bind the active claim digest to the corrected dated record.

## Capabilities

### Modified Capabilities

- `product-control-plane`: make local verification deterministic and make
  quantitative candidate evidence source-bound and internally consistent.

## Impact

The change affects focused Go tests, the governance evidence check, the dated
candidate record, and its claim digest. It does not change product commands,
configuration, provider behavior, release publication, or runtime state.
