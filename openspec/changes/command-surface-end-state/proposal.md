## Why

The command surface grew one feature at a time. Measured on 0.6.1 with a configuration of 3 Accounts, 51 Routes and 4 clients:

- 24 top-level commands overlap. Six answer "does it work?", four answer "which models?", and four create a provider or model.
- Scope has no common form: `aigw catalog` cannot name a provider, and `aigw balance` refuses when more than one Account exists.
- The user must invent a Route ID that restates the Account and the provider's model ID.
- Help and output speak the implementation's nouns (Account, Route, projection, binding, adapter, admitted, candidate) rather than the user's: provider, model and client.
- Every official archive embeds the private GitLab peer's origin and project path as an update source, and the owner ruled on 2026-11-06 that this identity must not be public.

On 2026-11-06 the owner called this systemic, asked for the ideal end state, and allowed complete breaking changes without historical compatibility.

## What Changes

- **BREAKING** Three nouns: provider (was Account), model, named `provider/model` (was Route), and client. A model's name is derived from the provider's own model ID, and no user-chosen ID remains.
- **BREAKING** 12 top-level commands replace 24: `status`, `use`, `check`, `model`, `provider`, `client`, `setup`, `sync`, `config`, `self`, `completion` and `version`. These commands are removed: `doctor`, `test`, `verify`, `catalog`, `models`, `route`, `account`, `add`, `rotate`, `balance`, `repair`, `rollback`, `install`, `installation`, `update`, `uninstall`, `config import`, `config migrate`, `account rename` and `route rename`.
- **BREAKING** One flag vocabulary. `--for`, `--account`, `--route`, `--upstream-model`, `--all`, `--purpose`, the per-protocol URL flags and the rename and import flags go. Protocols are named `anthropic`, `responses` and `chat`.
- `aigw check` becomes one ladder of evidence with depths `local`, `endpoint`, `model` and `client`. A depth that a client cannot run is skipped with its reason.
- `aigw model search` queries authenticated catalogs by text and provider, and `aigw use` adds a listed model in the same step.
- **BREAKING** One configuration file, `aigw.toml`, in schema 8, replaces `config.toml` and its four component files. `aigw sync` imports a schema 6 configuration once and leaves the old files untouched for running clients. The credential reader decodes a frozen view.
- **BREAKING** Official archives embed only the public GitHub release source. A GitLab peer is named at run time through `AIGW_GITLAB_RELEASE_ORIGIN` and `AIGW_GITLAB_RELEASE_REPOSITORY`, and release construction refuses an environment that sets either.
- Every failure names one exact fix, every command has examples, and every positional and filter flag completes from local configuration.

**Authority boundary:** AIGW still owns only its configuration, Tokens and client projections. It carries no traffic and manages no client session.

**Reuse stance:** the existing renderer, JSON document contract, transactional projection, credential storage, Cobra completion and acceptance-script harness carry over. No dependency is added.

**Non-goals:** a full-screen browser; a change to client files beyond the credential reader's path; credential storage or signing changes, and release-process changes beyond the embedded release source; runtime routing. The removal of the schema 6 import and files is the separate Change for 0.8.0.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `model-selection`, renamed from `route-client-selection` before this Change archives: how a client uses one provider's model, named `provider/model`, including adding it from the provider's catalog.
- `command-line`: the command tree, the flag vocabulary, failures with one fix, examples and completion.
- `cli-readiness`: the evidence ladder, its exit rule and the verified snapshot. The rename finalization goes.
- `product-control-plane`: providers, models and families in one configuration file.
- `progressive-team-onboarding`: guided setup that can run again, and team files applied by `setup --from`.
- `projection-format`: previews and sync output in the new vocabulary.
- `client-adapters`: adapter obligations in the new vocabulary, with Hermes provider IDs unchanged.
- `credential-continuity`: the frozen reader view and the one-time import that keeps running clients working.
- `secret-storage`: Token and balance credentials owned by `provider` commands.
- `program-lifecycle`: `self install`, `update`, `rollback`, `uninstall` and `status`, and official archives that embed only the public release source.
- `repository-organization`: wording only.

## Impact

Every command, the configuration format, all acceptance scripts, 10 capability specifications, the documentation and the team file change. Client files change only in the credential reader's path. The release is 0.7.0. Upgrading requires one `aigw sync`, and a rollback to 0.6.1 keeps working because the old files stay until 0.8.0.
