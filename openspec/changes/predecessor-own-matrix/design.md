## Context

`artifact.Names(version)` is the release matrix of the current code: one archive per entry of `targets`, three
documents, the provenance, the manifest and its signature. Every verifier used it, including the one for a published
predecessor (`preparePublishedBaseline`), the predecessor's rebuilt provenance (`ProvenanceSubjects`) and the native
input package (`extractNativePackage`). That held while the matrix never changed. 0.11.0 removed three targets, and
the acceptance of its candidate against v0.10.0 stopped on the predecessor's five archives.

## Goals / Non-Goals

**Goals:**

- A predecessor is verified against what it published and signed, so a change of the matrix does not stop the
  acceptance of the release that makes it.
- Every check that applied to a predecessor still applies: the signer, each digest, no extra or missing file, and the
  byte-exact provenance rebuilt from the signed source.

**Non-Goals:**

- The current release's verification, which keeps the current matrix.
- A provenance format that changes between releases; none has since v0.10.0, and that case would need its own design.

## Decisions

### The predecessor's signed manifest is its matrix

- **Rests on:** the manifest is signed by the release signer, and it lists every checksummed file of the release in
  the order in which `RewriteChecksums` wrote it: the archives, the three documents, then the provenance.
- **Considered:** a table of earlier matrices by version; running the predecessor's own verifier; the predecessor's
  manifest.
- **Chosen because:** the manifest is the release's own signed declaration, so no second record of old matrices is
  kept; a table would repeat what each release already signed, and the predecessor's verifier is not available to a
  successor's acceptance without building old code.
- **Applies to:** `artifact.ValidatePublishedMatrix`, `provenance.VerifyPublishedMatrix`.

### Each name is bound to the predecessor's version

- **Rests on:** a manifest could name files of another version that a mistake placed beside it.
- **Chosen because:** every checksummed name of a release begins with `aigw_<version>` followed by `_` or `.`, and
  none holds a path separator; the manifest must name no file twice and end with the provenance
  `aigw_<version>.provenance.json`, as `RewriteChecksums` writes it. The archive of the running platform is then read
  from the verified directory, and its absence stops the baseline preparation.
- **Applies to:** `artifact.PublishedNames`.

### The provenance subjects follow the manifest's order

- **Rests on:** the provenance lists its subjects in the matrix order, the manifest lists the same names in the same
  order followed by the provenance, and `provenance.go` has not changed since v0.10.0.
- **Chosen because:** the subjects of the published provenance are then the manifest's names without the provenance,
  so the rebuilt bytes are compared exactly as before.
- **Applies to:** `provenance.VerifyPublishedProvenance`.

## Risks / Trade-offs

- A future change of the provenance format would stop the acceptance of its release against the predecessor in the
  same way; the rebuild then needs the predecessor's own format. Not addressed here.
- The native input package can no longer list the predecessor's files before reading them, so it takes every regular
  file under `baseline/` once and leaves completeness to the matrix check that follows.
