## Context

The release lane's head `044445461` was proven twice, by a lane proof and by an accepted-root proof that a launchd job
signed as `agent:local:process:ethos`. ETHOS reads two passing proofs with different verifiers as a contradiction and
refuses the protected `main`, and a proof Attestation cannot be released. A new head needs a commit; this Change gives
it a real one.

## Goals / Non-Goals

**Goals:**

- Contributor guidance that would have caught the first candidate's failure before the release.

**Non-Goals:**

- A change of ETHOS's contradiction rule; reported to ETHOS's owner on 2026-11-24.

## Decisions

### State the scope of the real-client journeys where the acceptance is described

- **Rests on:** `tools/release/clients_*_test.go` carry `//go:build client_acceptance`; the candidate acceptance runs
  them through `accept-native --clients`.
- **Considered:** a test that fails when a JSON field the journeys read disappears; one paragraph of guidance.
- **Chosen because:** the journeys decode partial documents for their own assertions; a generic guard would repeat each
  document's schema beside its owner. The guidance names the search a Change author makes.
- **Applies to:** `CONTRIBUTING.md`, section "Native upgrade acceptance".

## Risks / Trade-offs

- Guidance relies on the author reading it; the candidate acceptance remains the gate that catches a miss.
