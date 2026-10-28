# Design

## Context

See `proposal.md` for the motivation and the measured baseline on `dev`
`1dd333c4`. This design depends on these facts about the current source:

- `internal/client` already declares the operational seam. `Adapter` has nine
  methods (`Spec`, `Discover`, `Converge`, `Plan`, `Apply`,
  `ProjectionChanged`, `Inspect`, `Verify`, `Withdraw`), and `Registry`
  validates adapters against the admitted client records. The seam still leaks
  client knowledge. `DiscoverySource` has `ClaudeDesktopExecutable`,
  `CodexHomeDirectory` and `HermesHomeDirectory`, and `Dependencies` carries
  `ClaudeSettingsPath`. The adapter implementations are split between
  `internal/client` (`claude_desktop.go`, `hermes.go`, `codex_selection.go`),
  `internal/codex`, `internal/claude`, `internal/claude/desktop` and
  `internal/hermes/configuration`.
- `configuration.ClientSpec` already works as a descriptor: ID, label, endpoint
  protocols, restart, session-route and qualification flags. One central list,
  `admittedClientSpecs`, names every client in `configuration`.
- The architecture gate `tools/architecture` reads
  `.config/checks/architecture/policy.toml`. That policy holds a closed
  `package_children` topology and 63 `allowed_import_edges` entries with 220
  targets. The gate already parses every package and import edge.
- The CLI root `internal/cli/app.go` assembles 25 top-level commands in four
  help groups from 16 command packages.
- The release tool's `tools/release/artifact` defines and verifies the release
  artifact matrix. `internal/upgrade/artifact` verifies and reads the same
  archives for program update.
- The ignored `dist/` at the repository root is the default output of the Mise
  `build` task (`go run ./tools/release build dist`).

## Goals / Non-Goals

**Goals:**

- A reader finds each behavior under one owner whose path names it. A
  client's behavior lives in one directory.
- Dependencies point one way, and the gate states that direction as a short
  layer table instead of an edge list.
- Each stage is independently provable and reviewable, and it can be reverted
  with one revert of its merge.

**Non-Goals:**

- Changes to commands, flags, output, exit codes, configuration schema,
  on-disk formats, client projections or release assets. Stage 6 owns
  user-visible change and gets its own Change.
- New clients or Providers. The rehearsal adapter is test-only and is never
  admitted or shipped.
- Performance work, or a release per stage.

## Decisions

### The target is one layer stack

Product packages form this stack, from top to bottom. A package may import its
own subtree's seam package and any package in a lower layer. It may not import
a package in its own layer or in a higher one.

| Layer | Packages                                                 | Owns                                                                                                                            |
| ----- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| 1     | `cmd/aigw`, `internal/clients`                           | binary entry and the composition root                                                                                           |
| 2     | `internal/cli`                                           | the command tree, flags and exit codes                                                                                          |
| 3     | `internal/app`, `internal/terminal`                      | use cases; human and JSON rendering, prompts, redaction                                                                         |
| 4     | `internal/client`, `internal/program`                    | the client seam and its adapters; AIGW's own install, update, rollback and artifact format                                      |
| 5     | `internal/provider`, `internal/store`, `internal/secret` | Provider-native observation; configuration files, schema versions and transactions; credential storage and native backends      |
| 6     | `internal/domain`, `internal/platform`                   | Accounts, Models, Routes, Client Bindings and their validation, without I/O; operating-system paths and bounded child processes |

`internal/client/<name>` may import `internal/client`, the seam. The seam
never imports an adapter, and adapters never import each other. `domain`
imports only the standard library. `terminal` renders values that `app`
returns; neither imports the other.

Repository tools sit above the product stack. A package under `tools/` may
import its own subtree and product packages in layers 4 to 6, and never
`cmd`, `cli` or `app`. The release tool already imports
`internal/upgrade/artifact`, and `tools/codex/catalog` imports `internal/codex`;
both keep importing the owner instead of re-encoding its knowledge.

Alternative considered: keep the closed topology and keep extending the edge
list. Rejected, because 220 hand-written targets cannot show a direction and
every new package needs a policy edit.

### The existing architecture gate enforces the layers

`tools/architecture` gains a `layers` table. The table is an ordered list of
package-path prefixes, and the gate checks every import against it. The table
replaces `allowed_import_edges`. `package_children` stays as the closed list of
direct owners. A violation names the importing package, the imported package
and both layers. Client-name confinement uses the same scan: identifiers and
string literals that spell an admitted client ID are refused in product
packages outside that client's package, the composition root and test
fixtures. Repository tools are outside the confinement rule because they
import the client's package rather than re-encode it.

Alternative considered: golangci-lint `depguard`. Rejected because it would
add a second authority next to `tools/architecture`. Its rules are deny lists
per file glob, so a layer order would need one rule per pair of layers.

### Clients sit behind the existing seam

Stage 1 deepens `client.Adapter` instead of adding another interface:

- Each adapter moves to `internal/client/<name>`. The package name is the
  client ID without hyphens: `claude`, `claudedesktop`, `codex`, `hermes`.
- Each adapter owns its native paths and discovery. `DiscoverySource`
  shrinks to client-neutral lookups (an executable by name, a file's
  presence, the user's home), and `Dependencies` loses `ClaudeSettingsPath`.
- The descriptor stays `ClientSpec`, moves to `domain` and is returned by each
  adapter's `Spec()`. The admitted list is the composition root's ordered
  registration, not a list in `configuration`.
- The composition root has two parts. `internal/clients` is the only list of
  the built-in adapters, and `cmd/aigw` injects that registry into the CLI, so
  no package below it holds a default registry. Tests that exercise real
  adapters use the same list; a synchronizer without a registry refuses to
  guess.
- Client-specific branches move from `configuration`, `synchronization`,
  `discovery` and `surface` into adapter methods. `AdoptCodexSelection` and
  `AdoptHermesSelection` become one seam method that each adapter implements.
- One contract suite in `internal/client/contract` takes an adapter factory
  and a private fixture home. It checks the seam's invariants: `Plan` lists
  every effect that `Apply` makes, `Rollback` restores bytes and modes,
  `Withdraw` leaves no owned file, and `Inspect` and `Verify` touch nothing.
  Every admitted adapter and a test-only rehearsal adapter run it.

### Domain, store and secret replace `configuration`

Stage 2 divides `configuration` by reason to change. Model types, validation
and manifest semantics go to `domain`; loading, saving, schema migration and
`transaction` go to `store`; client-specific projection code goes to the
adapters. `secrets`, `secrets/native` and the credential readers of
`credential` become `secret`. Validation that `credential` performs against
Account data moves to `domain`. Each move keeps the exported names that
callers outside the package use, and drops every exported identifier that has
no caller outside its package.

### Command packages mirror the command tree

Stage 3 gives each top-level command one package under `internal/cli` named
after it. The root keeps only the group assembly. Acceptance scripts run the
built binary against private homes for each command's help, a representative
success, a representative refusal and its JSON output. They become the
reviewed baseline that stage 6 changes on purpose. The scripts use
`rogpeppe/go-internal/testscript` if its admission shows less owned code than
the current CLI test harness. Otherwise the existing harness runs the same
scripts.

### One owner for the release artifact format

Stage 4 moves the artifact format (platform matrix, archive and checksum names,
`checksums.txt` grammar and signature verification) to
`internal/program/artifact`. The release tool and program update both import
that one owner. Each still validates its own authority-bound inputs: the
release tool its source and lock, program update its trust anchors and
downloaded bytes. The Mise `build` task writes to
`build/artifacts/<version>-<commit>/`, and the root `dist/` disappears.

Alternative considered: keep two owners and add a parity test. Rejected,
because a parity test detects drift after it happens, while one owner removes
the drift.

### Names follow one glossary

Stage 5 makes `docs/concepts/product-concepts.md` the glossary. Each concept
has one canonical term and declared rejected synonyms, and the text gate
refuses a rejected synonym in code identifiers, specifications and
documentation. Where code and commands disagree, the user-visible command
vocabulary wins, so stage 5 renames no command. Package base names stop
repeating; for example, the CLI's `readiness` package takes its command's
name. A Provider-native diagnostic is named after the protocol family it
probes, once its owner confirms which family that is.

### Specifications split by capability

Stage 5 also splits `product-control-plane`, `product-quality` and
`secret-storage` into capabilities. Each fits within the repository's
512-line document limit. Each requirement moves with every scenario through
`MODIFIED`, `REMOVED` and `ADDED` operations that name its surviving owner, so
the locked merge loses no scenario.

## Stages and acceptance

Each stage is one Work Lane and one PR into `dev`. A stage starts after the
previous stage is on GitHub and GitLab `dev`. Stages 1 to 5 must keep every
existing test, the full ETHOS proof, the hosted three-platform graph and the
real-client journeys unchanged. Stage 3 adds its scripts.

| Stage     | Measured result at the stage head                                                                                                                                                                                                                                                                                                                                                                                                |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 Clients | Client-name confinement reports 0 packages outside the four adapters, the composition root, fixtures, `configuration` and `credential`, which stage 2 deletes (baseline: 19 packages; Codex 13, Claude Code 13, Claude Desktop 7, Hermes 6). The contract suite passes for all four adapters and the rehearsal adapter. The rehearsal adapter's diff touches only its own directory, the test composition root and its fixtures. |
| 2 Domain  | `configuration`, `credential`, `secrets` and `transaction` are absent, and the confinement exception list is empty and removed. `domain` imports only the standard library. No exported identifier in `domain`, `store` or `secret` lacks a caller outside its package.                                                                                                                                                          |
| 3 CLI     | Each of the 25 top-level commands maps to one package named after it. Every command has passing acceptance scripts on macOS, Linux and Windows.                                                                                                                                                                                                                                                                                  |
| 4 Layers  | `allowed_import_edges` is absent and the layer table has at most 12 entries. The `artifact` package name occurs once. `mise run build` writes to `build/artifacts/`.                                                                                                                                                                                                                                                             |
| 5 Names   | No package base name repeats. The text gate refuses each declared rejected synonym. Every main specification fits within 512 lines.                                                                                                                                                                                                                                                                                              |

Each stage is reviewed by its PR, and `tasks.md` holds each stage's bounded
actions. Stage 6 needs its own written design and approval.

## Risks / Trade-offs

- **Large moves conflict with concurrent work** → stages run serially. Each
  stage starts from the current `dev` and is announced before landing, and
  other lanes refresh after it.
- **History becomes harder to follow** → each stage commits its pure moves
  (`git mv` with no edits) separately from its edits, so
  `git log --follow` and `git blame -C` keep working.
- **Per-package coverage floors move with the code** → each new package must
  meet the existing 95% statement floor on every hosted platform before its
  stage lands. Platform-specific files move with their build constraints.
- **A layer table can be too coarse** → a needed upward edge is a design
  defect. The fix is to move the behavior to the lower owner or to invert the
  dependency through the seam, never to add an exception.
- **Release tooling now imports a product package** → the format owner does
  no I/O beyond reading the files it is given. Each importer keeps its own
  trust and source validation, and both are tested against published
  archives.

## Migration Plan

There is no runtime migration: installed configuration, client projections
and release assets are unchanged. Each stage lands, publishes and retires like
any other lane. Rollback is the revert of that stage's merge. The next release
carries the completed stages; none of them needs a release of its own.

## Open Questions

- Whether `terminal` stays one package or splits into rendering and prompts.
  Stage 3 decides from the command packages' actual imports.
