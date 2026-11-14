## Context

The CI model generates both Forge files. Four upload step definitions and two artifact declarations of the
model, which generate 14 GitHub steps and 6 GitLab declarations, retain dependency, source, coverage, lock and
performance evidence. No workflow downloads an artifact: a person or an
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

### The test reads the rendered projections

It renders the projections from the model, as the drift check does, and walks every step that uses the upload
action in every GitHub workflow and every GitLab job that declares artifacts. Each must carry the model's value.
An exemption would be a name in the test; there is none. A step added by hand to a tracked file only is the
drift check's finding, not this test's.

## Risks / Trade-offs

- **Evidence of an old failure is gone when someone looks later** → a failed job is rerun for its evidence; the
  job log, which keeps the failing lines, follows the Forge's own log retention.
- **A release's dispatch evidence is wanted after more than a week** → the run's logs stay, and a release cites
  what it copied when it read the run.
