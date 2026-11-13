## Context

The candidate calculation is the dependency policy run against the publishers' datasources. At `2741f81f`, with
the release age at zero hours, it reports: Codex 0.169.0 and Claude Code 2.1.325 (pinned 0.167.0 and 2.1.319),
Prettier 3.10.1, jsdom 31.0.1, the Renovate image 44.239.1, the Mise image 2026.11.8 and `go.yaml.in/yaml` v4.0.0.
It holds nothing by age.

AIGW owns a verified Mise executable on GitLab's macOS and Windows runner hosts. The CUE authority pins its version
and digests, and each job there refuses another executable before any locked tool runs.

## Goals / Non-Goals

**Goals:**

- Take every reported update that needs no host change and no source migration.
- Leave the two others with a stated reason, an owner, a next action and a date, not as silent gaps.

**Non-Goals:**

- A host window. Replacing AIGW's Mise on two hosts is host work with its own announcement, and a failed attempt
  would stop every macOS and Windows job of this repository.
- The YAML migration. It rewrites the import path in every consumer and can change emitted bytes, so it needs its
  own tests and review.
- A release. No released program contains these inputs.

## Decisions

### The clients move together and pass their journeys first

Codex and Claude Code move in one step, so one Windows supplier name covers both. Claude Code published 2.1.324
and 2.1.325 within two hours on 2026-11-13; the pin takes the newest at the calculation. The journeys on Linux,
macOS and Windows run with the new pins on the proposal before the forges integrate them, as the requirement asks.
A journey that fails keeps the earlier pin for that client.

Claude Code also publishes a `stable` tag, 2.1.311 on that day, which is older than the pinned release. The pin
follows the `latest` tag, as every earlier refresh did.

### A runner-owned tool waits for its window

The requirement already says that a runner's executable is replaced in a window before any job consumes a new
declaration. It did not say what a refresh does before that window. The refresh keeps the pinned release and names
the window. The alternative, pinning 2026.11.8 now, would make every macOS and Windows job fail its preflight
until the hosts change.

Owner: the CI infrastructure owner. Next action: replace AIGW's Mise on both hosts in an announced window, then
advance the pin, the image and the digests in one Change. Review: 2026-11-17.

### The YAML major gets its own Change

`go.yaml.in/yaml/v4` is a new module path. The product reads and writes client configuration through v3, so a
migration is a product change with byte-level acceptance of what it emits. Owner: the AIGW session. Next action:
open a Change that migrates the importers and compares emitted files byte for byte. Review: 2026-11-17.

### The lock refresh applies the declared age

The policy proposes no update for a package that only the lock pins. The refresh therefore updates the lock
within its declared ranges itself, with no cut-off date because the age is zero, and checks registry signatures and
the audit afterwards.

## Risks / Trade-offs

- At a release age of zero, a client release can be an hour old when it is pinned. The journeys are the gate; no
  released program depends on these pins.
- Mise stays one release behind until the window.
