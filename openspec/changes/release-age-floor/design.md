## Context

The dependency policy is the one owner of release ages. Its root declared three days, which covered npm, and two
package rules set the age to zero for sources that cannot replace an artifact under the same identity. The
candidate calculation reads that policy, and a refresh takes what it reports.

The owner's decision of 2026-11-13 came in two steps on one evening. First, every source waits one day; this lane's
first commit encoded that. Then, for every governed repository: the age is zero, declared as one configurable value
in hours. The Change keeps its first name, which a lane cannot change after its first commit.

## Goals / Non-Goals

**Goals:**

- One place declares the release age, in hours, for every source.
- Raising or lowering it is a one-line change that no test has to follow.
- A rule that declares its own age fails the repository's checks.

**Non-Goals:**

- A second enforcement point, such as an age setting of npm or Go itself. Two owners of one rule would drift.
- Taking the releases that an age of zero now admits. They move in the next refresh, each with its own admission.
- Removing the admission of a release: signatures, the lock's checksum or digest, the checks and the proof stay.

## Decisions

### The root value is the declaration

`minimumReleaseAge` at the root of the policy is the repository's release age, written as a whole number of
hours. The two package rules that only carried an age are removed, so every source follows the root.
`vulnerabilityAlerts` keeps an age of zero hours: a security fix skips the age however high the root is set.

Rejected: a separate configuration file that generates the policy, which adds a generator for one value; and
keeping per-source ages, which the owner's decision replaces.

### The test reads the declaration, not the number

`TestRenovateDeclaresTheReleaseAgeOnce` walks the whole policy. It passes when the root declares an age in hours,
the security-fix rule declares zero hours, and nothing else declares an age. It passes for `24 hours` as for
`0 hours`, and fails for `3 days`, for a rule with its own age, and for a security-fix age above zero.

### A release without a publish time

The policy keeps `timestamp-required`. At zero it has no effect. Above zero, a registry that publishes no release
time, as GitHub's container registry does for the dependency tool's own image, leaves its releases held, and a
refresh takes such a release only after reading its age from the publisher's release record.

## Risks / Trade-offs

- With an age of zero, a release that its publisher withdraws within hours can already be locked. The lock's
  checksum, the repository's checks and the proof still stand between a proposal and integration, and the age is
  one line away from a higher value.
- npm can unpublish a release within three days. A lock that names an unpublished release fails to install, which
  the next check reports.
