# Design

## Context

`internal/claude/desktop` owns AIGW's Claude Desktop third-party profile, the
deployment modes and the configuration metadata entry. Its ownership record
holds a SHA-256 over the deployment modes, the applied identity, the metadata
entry and the raw profile bytes. A projection replaces the whole profile with
AIGW's encoding.

Claude Desktop 3.1623.0 ships its own editor for the same profile. Its schema
defaults Allow Cowork and Allow Code to true, treats a gateway without
`x-api-key` as bearer, and the editor omits those defaults when it saves. It
also stores Desktop-owned settings such as `desktopHome` in the profile.

## Goals / Non-Goals

**Goals:** a Desktop save that keeps AIGW's meaning is unchanged; a real edit
of a managed setting still refuses and names the setting; Desktop's own
settings survive AIGW projections; earlier AIGW releases keep reading the
ownership record.

**Non-Goals:** adopting edits of managed settings, changing withdrawal, or
modeling Desktop-owned settings.

## Decisions

### The profile's managed meaning is a projection request

AIGW reads the profile back into the request it would project: base URL,
credential helper and arguments, models with labels and tool search. The
remaining managed settings are AIGW's fixed policy. A missing Allow Cowork,
Allow Code or gateway authentication scheme takes Desktop's default; any other
missing or different policy value is a changed managed setting.

### The record keeps the projected request

After every write AIGW stores the request it projected in the ownership record
as `managed_request`. When the raw digest differs, AIGW compares the request
that the profile carries with the recorded one, so every later save by Desktop
is accepted and a difference names the changed settings.

### Re-encoding accepts records from earlier releases

A record from AIGW 0.4.8 or earlier holds only the digest of AIGW's own
encoding. AIGW then encodes the request it read with its own encoder and
computes the recorded digest over that encoding. A match proves that Desktop
changed only encoding and its own settings. A mismatch cannot be attributed to
a setting, so the refusal asks to restore the AIGW profile in Claude Desktop.

### Ownership record stays version 1

AIGW refreshes the raw digest over the bytes on disk after an equivalent save,
and records it after every write. Earlier releases read the same version and
digest and ignore the recorded request. An earlier release that projects again
replaces the whole profile, which is its documented behavior.

### Projection merges Desktop's keys

When the request changes, AIGW starts from the current profile, replaces the
managed keys with its encoding and keeps every other key. An unchanged request
keeps the current bytes.

## Risks / Trade-offs

- [A later Desktop changes a default] → that setting is then reported as a
  changed managed setting by name, and the defaults are one table to update.
- [An earlier release re-projects after this one] → Desktop's own settings are
  dropped as before; the next save by Desktop is again accepted here.

## Migration Plan

Release with the next AIGW version. The first `aigw sync` after an equivalent
Desktop save refreshes the ownership record without rewriting the profile.

## Open Questions

- None.
