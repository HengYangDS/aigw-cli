# Design

## Context

`status`, `check` and `doctor` classify every admitted client through the
client inspection in `internal/cli/readiness`. Activation in
`internal/activation` first decides which Accounts a read-only command
observes. With any client enabled, it observes the enabled clients' Accounts;
with none enabled, it considers the selected bindings in order and stops at the
first one that can proceed. It keeps those observations for the command.

For a client whose Account activation did not observe, the inspection read the
Account's credential metadata itself. With a credential backend present,
activation observes every enabled client's Account, so this extra read reached
only disabled bindings. When it failed, `ClassifyClient` reported the disabled
client as unavailable, because the credential observation case comes before the
"not installed or enabled" case. Status then selected `aigw doctor` as the next
action, and doctor reported problems while every enabled client was healthy.

## Goals / Non-Goals

**Goals:** a disabled binding's Account credential state cannot change another
client's state, the overall next action or the doctor result; read-only
commands read no credential metadata outside activation's scope.

**Non-Goals:** changing activation's scope or the classification order;
reporting a disabled binding's Token availability before it is enabled;
`verify`, `sync` and Account rename. A disabled binding still suggests
`aigw sync`, which skips disabled bindings; that pre-existing action is
unchanged here.

## Decisions

### Activation owns the observation scope

For a disabled binding, the inspection uses only activation's credential
observations. When activation did not observe the binding's Account, the
inspection classifies the binding without credential facts, so it is deferred
because its client is not installed or enabled. Enabled clients keep their
existing path. One owner now decides which Accounts a read-only command reads,
and no command reads an operating-system credential item for a client that the
operator disabled.

### The classification order stays

Moving the "not installed or enabled" case before the credential cases in
`ClassifyClient` would also hide a selected binding's missing Token when no
client is enabled, which activation reports by design. The scope rule fixes the
defect without that loss.

## Risks / Trade-offs

- [A disabled binding's missing Token is not shown while another client is
  enabled] → enabling the binding with `aigw use --for <client> <route>` brings
  its Account into activation's scope, where its Token state is reported.
- [A credential problem on a disabled binding stays hidden] → it surfaces when
  the binding is enabled; read-only commands never prompt for it.

## Verification

On `17d9bdf6`, `TestDisabledBindingAccountCannotChangeReadiness` failed as
expected: when Account two became unobservable, Claude Code turned unavailable
in `status`, `check` and `doctor`, status recommended `aigw doctor`, doctor
reported `ok: false`, and readiness read Account two. With the fix, the
regression and the `internal/cli/acceptance`, `internal/cli/readiness`,
`internal/cli/doctor`, `internal/activation` and `internal/readiness` packages
pass.

The fix shipped in 0.4.3. Tag `v0.4.3` (object `e86e0ece`) on `3c133d84`
passed its tag pipelines on both peers (GitHub run 38936264884, GitLab
pipeline 11318), and both peers' Releases carry the same notarized bundle
(Apple submission `c4a722d4`). On October 17 the operator's Homebrew upgrade
from 0.4.2, a rollback to 0.4.2 and the forward upgrade to 0.4.3 each ended
with all four enabled clients `configured` in `status`, `doctor` passing and
every client `inference_checked` by `aigw check`; the ten projected credential
commands passed Token-discarding probes after each step. The upgrade's first
sync refused an external edit of the AIGW-owned `ANTHROPIC_BASE_URL`, as
designed, and converged once the managed value was restored. Receipts are in
the main checkout under
`build/verification/3c133d84c7caf5bdbf804f7a55c9e12daf010532/`, in
`installed-cutover-20261017/` and `installed-lifecycle-20261017/`.
