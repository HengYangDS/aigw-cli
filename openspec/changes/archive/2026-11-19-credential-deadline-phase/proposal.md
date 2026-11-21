## Why

A native credential operation that reaches its five-second deadline reports only "context deadline exceeded". It does
not say whether the credential subprocess had been created, had been started, or had run without answering. On the
GitLab Windows guest this deadline has ended tests of the native credential path on several days since 2026-11-05, in
the release pipelines of 0.8.1, 0.8.2 and 0.8.3 among them, and each time one retry passed. Every reading of the cause
so far is a guess, and the decision about the bound itself has waited since 2026-11-12 for a failure that names its
step. A person whose `aigw` reports the deadline on a slow machine reads the same bare sentence.

## What Changes

- A native credential operation that reaches its deadline says where it stood: its subprocess was not created in time;
  it was created and not started in time; or it was started and did not answer in time. It gives the time of each step
  that ended within the deadline and, when known, the time at which the step that missed it ended.
- The process runner tells a plan's caller when the operating system has created the child, before the runner owns the
  child and reports it started.
- `aigw credential` shows where the operation stood as the evidence of "Token read exceeded its deadline", and its fix
  names the credential service only when the subprocess was started; otherwise it says that the subprocess was not
  started in time.

The deadline stays five seconds, the failure stays a deadline for every caller that tests for one, and nothing is
retried. No behavior is removed.

Authority: `native-credential-store` owns the bound and what its failure says; `internal/platform/process` owns when a
child exists and when it is started. Reuse: the runner's existing start observation, with one more optional
observation beside it; no dependency is added.

Out of scope: the bound's length and the moment at which it starts, which are decided from failures that name their
step; the steps inside the subprocess, its own start and the call into the credential store, which stay one step here;
and the other fixed time bounds of the test suite.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `native-credential-store`: a deadline names the step at which the operation stood and its times.

## Impact

`internal/platform/process` (one optional observation of a plan), `internal/secret/native` (the deadline's error),
`internal/cli/credential` (the evidence and the fix of its deadline problem) and `CHANGELOG.md`. No JSON field, state or configuration changes.
