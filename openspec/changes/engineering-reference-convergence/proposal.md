# Proposal

## Why

AIGW is now a formally distributed product, but release success alone does not make the repository an engineering reference. The implementation, tests, configuration, documentation, and extension paths must converge on one understandable design whose necessary complexity is evident and whose behavior can be reproduced from a clean checkout.

## What Changes

- Audit the repository from product concepts and user journeys rather than preserve accidental file boundaries.
- Close setup, deferred activation, synchronization, credential, installation, update, rollback, uninstall, Provider extension, Client extension, and optional external-gateway journeys through their existing semantic owners.
- Reorganize source, tests, tools, configuration, and documentation where physical layout obscures responsibility; delete parallel implementations, forwarding shells, obsolete compatibility paths, stale evidence, and unconsumed entities.
- Consolidate quality policy into one declarative authority with deterministic GitHub and GitLab projections, broad format coverage, and risk-derived thresholds.
- Qualify the complete declared product graph on macOS, Linux, and Windows using released artifacts and realistic client, credential, and lifecycle inputs.
- Upgrade direct supply-chain inputs to current stable releases only after compatibility and native acceptance.
- Document the smallest reproducible contribution and extension journeys so a new engineer can locate an invariant, change its owner, and disprove a faulty implementation.
- **BREAKING**: remove unsupported aliases, duplicate configuration, obsolete compatibility behavior, and historical carriers that have no current consumer. Migration instructions are required only for supported public behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `repository-organization`: make semantic traceability and clean-checkout contribution reproducibility explicit repository obligations.
- `product-quality`: define engineering-reference acceptance as evidence from real journeys, adversarial tests, comprehensible ownership, and justified complexity rather than additional framework or metric count.

## Impact

The change may affect every current AIGW surface: CLI journeys, provider and client adapters, credential backends, portable and package-managed lifecycle, release tooling, CI generation, source and test topology, quality configuration, documentation, and dependency locks. It does not add traffic proxying, background services, conversation ownership, a parallel progress ledger, or repository-local copies of ETHOS lifecycle machinery. Codex Responses Proxy remains an independently installable composition peer and is changed only in its own repository.
