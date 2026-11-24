## Context

The proposal gives the gap and the measured times. A native job runs `ci native`, which runs the Go source check and
the native coverage run (`nativeCommands`); `--full-quality` replaces the source check with the whole quality graph.
The quality graph owns the `selection-matrix` gate's command. The CUE graph (`.config/ci/pipeline.cue`) generates both
Forges' pipelines; it places both quality jobs on the Linux runners by naming `nativeEvidence.linux` twice.

## Constraints

Every placement was checked against these before the decision; each one excludes options on its own.

1. The full local proof runs every check, so the design holds with no remote (owner, 2026-11-25 00:02 to 00:0x).
2. Each Forge runs the same check classes on the same operating systems, from one declaration, and no Forge relies on
   another's result (owner 2026-11-24 22:24 to 22:39; `AGENTS.md`; `product-quality` "one complete quality graph").
3. Each selected Forge has native macOS, Linux and Windows evidence for the product tree (`AGENTS.md`); the owner
   chose the matrix on all three systems on both Forges (2026-11-24 23:57, confirmed 2026-11-25 00:0x).
4. A proof runs each test once (`product-quality`), and no two runs do the same work for one head (host admission
   rule 5).
5. One owner per fact: the gate's command lives in the quality graph, the pipeline topology in the CUE graph.

## Goals / Non-Goals

**Goals:** the matrix on every platform of each Forge once per pipeline, with the local proof unchanged.

**Non-Goals:** reducing the matrix or the proof's time; changing ETHOS's gate scheduling; the proxy repository.

## Decisions

### The native jobs of the platforms without a quality job run the matrix

- **Rests on:** constraints 1 to 4. The local proof and the Linux quality jobs run the quality graph, which holds the
  gate; macOS and Windows are the platforms where no job of either Forge runs it.
- **Considered:** moving the gate out of the quality graph into every native job (breaks constraint 1, the local proof
  runs the quality graph); running it in all three native jobs as well (Linux twice per pipeline, constraint 4); running
  it on one Forge only (constraint 2).
- **Applies to:** `.config/ci/pipeline.cue`, `tools/ci/main.go`.

### The CUE graph says which native jobs run it; `ci native` runs the quality graph's own gate

- **Rests on:** constraint 5. Which jobs run on which platform is pipeline topology; the gate's command is the quality
  graph's.
- **Considered:** `ci native` choosing by platform (it would repeat the CUE graph's choice of the quality job's
  platform); a CUE step that runs one gate of `ci quality` (no such selector exists, and adding one widens the quality
  command for one caller).
- **Chosen:** the CUE graph declares the quality job's platform once; both quality jobs take their runners from it, and
  the native command of every other platform carries `--selection-matrix`. `ci native --selection-matrix`, in both its
  explicit and its GitLab environment form, runs the quality graph's `selection-matrix` command after the Go source
  check and before the coverage run. With `--full-quality` the quality graph already runs it, so it is not added; an
  artifact-only journey runs no source commands, so it runs no matrix either.
- **Applies to:** `.config/ci/pipeline.cue`, `.gitlab-ci.yml`, `.github/workflows/verify.yml`, `tools/ci/main.go`,
  `tools/ci/native_environment.go`.

### GitHub's native time limits come from the measured times

- **Rests on:** GitHub's native jobs on `dev` took 12.4 to 16.9 min on macOS and 10.5 to 11.0 min on Windows (pushes of
  `7e64d0b7d` and `51fdc384f`); the gate took 573 s and 837 s there (2026-11-24). The sums are about 26.5 and 25 min,
  over or at the current limit of 25 min. GitLab's project limit is 60 min (read 2026-11-25), against about 20 and
  23 min on its guests.
- **Chosen:** 40 min for a native job, 60 min with the official native clients (previously 25 and 45): about 1.5 times
  the longest measured sum, so a slow runner is not cut off while a hung job still ends within the hour.
- **Applies to:** `.config/ci/pipeline.cue`, `.github/workflows/verify.yml`.

## Risks / Trade-offs

- Each GitLab pipeline holds the macOS and Windows guests about 11 and 14 minutes longer, so a pipeline's ticket under
  host admission rule 6 is held longer.
- The hosted Windows runner has crashed a test binary once before (`0xc0000409`, Change A of 2026-11-23); the matrix is a
  new binary there, so its first runs are watched.
- The local proof's time is unchanged; the 600 s goal is not served by this Change.
