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
  `release` task (`go run ./tools/release build dist`).

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

Product packages form this stack, from top to bottom. A package may import
any package in a lower layer, its own descendants, and the seam packages that
the policy declares for its subtree. It may not import any other package of
its own layer or a package in a higher layer.

| Layer | Packages                                                 | Owns                                                                                                                                                                        |
| ----- | -------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | `cmd/aigw`                                               | binary entry, which injects the built-in adapters                                                                                                                           |
| 2     | `internal/cli`, `internal/clients`                       | the command tree, flags, exit codes and each command's own behavior; the list of built-in adapters                                                                          |
| 3     | `internal/app`, `internal/terminal`                      | use cases that several commands share; human and JSON rendering and prompts                                                                                                 |
| 4     | `internal/client`, `internal/program`                    | the client seam and its adapters; AIGW's own install, update, rollback and artifact format                                                                                  |
| 5     | `internal/provider`, `internal/store`, `internal/secret` | Provider-native observation; configuration files, schema versions and transactions; credential storage and native backends                                                  |
| 6     | `internal/domain`, `internal/platform`                   | Accounts, Models, Routes, Client Bindings, their validation and the redaction rules, without I/O; operating-system paths, guarded file mutation and bounded child processes |

`internal/client/<name>` may import `internal/client`, the seam. The seam
never imports an adapter, and adapters never import each other. `domain`
imports only the standard library. `terminal` renders values that `app`
returns; neither imports the other. The adapter list shares layer 2 with the
command tree because only `cmd/aigw` imports it in production code. Like
today's import edges, the layers bind production files; test files are
fixtures and may import the adapter list. The redaction rules live
in `domain`, which like them imports only the standard library, because
provider diagnostics, client verification and release tooling apply them below
`terminal`.

Repository tools sit above the product stack, in two rows: `tools/ci` above
the other tools, because CI drives the release tool. A production file under
`tools/` imports its own descendants, the seams that its tool declares, the
tools row below its own, and product packages from the row of `internal/client`
down, never `cmd`, `cli` or `app`. The release tool's packages share its
performance, provenance and readiness libraries as seams. The release tool
imports `internal/program/artifact`, and `tools/codex/catalog` imports
`internal/client/codex`; both keep importing the owner instead of re-encoding
its knowledge.

Alternative considered: keep the closed topology and keep extending the edge
list. Rejected, because 220 hand-written targets cannot show a direction and
every new package needs a policy edit.

### The existing architecture gate enforces the layers

`tools/architecture` gains a `layers` table. The table is an ordered list of
rows of package-path prefixes, and the gate checks every import of a
production file against it. A package belongs to the row of its longest
matching prefix. `layer_seams` declares a subtree's seams, and `layer_floors`
names the highest product row that a subtree's packages may import. The tools
have a floor at `internal/client`'s row, and so does `internal/terminal`, which
shares the row of `internal/app` but imports none of the use cases. The table
replaces `allowed_import_edges` and the peer rule for command packages. A
production package that no row places, a prefix that places no package and a
seam that nothing imports are findings. `package_children` stays as the closed
list of direct owners. A violation names the importing package, the imported package
and both layers. Client-name confinement uses the same scan: identifiers and
string literals that spell an admitted client ID are refused in product
packages outside that client's package, the composition root and test
fixtures. A client's own vocabulary can contain another client's ID, such as
the Hermes API mode `codex_responses`. The policy lists each such exact literal
as a native term of its client: the gate accepts it only in that client's
package and fails when it no longer occurs. Repository tools are outside the
confinement rule because they import the client's package rather than
re-encode it.

Inside layer 3, the packages that task 4.1 moves under `internal/app` stay
packages and take ordered rows below `internal/app`: `renaming` above
`synchronization` and `activation`, which are above `readiness`, which is
above `diagnostics`. Each imports only rows below it, and `internal/app`
imports its descendants. The product then takes ten rows, and with the two
tool rows the table has twelve, its limit. The adapters share
the declared seams `internal/client`, `internal/client/verification`,
`internal/client/discovery` and `internal/client/surface`. The Providers
share `internal/provider/diagnostic`.

Alternative considered: merge the five packages into `internal/app`. A
merge needs renames: `HTTPDoer` is declared in `app`, `diagnostics` and
`renaming`, and names such as `readiness.State` and `readiness.Client` lose
the qualifier that explains them. It also merges five test packages and their
fixtures into one. Rejected because it turns a move into renames, and
stage 5 owns names.

Alternative considered: golangci-lint `depguard`. Rejected because it would
add a second authority next to `tools/architecture`. Its rules are deny lists
per file glob, so a layer order would need one rule per pair of layers.

### Clients sit behind the existing seam

Stage 1 deepens `client.Adapter` instead of adding another interface:

- Each adapter moves to `internal/client/<name>`. The package name is the
  client ID without hyphens: `claude`, `claudedesktop`, `codex`, `hermes`.
- Each adapter owns its native paths and discovery. `DiscoverySource`
  shrinks to client-neutral lookups (an executable by name or path, a file's
  presence, and the observed platform: operating system, the user's home and
  the environment), and `Dependencies` replaces `ClaudeSettingsPath` with that
  observed platform. AIGW still refuses to start on a host where it cannot
  derive Claude Code's settings file or Claude Desktop's library: those two
  adapters implement an optional seam method that validates the observed host,
  and the CLI asks the registry right after deriving AIGW's own paths. On
  Windows, the host reads a variable name case-insensitively: the exact
  spelling wins, and otherwise the first spelling in sorted order. Windows
  returns the first spelling in its environment block instead, so the two can
  differ only when that block holds two spellings of one name, which neither
  Go's `os/exec` nor `SetEnvironmentVariable` produces. This difference is
  accepted.
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
  The registry adopts in admission order. When the Codex and Hermes adoptions
  fail together, AIGW therefore reports Codex's error where it reported
  Hermes' before; the exit status is the same. Keeping the old order would
  need an order rule with no meaning, so this one difference is accepted.
  Commands read client policy and wording from descriptor capabilities (the
  product name, explicit configuration targets, retry budgets, optional model
  discovery and doctor wording), and an adapter that validates explicit targets
  implements an optional seam method.
- One contract suite in `internal/client/contract` takes an adapter factory
  and a private fixture home. Each `ProjectionPlan` lists in `Effects` every
  file that applying it may write or remove: its target file, ownership state
  and companion files, as the adapter's own plan owner derives them. The field
  is not serialized, so command output is unchanged. The suite derives its
  checks from that model:
  - `Apply` changes no file outside the listed effects, each plan's
    `ChangesState` holds exactly when `Apply` changes one of its effects, and
    the target of a changing plan exists after a projection and, when it is a
    file, is one of the plan's effects;
  - a converged plan, its `Apply` and its receipt's `Rollback` change nothing;
  - `Rollback` restores every regular file's bytes and mode, every symbolic
    link and every directory's mode; only directories that the projection
    created may remain, with the mode that it gave them, as AIGW leaves them
    today, and a withdrawal may remove them;
  - withdrawal is planned and checked the same way, leaves no owned file, and
    its own receipt restores the projection;
  - `Inspect` and `Verify` (without a Route and with the fixture's selected
    Route, or a placeholder when it selects none), before and after a
    projection, and every `Plan` change nothing below the home, and these
    calls and every `Apply` leave the caller's configuration equal to its
    `Config.Clone` from before the call.

  A plan promises a change of its effects, not of its target alone: Claude
  Code changes only its ownership state when its settings file already holds
  the projected bytes. Each check runs with HOME, USERPROFILE, APPDATA,
  LOCALAPPDATA, XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME,
  XDG_CACHE_HOME, XDG_RUNTIME_DIR, TMPDIR, TMP and TEMP, and the working
  directory, pointed at an empty sentinel that must stay empty, and a fixture
  may not redirect them. That refuses an adapter that writes through those
  variables or a relative path instead of the observed host. Client-specific
  variables, values read before the suite starts, the operating system's user
  database and Windows known folders are outside the check. Every admitted
  adapter and a test-only rehearsal adapter run it.

### Domain, store and secret replace `configuration`

Stage 2 divides `configuration` by reason to change. Model types, validation
and manifest semantics go to `domain`; loading, saving, schema migration and
`transaction` go to `store`; client-specific projection code goes to the
adapters. `secrets`, `secrets/native` and the credential readers of
`credential` become `secret`. The requests that prove an Account's Token, each
protocol's models path and authentication headers, move to `domain`. Choosing
the probes, sending them and judging the answers is I/O; it goes to
`secret/probe`, beside the Token store, and `domain` performs none. Each move
keeps the exported names that callers outside the package use, unless a name
would repeat its new package's name, and drops every exported identifier that
has no user outside its package, as the next paragraph counts one.

The architecture gate's `used_exports_only` list holds `domain`, `store` and the
`secret` subtree to that rule, and since stage 3 also `app`, `terminal` and the
CLI's `invocation` package, which several commands share. An exported top-level function, type, variable or
constant of a listed package is used when a Go file outside the package's
directory selects it, the package's own tests excluded, or when the exported API
of a used identifier names it: the types in its signature or declared type, a
used type's exported fields and methods, also those of an unexported or
anonymous type that such an API names, and the constants declared with a used
type. Type parameter names are not the package's types. The gate does not
check methods and fields on their own; Go's type checker checked stage 2's
exported methods and fields once, counting an encoder that reads a tagged field
as a user.

`domain` names no client. Each `Config` carries its `Admission`: the ordered
client specifications that the composition root builds from the adapters'
`Spec()`. `NewConfig` and the store take it, and every rule that iterates or
looks up clients reads it from the configuration instead of a package list.
Client-specific rules become `ClientSpec` capabilities beside the existing
retry budgets: a default model provider, which also admits an explicit one;
client-native authentication, which needs a default model provider; and the
protocol of a pre-catalogue profile. An error that named the one client with a
capability derives that name from the admitted clients that have it, so its
text is unchanged. Tests that need the built-in clients take the composition
root's admission. A package that the composition root imports cannot import it
back: an adapter's own tests admit its specification, plus a fixture client
where a test needs a second one, and the other such packages admit fixture
specifications. Where a fixture copies the built-in records, an external test of
the same package keeps it equal to the composition root's admission.

Alternative considered: one registration of the admitted specifications at
start-up. Rejected because it keeps a package-level dependency that every
program and test must remember to install before it builds a configuration.

### Command packages mirror the command tree

Stage 3 gives each top-level command one package under `internal/cli` named
after it. The root keeps only the group assembly. Acceptance scripts run the
program in its own process against private homes for each command's help, a
representative success, a representative refusal and its JSON output. They
are written before any command moves, so they hold today's behavior, and they
become the reviewed baseline that stage 6 changes on purpose. The scripts use
`rogpeppe/go-internal/testscript`. Four representative scripts measured the
choice: the root help, `status --json` in an empty private home, a manifest
import followed by its Route list as JSON, and an unknown Route's refusal in
text and as JSON. For those four scripts the testscript harness needs 29
lines of Go code, as `scc` counts them, and each script runs the program's
`main` as a subprocess in its own private home. The committed harness, which
adds the scripted provider and Claude Code that other commands need, path
normalization and an exit-status check, has 291 lines. The module already requires `go-internal` v1.16.0, its
latest release, for `robustio`. testscript adds one indirect module,
`golang.org/x/tools` v0.52.0, also the latest release. Only its `txtar`
package compiles, into the test binary, and the program's package graph is
unchanged. OSV reports no advisory for either version.

Alternative considered: a script runner on the current CLI test harness. It
needs 172 lines of Go code for the same scripts: its own archive parser,
command loop, comparisons, update mode and re-execution of the test binary.
It still lacks the standard input, environment expansion and platform
conditions that other commands' scripts need. Rejected because it owns more
code for less capability. Both harnesses passed the four scripts on macOS and
failed when an expected output changed.

Rendering and prompts form one package, `internal/terminal`. Only the
composition root constructs the prompter, and commands reach it through the
invocation context. A separate prompt package would therefore add an owner
without removing knowledge from any caller.

A use case belongs to `internal/app` when more than one command, or the root
on a command's behalf, runs it, such as the client inspection that `status`,
`check` and `doctor` share. A command's own behavior stays in its package
beside its flags and rendering. A command that needs capabilities receives
them through the invocation context. CLI behavior that several commands
share, such as the optional Account argument and the rename subcommands'
argument rule, selection and plan output, lives in `internal/cli/invocation`;
no command package imports another.

Alternative considered: one `internal/app` use case per command, with the
command packages reduced to flags, rendering and exit codes. Rejected
because a use case with one caller is a seam without a second adapter: it
doubles each command's interface and splits its tests, and it moves the
command's JSON output, which stage 6 turns into a contract, away from the
command.

### One owner for the release artifact format

Stage 4 moves the artifact format to `internal/program/artifact`: the platform
matrix, archive and checksum names, the `checksums.txt` grammar, and the
verification of a complete matrix and its signature envelope. The release tool
and program update both import that one owner. Each still validates its own
authority-bound inputs. The release tool validates its source, its lock and its
signer, whose allowed-signers file and principal `ssh-keygen` evaluates. Program
update validates its build-bound trust anchor and the bytes it downloads. Both
read `checksums.txt` lines with one grammar, a digest and a name. The release
tool refuses a manifest with any other line, and program update reads only its
archive's line and skips the others, as it always has. The release tool accepts
a signature envelope only as `ssh-keygen` writes it, and its remaining
package, `tools/release/provenance`, holds its source provenance, its scanner
lock and its signer check. The Mise `release` task writes to
`build/artifacts/<version>-<commit>/`, and the root `dist/` disappears.

Alternative considered: keep two owners and add a parity test. Rejected,
because a parity test detects drift after it happens, while one owner removes
the drift.

Alternative considered: verify the release tool's signer natively, too.
Rejected, because the allowed-signers file is the operator's documented trust
input. Its principal patterns and options belong to `ssh-keygen`, and a second
reader of that file would be a second owner.

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
previous stage is on GitHub `dev`. GitLab `dev` receives the completed stages
in that peer's next catch-up, which waits for ETHOS to publish aigw's
historical release tags there (Change `gitlab-peer-reconciliation`). Stages 1
to 5 must keep every existing test, the full ETHOS proof, the hosted
three-platform graph and the real-client journeys unchanged. Stage 3 adds its
scripts.

| Stage     | Measured result at the stage head                                                                                                                                                                                                                                                                                                                                                                                                             |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 Clients | Client-name confinement reports 0 packages outside the four adapters, the composition root, fixtures, `configuration` and `credential`, which stage 2 deletes (baseline: 19 packages; Codex 13, Claude Code 13, Claude Desktop 7, Hermes 6). The contract suite passes for all four adapters and the rehearsal adapter. The rehearsal adapter's diff touches only its own directory, the test composition root and its fixtures.              |
| 2 Domain  | `configuration`, `credential`, `secrets` and `transaction` are absent, and the confinement exception list is empty and removed. `domain` imports only the standard library. No exported identifier in `domain`, `store` or `secret` lacks a user outside its package, as `used_exports_only` counts one, and no exported method or field of an exported type lacks a user outside its package, counting an encoder that reads a tagged field. |
| 3 CLI     | Each of the 25 top-level commands maps to one package named after it. Every command has passing acceptance scripts on macOS, Linux and Windows.                                                                                                                                                                                                                                                                                               |
| 4 Layers  | `allowed_import_edges` is absent and the layer table has at most 12 entries. The `artifact` package name occurs once. `mise run release` writes to `build/artifacts/`.                                                                                                                                                                                                                                                                        |
| 5 Names   | No package base name repeats. The text gate refuses each declared rejected synonym. Every main specification fits within 512 lines.                                                                                                                                                                                                                                                                                                           |

Each stage is reviewed by its PR, and `tasks.md` holds each stage's bounded
actions. Stage 6 needs its own written design and approval.

## Risks / Trade-offs

- **Large moves conflict with concurrent work** → stages run serially. Each
  stage starts from the current `dev` and is announced before landing, and
  other lanes refresh after it.
- **History becomes harder to follow** → files move with `git mv` and keep
  their content apart from import paths and the identifiers that the move
  requires, so Git's rename detection, `git log --follow` and `git blame -C`
  follow them. Each commit moves one client and still builds and tests, which
  a move without its edits would not.
- **Per-package coverage floors move with the code** → each new package must
  meet the existing 95% statement floor on every hosted platform before its
  stage lands. Platform-specific files move with their build constraints.
- **A layer table can be too coarse** → a needed upward edge is a design
  defect. The fix is to move the behavior to the lower owner or to invert the
  dependency through the seam, never to add an exception.
- **Release tooling now imports a product package** → the format owner reads
  the files it is given and writes only the canonical checksum manifest that
  release construction asks for. Each importer keeps its own
  trust and source validation, and both are tested against published
  archives.

## Migration Plan

There is no runtime migration: installed configuration, client projections
and release assets are unchanged. Each stage lands, publishes and retires like
any other lane. Rollback is the revert of that stage's merge. The next release
carries the completed stages; none of them needs a release of its own.

## Open Questions

None. Stage 4 settled the last one: the packages that task 4.1 moves under
`internal/app` stay subpackages in ordered rows of the layer table.
