## Context

`internal/adapter` is the seam every client adapter implements, and
`internal/platform` already owns guarded single-file writes. The review
behind this Change traced each duplicate to its callers on darwin, linux and
windows. Test fixtures copied between adapter packages stay where they are:
`docs/architecture/repository-tooling.md` keeps test readers package-local.

## Goals / Non-Goals

**Goals:**

- One owner for each step that several adapters repeat.
- No code that only tests call.

**Non-Goals:**

- Behavior changes to the released programs, including file bytes, modes,
  write order within an adapter, rollback order and error text.
- A shared test-support package.

## Decisions

### The seam owns what every adapter repeats

The Converge preamble, the encoders, the receipt slice and the state-file
suffix move to `internal/adapter`, because every adapter already depends on
it. Hermes's preamble checks that its executable exists before it reads the
secret store, and the helper keeps that order.

### Guarded multi-file writes stay with their adapters

Claude Code, Claude Desktop and Codex each write several files under a
preimage guard and restore them on failure, but the three differ. Claude
Code restores its settings before its state and attempts both, while Claude
Desktop and Codex restore in reverse write order. Codex and Claude Code wrap
their errors in their own words, and Claude Desktop returns them unwrapped.
One helper would need an order switch and per-adapter error formatting, so
each adapter keeps its own sequence, as Hermes already does.

### Forwarders go rather than stay as facades

DR-0009 rules out forwarding facades. Tests call the functions that
production calls.

## Risks / Trade-offs

- Codex's reconciliation is exported as `ReconcileConfigs`, with a
  `HomeTarget` constructor, because tests in other packages call it. It is
  the function production calls, not a forwarder.
- One refusal keeps released text that names an authority and a projection
  mode the code no longer models, so the printed text stays unchanged.
