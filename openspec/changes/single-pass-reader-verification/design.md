# Design

## Context

One `sync` that finds an existing reader reads the roughly 13 MB running
program twice and the reader twice. The App hashes the program to derive the
versioned reader path; `EnsureEntrypoint` hashes the reader, then hashes the
program again to compare it with the reader's receipt; after projection,
`finalizeCredentialEntrypoint` hashes the reader again. A first installation
reads the program a second time to copy it and reads the written reader back.
`TestExecutableDigestRechecksConcurrentSources` deliberately forbids reusing a
digest by file identity, because Linux and Windows timestamps are too coarse to
reveal a same-size rewrite.

## Goals / Non-Goals

**Goals:** within one command, stream the program once to verify it and each
versioned reader once, keep memory bounded, copy only bytes whose digest equals
the verified one, and keep every refusal that content verification provides
today.

**Non-Goals:** a digest cache keyed by path, size or time; any state that
outlives one command; changes to receipts, the private namespace, retained
readers or the native performance contract.

## Decisions

### One verification read per file, identities everywhere else

`credential.ReadProgram` streams the running program once and keeps only its
path and digest. `Program.EnsureEntrypoint` reads an existing reader once,
checks it against its receipt and versioned path, and compares its digest with
the program's. Only when the reader is absent does it read the program a second
time to copy it, and it refuses the copy unless those bytes hash to the verified
digest; the written reader is then read back once. Every later step consumes
these values instead of a path: the versioned path derives from the program's
digest, and `Entrypoint.Confirm` checks after projection that the reader path
still names the verified file and receipt without hashing it again. Each read
still hashes every byte and nothing is keyed by path or time, so
`TestExecutableDigestRechecksConcurrentSources` keeps its assertion.

Keeping the verified bytes instead would save the copy read on installation,
but every command that names the reader, including `status` and `doctor`, would
hold the whole program, about 11 MiB, raising peak memory past the release
policy's review threshold. An installation happens once per program version.

The post-projection check exists to notice a reader that disappeared or was
replaced while clients were projected; deletion and replacement remain
refusals. An in-place rewrite after verification is not re-detected within the
command, but AIGW never copies or records those bytes: the receipt keeps the
verified digest, and the next command's verification reads the file again.

### Reads go through an injected capability

`internal/` packages take capabilities by injection rather than package
variables. The App therefore carries a `credential.Files` capability that opens
executables, defaulting to the host file system, and resolves the program once
per process through it. `invocation.Context` and the synchronizer pass it on.
The acceptance regression injects a counting `Files` that serves the program
from another location, so a read that bypasses the capability cannot find it,
and runs `setup`, `sync` and `use --for`, each in its own App as in production,
counting opens per path.

## Risks / Trade-offs

- [An installation reads the program twice] → once per program version; the
  copy is refused unless it hashes to the verified digest.
- [A caller keeps a path-based check by accident] → the regression counts every
  open through `Files`, and a program read that bypasses it fails.
- [The 0.4.1 measurements also vary with hosted runners] → task 2.1 compares
  all eight native workloads against the published 0.4.1 predecessor and records
  any accepted exceedance.
