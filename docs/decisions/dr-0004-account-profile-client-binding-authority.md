# DR-0004: Use Account, Profile, and Client Binding as Configuration Authority

- Status: accepted
- Date: 2026-08-07
- Last amended: 2026-09-21

## Context

Provider endpoints, credentials, reusable model definitions, and per-client
choices have different lifecycles. Encoding client identity in every Profile or
maintaining separate Route and Adapter selections duplicates state and creates
ambiguous fallback.

## Decision

An Account owns provider endpoints and one logical Token boundary. A Profile
owns one reusable `account + model` identity plus the protocols verified for
that exact pairing. Its optional Flagship/Daily tier is catalogue guidance, not
runtime policy. A Client Binding selects one compatible Profile for one client
and owns enabled intent, the active protocol, authentication, native targets,
and genuinely client-specific options.

Accounts, Profiles, recommendations, and Client Bindings form the configuration
SSOT. A recommendation is import guidance, not a local selection. Setup may use
it only when the corresponding binding is unselected. Explicit bindings survive
missing credentials, newly imported Accounts, and discovery of unrelated
clients.

An admitted Client Adapter implements discovery, guarded native projection,
inspection, verification, compensation, and withdrawal. It does not own another
selection state. Provider diagnostics are optional Account capabilities and
cannot create a Profile, Client Binding, or hidden provider fallback.

The immediately preceding schema is readable only by the explicit
`aigw config migrate` operation. Normal runtime accepts the current schema and
contains no Route or Adapter compatibility authority.

## Consequences

Adding an ordinary provider changes catalogue data rather than shared client
logic. Selecting one binding never copies a Token into client files, changes
another binding, or retries traffic through an unselected provider. Status and
check expose one client-keyed state tree; no parallel Route inventory exists.

## Revisit Trigger

Revisit if AIGW adopts a different canonical domain model that preserves the
same explicit endpoint, credential, client-choice, projection, and withdrawal
boundaries without adding a second configuration authority.
