---
subject: aigw:decision:0012
role: decision
state: canonical
decision_id: DR-0012
decision_status: accepted
relations:
  canonical_for: Name Providers, Models and Clients, and Upgrade Without an Importer
---

# DR-0012: Name Providers, Models and Clients, and Upgrade Without an Importer

- Date: 2026-11-12
- Supersedes: [DR-0003](dr-0003-account-rename-credential-migration.md) and [DR-0004](dr-0004-account-route-client-binding-authority.md)
- Owner: AIGW maintainers

## Context

DR-0004 made Account, Route and Client Binding the configuration authority, and
DR-0003 made an Account rename a two-phase credential migration. Measured on
0.6.1, those implementation objects reached help and output, and the user had
to invent a Route ID that restated the Account and the provider's model ID. Five
configuration files existed because earlier readers had to keep decoding
`config.toml`, and the account rename command took about 700 lines of code to protect a
rare operation.

On 2026-11-06 the owner ruled that complete breaking changes are allowed and
that history does not bind the end state. On 2026-11-10 the owner restated the
ruling for 0.7.0: the release carries no code for an earlier release, so it has
no importer, no rollback across 0.7.0 and no map from earlier Hermes provider
IDs.

## Decision

AIGW speaks of three nouns: provider, model and client.

- A provider is a service whose Token the user holds: an ID, a label, one
  endpoint per protocol, and a Token. It replaces the Account.
- A model is what one provider serves. Its only name is `provider/model`,
  derived from the provider's ID and the provider's own model ID, byte for
  byte, split at the first `/`. The user never invents it. It replaces the
  Route and its ID. An optional family names the canonical model that
  variants and providers share; it replaces the former canonical Model.
- A client selects one model and keeps its enabled intent, protocol,
  authentication and client-specific options. Protocols are `anthropic`,
  `responses` and `chat`.

Providers, which nest their models, families, recommendations and clients live
in one file, `aigw.toml`, at version 8. A team file has the same format
without the `[clients]` table. Array order is display order.

No command renames a provider or a model. To change a provider's ID, add the
provider under the new ID, point each client at its models with the use command,
then remove the old provider.

0.7.0 upgrades without an importer:

1. It reads and writes only `aigw.toml`, and never reads, changes, renames or
   deletes `config.toml` or its component files. A home that holds only those
   files has no configuration, and the bare program names the setup command.
2. The upgrade is the setup command, with a team file or without one, then the
   use command for each client and model. Token items are built
   from provider IDs, which do not change, so a provider added under its
   earlier ID reuses its stored Token without a prompt.
3. A client session that started before the upgrade keeps calling the
   credential reader it started with, which still reads the untouched
   `config.toml`. The user deletes the old files after those sessions end.
   AIGW never deletes them.
4. The credential reader decodes a frozen view of `aigw.toml` and ignores
   everything else: each provider's `id` and `endpoints`, and each client's
   `enabled` and `authentication`. Changing a frozen field takes a new file
   name, as this release does.
5. The program rollback command gives the retained program the current
   configuration and refuses a program that cannot read it, so it cannot return
   to a release older than 0.7.0. The Homebrew rollback keeps its Keychain
   custody step and has no retry-policy refusal. Hermes provider IDs join aigw,
   the provider ID and the protocol name, so a Hermes session that recorded an
   earlier ID does not find it after the upgrade.

## Alternatives Rejected

- **Keep the objects and unify only the grammar:** rejected because it keeps the
  invented Route ID and the three steps from a catalog entry to real use.
- **A full-screen terminal browser:** rejected until prompts and completion are
  shown to fall short.
- **An importer, a map of earlier Hermes IDs, or a rollback across 0.7.0:**
  rejected by the owner's ruling of 2026-11-10 and by DR-0009; each would be a
  reader of the old format that every later release carries.
- **A one-step rename command:** rejected because replacing a provider takes
  three commands and the operation is rare. It can return as one step if the
  owner wants it.
- **Deriving a family from a model ID's prefix or inferring a protocol from a
  catalog:** rejected because a wrong guess silently misdescribes a model's
  metadata or endpoint.

## Consequences and Boundary

Every user runs the setup command after upgrading. A team file restores providers,
models and recommendations; client selections, forwarding and retries are set
again with the use and client edit commands. A team member on 0.6.1 cannot
read a version 8 team file, so everyone upgrades first.

The old files stay on disk and hold no secrets. Removing a provider deletes
a Token that `config.toml` still names, so a surviving old session loses that
provider, which is what removing it means. An earlier release that the user
installs by other means finds its own files as they were before the upgrade,
without later changes.

Credential storage, Keychain item labels, the contents of client files beyond
the credential reader's path and the Hermes provider IDs, and the credential
command line that client files embed do not change.

## Evidence and Revisit

The OpenSpec Change `command-surface-end-state` holds the design, the
specifications and the acceptance for this release, including the rehearsed
operator steps by which the owner's own state crosses once.

Revisit only for a concrete obligation to read an earlier release's files, with
named users, a sunset condition, an owner and verification, as DR-0009
requires, or if the owner asks for a one-step rename.
