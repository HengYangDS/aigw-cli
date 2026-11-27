## 1. Specifications and decisions

- [x] 1.1 Write the deltas of every capability that schema 9 changes. Done when
      `openspec validate configuration-holds-intent --strict` passes and the lane's archive rehearsal reports every
      merged specification within the size limit.
- [x] 1.2 Add DR-0013 to `docs/decisions/` with its index entry, and DR-0012 point 4's sentence on removing
      `authentication`. Done when the prose and Markdown gates pass on the three files.

## 2. Schema 9 without the client-native path

- [x] 2.1 Move the configuration and team-file version to 9 and refuse version 8 before reading anything else, with the
      fixed text that names the edits. Done when a store test pins the refusal of a version 8 file and of a
      version 9 file that still holds `executable`, `authentication` or `model_provider`, each by field and line.
- [x] 2.2 Remove `authentication` and `model_provider` from the client and recommendation documents and the domain,
      the client specification's two client-native fields, and their validation and retry rules. Done when no source
      outside tests names the removed identifiers and the package tests of `internal/domain/...`, `internal/store/...`
      and `internal/cli/setup/...` pass.
- [x] 2.3 Write Codex's provider table always under AIGW's own provider name with the credential command. Done when
      the package tests of `internal/adapter/codex/...` pass and no Codex test projects a client-native table.
- [x] 2.4 Stop decoding `authentication` in the credential reader's view. Done when the reader's tests pass and the
      view decodes only providers' IDs and endpoints and clients' enabled state.
- [x] 2.5 Move `manifests/team.toml` and `aigw config export` to version 9. Done when the shipped team file's test and
      the export tests pass.

## 3. The program declaration

- [x] 3.1 Replace the client's `executable` with `program` and `extensions` in the domain and the store, syntax-checked
      at load (absolute, clean, not both). Done when store and domain tests pin each refusal by field.
- [x] 3.2 Resolve `extensions` in `adapter.ClientProgram` from each adapter's declared extension ID and program path:
      this host's platform, `.obsolete` dropped, highest version. Done when fake-tree tests pass for two versions side
      by side, an obsolete highest version, no `.obsolete`, an unreadable `.obsolete`, no entry for this platform and a
      highest entry without its program.
- [x] 3.3 Adopt, for a client that declares neither, the program found on the PATH or at the client's fixed
      application path, write it as `program` and report the adoption. Done when a sync test reads the written
      declaration and the report, and a second sync leaves `aigw.toml` byte-identical.
- [x] 3.4 Give `aigw client enable` and `aigw client edit` `--program` and `--extensions`, exclusive, with no program
      flag required; refuse `--extensions` for Claude Desktop and Hermes, an `--extensions` directory that resolves to
      no runnable program, and a `--program` path inside a versioned extension directory with the `--extensions`
      command. Done when the client command tests and `sync.txtar` pass.
- [x] 3.5 Name `--program` in every repair that named `--executable`. Done when no file under `internal/` or `cmd/`
      names `--executable` except the test that refuses it as an unknown flag.
- [x] 3.6 Report `program` and `extensions` in `aigw client list`, human and JSON. Done when the list tests and their
      golden files pass.
- [x] 3.7 Derive whether a client's files are projected from its enabled state, its targets and AIGW's record. Done
      when a test of an enabled client with a declared but unresolved program reports its files' real state.

## 4. Forwarding through a service root

- [x] 4.1 Make a client's `forwarding` one string, the root, in the document and the domain, and add the fourth
      schema 9 edit to the version 8 refusal. Done when store and domain tests pin the root's refusals (not
      `https` or loopback `http`, user information, query, fragment) and the refusal text names the four edits.
- [x] 4.2 Compose the destination `<root>/<scheme>/<authority>/<base path>` from the selected provider's Responses
      endpoint, host in lower case without a default port, and refuse an endpoint outside openai-responses-proxy
      8.0.0's grammar, in one function of the domain that resolution, selection and provider edits use. Done when
      table tests pass for the four providers in use, a port, a default port, an upper-case host, a trailing slash
      and each refused form.
- [x] 4.3 Keep forwarding through a change of model, provider or provider endpoint, refuse a command's change that the
      grammar cannot name with the `--direct` fix, let a synchronization that follows a model chosen in the client
      drop the forwarding with `forwarding_cleared`, and clear retry budgets on a change of provider as before. Done
      when the selection, synchronization and provider-edit tests of each scenario of the two forwarding requirements
      pass.
- [x] 4.4 Project the composed destination as Codex's and Hermes's base URL, and name `--via <root>` in help and
      examples. Done when a Codex projection test reads `http://127.0.0.1:8792/https/www.dmxapi.cn/v1` and no source
      or golden file names `--via http://127.0.0.1:8792/v1`.

## 5. Release journeys

- [ ] 5.1 Make the published-predecessor lifecycle journey migrate a version 8 configuration: the candidate refuses it
      and changes nothing, the journey makes the edits, the predecessor's captured credential command returns
      the same Token from the version 9 file, a program rollback refuses until the kept copy is restored, and the
      re-upgrade repeats the migration. Done when the journey compiles with `-tags client_acceptance` and its
      source-baseline path passes in `mise run native`.
- [x] 5.2 Use `program` in every other release journey and fixture, and give each step that the published 0.11.0
      runs its own flag through one helper, the only place under `tools/` that names `--executable`. Done when
      `go vet -tags client_acceptance ./tools/release/...` passes.
- [x] 5.3 Remove the `forwarding-performance` scope and the `--performance-forwarding` flag from the release tool,
      its samples and its documentation. Done when `go vet -tags performance_acceptance ./tools/release/...` passes
      and no file outside the CHANGELOG and the Changes names either.
- [x] 5.4 Admit a program rollback whose retained program exports the configuration that it was given in that
      file's own schema, so that a version 8 copy restored after the upgrade goes back to 0.11.0, as the accepted
      program-lifecycle requirement asks, and name that copy in the refusal's repair. Done when the install tests admit
      a restored version 8 file and refuse an export in another schema than the file's, and the rollback acceptance
      test finds the copy in the repair.

## 6. Documents

- [ ] 6.1 Update the configuration and client references, the team rollout guide and the terminal-experience document
      for `program`, `extensions` and schema 9. Done when the prose and Markdown gates pass.
- [ ] 6.2 Add the breaking changes and the upgrade edits, with an example, under `## Unreleased` in `CHANGELOG.md`.
      Done when `go run ./tools/release validate-changelog` passes.

## 7. Local gates

- [ ] 7.1 Pass `mise run check` on the lane head, in the host queue.
- [ ] 7.2 Pass `mise run native` on the macOS host on the lane head, in the host queue.

## 8. Review

- [ ] 8.1 A fresh review of the whole change by a reviewer who has not seen it. Done when its findings are answered in
      code or in the design.

## 9. Exact-HEAD proof

- [ ] 9.1 Pass the ETHOS full proof of the lane head.

## 10. Hosted CI

- [ ] 10.1 Pass the proposal's GitHub checks.
- [ ] 10.2 Pass the proposal's GitLab pipeline on the macOS and Linux runners.

## 11. Publication

- [ ] 11.1 Publish the proven lane head as the lane's proposal branch on both forges. Done when each forge reads back
      the exact head.
