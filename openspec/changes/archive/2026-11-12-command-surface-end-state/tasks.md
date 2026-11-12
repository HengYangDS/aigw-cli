## 1. Names in code

- [x] 1.1 Rename the Go identifiers of `internal/domain`, `internal/store` and
      `internal/secret` from Account, Route and canonical Model to Provider,
      Model and Family; TOML and JSON tags, user-visible text and file bytes
      stay, and every acceptance script passes unchanged.
- [x] 1.2 Apply the same rename in `internal/app`, `internal/adapter`,
      `internal/cli` and `tools`; the architecture gate and every acceptance
      script pass unchanged.

## 2. Shared grammar and output

- [x] 2.1 Add `provider/model` references, split at the first `/` and resolved
      to a configured model, with closest-match refusals for unknown providers,
      clients and models; tests cover a model ID that contains `/` and an
      argument without one.
- [x] 2.2 Accept `anthropic`, `responses` and `chat` as protocol names at the
      command line and refuse any other spelling with the valid list.
- [x] 2.3 Render every failure as Problem and one exact Fix, without the
      generic Impact text or an unrelated fix; existing scripts show the new
      failure blocks.
- [x] 2.4 Add completion functions for clients, providers, `provider/model`,
      protocols, depths, families and capabilities that read only local
      configuration; a test whose secret store and HTTP client fail on any use
      shows that they read no Token and send no request.
- [x] 2.5 Refuse a path that runs through a regular file as `not a directory`,
      naming that file, at every product check that takes a missing path for
      an absent file, through one `internal/platform` function; Windows reports
      such a path as ERROR_PATH_NOT_FOUND, and tests that map an error to it
      show the refusal for the file snapshot, the configuration load, the file
      Token store and the install checks.

## 3. Model, provider and client commands

- [x] 3.1 Add `aigw model list|show|add|edit|remove`: list filters by query,
      `--provider`, `--client` and `--protocol`; add and edit take `--protocol`,
      `--family`, `--capability` and `--label`; add checks the provider's
      catalog unless `--unlisted`, and an unreadable or empty catalog refuses
      naming `--unlisted`; remove refuses a model in use and drops its
      recommendations; `model.txtar` covers each.
- [x] 3.2 Add `aigw model search [query] [--provider]`, one row per model with
      state `added`, `available`, `missing` or `deprecated`, merging the
      per-protocol reads within the existing bound; an unreadable catalog
      prints its fix while the others print, and the command exits 1.
- [x] 3.3 Add `aigw provider list|show|add|edit|rotate|balance|remove`: add
      validates its Token with one model-free request; edit takes `--label`,
      `--endpoint` and the balance credentials; balance reports every provider
      by default; remove deletes the provider's models and Token items unless a
      client uses one; without a terminal, add and rotate never prompt and
      refuse naming `--token-stdin`; `provider.txtar` covers each.
- [x] 3.4 Add `aigw client list` and `aigw client edit` with `--executable`,
      `--target`, `--via`, `--direct` and the retry flags, while
      `client enable` keeps `--executable` and `--target`; `client.txtar`
      covers each.
- [x] 3.5 Show each client's model as `provider/model`, with the new words, in
      `aigw status` and its document; `status.txtar` covers a configured, a
      deferred and an unselected client.
- [x] 3.6 Accept a configuration with providers and no model: `provider add`
      works from an empty home, `model remove` and `provider remove` may take
      the last model, and clients left without one report it; tests cover
      each.

## 4. Use, check and setup

- [x] 4.1 Rework `aigw use <client> <provider/model>`: it prompts for missing
      arguments only in a terminal, adds an unadded model or protocol after the
      catalog check, derives the family, reports a lost tool search, clears
      forwarding when the upstream endpoint changes and retries only on a change
      of provider, and names other clients whose files change; it binds the
      `provider/model` completion of `use <client> <TAB>`; `use.txtar` covers
      each branch and runs the `aigw use …` fixes that the model commands
      already name.
- [x] 4.2 Rework `aigw check [client...] --depth local|endpoint|model|client`:
      each row names the depth reached, a depth that a client cannot run is
      skipped with its reason, and the command exits 1 when a checked client
      failed, or when no client is named and none is enabled, and it lists
      stored Tokens that no provider uses; `check.txtar` covers each reason
      and a named disabled client.
- [x] 4.3 Add `--verbose`, and `--model <provider/model>` with an optional
      `--token-stdin` at depths `endpoint` and `model`, to `aigw check`; the
      one-time Token is neither read from nor written to the store.
- [x] 4.4 Make `aigw check --depth client` without named clients write the
      verified snapshot when no client failed and every session-capable client
      ran one, recording each client's depth; a test with Claude Desktop
      enabled writes it.
- [x] 4.5 Rework `aigw setup` into a guided first run that can run again, and
      `aigw setup --from <file>` with the team-file merge rules of design.md;
      without a terminal, bare `aigw setup` refuses naming `--from`;
      `setup.txtar` covers a differing endpoint kept and reported, `--prune`,
      `--provider <id> --token-stdin` and a deferred Token.
- [x] 4.6 Take Codex's bundled model metadata from the model's family and retire
      the Codex model catalog's suffix-derived namespace mirror; tests cover a
      variant linked by family and a model that is its own family.
- [x] 4.8 Show a path inside home as `~/…` in the success lines of
      `client edit` and `self install`, which print the absolute home path;
      scripts cover both.
- [x] 4.9 Name every cause that blocks a write in the first fix: a file that
      is both lock-flagged and sandbox-denied names the flag and the denial
      together; a test covers it.
- [x] 4.10 Give a Token store read that a sandbox denies the generic denial
      fix naming its `~/…` folder, in `check` and `status`; a test covers both.
- [x] 4.11 State in the command-line delta the reason that each path
      condition prints on every platform, where the scenario now asks for the
      operating-system text; strict validation passes.
- [x] 4.12 Keep a disabled client disabled when the fix of its refused
      selection runs; a script covers it.
- [x] 4.13 Make `model show` and `model list` mark a protocol whose provider
      has no endpoint and name `aigw provider edit <provider> --endpoint`,
      instead of an empty URL; scripts cover both.
- [x] 4.14 After a deferred Token is stored, make `status` name
      `aigw setup --from <file>` again rather than `aigw provider rotate`; a
      script covers it.
- [x] 4.15 Report a client whose provider holds an empty Token as deferred,
      with the fix that stores one, in `status`; a test covers it.
- [x] 4.16 Name the entry of the team file to change in each refusal of an
      invalid team file, instead of `aigw check`; scripts cover each refusal.
- [x] 4.17 Report each model once in `setup --from`; a test covers a model
      that the file and the local configuration both name.
- [x] 4.18 State how a team file's provider order merges, in the
      progressive-team-onboarding delta, and make setup follow it; strict
      validation passes and a test covers it.
- [x] 4.19 Name the one compatible provider's command in an activation fix
      that has one choice, instead of "Choose one compatible provider"; a
      test covers it.
- [x] 4.20 Refuse in strict decoding a model without a protocol, naming the
      model, and a provider ID that has an uppercase letter or differs from
      another only in case; tests cover each (`2d03e14c`, `1095c180`).
- [x] 4.21 Make guided `aigw setup` enable the client it configures and say
      so, and give a disabled client that has a selection the fix
      `aigw client enable <client>`; the progressive-team-onboarding delta
      has the scenario, and tests cover both (`16bfb30c`, `e09db347`).

## 5. Maintenance commands and the command tree

- [x] 5.1 Fold `repair` into `aigw sync`, which shows models as
      `provider/model`, and add `aigw config path`, `aigw config export`,
      which writes a team file, and `aigw config rollback` with
      `--last-change`.
- [x] 5.2 Add `aigw self install|update|rollback|uninstall|status`, replacing
      `install`, `update`, `update --rollback`, `uninstall` and
      `installation`.
- [x] 5.3 Add `aigw version`, and make bare `aigw` show `status` or, without
      configuration, name `aigw setup`.
- [x] 5.4 Remove every superseded command, flag and package, with
      `internal/app/renaming`; every script and acceptance test uses only the
      new tree, except that a journey's baseline program keeps its own
      commands, root help groups it as design.md does, and the command-tree
      test lists exactly its 12 top-level commands and 32 leaves.
- [x] 5.5 Give every leaf command one to three help examples, and add tests
      that fail a leaf without examples, a positional argument or filter flag
      without completion, and any help, golden human output or JSON key that
      names an Account, Route, projection, binding, adapter, admitted or
      candidate; decide how help shows a boolean flag and take
      `spf13/pflag` v1.1.0, whose boolean help moves every help golden.
- [x] 5.6 Give each instrumented `aigw` process of the acceptance scripts its
      own coverage directory, merged after the run, so that concurrent
      processes no longer race on the coverage metadata file; the Windows
      guest job passes without that diagnostic.
- [x] 5.7 Embed only the GitHub release source in official builds: remove the
      GitLab build variables and their linker flags, make release construction
      refuse an environment that sets a GitLab release source, and update
      `docs/architecture/repository-tooling.md`; tests show the refusal and
      that `aigw self update` still reads a GitLab peer named at run time.
- [x] 5.8 Add a testscript command that reruns a failure's `next_action` and
      fails when the same problem recurs, and use it in each script that
      covers a failure.
- [x] 5.9 Print the JSON failure document of a usage error in a command run
      with `--json`, wherever `--json` stands; fixloop's root exemption goes.
- [x] 5.10 Name a model and a provider in every configuration failure text,
      without "route" or "account", and use no retired word in any other
      user-facing failure text; phase 1's final review found about 150
      strings to change, such as `internal/domain/model_errors.go:12` and
      `internal/adapter/claude/adapter_verify.go:37`. Add a test that renders
      each `terminal` explanation of a domain error and fails on a retired
      word.

## 6. One configuration file

- [x] 6.1 Keep configuration only in a schema 8 `aigw.toml`, with its backup
      and verified snapshot, and use the short protocol names internally;
      component files, `order` handling and the schema 3 and 5 migrations are
      gone, and round-trip and strict-decode tests pass.
- [x] 6.4 Make the credential reader decode only the frozen view and ignore
      every other field; a test with unknown fields and version 99 passes.
- [x] 6.8 Read and write configuration only in `aigw.toml`: a home that holds
      only `config.toml` and its component files is unconfigured and names
      `aigw setup`, and a test shows that those files and an existing versioned
      reader keep their bytes through `setup`, `use` and `sync`.
- [x] 6.9 Make `aigw setup` and `aigw setup --from` use a Token already stored
      for a provider they add, without a prompt, and say so; a script covers
      both.
- [x] 6.10 Drop the Homebrew rollback's retry-policy refusal, and make
      `aigw self rollback`'s help say that its check gives the previous program
      the current configuration; tests cover a Homebrew rollback with retry
      budgets and a portable rollback whose program cannot read that
      configuration.
- [x] 6.11 Derive Hermes provider IDs from the provider ID and the protocol
      name, without a map, once 6.1 shortens the names; a test pins
      `aigw-dmxapi-anthropic`, `aigw-dmxapi-responses` and `aigw-dmxapi-chat`.
- [x] 6.12 Rename the balance-credential variables to
      `AIGW_BALANCE_TOKEN_<ID>` and `AIGW_BALANCE_USER_ID_<ID>`, without the
      old names; every fix, document and CI file names only the new ones, and
      a test covers the environment backend.

## 7. Documents

- [x] 7.1 Rewrite `manifests/team.toml` as a schema 8 team file, which
      `aigw setup --from` applies in a test; it keeps the 2026-11-07 model
      refresh that lands on dev first. Plain-DMXAPI `gpt-6.1-terra` and
      `claude-haiku-5-5` stay out until a resumed tool turn qualifies them,
      in a later team-file refresh (`275bcdef`).
- [x] 7.2 Rewrite the README, guides, concepts, terminal experience, the cask
      caveats and every other document that names a removed command or word;
      the Markdown, link and spelling checks pass.
- [x] 7.3 Add DR-0012, which supersedes DR-0003 and DR-0004, and register
      it (`6f160c37`, `b036069a`).
- [x] 7.5 Rename the `route-client-selection` specification to
      `model-selection`, and rewrite the Purpose sections that still name an
      Account or a Route, which a delta cannot change.
- [x] 7.4 Record the breaking changes and the upgrade step in CHANGELOG, set
      `VERSION` to 0.7.0 and give CHANGELOG its 0.7.0 section;
      `validate-changelog` passes.

## 8. Verification

- [x] 8.1 Run the local quality graph at the lane head (check r-1 at
      `9bf7612c`, coverage 95.54%).
- [x] 8.2 Simulate the archive on a copy: strict validation passes, and no
      merged specification exceeds 500 non-blank lines (15 of 15 valid;
      largest 498 after `19bc1218`).
- [x] 8.3 Confirm the hosted three-platform graph on the proposal (GitHub
      run 43103038473 and GitLab pipeline 15075 at `9bf7612c`).
- [x] 8.5 Refuse a published predecessor older than 0.7.0 before any release
      journey downloads or runs it, and retire the GitHub transport-release
      carrier of the hosted journeys; `git grep` finds no `input_release` or
      `native-inputs-` outside archived Changes and released CHANGELOG
      sections.
- [x] 8.6 Rehearse the owner's upgrade in an isolated home with a file Token
      store: from a read-only record of the owner's non-secret configuration,
      recreate it with the lane head's `setup --from`, `provider edit`,
      `model add`, `client edit` and `use`, and compare the result field by
      field with the record; the command list is kept for 9.2.

## 9. Publication and acceptance

- [x] 9.1 Tag v0.7.0, notarize its macOS archives, and publish its Releases on
      both peers and its Homebrew Cask, after task 5.7; a deep scan of every
      0.7.0 asset, nested archives included, finds no form of the private
      peer's identity (tag `c08b5ec3`, notarization `244fe3fc`, tap
      `7ea1d84`; 0 hits in 12 assets).
- [x] 9.2 Install 0.7.0 through Homebrew on this Mac and accept it: with each
      client file and sidecar recorded first, the rehearsed commands recreate
      the owner's state and it matches the record, `aigw check --depth client`
      passes with the real clients, a Codex session started before the
      upgrade keeps its credential, and `aigw --help` matches design.md
      (2026-11-12: 176 of 176 fields match; every client check passes from a
      clean environment, with a locally patched Hermes v0.23.1; the 0.6.1
      reader's credential commands return their Tokens; the defects found go
      to Change `upgrade-acceptance-fixes`).
