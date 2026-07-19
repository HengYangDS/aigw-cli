# ADR-0004: Treat Exact Air Copies as External Host Mirrors and Recover Only Exact Orphans

- Status: accepted
- Date: 2026-07-19
- Owner: Yang HENG

## Context

The standalone Codex CLI projection is AIGW-owned. Air, PyCharm Codex, and
Junie CLI remain JetBrains AI surfaces. Air may copy the standalone projection
bytes into its own configuration without copying AIGW's sidecar. Equal bytes
or markers do not transfer ownership to AIGW.

Treating every such copy as orphaned creates unsafe cleanup guidance. Treating
every AIGW marker as a legitimate host copy would hide partial or foreign
residue. The classification therefore needs positive reference proof and a
separate recovery path for the one exact unattributed shape.

This decision does not supersede ADR-0003. ADR-0003 covers a sidecar-present
fallback/full-selection mismatch through `aigw route recover air`. This
decision covers a sidecar-absent exact generated full selection through
`recover-orphan` and `settle`. It also preserves ADR-0002: no paid GitHub
feature or rewrite of an existing provider-native tag or Release is required.

## Decision

Classify Air as `external-host-mirror` only when its exact managed projection
has the same versioned fingerprint as a current standalone full-selection
projection with a recognized, hash-matching AIGW sidecar. Fingerprint
normalization changes CRLF to LF only. It does not trim whitespace, reorder
fields, parse and re-emit TOML, or expose the digest publicly. A host mirror
remains JetBrains-owned, is not AIGW-managed, and does not imply AIGW target
membership.

When the Air sidecar is absent and Air contains one exact generated full
selection without positive reference proof, classify it as
`orphaned-exact-full-selection`. Partial or duplicate markers, fallback data,
foreign or invalid sidecars, unmanaged selections, and changed generated
blocks remain fail-closed conflicts.

Provide the read-only command:

```text
aigw route attest air --json
```

Attestation reads only bounded Air forwarding records from one log generation.
It applies strict logger, message, URL, freshness, line-size, and scan-size
rules and ignores headers, bodies, prompts, responses, tokens, sessions, and
unrelated output. It is ephemeral forwarding evidence. It does not prove a
process start, JetBrains login or authentication, quota or billing, terminal
outcome, or a user-visible reply.

Provide the case-bound recovery workflow:

```text
aigw route recover-orphan air --dry-run --json
aigw route recover-orphan air --case-id <id> --confirm-host-idle --ack-unset-external-selection
aigw route settle air --case-id <id> --dry-run --json
aigw route settle air --case-id <id>
```

Apply requires the exact case ID, an operator attestation that Air is idle, and
an explicit acknowledgement that no replacement JetBrains selection will be
written. There is no `--force`. Case IDs use
`air-%06d-<first-12-preimage-sha256>` with generations from 1 through 999999.

Admission and settlement bind immutable snapshots of the Air config, Air
sidecar, standalone config, and standalone sidecar. Recovery stores the
byte-exact Air preimage in a private quarantine, removes only the exact orphan,
and leaves an unset external baseline. Settlement never writes Air. The
persistent lifecycle is `prepared`, `awaiting-host-roundtrip`, and `settled`;
unexpected residue becomes `quarantined`.

Recovery directories use mode `0700`; ledger and quarantine files use `0600`.
Complete recovery digests remain private ledger fields. Writes and removals are
guarded by captured preimages, and reverse compensation restores only this
transaction's unchanged postimages so it does not overwrite a concurrent
writer.

## Consequences

`aigw route doctor` treats a proven host mirror as healthy external management
without mutation guidance. It recommends ADR-0003 recovery for the recognized
sidecar mismatch, exact-orphan recovery for the sidecar-absent exact orphan,
and ordinary repair preview for other conflicts.

Recovery does not broaden AIGW's Air authority, stage an AIGW provider block,
authenticate a client, manage Air's lifecycle, or access conversation JSONL,
SQLite, model metadata, credentials, prompts, or responses.

## Evidence

Adapter, attestation, recovery, and CLI regression suites cover exact
classification, bounded log parsing, public JSON allowlists, immutable
four-surface snapshots, private permissions, crash resume, guarded rollback,
settlement, mutation locking, and path-free output. Repository and release
gates remain the publication authority.

## Revisit Trigger

Revisit this decision only if JetBrains changes Air's configuration authority,
provides a stronger host-owned attribution contract, or the product explicitly
admits Air as an AIGW-managed target.
