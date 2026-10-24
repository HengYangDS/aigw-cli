# Design

## Context

`codex.CopyProjection` copies an operator's Codex configuration, its AIGW state
and its AIGW catalog into a private home for `aigw verify`. Its
`rebaseCopiedCodexCatalog` moves only an AIGW-owned catalog reference. A
user-owned `model_catalog_json` is copied unchanged. Codex resolves a relative
reference from the directory of the configuration that names it, so the copy
points into the private home. Codex verifies an explicit Route through
`isolateCodexProjection`, which copies the projection and then switches the copy
to that Route, so both paths share this copy.

`clientverification.verificationFailure` names the retry
`aigw verify --for <client>` for a failed Codex or Claude request.
`VerifyCodexInvocation` and `VerifyClaudeRuntime` receive the resolved Runtime
but not whether the Route was explicit. `aigw test` names the same command when
a Route's authentication belongs to the client or an external helper.

`catalog.ParseIDs` decodes `/models` bodies with `encoding/json`. That decoder
accepts `null` for a slice and keeps the last of repeated object members.

`presentation.Rows` stacks a group whose values lack room and then wraps every
label with the explanatory-text rule. Root help puts its start-path commands
and subcommand names into labels.

`credential.TokenRecovery` names `AIGW_TOKEN_<ACCOUNT>` for the read-only
environment backend and `aigw rotate` otherwise. `diagnostics.Probe` classifies
HTTP responses without knowing the backend, and its HTTP 401 advice names
`aigw rotate`. `aigw status` already names both diagnostic variables for the
environment backend, and `aigw account balance` does so only when the
diagnostic credential is unreadable.

`configuration.Export` copies the declared recommendations and then sets each
bound client's primary recommendation to its binding. The manifest has no
binding field, so the export cannot tell a reviewed preference from a local
choice, and an import treats both as reviewed.

## Goals / Non-Goals

**Goals:** verification copies reach the operator's own catalog; retry advice
repeats the invocation; malformed catalogue responses establish nothing; help
commands stay copyable; recovery advice is executable on the selected backend;
an exported manifest carries only reviewed recommendations.

**Non-Goals:** copying, adopting or validating a user catalog; `~`, profile,
project or command-line catalog references; Codex metadata for non-GPT models;
new commands, flags, JSON fields or recovery frameworks; rejecting unrelated
provider fields; sharing client bindings through a manifest.

## Decisions

### The verification copy rebases a relative user catalog reference

When AIGW does not own the catalog, `rebaseCopiedCodexCatalog` reads the root
`model_catalog_json` value with the TOML parser. A relative value is rewritten
in the copy to that value joined with the source configuration's directory,
which is how Codex resolves it. Absolute values stay as they are. So do values
that start with `~`: verification keeps `HOME`, and whether Codex expands them
is unproved. Copying the catalog into the private home was rejected because
AIGW would duplicate user data that it does not own. Running the client against
the operator's own home was rejected because verification must not share its
state.

### The explicit Route travels to the retry advice

The Codex and Claude adapters pass their explicit Route to
`VerifyCodexInvocation` and `VerifyClaudeRuntime`. One `clientverification.Command`
composes `aigw verify --for <client>` and appends `--route <route>` only when
the invocation had one; `aigw test` uses it too, through an import edge that
the architecture policy now declares. Always appending the resolved
Route was rejected: an explicit Route takes the isolated switching path, so a
retry of a selected-binding verification would exercise another path. Editing
error text in the command layer was rejected because the failure owner already
composes it.

### Catalogue parsing refuses ambiguous members, not unknown ones

`ParseIDs` walks the top-level object and each object item member by member. It
refuses a repeated `data`, a `data` that is not an array, and a repeated `id`,
`model` or `name` within an item. Names match exactly, as item fields already
do. Other members, repeated or not, are ignored as before. Rejecting every
unknown or repeated member was rejected because provider extension fields are
not evidence. A second validating pass was rejected because two parsers could
disagree.

### Executable labels are written whole

`presentation.Field.Command` marks a label that is executable text. A stacked
group writes such a label on its own line without wrapping, as `Command` does
for examples, and the terminal soft-wraps it. Root help marks its start-path
commands and subcommand names. The public help width test exempts only the
start-path command lines, the sole labels wider than its narrowest width. Never
wrapping any label was rejected because other long labels are explanatory, and
the narrow-width contract bounds them. A separate command renderer was rejected
because it would repeat the group layout.

### Callers that know the backend supply its recovery

`aigw check` applies the backend's `credential.TokenRecovery` instruction to an
HTTP 401 result through `diagnostics.Result.WithTokenRecovery`, so human and
JSON output agree. The probe keeps its classification and HTTP knowledge, and
its default advice still names `aigw rotate`. `aigw account balance` names both
diagnostic variables for a read-only backend in the missing case too, as it
already does when the credential is unreadable. Passing the credential store
into the probe was rejected because the probe does not own credential backends.

### Export publishes declared recommendations only

`configuration.Export` writes the declared recommendations as they are and
derives nothing from Client Bindings, which stay in the local configuration.
This is the operator's decision for this Change. Removing only the duplicate
alternative was rejected because the binding would still displace the reviewed
primary. Exporting bindings in a separate, labelled section was rejected
because bindings are local choices, not team policy. A team that wants a
selection recommended declares it in the manifest.

## Risks / Trade-offs

- [A provider returns a null or repeated model list] → its Account shows a
  failed catalogue observation instead of a listing, which is the truthful
  state.
- [A command label is wider than the terminal] → the terminal soft-wraps it and
  the bytes stay intact; the copyable-command rule takes precedence over the
  width bound for these lines.
- [A relative catalog reference names a missing file] → the copy names the same
  missing file, so verification fails as the operator's Codex would.
- [A relative reference leaves a symlinked Codex home through `..`] → the copy
  joins it lexically; whether Codex resolves it the same way is unproved.
- [An administrator built a manifest by exporting local selections] → the next
  export carries no recommendation for those clients until the manifest
  declares them; the guide says so.

## Migration Plan

None.

## Open Questions

- None.
