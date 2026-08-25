## Context

Provider choice and model choice are separate dimensions. The manifest owns the
reviewed route preference; local credential availability decides which Account
can currently satisfy it. Sorting Profile IDs is deterministic but does not
preserve that semantic intent.

## Decision

Route selection remains owned by `configuration.Config`. For each admitted
client, keep the current recommended Profile when its Account is connected.
Otherwise, first select a connected Profile whose model equals the recommended
Profile's model for that client. Only when no such Profile exists may selection
fall back to the first valid connected Profile in canonical Profile order.

The default route follows the selected route for its original client. This
requires no Provider-specific branch, compatibility alias, or second policy
surface.

## Verification

Use model-level regression tests for selection semantics and CLI acceptance tests
for the team manifest with one Account Token. Run isolated binary journeys for
no-token import, each single-Provider environment Token, absent clients, and
post-install synchronization.

## Requirement To Task To Proof

| Requirement                                                                | Task  | Proof                                                                                                  |
| -------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------ |
| `product-control-plane:Reviewed team configuration is directly consumable` | `1.1` | `go test ./internal/configuration -run TestSelectRoutesForConnectedAccountsPreservesRecommendedModels` |
