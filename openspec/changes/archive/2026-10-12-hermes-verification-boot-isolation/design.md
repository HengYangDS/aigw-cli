# Design

## Context

See [the proposal](proposal.md). Hermes 0.21.6 `hermes_cli/main.py` runs
`maybe_run_boot_bootstrap()` before every command except `update`, including
`--version` and `chat`. `boot_bootstrap.record_path()` writes
`install_state_dir(root)/bootstrap/<active profile>.json`, and no setting opts a
launch out. `HERMES_DISABLE_LAZY_INSTALLS` gates dependency synchronization, not
boot maintenance. A disposable home outside the native root resolves as profile
`default`. With the whole `installs/` directory linked, its record path is the
operator's own `bootstrap/default.json`. A temporary profile under the native
root avoids that collision but leaves a new record in the shared directory on
every run.

The terminal lane `work/20260926-terminal-product-convergence` qualified a
bounded repair in commit `46f451c4`. That lane also carries unrelated in-progress
convergence work, so accepted `dev` receives this repair through its own Change.

## Goals / Non-Goals

**Goals:** Keep every verification boot record inside the disposable home while
reusing the committed dependencies, keep the original selection lock in force,
and prevent verification from publishing launchers.

**Non-Goals:** Changing Hermes, using an upstream isolation API that does not
exist yet, qualifying Windows or Linux, and landing the terminal lane's other
work.

## Decisions

1. Port the bodies of `CreateVerificationHome` and `borrowVerificationInstall`
   from `46f451c4` into the Hermes configuration owner, in a file built only
   for non-Windows hosts. The home is created outside both the configured and
   the platform-default Hermes roots, compared by physical path.
2. Borrow per installation only `facts.json`, `environments/`, `pm-runtime` and
   the original `.install.lock`; keep `bootstrap/` private. A recorded selection
   without its original lock is refused, because a private lock would let
   verification race the operator's dependency operations. This couples AIGW
   to Hermes' current PM layout and edits no facts or selections.
3. Set `cli.expose_on_path: false` beside the existing lazy-install and
   update-check opt-outs. Installed Windows Hermes ignores it, so the Hermes
   adapter refuses on Windows before starting the client instead of claiming
   cross-platform isolation. The refusal is selected at build time: Windows
   builds compile neither the borrowing nor the probe code, so each platform's
   aggregate coverage measures only behavior it can run. The terminal lane's
   runtime `GOOS` check becomes this build constraint.
4. Keep the requirement text, which stays accurate at 357 characters, and add
   only the refusal scenario. The terminal lane's 525-character text would fail
   strict validation.
5. Remove the replaced whole-directory `shareHermesInstallState`.
6. Spell the `mise.toml` lychee entry as a standard table, like its OSV Scanner
   entry. The bound ETHOS runtime checks TOML against its shipped baseline
   (`compact_inline_tables = false`) as well as this repository's policy
   (`true`), so the single-line inline table could satisfy only one of them and
   blocked every proof. The parsed configuration is unchanged; reconciling the
   two policies stays with their owners.

## Risks / Trade-offs

- [Hermes changes its PM layout] → The native regression drives Hermes' real
  `run_boot_bootstrap()` and fails if the record leaves the disposable home.
- [Boot maintenance re-runs on every verification] → The record is discarded
  with the home, so `BOOT_HOME_STEPS` run each time against the disposable home.
  `expose_cli` is disabled through configuration on supported platforms.
- [Windows users lose Hermes verification] → They get a categorized refusal
  before any client process starts. The upstream report
  [NousResearch/hermes-agent#136094][upstream] covers the missing isolation.
- [Two layouts during integration] → The function bodies match `46f451c4`.
  Merging the terminal lane keeps this file layout and drops that lane's copies
  from `document.go` and `hermes.go`; its owner receives the exact resolution.

## Migration Plan

No data migration or client configuration change. Rollback is reverting this
Change, which restores the accepted whole-directory sharing and its stale-record
hazard. Proof, candidate landing, archive and accepted closeout are governed
effects recorded in ETHOS Attestations. Publication needs operator approval.

## Local Qualification

Against accepted `f12650c8`, the updated adapter tests failed because
verification permitted launcher publication and let Hermes rewrite the
operator's boot record. The configuration test kept `cli.expose_on_path: true`,
and the `CreateVerificationHome` and native boot regressions did not compile.
After the change, `internal/client`, `internal/hermes/configuration` and
`internal/cli/acceptance` passed. The `native_hermes` suite also passed against
the installed Hermes 0.21.6 Python, including
`TestHermesNativeVerificationBootIsolation`. `go vet` with and without that tag
and the repository's golangci-lint policy passed for darwin, linux and windows
targets; Windows tests were compiled, not run. `mise run check` and
`mise run native` passed on the staged tree with 95.12% aggregate statement
coverage (15,078 of 15,851 statements).

On the operator host, a source-built candidate passed the version probe against
the installed Hermes and stopped at inference, because the keychain does not
trust its unsigned credential reader. A manual probe with the same layout,
policy and arguments, using the operator's signed reader, returned `AIGW_OK`.
Its stderr held only the passive install verdict and a session identifier, which
AIGW's classifier accepts, and its boot record stayed inside the disposable
home. Across 96,956 observed nodes of Hermes install state, tools, launchers and
AIGW data, each run changed only the shared environment's lease directory
timestamp, which Hermes updates when a process takes and releases its dependency
lease. No disposable home remained. A complete `aigw verify --for hermes` still
needs a signed, installed candidate and stays a release obligation.

[upstream]: https://github.com/NousResearch/hermes-agent/issues/136094
