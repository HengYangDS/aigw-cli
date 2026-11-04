## Context

The 2026-11-05 candidate calculation with the repository's Renovate policy
reports direct updates only. Renovate's npm manager proposes lock-file
maintenance only when `lockFileMaintenance` is enabled, and the policy leaves
it off. A separate lock refresh on 2026-11-04 found 33 transitive npm moves
that the calculation never listed. That refresh used a 24-hour age, against
this repository's three-day npm age. Under the three-day age, parse5 8.1.0 is
the first of them to qualify, and it brings entities 9.0.0. The d3 series
qualifies on 2026-11-06.

## Goals / Non-Goals

**Goals:**

- Take every stable release whose admission evidence passes, including
  transitive npm releases past the release age.
- Make the candidate calculation report transitive npm moves, so the supply
  chain is not described as current while the lock holds older admitted
  releases.

**Non-Goals:**

- Shortening the npm release age.
- Taking Claude Code 2.1.318 or Mise 2026.11.0 before their holds end.

## Decisions

### Enable lock-file maintenance in the candidate policy

The policy enables Renovate's `lockFileMaintenance` under the repository's
existing three-day npm age. The calculation then reports a lock refresh as one
proposal beside the direct updates, so a single run answers whether the
supply chain is current.

A separate scripted `npm update --package-lock-only` in the calculation was
rejected: it duplicates what Renovate already computes from the same policy.

### Take parse5 8.1.0 with entities 9.0.0

entities 9.0.0 is a major release reached only through parse5 8.1.0, which
declares it. The repository's npm tools use parse5 through their own
dependencies, not directly. `mise run check` exercises every npm tool, so it
verifies that the major release is compatible.

## Risks / Trade-offs

- Lock-file maintenance adds one proposal per calculation while transitive
  releases are pending; that is the intended signal.
