# Design

## Context

`internal/configuration/ordering.go` owns presentation order. `CatalogAccountIDs`
orders Accounts by their optional `order` rank, then by identity, and the
catalogue and Route views already use it. `newListCommand` in
`internal/cli/account` builds its rows from `slices.Sorted(maps.Keys(...))`,
so it ignores declared ranks in both output forms.

## Goals / Non-Goals

**Goals:** Account lists honor the declared presentation order.

**Non-Goals:** a second ordering policy, changing serialization or export
order, or changing which Accounts are listed.

## Decisions

### Consume the existing presentation owner

The list iterates `cfg.CatalogAccountIDs()`. Accounts without a rank still
follow identity order, so configurations without ranks list exactly as before.
A local sort was rejected because it would duplicate the presentation policy.

## Risks / Trade-offs

- [Automation relied on lexical order] → identity remains the order for
  Accounts without ranks; only configurations that declare ranks change, and
  their JSON already carries the ranks.

## Migration Plan

None; the next release lists Accounts in declared order.

## Open Questions

- None.
