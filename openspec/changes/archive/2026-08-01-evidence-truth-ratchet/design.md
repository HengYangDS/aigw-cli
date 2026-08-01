## Context

See `proposal.md`. The coverage gate already enforces the product threshold,
but a test can still make the run depend on external state and prose can still
misstate an otherwise valid observation.

## Goals / Non-Goals

**Goals:**

- Keep the local coverage gate offline and repeatable.
- Make every exact coverage observation independently checkable.
- Preserve one coverage-policy owner and one claim-to-Chronicle digest binding.

**Non-Goals:**

- Persist every transient coverage profile in Git.
- Turn a local gate into hosted-CI, publication, installation, or runtime proof.
- Change self-update fallback behavior or the greater-than-95-percent policy.

## Decisions

### Tests own their complete external boundary

The GitHub fallback test supplies a transport that accepts only the exact
official API request and returns 404 in memory. A local server URL cannot model
the production-only `github.com` fallback predicate, and a real public request
is not a unit-test fixture.

### Behavioral branches use explicit fixtures

The doctor test constructs an enabled Codex adapter with an executable and no
targets, then asserts its bounded diagnostic. Coverage is a consequence of the
behavior contract, not a target reached through unrelated state.

### Quantitative evidence is source-bound

The dated record carries source commit, source tree, covered statements, total
statements, and the derived two-decimal percentage. The governance test verifies
the Git object binding, recomputes the percentage, and verifies the claim SHA-256.
The durable policy continues to own only the strict threshold.

## Risks / Trade-offs

- A historical source commit can later become unreachable from a branch. The
  evidence check resolves the Git object directly, so missing history fails
  closed.
- A percentage can round to the same value for different counts. Raw counts are
  mandatory and remain the primary observation.

## Migration Plan

First make the two focused tests deterministic and commit that source state.
Then run the full gate against that commit, correct the dated record and claim,
archive this change, and execute a fresh exact-HEAD proof. Revert both commits if
the full gate cannot be reproduced.
