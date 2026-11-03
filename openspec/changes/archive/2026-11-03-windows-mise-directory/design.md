## Context

GitLab's Windows ARM64 runners have no Mise in their job images: each job
verifies a runner-owned Mise executable and shim against the CUE pins, then
gives Mise a job-owned directory for its data. That executable lived in one
directory that four projects share.

## Goals / Non-Goals

**Goals:** AIGW's jobs read a Mise that only AIGW's declaration changes, with
the same digest and version checks.

**Non-Goals:** moving the other projects, a new Mise release, or changing the
job-owned Mise data directory.

## Decisions

- The directory sits under Program Files, which keeps the executable read-only
  for job identities (Users read and execute), and its name, `mise-aigw-cli`,
  says whose declaration updates it.
- One CUE value names the directory; the job script derives the executable and
  shim paths from it.
- AIGW leaves the shared directory in place for the projects that still use it.

## Risks / Trade-offs

- Until the other projects move, the host keeps two copies of one Mise release,
  about 200 MB.
- An AIGW Mise refresh still replaces this directory in a window without
  running jobs, as the supply-chain specification requires, but no longer
  touches other projects.
