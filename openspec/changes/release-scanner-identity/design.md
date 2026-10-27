# Design

## Context

Release construction runs OSV-Scanner by name through `executeTool`, which
records stdout, stderr and the exit status. Mise installs the scanner from a
locked release asset: `.config/mise/mise.lock` records the asset URL and
SHA-256 for each of six platforms. The GitHub backend installs the raw binary,
so the installed executable has the asset's digest; on the operator Mac the
installed 2.7.0 file and the lock's macos-arm64 checksum are both `d4926ef8…`.
Nothing in the release evidence ties the reports to that asset.

## Goals / Non-Goals

**Goals:** every scan, and the reports and provenance of the next release,
record the scanner that actually ran, and any other scanner is refused.

**Non-Goals:** reconstructing scanner evidence for 0.4.18 or earlier releases,
observing the images of the other release tools, a new release asset, or a
release.

## Decisions

### The execution owner observes the image

`toolCall` gains an optional `Image`. When it is set, `executeTool` resolves the
name with `exec.LookPath`, digests the resolved file, runs that path rather
than the name, and returns the path and digest. Observation is opt-in, and only
the scanner uses it. Test fakes fill it as the real runner does.

### Admit the locked image before scanning

The scan admits its scanner in two steps. The version report runs with image
observation; its digest must equal the lock's SHA-256 for the host platform,
in Mise's platform names (`macos`, `linux`, `windows`; `x64`, `arm64`), and its
reported `osv-scanner version` must equal the locked version. The scan then
runs by name with image observation again and must observe the same path and
digest. A shim on `PATH` resolves to Mise rather than the scanner and is
refused; `mise exec` and `mise run` put the installed file first.

### The reports name the scanner; the raw digest stays local

Both reports gain `scanner` at schema 2: tool, version, platform, image SHA-256
and arguments. The arguments are the executed vector with the source root and
the evidence directory written as `{source}` and `{evidence}`, so they are
host-independent and two construction passes still compare equal. The raw
report's digest is not published: the raw report holds absolute host paths and
upstream modification times, so its digest would make the published reports
host-dependent and break `build-ci`'s comparison. A local invocation receipt
records it, with the exact arguments, the executable's path and digest and the
complete version report.

### Provenance derives the scanner from the reports

`provenanceBytes` reads both reports' scanner blocks, which must agree. The
scanner must be the image that the source's Mise lock selects for the named
platform and version; it becomes a resolved dependency whose URI is the locked
asset URL. Verification recomputes provenance from the source and the
published files, so it refuses reports that name any other image. A schema 1
report must name no scanner and contributes no dependency, so earlier releases
and published baselines verify unchanged. Another schema, or a schema that
disagrees with the presence of a scanner, is refused.

## Risks / Trade-offs

- **Platform-dependent reports.** The reports name the build platform's image,
  so a release built on another platform publishes another digest. Releases are
  built on macOS, and the digest is the truth for the build.
- **The published version is the locked one.** The scan refuses unless the
  image reports the same version, and the receipt keeps the complete report.
- **Provenance reads the reports.** Fixtures that used opaque report bytes now
  use schema 1 JSON.
