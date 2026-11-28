## Context

The proposal gives the repetition and its measurement. The probe lives in `internal/adapter/codex/catalog.go`:
`ReadBundledCatalog` creates an empty probe home (`withCatalogProbe`), hashes the program file and runs `--version`
(`IdentifyExecutable`), then runs `debug models --bundled`. The adapter reaches it through one package seam,
`codexBundledCatalog`, from two places: `codexCatalogProjection`, which each preparation of a Codex target calls in
both the plan (`planCodexReconciliationTransition`) and the apply (`ReconcileConfigs`), and `producerRepair`, which a
local check calls when the table's recorded producer differs from the configured program. The repository's own
verification (`tools/codex/catalogcheck`) and the release tools call `ReadBundledCatalog` directly.

## Goals / Non-Goals

**Goals:** one probe of an unchanged Codex program per process; the same answer, failure included, for the plan, the
apply and the check of one command; concurrent reads that share the probe.

**Non-Goals:** those of the proposal.

## Decisions

**D1. What can change a probe's answer, and the key.** The cases, each with how the key covers it:

1. The program's bytes. Hashed with SHA-256, the identity that AIGW already records as the table's producer
   (`catalog_client_sha256`).
2. The file that the configured path resolves to. In the key: a link moved to another installation is another key
   even when the bytes are equal, because a launcher may find its package beside the file it resolves to.
3. The configured path itself. In the key: a shell launcher receives it as `$0`.
4. Other files of the program's package, such as the binary that a launcher starts. Not hashed: the version that the
   package manifest declares stands for them, read by `packageVersion`, the one reader of `codex-package.json`, as in
   the producer comparison that `missing-program-reported-truthfully` introduced (its D3). A package rebuilt in place
   at the same version while one command runs is not seen; the same race already exists between the two starts of
   one probe.
5. The probe's home and temporary root. Created empty for each probe and removed after it; nothing to key.
6. The environment. The process's own, without Codex's session variables (`codexEnvironment`). No production code
   changes the process environment: no call of `os.Setenv`, `os.Unsetenv` or `os.Clearenv` outside tests under
   `internal` or `cmd` (read on 2026-11-28). It is constant within a process.
7. Load and time. A probe that misses its bound under load fails; D3 decides what a later read gets.
8. What a run needs outside the package: the file's mode, the interpreter that a script's first line names (an npm
   launcher, which also has no manifest beside it), the working directory and the operating system's admission of
   the program. Not in the key. No production code changes the working directory (no call of `os.Chdir` or
   `syscall.Chdir` outside tests under `internal` or `cmd`, read on 2026-11-28), and AIGW changes none of the others;
   a change by another program while one command runs is not seen, and the next command reads the program again.

The key is the configured path, the resolved path, the file's SHA-256 and the declared version. Reading it starts no
program. The first read of a key hashes the file twice, for the key and inside the probe; a probe that measured other
bytes than the key's, because the file changed in between, is not kept, so the next read probes again
(`TestAProbeOfOtherBytesIsNotKept`). Every read costs one hash: `missing-program-reported-truthfully` measured 131 to
154 ms for a 285 to 291 MB program with a warm cache.

Alternative rejected: a key from file metadata (device, inode, size, modification and change times), which the
experiment of the proposal used. It needs fields that differ per platform (`Mtimespec` on macOS, `Mtim` on Linux) and
trusts the file system's time granularity, while the bytes are already hashed by every probe and are the identity
that AIGW records.

**D2. The process is the scope.** AIGW runs one process per command and no daemon, so an answer cannot outlive the
command that read it; the next command reads the program again, after an upgrade too. An answer kept on disk or across
processes would need its own invalidation, which nothing here needs. A test process runs many commands, so a test that
changes what the key does not cover between two commands starts the second as a new process would, with no probes:
the fixture of the Codex adapter's producer tests starts every command so (`asNewProcess`), and
`TestAnUnchangedLauncherWhoseProgramIsGoneStopsSyncWhileCheckPasses`, whose launcher loses its target between two
syncs (D1, case 4), depends on it. Memory: one entry per distinct key read in the process, holding the table that the
program printed (517,840 bytes for Codex 0.153.4, as `catalog.go` records); in a command that is one entry per
configured Codex program, in a test process one per test program.

The proposal's figures come from an experiment keyed by file metadata, which hashed nothing when it reused a probe;
this reader hashes the file on every read, and the profile of the baseline put 105 s of samples in `fileSHA256`. Task
4.1 measures this reader on the same matrix. If hashing takes back most of the gain, the key is revisited before the
Change lands. In the experiment the matrix's peak resident memory fell from 1.03 GB to 0.89 GB; its cause was not
measured.

**D3. A failure is kept.** A probe that fails, because the program does not run, prints no version or no table,
warns, or misses its bound, gives every later read of the same key in the process the same failure, with the identity
it measured; the accepted scenario of a catalog retrieval that fails after identifying the executable describes that
one probe, unchanged. Before this Change each read probed again: a sync could plan with one answer and apply with
another, and a probe that missed its bound cost the 30-second bound again in every preparation. A program whose key
cannot be read, because its configured path is empty or names no readable file, is not kept: the reader runs as
before and returns its own error, and starts no program, because the hash fails before the first start. A probe that
ends without an answer, as a panic ends it, gives a read that waited for it a failure, never an empty success
(`TestAReadThatWaitedForAnUnfinishedProbeFails`); nothing in production recovers a panic, so the process ends first.

**D4. Concurrent reads share one probe.** The first read of a key creates its entry and runs the probe; a read that
finds the entry waits until the probe has ended; reads of different keys do not wait for each other. The returned table
is a copy, so a caller that changes it cannot change what another read receives.
`TestConcurrentReadsOfOneProgramShareOneProbe` forces the order: in a `testing/synctest` bubble the probe blocks until
every other reader is blocked waiting for it, the test then counts one probe, releases it, and every reader receives
the same answer. `TestReadsOfDifferentProgramsDoNotWait` holds one program's probe open in a bubble and reads another
program meanwhile.

**D5. One owner.** The reader that keeps a probe per key is the value of the seam `codexBundledCatalog` and wraps
`ReadBundledCatalog`, which stays the fresh measurement for the repository's verification and the release tools. A
test that replaces the seam replaces the reuse with it. The requirements belong to `client-programs`, which owns which
program each client uses and what follows when a Codex program cannot produce its model table, so when a program
counts as changed joins it there. `product-control-plane` keeps the lifetime of one probe; its 477 non-blank lines
would pass the repository's limit of 500 for a specification with these 32.

## Risks / Trade-offs

- A program whose package changes without a new declared version while one command runs is read once → the command
  acts on the answer it read first; the next command reads the new package (D1, case 4).
- A probe that failed for a transient reason stays failed for the rest of the command → the command reports that
  failure once; the next command probes again (D3).
- A test process keeps one entry per test program until it exits → bounded by the number of test programs and their
  small tables (D2).

## Migration Plan

None: no configuration, command, output or file changes. Rollback is a revert of the Change.
