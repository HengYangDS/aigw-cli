## Context

The CI model generates both Forge files. Four GitHub upload steps and two GitLab artifact declarations retain
dependency, source, coverage, lock and performance evidence. No workflow downloads an artifact: a person or an
agent reads them after a failed job or a dispatched run.

## Goals / Non-Goals

**Goals:**

- One value, declared once, that both projections follow.
- A test that fails when a new upload forgets it.

**Non-Goals:**

- Deleting what exists. That is an operation on each Forge, done on 2026-11-14.
- A lifetime per artifact kind.

## Decisions

### Seven days

The owner's deletion used seven days, and the sibling proxy repository declares seven days for its uploads. No
measurement says how late evidence is read; the value is one field, and a longer need changes that field.

### One field in the model

`evidenceRetentionDays` sits beside the evidence paths. The GitHub steps read it as a number and the GitLab
declarations as `"7 days"`, built from the same field, so the two projections cannot differ.

### The test reads the projections

It walks every step that uses the upload action and every job that declares artifacts, in the generated files,
and compares each with the model's value read from the same generation. An exemption would be a name in the
test; there is none.

## Risks / Trade-offs

- **Evidence of an old failure is gone when someone looks later** → a failed job is rerun for its evidence; the
  job log, which keeps the failing lines, follows the Forge's own log retention.
- **A release's dispatch evidence is wanted after more than a week** → the release route copies what it cites
  into the repository's own evidence folder when it reads the run.
