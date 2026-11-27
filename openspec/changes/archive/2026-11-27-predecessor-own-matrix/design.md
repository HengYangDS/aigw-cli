## Context

`artifact.Names(version)` is the release matrix of the current code: one archive per entry of `targets`, three
documents, the provenance, the manifest and its signature. Every verifier used it, including the one for a published
predecessor (`preparePublishedBaseline`), the predecessor's rebuilt provenance (`ProvenanceSubjects`) and the native
input package (`extractNativePackage`). That held while the matrix never changed. 0.11.0 removed three targets, and
the acceptance of its candidate against v0.10.0 stopped on the predecessor's six archives.

## Goals / Non-Goals

**Goals:**

- A predecessor is verified against what it published and signed, so a change of the matrix does not stop the
  acceptance of the release that makes it.
- Every check that applied to a predecessor still applies: the signer, each digest, no extra or missing file, the
  three dependency reports and the byte-exact provenance rebuilt from the signed source.

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
- **Applies to:** `artifact.ValidatePublishedMatrix`, `provenance.VerifyPublishedRelease`.

### Each name is bound to the predecessor's version

- **Rests on:** a manifest could name files of another version that a mistake placed beside it.
- **Chosen because:** the manifest must name at least one archive `aigw_<version>_<os>_<arch>` (`.tar.gz`, or `.zip`
  as earlier releases published for Windows), then exactly the documents that the current matrix names, in its order:
  the SBOM, the vulnerability and license reports, and the provenance. No document name depends on the platforms, so
  a predecessor without them is refused as before. A version cannot contain `_`, so an archive of another version,
  such as a later prerelease `aigw_<version>.1_…`, is never read as one of this version's. The manifest names no file
  twice. The archive of the running platform is then read from the verified directory, and its absence stops the
  baseline preparation.
- **Applies to:** `artifact.PublishedNames`, `artifact.PublishedFile`.

### The provenance subjects follow the manifest's order

- **Rests on:** the provenance lists its subjects in the matrix order, the manifest lists the same names in the same
  order followed by the provenance, and `provenance.go` has not changed since v0.10.0.
- **Chosen because:** the subjects of the published provenance are then the manifest's names without the provenance,
  so the rebuilt bytes are compared exactly as before.
- **Applies to:** `provenance.VerifyPublishedRelease`.

### One entry point verifies a predecessor

- **Rests on:** the predecessor's matrix and its provenance are verified together everywhere, and a second pair of
  exported functions beside `VerifyMatrix` and `VerifyProvenance` let a caller mix the current and the published
  forms.
- **Chosen because:** `provenance.VerifyPublishedRelease` checks the signed matrix and then the provenance, and its
  test builds a predecessor with a signed tag and an earlier platform: the function accepts it, the current
  release's two checks refuse it, and the test fails when the rebuild takes the current subjects. Its one caller,
  `preparePublishedBaseline`, is proved by the candidate acceptance against the published v0.10.0; a construction
  test would need a predecessor provenance that only the provenance package can write.
- **Applies to:** `provenance.VerifyPublishedRelease`, `preparePublishedBaseline`.

### Each document name has one owner

- **Rests on:** the build, the provenance verifier and the matrix each spelled `aigw_<version>.<kind>.json`.
- **Chosen because:** `artifact.DocumentName` names each document, and the matrix, the predecessor rule and the
  build derive their names from it.
- **Applies to:** `internal/program/artifact`, `tools/release/provenance`, `tools/release/construction`.

## Risks / Trade-offs

- A future change of the provenance format would stop the acceptance of its release against the predecessor in the
  same way; the rebuild then needs the predecessor's own format. Not addressed here.
- The native input package can no longer list the predecessor's files before reading them, so it takes every file
  under `baseline/` that can belong to the predecessor's matrix, each once, and leaves completeness to the matrix
  check that follows.
- The manifest is read once for its names and once for its signature, as before; only a writer inside the private
  scratch directory could change it between the two reads.
