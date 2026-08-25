## Why

Developer review and maintainer publication are different lifecycle events.
The current CI projection nevertheless treats every `dev` push as an accepted
publication and requires `main == dev`, so a valid proposal merge fails until a
later maintainer promotion repairs the refs.

## What Changes

- Route proposal verification only through pull-request and merge-request
  pipelines targeting `dev`.
- Route accepted-object verification and protected-ref parity through the
  atomic maintainer publication on `main`.
- Stop creating an expected-failure `dev` pipeline between review and accepted
  publication.
- Enforce official CUE formatting before comparing generated Forge projections.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-quality`: make CI evidence follow the actual review and accepted
  publication states rather than inferring acceptance from a `dev` push.

## Impact

The CUE CI authority, its GitHub and GitLab projections, focused CI tests, the
product-quality contract, and the existing lifecycle decision record change.
Product gates and supported platforms do not change.
