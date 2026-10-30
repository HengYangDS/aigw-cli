## Why

AIGW's package layout no longer shows where a behavior lives or who owns it.
Measured on `dev` `1dd333c4`:

- **Client knowledge is spread across shared packages.** The architecture
  gate's client-name scan finds 19 product packages under 11 top-level owners
  that name a client: Codex in 13, Claude Code in 13, Claude Desktop in 7 and
  Hermes in 6. `configuration` (15 files, about 11,000 lines, 117 exported
  identifiers) branches on all of them, and `discovery`, `surface` and
  `synchronization` hard-code them. Adding Hermes touched 14 internal
  packages.
- **The tree is flat and its dependency rule is hand-written.** `internal/`
  has 23 sibling owners, and the architecture policy allows dependencies
  through 63 hand-maintained import-edge entries with 220 targets. No rule
  says which way dependencies point.
- **Names drift.** Package base names repeat (`readiness` and `catalog` three
  times; `verification`, `renaming`, `credential`, `configuration`, `client`
  and `artifact` twice). Both "catalogue" (59 files) and "catalog" (54 files)
  are in use, and two packages define the release artifact format.
- **The CLI does not mirror its command tree.** Its 25 top-level commands are
  implemented across 16 command packages and a 1,628-line root.
- **One specification holds several capabilities.** `product-control-plane`
  has 2,803 lines and 77 requirements that mix control-plane behavior,
  release, quality and repository rules. `product-quality` (1,130 lines) and
  `secret-storage` (918) also exceed one reading unit.
- **Release builds write outside the generated-output root.** The `build`
  task writes to an ignored `dist/` at the repository root, not to
  `build/artifacts/`.

These costs grow with every client and Provider. A reader cannot find one
client's behavior in one place, and a change cannot be reviewed against a
dependency rule.

## What Changes

The restructure runs in six stages. Each stage is a separate Work Lane with
its own proof, review and rollback. Stages 1 to 5 preserve behavior: each one
is either a move or a single semantic change, and its acceptance criteria are
measured against the baseline above.

1. **Client packages.** Everything specific to one client moves to
   `internal/client/<name>/`, named after the client ID (`claude`,
   `claudedesktop`, `codex`, `hermes`), behind the existing adapter interface
   and its descriptor, which declares the client's capabilities. One
   composition root registers the adapters. One contract suite runs against
   every adapter and against a test-only rehearsal adapter that proves the
   extension path.
2. **Domain, store and secret.** `configuration` divides into `domain`
   (Accounts, Models, Routes, Client Bindings and their validation, without
   I/O), `store` (files, schema versions and transactions) and the client
   adapters. `credential`, `secrets` and `transaction` fold into `secret` and
   `store`.
3. **CLI aligned with the command tree.** Each top-level command has one
   package named after it. Acceptance scripts run the program in its own
   process for every command's help, exit status and JSON output, and fix
   today's behavior as the baseline for stage 6.
4. **Layers and tooling.** The architecture gate replaces the import-edge
   allowlist with an ordered layer table. One package owns the release
   artifact format, for both release tooling and program update. Release
   builds write to `build/artifacts/`.
5. **Terminology, specifications and documentation.** One canonical name per
   concept, enforced with declared rejected synonyms. Package base names no
   longer repeat. Main specifications are split by capability into reading
   units, and the documentation follows the new layout.
6. **Command surface convergence and JSON contracts.** This stage is
   user-visible and breaking. It needs its own design and review in a later
   Change and is out of scope here.

**Breaking changes:** none for users in stages 1 to 5. Internal package
paths change, and no other module imports them. The architecture policy
format changes in stage 4. Stage 6 will be **BREAKING** and is specified
separately.

**Reuse stance:** the existing `client.Adapter` seam and the repository's
own architecture gate (`tools/architecture`) are deepened instead of adding a
second interface or dependency checker. `rogpeppe/go-internal/testscript`,
the Go toolchain's script-test format as a library, is admitted in stage 3
only if it lowers owned complexity compared with the current CLI test
harness.

**Non-goals:** new client or Provider behavior, changes to the command
surface, configuration schema or on-disk formats, a release per stage, and
work on the proxy repository.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: product packages form one downward layer stack;
  client knowledge stays in its client package; command packages mirror the
  command tree; one canonical name per concept; main specifications stay
  within one reading unit; release builds write to `build/artifacts/`.
- `product-control-plane`: the composable extension boundary requires every
  client adapter to pass one shared contract. The release artifact format has
  one owner shared by release tooling and program update.

## Impact

Go packages under `internal/` and `tools/`, their tests and fixtures, the
architecture policy and its gate, the Mise `release` task, the OpenSpec main
specifications (split in stage 5) and the documentation. There is no change
to installed behavior, configuration files, client projections, release
assets or the command surface. Each stage's PR is reviewed on its own.
