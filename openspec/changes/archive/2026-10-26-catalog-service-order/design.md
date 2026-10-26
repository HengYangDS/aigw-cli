# Design

## Context

`CatalogRouteIDs` already owns Route presentation for human and JSON lists and
selection consumers. `CompareProtocols` owns the native display order used by
catalogue observations. Canonical `RouteIDs` and export enumeration remain
lexical identity owners.

## Goals / Non-Goals

**Goals:** Repair service grouping at the shared presentation owner and preserve
Model order within each group, including catalogues without explicit ranks.

**Non-Goals:** Change protocol admission, endpoint selection, recommendations,
bindings, serialization, credentials, or installed products.

## Decisions

Compare Accounts first, then each Route's independently copied protocol set
sorted by `CompareProtocols`, then Models and Route identities. Compare complete
sets lexicographically so multi-interface Routes have a deterministic group
without selecting a runtime protocol. Cache those small sets once per listing.

Remove the rank-free early return: Account and Model comparators already provide
identity fallbacks. Keeping it would bypass service grouping for ordinary custom
catalogues. A CLI-local sort would duplicate policy; changing admitted protocol
enumeration would unnecessarily affect routing and export consumers.

## Risks / Trade-offs

Display positions change even without explicit ranks. Focused regressions retain
bindings and stored bytes, and canonical enumeration remains unchanged.
Multi-interface groups cannot duplicate one Route across several list positions;
their sorted protocol set supplies the stable grouping key instead.
