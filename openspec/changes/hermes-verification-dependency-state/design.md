# Design

## Context

See [the proposal](proposal.md). Hermes 0.21.6 is the current stable release;
its `pm/environments.py`, `hermes_bootstrap.py`, `hermes_cli/venv_sync.py`, and
`hermes_constants.py` are byte-identical in the operator's installed
`v0.21.6-1-g5ccb59b` checkout. `HERMES_HOME` selects both the configuration
home and the data root. The data root holds `installs/<install-key>/` with the
committed dependency environment and `tools/` with managed interpreters and
tools ([package management reference][pm-state]).

A home outside the owning data root is a separate root by design: its first
non-metadata launch synchronizes a new environment, while `--version` skips
that synchronization and then refuses activation. The adapter's fresh home hits
exactly that refusal. Hermes offers no configuration-file flag, and
`HERMES_CONFIG` is only a name in environment denylists. Profiles share an
installation's environment, but they are first-class tenants that gateways,
dashboards, backups, and updates enumerate.

## Goals / Non-Goals

**Goals:** Let the disposable verification home run the installed Hermes
against its committed environment, keep that installation state unchanged,
and preserve the existing projection, probe contract, and refusal categories.

**Non-Goals:** Changing Hermes, its update or plugin policy, or the operator's
home; adding a profile; loading operator plugins; adding Windows junctions.

## Decisions

1. Link the configured home's `installs` and `tools` directories into the
   disposable home when they exist. This is the per-task home layout that
   Hermes' end-to-end test `test_per_task_home_sharing_the_tools_store_by_symlink_is_current`
   exercises for orchestrators. Linking `installs` alone fixes `--version` but
   leaves the managed tools unresolved, and Hermes' passive startup verdict
   then reports them missing.
2. Set `HERMES_DISABLE_LAZY_INSTALLS=1` for every probe. The Hermes
   [environment reference][hermes-env] documents it as the PM policy for tests
   and install probes. It makes the launcher skip source-update completion
   before it can synchronize the shared environment or republish the shared
   checkout launcher. A synthetic layout driven by Hermes' own launcher code
   showed that, without it, a per-task home rebinds the checkout launcher to
   the per-task `tools` path, which would break every `hermes` once AIGW
   removes the disposable home.
3. Do not link `plugins`, `profiles`, or other home content. Operator plugins
   stay unloaded, as before, so verification depends only on the selected
   endpoint. In this layout Hermes may print a passive `venv: out of sync`
   verdict because the disposable home's plugin union is smaller than the
   committed one. The verdict does not change activation, and the adapter's
   existing diagnostic classifier remains its sole owner.
4. Fail closed when a link cannot be created, before starting Hermes. The
   error names the store and the platform cause without paths. Absent stores
   are skipped, so a Hermes without a managed data root behaves as before.
5. Cleanup keeps the existing `RemoveAll` of the disposable home, which
   removes the links without following them.

Rejected alternatives: a temporary profile (tenant visibility and per-profile
records persist), a borrowed root with its own synchronized environment
(network provisioning per verification), a copied environment or
`HERMES_RUNTIME_DIR` alone (activation still lacks `installs`), and running in
the operator's real home (writes sessions and cannot apply the probe policy).

## Risks / Trade-offs

- [Hermes changes its data-root layout] → The focused regression fixes the
  contract the adapter relies on, and the real-client journey must be rerun
  after a Hermes upgrade.
- [A damaged installation] → Hermes' own early recovery may repair it through
  the shared state, exactly as the operator's next launch would.
- [Windows hosts without symbolic-link permission] → Verification refuses with
  a categorized error instead of running in an unprovisioned home.
- [A configured home that is itself a Hermes profile] → Its stores live in the
  parent data root, so no store is shared and the version probe keeps its
  existing refusal.
- [Upstream defect] → The per-task layout republishes the shared launcher when
  on-demand installation is allowed. It is reported as
  [NousResearch/hermes-agent#136094][upstream-issue] (open on 2026-10-10); the
  probe policy keeps AIGW off that path meanwhile.

## Migration Plan

No data migration or client configuration change. Rollback is reverting the
adapter change; the operator's Hermes state is not modified either way.

## Local Qualification

The focused regression `TestHermesVerificationSharesTheInstalledDependencyState`
failed against the unchanged adapter at the missing probe policy, then passed
after the change together with every Hermes test in `internal/client` and
`internal/hermes/configuration`. Strict official OpenSpec validation passed for
all changes and specifications. `mise run check` and `mise run native` passed on
the staged tree with 95.17% aggregate statement coverage.

On the operator host, the installed AIGW 0.3.3 reproduced "hermes executable
identity could not be observed". A source-built candidate then passed the
version probe against the installed Hermes 0.21.6 and stopped at inference:
its own versioned credential reader is neither staged nor trusted by the
macOS keychain, so it cannot read the Account Token. A manual probe with the
same disposable layout, probe policy, and arguments, using the operator's
existing signed reader, returned `AIGW_OK`. Its only stderr lines were the
passive `venv: out of sync` verdict and a session identifier, which the
adapter's classifier accepts. Snapshots of Hermes configuration, launchers,
installation state, plugins, and AIGW configuration were unchanged by both
runs, and no disposable home remained.

A complete `aigw verify --for hermes` from AIGW itself requires a signed,
installed candidate whose versioned reader the keychain trusts. That run belongs
to the post-archive release workflow and stays a delivery obligation, not a
pre-archive task.

[pm-state]: https://github.com/NousResearch/hermes-agent/blob/v0.21.6/website/docs/reference/package-management.md#writable-state
[hermes-env]: https://github.com/NousResearch/hermes-agent/blob/v0.21.6/website/docs/reference/environment-variables.md
[upstream-issue]: https://github.com/NousResearch/hermes-agent/issues/136094
