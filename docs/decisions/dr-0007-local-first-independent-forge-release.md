---
subject: aigw:decision:0007
role: decision
state: canonical
decision_id: DR-0007
decision_status: accepted
relations:
  canonical_for: Local Product Objects and Independent Forge Peers
---

# DR-0007: Local Product Objects and Independent Forge Peers

- Date: 2026-08-19
- Owner: Release maintainers

## Context

Forge-specific identity replay created different commit and tag objects for
the same product source. It made cross-Forge parity weaker than Git object
identity, duplicated signing and repair logic, and coupled two publication
planes that must remain independently usable.

## Decision

Local Git is the sole product-object authority. A commit or annotated release
tag is constructed and signed once, then published unchanged to any selected
GitLab or GitHub peer.

The invariants are:

- exact commit OID across local Git and selected peers;
- exact annotated tag OID, peeled commit, and tree for new releases;
- product signing independent from SSH, PAT, OIDC, or other transport
  authentication;
- no history replay, identity rewrite, provider-qualified tags, commit maps,
  or tree-only parity;
- peer-local compare-and-swap publication and post-push observation;
- zero peers remains a complete local product topology.

Additional organizational approval uses detached attestations rather than
changing the product object.

## Alternatives Rejected

- **Forge-specific identity replay:** rejected because it gives each Forge a
  different object for the same source, weakens parity below Git object
  identity, and duplicates signing and repair logic.
- **Parity through provider-qualified tags, commit maps or tree comparison:**
  rejected because each substitutes a mapping for the identity of one signed
  object.

## Consequences and Boundary

Each Forge may show a different account-level verification presentation and
use different credentials, while still hosting the same signed object. A peer
failure never blocks local acceptance or the other peer. Existing divergent
history requires one bounded cutover; old formal releases are not rewritten
without a separate explicit retention and migration decision.

## Evidence and Revisit

Revisit only if local Git stops being the product-object authority or a future
distribution system cannot transport an existing signed Git object unchanged.
