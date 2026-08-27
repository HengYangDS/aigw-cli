## Context

See [proposal.md](proposal.md). AIGW already selects one portable Token backend,
but provider diagnostics previously bypassed that authority and used a separate
native keyring service. Setup also needs the same backend semantics whether it
imports a catalogue with zero connected Accounts, discovers one environment
Token, or connects one explicitly selected Account.

## Goals / Non-Goals

**Goals:**

- One selected backend and one persistence policy for every AIGW credential.
- Typed slot isolation without adding a second public storage abstraction.
- Deterministic, collision-free environment projection for lowercase Account
  IDs.
- An actionable zero-or-one-Account team setup journey across supported hosts.

**Non-Goals:**

- Migrating or reading the retired diagnostic-only keyring service.
- Installing clients or a Responses compatibility service.
- Owning provider transport, client-private state, or external service
  lifecycle.

## Decisions

### Compose typed views over the selected backend

The existing secret store remains the single backend-selection authority. A
narrow typed view adds the credential purpose to the backend's physical slot.
This reuses the mature keyring, environment, file, and DPAPI implementations
without a parallel repository or caller-specific fallback.

Rejected alternatives were a second diagnostic store, dual reads from the old
keyring service, and caller-managed slot prefixes. Each would create another
authority or expose storage layout outside the owning package.

### Encode Account punctuation reversibly

Environment keys uppercase the lowercase Account ID and encode punctuation as
its ASCII hexadecimal byte. The allowed Account alphabet is ASCII, so this is
deterministic and collision-free without a registry or hash lookup. Lowercase
IDs make configuration spelling, environment projection, and CLI selection one
canonical identity.

### Treat missing credentials and clients as deferred capability

Manifest import is successful with zero connected Accounts. One explicit or
already available Token activates only compatible Routes; absent clients remain
unconfigured and can be projected later by `aigw sync`. Output reports facts
and the smallest safe next action rather than treating optional capability as
an error.

## Risks / Trade-offs

- **Existing uppercase Account IDs become invalid** → fail validation with the
  exact Account ID; manifests are credential-free and can be corrected without
  secret migration.
- **An old diagnostic-only keyring item remains outside AIGW's authority** → do
  not read it implicitly; users reconnect diagnostics through the selected
  backend, avoiding a permanent compatibility path.
- **Native credential APIs differ by platform** → keep one behavioral contract
  and require focused native journey evidence in addition to portable unit
  tests.

## Migration Plan

1. Release the typed backend and lowercase validation together.
2. Existing API Tokens remain in their current slots; new diagnostic values use
   purpose-qualified slots in the already selected backend.
3. Do not copy or delete foreign legacy keyring values automatically.
4. Roll back by restoring the prior binary; repository configuration and API
   Token slots remain readable by that version.
