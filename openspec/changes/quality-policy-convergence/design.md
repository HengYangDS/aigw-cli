# Design

## Admission Classes

| Class | Merge authority | Basis |
| --- | --- | --- |
| Product invariant | Blocking | Observable correctness, security, state, portability, or evidence failure |
| Quantitative risk policy | Blocking | One owner, exact measurement, risk model, false-positive cost, repair path, review condition |
| Presentation heuristic | Review only | Formatter output or human/agent review |

## Ownership

- `.config/checks/coverage/policy.toml` owns coverage admission semantics.
- `tools/coverage` executes that policy and binds raw package and aggregate
  evidence.
- `tools/architecture/text_layout.go` enforces only deterministic byte
  invariants shared by supported hosts.
- Language-native formatters own source layout. Serializer tests own generated
  TOML layout. Repository-wide code does not duplicate either policy.
- `product-quality` owns the general quality contract;
  `product-control-plane` references its positive semantic consequences without
  inventing a parallel negative taxonomy.

## Review Trigger

The coverage floor is reviewed when repeated legitimate changes are blocked by
denominator granularity rather than missing product behavior, or when the
credential/configuration risk boundary changes. A review changes the existing
policy owner; it does not add an exception list.
