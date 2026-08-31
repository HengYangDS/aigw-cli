## Context

See `proposal.md`. The existing architecture already separates local intent and
projection from traffic processing. The remaining work is to make that boundary
and its extension consequences explicit in the two documents that already own
them.

## Goals / Non-Goals

**Goals:**

- Give each Provider, authentication, Client, and wire extension one owner.
- Prefer composition with mature products when it reduces AIGW's maintenance
  surface.
- Preserve the portable binary and transactional projection contracts.

**Non-Goals:**

- Selecting or embedding a universal gateway.
- Adding a Provider, Client, framework, compatibility shim, or feature flag.
- Creating another comparison or extension-policy document.

## Decisions

### Keep traffic products outside AIGW

General gateways already own protocol aggregation, load balancing, quotas,
policy, metering, and observability. AIGW will compose with them as Account
endpoints instead of embedding or reimplementing those capabilities. This keeps
configuration usable when AIGW is not running and avoids a second traffic-policy
authority.

### Classify before extending

The extension path follows the authority that changes:

1. compatible endpoint or model -> Account data;
2. distinct credential exchange -> Account authentication;
3. new local configuration target -> complete Client Adapter;
4. incompatible wire behavior -> independent data-plane product.

A provider name does not select behavior, and a Client Adapter cannot gain
authority over another client's state.

### Require net deletion from dependencies

A library or framework is worthwhile only if it replaces an existing owned
boundary while preserving portability and transaction guarantees. Comparing
responsibilities rather than current feature inventories keeps the guidance
stable and prevents dependency adoption from becoming another feature list.

## Risks / Trade-offs

- **Product comparisons become stale** -> describe durable ownership boundaries,
  not versions, popularity, or exhaustive feature lists.
- **A gateway leaks into the control plane** -> admit only its endpoint unless a
  separately reviewed authority boundary changes.
- **A generic Adapter erases client differences** -> require the complete native
  discovery, projection, verification, rollback, and uninstall slice.
