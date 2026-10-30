---
subject: aigw:decision:0004
role: decision
state: canonical
decision_id: DR-0004
decision_status: accepted
relations:
  canonical_for: Use Account, Route, and Client Binding as Configuration Authority
---

# DR-0004: Use Account, Route, and Client Binding as Configuration Authority

- Date: 2026-08-07
- Last amended: 2026-09-21

## Context

Provider endpoints, credentials, reusable model definitions, and per-client
choices have different lifecycles. Encoding client identity in every Route or
maintaining separate Route and Adapter selections duplicates state and creates
ambiguous fallback.

## Decision

An Account owns provider endpoints and one logical Token boundary. A Model owns
canonical model identity independently of provider access. A Route owns one
Account-to-Model path, its exact upstream identifier, and the protocols and
capabilities verified for that pairing. A Client Binding selects one compatible Route for one client
and owns enabled intent, the active protocol, authentication, native targets,
and genuinely client-specific options.

Accounts, Routes, recommendations, and Client Bindings form the configuration
SSOT. A recommendation is import guidance, not a local selection. Setup may use
it only when the corresponding binding is unselected. Explicit bindings survive
missing credentials, newly imported Accounts, and discovery of unrelated
clients.

An admitted Client Adapter implements discovery, guarded native projection,
inspection, verification, compensation, and withdrawal. It does not own another
selection state. Provider diagnostics are optional Account capabilities and
cannot create a Route, Client Binding, or hidden provider fallback.

The immediately preceding schema is readable only by the explicit configuration
migration command. Normal runtime accepts the current schema and contains no
Route or Adapter compatibility authority.

## Alternatives Rejected

- **Client identity encoded in every Route:** rejected because it duplicates
  client choice across Routes and makes fallback between them ambiguous.
- **Separate Route and Adapter selections:** rejected because two selection
  states for one client must agree, and their disagreement has no single owner.

## Consequences and Boundary

Adding an ordinary provider changes catalog data rather than shared client
logic. Selecting one binding never copies a Token into client files, changes
another binding, or retries traffic through an unselected provider. Status and
check expose one client-keyed state tree; no parallel Route inventory exists.

## Evidence and Revisit

Revisit if AIGW adopts a different canonical domain model that preserves the
same explicit endpoint, credential, client-choice, projection, and withdrawal
boundaries without adding a second configuration authority.
