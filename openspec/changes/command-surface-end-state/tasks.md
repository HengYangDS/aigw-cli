## 1. Names in code

- [ ] 1.1 Rename the Go identifiers of `internal/domain`, `internal/store` and
      `internal/secret` from Account, Route and canonical Model to Provider,
      Model and Family; TOML and JSON tags, user-visible text and file bytes
      stay, and every acceptance script passes unchanged.
- [ ] 1.2 Apply the same rename in `internal/app`, `internal/adapter`,
      `internal/cli` and `tools`; the architecture gate and every acceptance
      script pass unchanged.

## 2. Shared grammar and output

- [ ] 2.1 Add `provider/model` references, split at the first `/` and resolved
      to a configured model, with closest-match refusals for unknown providers,
      clients and models; tests cover a model ID that contains `/` and an
      argument without one.
- [ ] 2.2 Accept `anthropic`, `responses` and `chat` as protocol names at the
      command line and refuse any other spelling with the valid list.
- [ ] 2.3 Render every failure as Problem and one exact Fix, without the
      generic Impact text or an unrelated fix; existing scripts show the new
      failure blocks.
- [ ] 2.4 Add completion functions for clients, providers, `provider/model`,
      protocols, depths, families and capabilities that read only local
      configuration; a test whose secret store and HTTP client fail on any use
      shows that they read no Token and send no request.

## 3. Model, provider and client commands

- [ ] 3.1 Add `aigw model list|show|add|edit|remove`: list filters by query,
      `--provider`, `--client` and `--protocol`; add and edit take `--protocol`,
      `--family`, `--capability` and `--label`; add checks the provider's
      catalog unless `--unlisted`, and an unreadable or empty catalog refuses
      naming `--unlisted`; remove refuses a model in use and drops its
      recommendations; `model.txtar` covers each.
- [ ] 3.2 Add `aigw model search [query] [--provider]`, one row per model with
      state `added`, `available`, `missing` or `deprecated`, merging the
      per-protocol reads within the existing bound; an unreadable catalog
      prints its fix while the others print, and the command exits 1.
- [ ] 3.3 Add `aigw provider list|show|add|edit|rotate|balance|remove`: add
      validates its Token with one model-free request; edit takes `--label`,
      `--endpoint` and the balance credentials; balance reports every provider
      by default; remove deletes the provider's models and Token items unless a
      client uses one; without a terminal, add and rotate never prompt and
      refuse naming `--token-stdin`; `provider.txtar` covers each.
- [ ] 3.5 Show each client's model as `provider/model`, with the new words, in
      `aigw status` and its document; `status.txtar` covers a configured, a
      deferred and an unselected client.
- [ ] 3.4 Add `aigw client list` and `aigw client edit` with `--executable`,
      `--target`, `--via`, `--direct` and the retry flags, while
      `client enable` keeps `--executable` and `--target`; `client.txtar`
      covers each.

## 4. Use, check and setup

- [ ] 4.1 Rework `aigw use <client> <provider/model>`: it prompts for missing
      arguments only in a terminal, adds an unadded model or protocol after the
      catalog check, derives the family, reports a lost tool search, and clears
      forwarding and retries only on a change of provider; `use.txtar` covers
      each branch.
- [ ] 4.2 Rework `aigw check [client...] --depth local|endpoint|model|client`:
      each row names the depth reached, a depth that a client cannot run is
      skipped with its reason, and the command exits 1 only when a checked
      client failed or no client is enabled; `check.txtar` covers each reason.
- [ ] 4.3 Add `--verbose`, and `--model <provider/model>` with an optional
      `--token-stdin` at depths `endpoint` and `model`, to `aigw check`; the
      one-time Token is neither read from nor written to the store.
- [ ] 4.4 Make `aigw check --depth client` without named clients write the
      verified snapshot when no client failed and every session-capable client
      ran one, recording each client's depth; a test with Claude Desktop
      enabled writes it.
- [ ] 4.5 Rework `aigw setup` into a guided first run that can run again, and
      `aigw setup --from <file>` with the team-file merge rules of design.md;
      without a terminal, bare `aigw setup` refuses naming `--from`;
      `setup.txtar` covers a differing endpoint kept and reported, `--prune`,
      `--provider <id> --token-stdin` and a deferred Token.

## 5. Maintenance commands and the command tree

- [ ] 5.1 Fold `repair` into `aigw sync`, which shows models as
      `provider/model`, and add `aigw config path`, `aigw config export`,
      which writes a team file, and `aigw config rollback` with
      `--last-change`.
- [ ] 5.2 Add `aigw self install|update|rollback|uninstall|status`, replacing
      `install`, `update`, `update --rollback`, `uninstall` and
      `installation`.
- [ ] 5.3 Add `aigw version`, and make bare `aigw` show `status` or, without
      configuration, name `aigw setup`.
- [ ] 5.4 Remove every superseded command, flag and package, with
      `internal/app/renaming`; every script and acceptance test uses only the
      new tree, root help groups it as design.md does, and the command-tree
      test lists exactly its 12 top-level commands and 32 leaves.
- [ ] 5.5 Give every leaf command one to three help examples, and add tests
      that fail a leaf without examples, a positional argument or filter flag
      without completion, and any help, golden human output or JSON key that
      names an Account, Route, projection, binding, adapter, admitted or
      candidate.
- [ ] 5.6 Give each instrumented `aigw` process of the acceptance scripts its
      own coverage directory, merged after the run, so that concurrent
      processes no longer race on the coverage metadata file; the Windows
      guest job passes without that diagnostic.

## 6. One configuration file

- [ ] 6.1 Keep configuration only in a schema 8 `aigw.toml`, with its backup
      and verified snapshot, and use the short protocol names internally;
      component files, `order` handling and the schema 3 and 5 migrations are
      gone, and round-trip and strict-decode tests pass.
- [ ] 6.2 Add the one-time import of a schema 6 `config.toml` and its four
      component files; golden tests cover an owner-shaped configuration of 3
      providers and 51 models with forwarding and capabilities, and the shipped
      team manifest.
- [ ] 6.3 Run the import from `aigw sync` as design.md specifies; tests show
      that `aigw.toml` wins when both files exist, that a repeated sync imports
      nothing, that other commands name `aigw sync`, and that `config.toml` and
      its components stay byte-identical.
- [ ] 6.4 Make the credential reader decode only the frozen view and ignore
      every other field; a test with unknown fields and version 99 passes, and
      fingerprints equal 0.6.1's for the same client, provider and endpoint.
- [ ] 6.5 Stage the file that the previous program reads for the program check
      of `aigw self rollback`, `config.toml` before 0.7.0 and `aigw.toml`
      otherwise, let it run while an import is pending, and drop the
      retry-policy refusal of the Homebrew rollback; tests cover each.
- [ ] 6.6 Keep Hermes provider IDs in today's spelling through an adapter-owned
      map; a test pins the IDs for all three protocols.

## 7. Documents

- [ ] 7.1 Rewrite `manifests/team.toml` as a schema 8 team file, which
      `aigw setup --from` applies in a test.
- [ ] 7.2 Rewrite the README, guides, concepts, terminal experience, the cask
      caveats and every other document that names a removed command or word;
      the Markdown, link and spelling checks pass.
- [ ] 7.3 Add a decision record that supersedes DR-0003 and DR-0004, and
      register it.
- [x] 7.5 Rename the `route-client-selection` specification to
      `model-selection`, and rewrite the Purpose sections that still name an
      Account or a Route, which a delta cannot change.
- [ ] 7.4 Record the breaking changes and the upgrade step in CHANGELOG, set
      `VERSION` to 0.7.0 and give CHANGELOG its 0.7.0 section;
      `validate-changelog` passes.

## 8. Verification

- [ ] 8.1 Run the local quality graph at the lane head.
- [ ] 8.2 Simulate the archive on a copy: strict validation passes, and no
      merged specification exceeds 500 non-blank lines.
- [ ] 8.3 Confirm the hosted three-platform graph on the proposal.
- [ ] 8.4 Run the real-client journeys from v0.6.1, including the upgrade
      through `aigw sync` and a rollback.

## 9. Publication and acceptance

- [ ] 9.1 Tag v0.7.0, notarize its macOS archives, and publish its Releases on
      both peers and its Homebrew Cask.
- [ ] 9.2 Install 0.7.0 through Homebrew on this Mac and accept it:
      `aigw check --depth client` passes with the real clients, a Codex session
      started before the upgrade keeps its credential, a rollback to 0.6.1 and
      back keeps every Token and client, and `aigw --help` matches design.md.
- [ ] 9.3 Open the Change that removes the schema 6 import and files in 0.8.0.
