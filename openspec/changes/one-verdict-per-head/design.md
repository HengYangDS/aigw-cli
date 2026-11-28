## Context

The proposal gives the rule, the accepted requirement it contradicts and the cost measured on `codex-probe-once`. The
route that the rule produced for that Change: a full check of the implementation head; a full proof of the head that
ticked the check, the native run and the proof; the proposal published and GitHub's four checks passed on that head;
then the archive commit, a second full proof and the same four checks again, because GitHub's protection of `dev`
requires them on the exact published head. `tools/ci source`, the check, runs the quality graph and the coverage gate,
which are the proof's own gates (`.ethos/profile.toml`, `tools/ci/ethos_profile_test.go`).

## Goals / Non-Goals

**Goals:** one proof per Change, of its archive head; one hosted run per forge per Change; an archive that an
unavailable peer does not hold.

**Non-Goals:** those of the proposal.

## Decisions

### D1. The proof, hosted CI and publication are landing gates, not tasks

- **Rests on:** ETHOS refuses to land a head without its own passing full proof, and refuses an archive while any task
  is open; GitHub refuses a push to `dev` without its required checks on that commit. Each of those owners already
  enforces and records its result. A task can only be ticked by a commit, which is a new head that the proof and
  GitHub must judge again.
- **Considered:** (a) keep the tasks; (b) tick them in a records Change after landing, which needs its own lane and
  proof for every Change, as the 2026-11-06 Changes did; (c) leave them out of the tasks and let their owners gate the
  landing.
- **Chosen because:** (c) is the only one in which each head is judged once and the archive does not wait for a peer;
  it is what the accepted scenario "External delivery follows local readiness" already requires.
- **Applies to:** the task rules in `openspec/config.yaml`, for every Change created after this one lands.

### D2. What still guards a publication

GitHub still runs its required checks on the archive head before `dev` moves, and ETHOS still proves that head before
it lands. A Change that once found a hosted failure only after its local gates now meets it at the archive head; the
cost is one more commit and proof, the same as a failure found today after the tasks were ticked. A peer that runs
on this host's guests, GitLab, starts its pipeline after the proof has passed, so a failure that the proof finds takes
no guest time.

## Risks / Trade-offs

- A hosted failure arrives after the archive → fix in the same lane, archive again with ETHOS's compensation, prove the
  new head (D2).
- Fewer tasks record what happened → the forges keep their CI records and ETHOS its proof attestations, which is where
  the rules already keep execution history.

## Migration Plan

None for the product. Active Changes created under the old rule keep their tasks; Changes created after this one
lands follow the new rule. Rollback is a revert of the Change.
