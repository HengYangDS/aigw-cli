# Verification Recovery Truth

## Why

Independent qualification of AIGW 0.4.13 and 0.4.15 found six places where
AIGW's evidence, exports or recovery advice are not truthful:

- `aigw verify --for codex` copies the operator's Codex configuration into a
  private home. A user-owned `model_catalog_json` relative to that configuration
  then names a file that does not exist there, so verification fails before
  inference although the operator's Codex reads the catalog.
- After that failure, an explicit-Route verification recommends
  `aigw verify --for codex`, which drops `--route` and verifies another Route.
  `aigw test --route` drops it in the same way when it defers to `aigw verify`.
- `aigw catalog` and `aigw models` accept `"data": null` as an empty catalogue
  and keep the last of repeated `data` or identifier members. A malformed
  response becomes a confident observation, and `models` reports its Routes as
  "Not listed".
- At width 32, root help splits `aigw use --for <client> <route>` across two
  lines, although copyable commands must keep their logical lines.
- On the read-only environment credential backend, `aigw check` answers HTTP 401
  with `aigw rotate`, and `aigw account balance` answers missing diagnostic
  credentials with `aigw account diagnostics enable`. That backend refuses both.
- `aigw config export` writes each client's active binding as that client's
  primary recommendation. An operator's alternative or manual-only choice thus
  becomes the team's reviewed preference when the manifest is shared, and a
  selected alternative appears twice.

## What Changes

- A Codex verification copy rewrites a relative user-owned `model_catalog_json`
  to the absolute path that the operator's configuration resolves to. The
  catalog and the operator's configuration stay unchanged, and AIGW does not
  take ownership of the catalog.
- A failed verification names a retry with the invocation's `--route`, and
  `aigw test --route` names the same Route when it defers to `aigw verify`.
- Catalogue parsing requires the model list to be an array and refuses repeated
  `data`, `id`, `model` or `name` members, so such a response is a failed
  observation. Empty lists, the supported identifier shapes and other provider
  fields keep working.
- Help rows whose label is executable text keep the command whole at every
  width; their descriptions still wrap.
- On the environment backend, recovery advice names the variables to set:
  `AIGW_TOKEN_<ACCOUNT>` for a rejected Token, and both diagnostic variables
  when diagnostic credentials are absent.
- Export publishes the declared recommendations unchanged. Client bindings are
  the operator's own choices and stay local. **BREAKING:** a configuration whose
  clients are bound but which declares no recommendations now exports none.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `route-client-selection`: "Connectivity and verification use explicit Route
  semantics" keeps the Route in retry advice and keeps the operator's relative
  catalog reachable from the private projection. "Provider preference is
  advisory control-plane input" keeps Client Bindings out of exported
  recommendations.
- `product-control-plane`: "Model discovery reports catalogue evidence only"
  treats an ambiguous response as a failed observation.
- `secret-storage`: gains the requirement "Recovery advice follows the selected
  credential backend".

The narrow-width defect violates the existing `product-quality` requirement
"Terminal layout follows semantic fields and display width", which needs no
change.

## Impact

Recovery text changes only for the failures above, in human and JSON output.
A verification copy differs from the operator's configuration only in the
rewritten catalog reference. A provider response with a null or repeated model
list or identifier now shows a failed catalogue observation instead of a
listing. An exported manifest no longer depends on the exporting operator's
bindings; a team that relied on export to turn selections into recommendations
declares them in the manifest instead. No credentials, sessions, operator client
configuration or catalogs change, and no command or flag is added.

Out of scope: Codex model metadata for non-GPT models, `~`, profile and
command-line catalog references, and Hermes or Claude verification beyond the
retry text.
