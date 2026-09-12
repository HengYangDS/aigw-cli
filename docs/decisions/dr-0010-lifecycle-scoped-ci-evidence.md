# DR-0010: Scope CI Evidence to Product Lifecycle Stages

- Status: accepted
- Date: 2026-08-22

## Context

A proposal push and its review update can request equivalent verification.
Conversely, omitting accepted-branch verification assumes every `dev` update
has reviewed evidence for that exact object. That assumption fails for direct
maintainer pushes and cannot be repaired by merely observing a green `main`.
The current system has no cross-event evidence-reuse verifier.

## Decision

[Workspace roles](../../.ethos/workspace.toml) own accepted integration and
release branch names. The [CUE graph](../../.config/ci/pipeline.cue) consumes
that native TOML and owns event routing; generated files own no policy.

| Event                                                            | Required execution                                              |
| ---------------------------------------------------------------- | --------------------------------------------------------------- |
| Open or update a proposal review into `dev`                      | Complete source verification for the review's selected object   |
| Open or update an integration review into `main`                 | Complete source verification for the review's selected object   |
| Push `dev`, including a maintainer fast-forward or merged review | Complete source verification for the resulting accepted object  |
| Push `main`                                                      | Complete source verification plus exact `main`/`dev` ref parity |
| Release tag                                                      | Declared signed-source, artifact and publication graph          |
| Explicit manual dispatch                                         | Declared diagnostic or release workflow with explicit inputs    |

Proposal branch pushes do not start a second graph alongside review events.
Review admission includes both integration and release targets on both Forges;
GitLab workflow admission and verification jobs consume the same conditions.
Release-review checks do not require accepted-ref parity before the merge:
that observation belongs to the resulting release-branch push.
Accepted and release events do run separately, even when their object IDs match:
no job currently consumes and verifies another event's complete evidence. Never
silently omit the `dev` route to simulate deduplication. Every job measures the
exact Git object selected by its event; the event alone is not proof of success.

## Consequences

Review updates invalidate earlier green results. Maintainer integration does
not need a fabricated review event, and accepted `dev` receives real checks.
Ref parity belongs to release promotion, so an ordinary `dev` update need not
already equal `main`. Separate accepted and release verification costs runner
time; this is explicit, not a claim that all duplicate execution is eliminated.

## Revisit Trigger

Revisit if either Forge gains a portable, immutable evidence-reuse primitive
that can bind one successful graph to another lifecycle stage without rerunning
or weakening any gate.
