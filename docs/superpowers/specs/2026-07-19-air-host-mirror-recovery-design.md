# Air host-mirror and orphan recovery design

## Status

Approved implementation design for the RC.67 change set. This document is a
design carrier, not release evidence and not authority to mutate a live Air
installation.

## Decision

Teach AIGW to distinguish a JetBrains-owned Air copy of the admitted standalone
Codex projection from a true, unattributed Air orphan. Add a read-only
`aigw route attest air` command for bounded runtime evidence, and add a
case-bound `recover-orphan` / `settle` workflow for the exact orphan shape.

The workflow does not broaden AIGW's Air authority. Air remains owned by
JetBrains AI. AIGW does not edit Air sessions, JSONL, SQLite, model metadata,
credentials, logs, processes, or provider services. It never invents a
JetBrains selection.

## Problem

`InspectCodexConfig` currently classifies any AIGW marker without an adjacent
sidecar as `orphaned-aigw-marker`. That is correct for unattributed residue, but
too broad for Air. Air can mirror the standalone Codex configuration into its
own JetBrains-owned configuration. The mirror contains valid AIGW selection
bytes but no Air-side AIGW sidecar because AIGW did not write that copy.

Treating the mirror as an orphan creates unsafe guidance: an operator may be
invited to remove bytes that the host legitimately recreated. Treating every
marker as a mirror is worse because it would hide partial, stale, or foreign
residue. RC.67 therefore requires positive reference proof for a mirror and a
separate, ledgered recovery for a true orphan.

ADR-0003 remains unchanged. Its `route recover air` command still handles only
the recognized fallback-sidecar/full-selection mismatch. The new orphan path
is for a sidecar-absent exact generated projection that cannot be attributed to
the current standalone projection.

## Authority boundaries

- The standalone Codex CLI projection and its recognized sidecar are AIGW
  authority.
- Air's configuration, runtime selection, authentication, and process remain
  JetBrains authority.
- An Air host mirror is evidence that JetBrains copied an AIGW-owned
  standalone projection. It does not make the Air copy AIGW-owned.
- A true Air orphan is recoverable only because its AIGW-generated shape is
  exact and removal is explicitly acknowledged. Shape recognition does not
  confer general ownership over the file.
- Air logs are external evidence. AIGW may read the narrow forwarding record
  described below, but must ignore headers, request bodies, prompts, responses,
  tokens, session identifiers, and unrelated logger output.

## Exact host-mirror classification

### Canonical managed-projection fingerprint

The comparison is intentionally narrower than whole-file equality and stricter
than semantic TOML equality. A host may retain unrelated JetBrains settings,
but AIGW must not normalize or reinterpret the bytes it claims to recognize.

The `v1` fingerprint input is:

```text
"aigw-codex-full-selection-v1\x00"
+ normalized exact managed model_provider line + "\n"
+ normalized optional exact managed model line + "\n"
+ normalized exact generated provider block
```

Normalization changes only `CRLF` to `LF`. It does not trim spaces, reorder
keys, parse and re-emit TOML, omit the selected model, or normalize the endpoint.
The result is SHA-256 encoded as lower-case hexadecimal. The digest is internal
comparison material; route-doctor output does not expose it.

An exact generated full selection has all of these properties:

1. exactly one top-level
   `model_provider = "aigw" # managed by AIGW` line;
2. zero or one top-level `model = "..." # managed by AIGW` line;
3. exactly one complete `# >>> AIGW managed provider >>>` ownership region;
4. exactly one `[model_providers.aigw]` table with the current generated field
   order and values admitted by `exactCodexManagedBlock`;
5. exactly one matching end marker; and
6. no fallback marker/table, duplicate marker, partial block, unmanaged AIGW
   selection, or second AIGW provider table.

### `external-host-mirror`

Air is classified `external-host-mirror` only when every condition holds:

1. discovery identifies the present `jetbrains-air-codex` surface;
2. the Air sidecar is absent;
3. Air contains one exact generated full selection as defined above;
4. the canonical standalone `codex-cli-standalone` surface is present;
5. the standalone sidecar is recognized, writer `aigw-cli`, mode
   `full-selection`, and its managed-block hash matches the standalone block;
6. the standalone selection is itself exact and AIGW-managed; and
7. the two canonical managed-projection fingerprints are equal.

Whole-file equality is not required. A mirror remains external and
JetBrains-owned; `AIGWManaged` stays false and no Air target membership is
inferred.

### True orphan and other conflicts

An Air file with an absent sidecar and one exact generated full selection is a
true `orphaned-exact-full-selection` when the positive mirror proof above is
unavailable or differs. It is the only shape admitted to `recover-orphan`.

The generic single-file `InspectCodexConfig` helper may retain its existing
internal `orphaned-aigw-marker` result for compatibility. The Air-aware
two-surface classifier must never expose that generic name for this exact shape:
it maps it to `external-host-mirror` after positive reference proof or to
`orphaned-exact-full-selection` otherwise. Partial and foreign shapes keep
their distinct conflict states.

Partial markers, duplicate blocks, unmanaged AIGW selections, changed generated
fields, a present invalid/foreign sidecar, or a fallback block are not promoted
to a recoverable orphan. They remain their existing fail-closed conflict states.
If Air remains listed as an AIGW target, ordinary `repair --dry-run` must resolve
target membership before orphan recovery is admitted.

## Route-doctor behavior

`aigw route doctor` compares Air with the canonical standalone surface:

- `external-host-mirror` is healthy external JetBrains management. It does not
  set `report.ok` false and does not recommend mutation.
- a true exact orphan sets `report.ok` false and recommends only
  `aigw route recover-orphan air --dry-run --json`;
- ADR-0003's recognized mismatch keeps the existing
  `recoverable-stale-full-selection` guidance;
- partial, foreign, listed, and other ownership conflicts keep the ordinary
  fail-closed repair guidance.

If a recovery ledger is active, doctor also reports its bounded lifecycle
state. It never prints a path, configuration body, endpoint, model, case
quarantine content, or raw digest.

## Secret-free runtime attestation

Add:

```text
aigw route attest air --json
```

The command is read-only, credential-free, and lock-free. It does not execute
Air, contact a provider, inspect a process, read sessions, or claim a terminal
reply, authentication, or billing result.

### Evidence source and time binding

On macOS the source is `~/Library/Logs/JetBrains/Air/air.log`. Rotated
`air1.log` through `air9.log` may be read only to find the beginning of the same
selected PID generation; events from different PID generations are never
combined.

An admitted line must have the current anchored timestamp-and-PID prefix, exact
logger `CodexOpenAiApiRouterServer`, and exact message shape:

```text
Forwarding CallTraceId(id=<bounded-id>)/POST:/responses to <absolute-URL>
```

All other lines are ignored, especially `Headers:` and `Request body:` lines.
The parser caps line length and total scanned bytes and never includes a raw log
line in output or an error.

The current `air.log` selects the latest admitted Air generation token. The
evidence window starts at the earliest admitted forwarding event for that same
generation and ends at its latest admitted event. Rotations may extend the
window only for that exact generation. The latest event must be no more than 24
hours old. Future-dated, stale, malformed, or absent evidence yields
`runtime_authority: "unknown"`; it is not an error that authorizes recovery.

### URL classification and redaction

The parser uses `net/url` and compares the complete normalized AIGW route
identity internally: scheme, host, explicit/default port, and path prefix.
JetBrains admission is HTTPS and an exact host `jetbrains.ai` or suffix
`.jetbrains.ai`, never a substring match. Every other admitted authority is
counted as other.

The JSON contract is `AirRuntimeAttestation`:

```json
{
  "surface_id": "jetbrains-air-codex",
  "configuration_state": "external-host-mirror",
  "state": "host-mirror-runtime-attested",
  "runtime_authority": "jetbrains-ai",
  "window_start": "2026-07-19T00:00:00Z",
  "window_end": "2026-07-19T00:10:00Z",
  "request_count": 4,
  "jetbrains_request_count": 4,
  "aigw_request_count": 0,
  "other_request_count": 0,
  "host_hashes": ["<sorted SHA-256 hex>"],
  "host_authentication": "not-probed",
  "billing_evidence": "unknown",
  "evidence_source": "air-log",
  "read_only": true
}
```

`observed_process_start` is omitted in RC.67 because forwarding records do not
prove an operating-system process start. Hashes use a versioned domain
separator and the normalized route authority; raw host, URL, port, path,
endpoint, model, generation token, CallTraceId, file path, token, prompt, and
response text are never emitted.

Authority aggregation is exact: JetBrains-only is `jetbrains-ai`, configured
AIGW-only is `aigw`, JetBrains plus AIGW or any recognized route plus other is
`mixed`, and zero events or other-only is `unknown`.

`state` is `host-mirror-runtime-attested` only when configuration classification
is `external-host-mirror` and the fresh selected generation is JetBrains-only.
An AIGW-only, mixed, other-only, stale, or absent generation remains
`host-mirror-runtime-unattested`; a non-mirror is `not-a-host-mirror`. This is an
ephemeral, log-bounded observation, not a persisted authority grant or proof of
authentication, billing, or a terminal response.

## Orphan recovery ledger and quarantine

### Storage

Use AIGW's platform data directory:

```text
<data-dir>/recovery/air/ledger.json
<data-dir>/recovery/air/quarantine/<case-id>/config.toml
```

Directories are `0700`; files are `0600`. The quarantine file contains the
byte-exact Air preimage. The ledger records its original POSIX mode so an
in-process compensation can restore the preimage exactly. Quarantine is a
private artifact, not a public command.

The ledger is secret-free and contains no path, endpoint, URL, model, token, or
configuration content. Its schema contains:

```text
schema_version
surface_id
recovery_generation
case_id
state
created_at
recovered_at
settled_at
projection_fingerprint_sha256
config_preimage_sha256
config_preimage_mode
cleaned_postimage_sha256
observed_roundtrip_sha256 (optional)
quarantine_sha256
```

### Case identity

The next generation is the previous ledger generation plus one, beginning at
one. The preview case ID is deterministic for the captured preimage:

```text
air-%06d-<first-12-hex-of-config-preimage-sha256>
```

Apply must receive that exact `--case-id`. A changed preimage, generation, or
ledger invalidates the preview and causes a no-write refusal.

### States

The persistent state machine is:

```text
none -> prepared -> awaiting-host-roundtrip -> settled
                      |
                      +-> quarantined
```

- `prepared` is an internal resumable journal state. The quarantine exists and
  the exact case is recorded, but the final cleaned postimage is not yet
  committed or finalized.
- `awaiting-host-roundtrip` means the exact orphan was removed and the host has
  not yet produced acceptable new state.
- `quarantined` means a later observation contains unexpected, partial, or
  different AIGW residue. The quarantine is retained and no Air mutation is
  attempted.
- `settled` means an acceptable host roundtrip was observed, the ledger was
  reduced to hashes/timestamps, and the quarantine payload was removed.

While awaiting, doctor may derive `reappeared-after-recovery` when the same
managed projection fingerprint appears again. That derived state is not itself
a persisted ledger state. It becomes settleable only if the current standalone
reference now proves an exact `external-host-mirror`.

## `recover-orphan`

Commands:

```text
aigw route recover-orphan air --dry-run --json
aigw route recover-orphan air \
  --case-id <preview-case-id> \
  --confirm-host-idle \
  --ack-unset-external-selection
```

There is no `--force`. Dry-run takes no mutation lock and writes nothing. Apply
requires all three bindings: exact case ID, operator attestation that Air is
idle, and explicit acknowledgement that recovery removes the AIGW selections
without writing a replacement external selection.

Admission requires a true exact orphan, no sidecar, no Air AIGW target
membership, and no active unsettled case for a different preimage. The cleaner
removes only the exact managed `model_provider`, optional managed `model`, begin
marker, exact provider table, and end marker. All unrelated bytes and mode are
preserved. The result is an unset external baseline.

The transaction prepares every snapshot first, then uses guarded writes in this
order: quarantine, `prepared` ledger, cleaned Air config, and
`awaiting-host-roundtrip` ledger. A failure compensates unchanged postimages in
reverse order. A retry recognizes the same deterministic case and safely
resumes a `prepared` journal. The existing transaction helpers are best-effort
preimage guards, not a cross-process CAS; `--confirm-host-idle` remains
mandatory.

## `settle`

Commands:

```text
aigw route settle air --case-id <case-id> --dry-run --json
aigw route settle air --case-id <case-id>
```

Settle never changes the Air configuration. It requires the active case and:

- rejects without writing when Air still equals the cleaned postimage, because
  no host roundtrip is proven;
- settles an external-clean host rewrite;
- settles an exact reference-matching `external-host-mirror`, including a
  safely recreated projection;
- records `quarantined` and retains the payload for partial, changed, or
  unattributed AIGW residue; and
- refuses a stale case ID or changed ledger/quarantine preimage.

Successful settle records the observed postimage hash, timestamp, and
`settled`, then removes only the matching quarantine payload with guarded,
reverse-compensated writes. The hashes-only ledger remains as the recovery
chronicle.

## Error handling and idempotence

- Every classification and preview fails closed on unreadable or changing
  files.
- Dry-runs do not create directories, locks, ledger files, or quarantine.
- A completed mirror classification, attestation, recovery preview, or settle
  preview is safe to repeat.
- Reapplying an already finalized recover command reports the active case
  rather than creating a new generation.
- Reapplying a settled case is an idempotent `already-settled` result.
- Raw paths and file contents remain in internal errors only where the current
  CLI redaction boundary already permits them; public JSON remains path-free.

## TDD and verification

Implementation proceeds test-first at each boundary:

1. adapter fingerprint and mirror/orphan classification;
2. doctor routing and path-free output;
3. log parser freshness, PID generation, URL classification, caps, and
   redaction;
4. read-only `route attest` command and mutation-lock classification;
5. ledger generation, deterministic case ID, recovery admission, journal
   resume, guarded rollback, and quarantine permissions;
6. CLI acknowledgements, exact case binding, settle outcomes, and no Air write;
7. governance documents, release chronicle, and the complete repository gate
   bundle.

Tests must inject concurrent preimage changes and write failures at every
recovery artifact boundary. They must also scan JSON and errors for paths,
URLs, raw hosts, models, CallTraceIds, headers, bodies, and credential-shaped
values.

## RC.67 release constraints

- Keep `## [Unreleased]` first, then add
  `## [0.1.0-rc.67] - 2026-07-19` to `CHANGELOG.md`.
- Do not edit `internal/cli/root.go`; `0.1.0-dev` remains the development
  fallback and packaging injects `0.1.0-rc.67` with linker flags.
- Keep Go `1.25.12` and existing CI, forge-source, and signer manifests unless a
  separately scoped change requires them.
- Build with `SOURCE_DATE_EPOCH=1784419200`, derived from the committed release
  heading, and require two byte-identical 15-artifact matrices on the dedicated
  macOS arm64 release runner.
- Create independent SSH-signed annotated `v0.1.0-rc.67` tags for GitLab and
  GitHub using their respective identities and trust anchors. Never copy a tag
  object between forges.
- Stop if an RC.67 GitLab package or Release already exists; the existing
  release reuse path does not prove local-to-remote byte equality.
- Publish GitLab first, then GitHub only after the GitLab release succeeds.
  Accept the release only after both provider pipelines and post-publication
  artifact comparison pass.
- RC.67 may claim checksum-verified prerelease artifacts and an SPDX SBOM. It
  must not claim GA signing, notarization, host-enforced GitHub tag immutability,
  runtime authentication, billing, or a user-visible Air reply.

## Rejected approaches

1. **Treat every sidecar-absent marker as a mirror.** Rejected because it hides
   true orphan, partial, and foreign residue.
2. **Require whole-file equality.** Rejected because JetBrains may retain
   unrelated host-owned settings; only the exact managed projection is relevant.
3. **Delete an orphan directly from route doctor.** Rejected because diagnosis
   is observational and a destructive baseline change needs preview, case
   binding, quarantine, idle attestation, and explicit acknowledgement.
4. **Restore a historical or guessed JetBrains selection.** Rejected because a
   backup is not proof of the current external baseline.
5. **Read headers or request bodies for runtime proof.** Rejected because those
   log lines may contain credentials, prompts, or response material.
6. **Add `--force` or a public quarantine editor.** Rejected because either
   bypasses the bounded recovery state machine.
