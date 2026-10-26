# Proposal

## Why

Route lists agree between human and JSON output but interleave service types
because Model rank is compared before protocol. This leaves five observed
grouping inversions in the installed team catalogue.

## What Changes

- Group Route presentation by Account, admitted protocol set, Model, and Route
  identity, using the existing protocol comparison.
- Preserve canonical identity enumeration, exports, bindings, and routing.
- Cover ranked and unranked catalogues with focused shared-owner and public CLI
  regressions.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Route lists consistently group admitted service types
  independently of identity and client selection.

## Impact

- `internal/configuration/ordering.go`: shared Route presentation comparison.
- `internal/configuration/model.go`: clarify canonical identity enumeration.
- `internal/configuration/ordering_test.go`: protocol-set and identity regressions.
- `internal/cli/acceptance/route_order_test.go`: public human/JSON grouping and
  stored-byte preservation.
- `internal/cli/acceptance/route_test.go`: unranked ordering and purpose guidance.
- `internal/cli/selection/interactive_test.go`: choice labels remain bound to Route
  identities rather than incidental display positions.

These owners change display order only. No schema, dependency, credential, client
configuration, release, or installed service changes. Claude retains publication
and installation.
