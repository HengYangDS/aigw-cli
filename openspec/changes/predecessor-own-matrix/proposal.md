## Why

The 0.11.0 candidate's acceptance stopped on 2026-11-27 at 17:21, before any product ran: "verify published baseline:
release artifact matrix: unexpected or missing files". Native acceptance checks the published predecessor against the
current release's platform matrix. 0.11.0 dropped the Intel macOS and Windows archives
(`drop-windows-and-intel-macos`), so v0.10.0, published with five archives, can never match the three that the
current matrix names. The rebuilt provenance of the predecessor names the same current matrix, and the native input
package selects the predecessor's files by it, so both fail for the same reason. A release whose matrix changes can
therefore never be accepted against its predecessor, locally or in a hosted lifecycle run.

## What Changes

- A published predecessor is verified against the matrix that its own signed checksum manifest names: the manifest's
  signature by the release signer, every named file present with its digest, no other file, and every name bound to
  the predecessor's version.
- The predecessor's provenance is rebuilt from its signed source with the subjects that its manifest names, in the
  manifest's order, and must equal the published provenance byte for byte, as before.
- The native input package takes the predecessor's files under `baseline/` as the predecessor's manifest names them;
  the candidate's files stay those of the current matrix.
- The current release's own verification does not change: a candidate and a tagged release are still checked against
  the current matrix.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ci-diagnostics`: a published predecessor is verified against its own signed matrix, which may name other platforms
  than the current release.

## Impact

`internal/program/artifact` (the published matrix), `tools/release/provenance` (the predecessor's matrix and
provenance), `tools/release/construction` (the baseline preparation and the native input package) and their tests.
No product command changes; no user-visible change, so no changelog entry.
