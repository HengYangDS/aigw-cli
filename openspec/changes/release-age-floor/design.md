## Context

The dependency policy is the one owner of release ages. Its default of three days covers npm, whose registry can
still unpublish a release. Two package rules set the age to zero for sources that cannot replace an artifact under
the same identity: Go modules, Mise tools locked by checksum, and Actions, images and the Hermes tag pinned by
commit or digest. The candidate calculation reads that policy, and a refresh takes what it reports.

## Goals / Non-Goals

**Goals:**

- No release enters the repository before it is one day old, whatever its source.
- The policy stays the single owner of the age; a test fails when a rule drops below one day.

**Non-Goals:**

- Changing npm's three days, or the rule that a security fix skips the age.
- Moving back pins that the old rule admitted. uv 0.13.8, gh 2.104.0, goreleaser 2.20.2 and the dependency
  tool's image 44.236.5 were taken at 3 to 13 hours old; the last of them is one day old at 2026-11-13T23:45Z, so
  they stay.
- A second enforcement point, such as an age setting of npm or Go itself. Two owners of one rule would drift.

## Decisions

### One day in the two rules that held zero

The two package rules for locked sources change from `0 days` to `1 day`. The policy's default, npm's three days,
and the zero age of `vulnerabilityAlerts` stay. A checksum or digest proves that a release is the one its publisher
served; it does not prove that the release is sound. One day gives the publisher and its first users time to
withdraw a bad release.

Rejected: one common age of three days for every source, which the owner did not ask for and which would delay
toolchain fixes; and an age only in the refresh procedure, which no check would hold.

### A release without a publish time is held

The policy keeps `timestamp-required`. A registry that publishes no release time, as GitHub's container registry
does for the dependency tool's own image, leaves its releases held by the candidate calculation. A refresh takes
such a release only after reading its age from the publisher's release record. Rejected: `timestamp-optional` for
images, which would propose an image minutes after its push.

### The policy test reads every age

`TestRenovateHoldsEveryReleaseForADay` walks the whole policy and compares every `minimumReleaseAge` with the
expected set: three days at the root, zero for security fixes, and one day in the two rules. A new rule with a
lower age fails it.

## Risks / Trade-offs

- A fix that is not a security advisory waits one day. The advisory path stays open for those that are.
- The dependency tool's own image no longer appears as an update; a refresh has to look it up. The specification
  states this case, so it is not silent.
