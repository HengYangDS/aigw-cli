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

### The platform owns the guarded multi-file write

Claude Desktop's version, with Codex's exact-mode write, moves beside the
platform's guarded file functions. Test fault injection becomes a parameter
instead of package variables. Hermes restores its config before its state on
purpose, so it keeps its own sequence.

### Forwarders go rather than stay as facades

DR-0009 rules out forwarding facades. Tests call the functions that
production calls.

## Risks / Trade-offs

- A shared write path touches every adapter's files at once. Each adapter's
  existing write, refusal and rollback tests must pass unchanged, and a
  removal is skipped where it would change bytes, modes or order.
