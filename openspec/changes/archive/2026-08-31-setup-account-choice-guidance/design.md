## Context

See `proposal.md` for the user-visible defect. The imported configuration
already retains the full Account and Profile catalogue, and
`Config.SelectRoutesForConnectedAccounts` already owns deterministic Route
selection. The missing link is that synchronization discovers clients from the
stored Routes without first applying that existing selection authority to the
Accounts now readable from the configured secret backend.

Setup guidance has the same authority mismatch: it derives one environment
variable from the first stable Account ID, although the catalogue models those
Accounts as alternatives.

## Goals / Non-Goals

**Goals:**

- Reuse one Route-selection authority in setup and synchronization.
- Make dry-run and apply compute the same non-mutating, secret-free plan.
- Derive setup choices from the imported Account catalogue.

**Non-Goals:**

- Add provider priority, automatic failover, or a global default Route.
- Store, copy, validate, or reveal Token values during synchronization.
- Add Proxy knowledge or manage an external endpoint lifecycle.
- Add a command, configuration field, compatibility layer, or dependency.

## Decisions

### Select before discovery in the existing synchronization transaction

Synchronization will identify which configured Accounts are readable through
the existing secret abstraction, pass only their IDs to
`SelectRoutesForConnectedAccounts`, and give the selected configuration to the
existing client discovery and projection planner. This preserves the current
transaction and makes Route selection precede every dependent plan.

The rejected alternative is a second sync-specific selection algorithm or
persisted activation state. Either would create a parallel authority and make
setup, `use`, and sync disagree.

### Enumerate choices instead of inventing a default

Environment-backed setup will derive one activation choice per compatible
Account from the stable manifest order. The result remains one semantic value
rendered as human or JSON output.

The rejected alternative is retaining a lexical example with stronger
wording. It remains misleading because a user can reasonably choose another
listed Account and expect the advertised continuation to work.

### Treat credential availability as capability, not transport

Synchronization will ask the configured credential backend whether each
catalogue Account is available. A backend may internally read its own value to
answer that question, but synchronization neither receives nor exposes that
value and never moves, compares, or persists credentials. Dry-run therefore
observes the same available-Account set as apply while remaining non-mutating.

Route selection belongs only to `sync`. The lower-level client-discovery
operation and `repair` retain their narrower existing semantics, so repairing
owned projections cannot silently replace an operator-selected Route.

## Risks / Trade-offs

- **A credential is unavailable or unreadable** → treat that Account as
  unavailable and perform no selection or projection for it.
- **Several Accounts may be available** → retain the existing deterministic
  selection rules; do not add provider preference.
- **No Account is available** → preserve the imported catalogue and current
  disabled Routes; synchronization remains a no-op rather than guessing.
