## Context

The proposal gives the gap and the measured times. A native job runs `ci native`, which runs the Go source check and
the native coverage run (`nativeCommands`); `--full-quality` replaces the source check with the whole quality graph.
The quality graph owns the `selection-matrix` gate's command. The CUE graph (`.config/ci/pipeline.cue`) generates both
Forges' pipelines; it placed both quality jobs on the Linux runners by naming `nativeEvidence.linux` twice.

## Constraints

Every placement was checked against these before the decision; each one excludes options on its own.

1. The full local proof runs every check, so the design holds with no remote (owner, 2026-11-25 00:02 to 00:0x).
2. Each Forge runs the same check classes on the same operating systems, from one declaration, and no Forge relies on
   another's result (owner 2026-11-24 22:24 to 22:39; `AGENTS.md`; `product-quality` "one complete quality graph").
3. Each selected Forge has native macOS, Linux and Windows evidence for the product tree (`AGENTS.md`); the owner
   chose the matrix on all three systems on both Forges (2026-11-24 23:57, confirmed 2026-11-25 00:0x).
4. A proof runs each test once (`product-quality`), and no two runs do the same work for one head (host admission
   rule 5).
5. A hang bound leaves room for a host three times slower than a quiet one, and stops a hang with Go's diagnostics
   before the job's own time budget ends (`product-quality` "Hang bounds tolerate a shared CI host").
6. One owner per fact: the gate's command lives in the quality graph, the pipeline topology in the CUE graph.

## Goals / Non-Goals

**Goals:** the matrix on every platform of each Forge once per verification pipeline, with the local proof unchanged.

**Non-Goals:** reducing the matrix or the proof's time; changing ETHOS's gate scheduling; the proxy repository.

## Decisions

### The native jobs of the platforms without a quality job run the matrix

- **Rests on:** constraints 1 to 4. The local proof and the Linux quality jobs run the quality graph, which holds the
  gate; macOS and Windows are the platforms where no job of either Forge ran it.
- **Considered:** moving the gate out of the quality graph into every native job (breaks constraint 1, the local proof
  runs the quality graph); running it in all three native jobs as well (Linux twice per pipeline, constraint 4); running
  it on one Forge only (constraint 2).
- **Applies to:** `.config/ci/pipeline.cue`, `tools/ci/main.go`.

### The CUE graph says which native jobs run it; `ci native` runs the quality graph's own command

- **Rests on:** constraint 6. Which jobs run on which platform is pipeline topology; the gate's command is the quality
  graph's.
- **Considered:** `ci native` choosing by platform (it would repeat the CUE graph's choice of the quality job's
  platform); a CUE step that runs one gate of `ci quality` (no such selector exists, and adding one widens the quality
  command for one caller).
- **Chosen:** the CUE graph declares the quality jobs' platform once (`qualityPlatform`; it moves their runners, while
  their toolchain setup stays Linux's); the native command of every other platform carries `--selection-matrix`.
  `ci native --selection-matrix`, in its explicit and its GitLab environment form and through the Linux Secret Service
  session, runs the quality graph's `selectionMatrixCommand` after the Go source check and before the coverage run, with
  the coverage run's isolated environment (its own temporary folder, no Forge credentials). With `--full-quality` the
  quality graph already runs it, so it is not added; an artifact-only journey runs no source commands and no matrix.
- **Applies to:** `.config/ci/pipeline.cue`, `.gitlab-ci.yml`, `.github/workflows/verify.yml`, `tools/ci/main.go`,
  `tools/ci/native_environment.go`, `tools/ci/quality.go`.

### The matrix's bound and the budgets around it

- **Rests on:** constraint 5 and the gate's measured runs (proposal): the slowest, 855 s on the GitLab Windows guest,
  times three is about 43 minutes. The bound was one hour, longer than the GitHub quality job (25 minutes), the GitHub
  native jobs (25 minutes) and the local proof's static gate (2,400 s), so a hang there would have ended without Go's
  diagnostics.
- **Chosen:** the matrix's bound is 45 minutes. Every GitHub job that runs it, ordinarily or with `--full-quality`, gets
  60 minutes: the quality job and every native job, whose one budget also covers the official clients' journeys
  (45 minutes before; GitHub runs those only with a published baseline, which runs no source tests). The local proof's
  static gate gets 3,600 s. A test of the tracked files requires every GitHub job and proof gate that runs the matrix to
  outlast its bound. GitLab's jobs take the project's limit of 60 minutes (read 2026-11-25, a Forge setting outside the
  repository); before the matrix they run setup and the source check, about 5 minutes on the guests.
- **Applies to:** `tools/ci/quality.go`, `.config/ci/pipeline.cue`, `.ethos/profile.toml`.

## Risks / Trade-offs

- Each GitLab pipeline holds the macOS and Windows guests about 11 and 14 minutes longer, so a pipeline's ticket under
  host admission rule 6 is held longer.
- The hosted Windows runner has crashed a test binary once before (`0xc0000409`, Change A of 2026-11-23); the matrix is a
  new binary there, so its first runs are watched.
- Manual pipelines differ from the verification pipelines and keep their behavior: a full-quality run also runs the
  matrix in the Linux native job beside the quality job, by its purpose of qualifying every tool on each host; and a run
  with only a published baseline runs source tests, the matrix included, on GitLab but not on GitHub. The second is a
  difference between the Forges that predates this Change; it goes to ETHOS's peer-symmetry item R-25.
- The local proof's time is unchanged; the 600 s goal is not served by this Change.
