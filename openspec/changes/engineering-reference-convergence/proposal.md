# Proposal

## Why

AIGW is now a formally distributed product, but release success alone does not make the repository an engineering reference. The implementation, tests, configuration, documentation, and extension paths must converge on one understandable design whose necessary complexity is evident and whose behavior can be reproduced from a clean checkout.

## What Changes

- Audit the repository from product concepts and user journeys rather than preserve accidental file boundaries.
- Close setup, deferred activation, synchronization, credential, installation, update, rollback, uninstall, Provider extension, Client extension, and optional external-gateway journeys through their existing semantic owners.
- Deliver Hermes and Claude Desktop third-party inference through explicit Adapters, and qualify cross-model use in Codex and Claude against actual protocol and capability contracts.
- Complete bounded assessments for OpenCode, Pi, CodeBuddy, WorkBuddy, Qoder, ChatGPT surfaces, and AWS/provider expansion; distinguish implementation obligations from research conclusions.
- Reorganize source, tests, tools, configuration, and documentation where physical layout obscures responsibility; delete parallel implementations, forwarding shells, obsolete compatibility paths, stale evidence, and unconsumed entities.
- Consolidate quality policy into one declarative authority with deterministic GitHub and GitLab projections, broad format coverage, and risk-derived thresholds.
- Qualify immutable candidate artifacts on macOS, Linux, and Windows with realistic client, credential, and lifecycle inputs; complete and archive this Change before stable publication and subsequent download verification.
- Upgrade direct supply-chain inputs to current stable releases only after compatibility and native acceptance.
- Document the smallest reproducible contribution and extension journeys so a new engineer can locate an invariant, change its owner, and disprove a faulty implementation.
- **BREAKING**: replace client-bound Profiles and parallel Route/Adapter selection with reusable model Profiles and one explicit client binding. Redesign setup, selection, synchronization, status and withdrawal around user intent. Provide an explicit retained-state migration; remove the replaced runtime schema and duplicate orchestration instead of preserving compatibility facades.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: require the requested client surfaces, model-neutral routing, honest capability admission, and independent configuration ownership.
- `release-distribution`: separate completed Change acceptance from stable publication, while preserving candidate, published-byte, and installation evidence.
- `repository-organization`: make semantic traceability and clean-checkout contribution reproducibility explicit repository obligations.
- `product-quality`: define engineering-reference acceptance as evidence from real journeys, adversarial tests, comprehensible ownership, and justified complexity rather than additional framework or metric count.

## Impact

The change may affect every current AIGW surface: CLI journeys, provider and client adapters, credential backends, portable and package-managed lifecycle, release tooling, CI generation, source and test topology, quality configuration, documentation, and dependency locks. It does not add traffic proxying, background services, conversation ownership, a parallel progress ledger, or repository-local copies of ETHOS lifecycle machinery. Codex Responses Proxy remains an independently installable composition peer and is changed only in its own repository.
