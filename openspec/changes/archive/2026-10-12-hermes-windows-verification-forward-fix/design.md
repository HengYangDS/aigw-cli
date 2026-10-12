# Design

## Context

See [the proposal](proposal.md). GitHub Actions run `38235388359`, job
`114764677715` (Native Windows acceptance), failed on `a278c443` with
`credential_policy_test.go:195: external client failure = Hermes verification
cannot isolate native launcher publication on Windows; no client process was
started`. Every other package in that job and the other four required checks
passed on the same commit. The job stopped before its aggregate coverage gate.

## Goals / Non-Goals

**Goals:** Make the registry-level credential-policy test assert the accepted
Windows refusal, and state in the refusal scenario that it discloses no
credential diagnostics.

**Non-Goals:** Changing Hermes verification behavior, qualifying Windows
beyond the hosted gate, and repairing the GitLab tool mirror.

## Decisions

1. Branch on `runtime.GOOS == "windows"` for the Hermes subtest only, beside
   the existing Claude Desktop branch. Require the categorized refusal, zero
   client calls and no credential canary in the error. Rename the local
   `runtime` variable to `clientRuntime` so the `runtime` package can be used.
2. Keep the product behavior. The refusal is the accepted requirement, and the
   test expectation was incomplete.
3. Extend the existing refusal scenario instead of adding a requirement, so
   the requirement text and its scenario names stay unchanged.

## Risks / Trade-offs

- [The Windows branch only runs on a native Windows host] → `GOOS=windows`
  vet and lint run locally, and the pull request's Native Windows acceptance is
  the acceptance check.
- [Aggregate Windows coverage is still unmeasured] → The failed job stopped
  before its coverage gate, so the next hosted run measures it first.

## Migration Plan

No data or configuration change. Rollback reverts this Change. Proof,
candidate landing, archive, closeout and publication are governed effects
recorded in ETHOS Attestations.

## Local Qualification

The failing evidence is the hosted Native Windows acceptance of `a278c443`
(run `38235388359`, job `114764677715`). The test's Windows branch runs only on
a native Windows host. The fix was therefore compiled, vetted and linted for
darwin, linux and windows targets, and `internal/client` passed on macOS.
`mise run check` and `mise run native` passed on the staged tree with 95.12%
aggregate statement coverage (15,078 of 15,851 statements). The next hosted
Native Windows acceptance is the passing check.
