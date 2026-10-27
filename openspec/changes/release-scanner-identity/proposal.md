## Why

The independent review of the AIGW 0.4.18 release evidence accepted the
published dependency reports' content, which normalizes exactly from the
retained raw OSV report, but found that the evidence does not show which
scanner produced them: the arguments that actually ran, the executable image
the name resolved to, its version and the raw output it wrote. The execution
owner, `construction.executeTool`, retains streams and the exit status but not
the image it ran, and the provenance's tool versions are read from the Mise
declarations, not observed.

## What Changes

- The release tool's execution owner can observe the executable that a call
  runs: it resolves the name once, digests that file and runs it by path.
- A dependency scan first runs the scanner's version report and admits only the
  image whose SHA-256 the Mise lock records for the host platform and which
  reports the locked version; the scan observes its image again and must run
  the same file.
- Both dependency reports move to schema 2 and name the scanner: tool, version,
  platform, image SHA-256 and the scan arguments, with the scanned source and
  the scan's evidence directory written as `{source}` and `{evidence}`.
- Each scan retains an invocation receipt beside its raw report: the
  executable's path and SHA-256, its complete version report, the exact
  arguments and the raw report's SHA-256.
- Release provenance lists the scanner's locked release asset, its URL and
  SHA-256 from the Mise lock, as a resolved dependency. Provenance derives it
  from the reports and checks it against the source's lock, so verification
  refuses reports that name another image. Schema 1 reports name no scanner,
  and earlier releases verify unchanged.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: dependency evidence names the locked scanner image that
  produced it, and release provenance binds that image to the Mise lock.

## Impact

Release construction and verification tooling, the published dependency report
schema, tests and documentation. No product behavior changes and no release;
the next release is the first to publish schema 2 reports. Earlier releases
keep their reports and provenance.
