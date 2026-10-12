# Design

## Context

See [the proposal](proposal.md). `TestNativeVerificationResources` runs the
built product's `verify` command against a copied fixture client. The fixture
starts an owned child, publishes both processes, waits for a release signal and
then ends according to the case: success, failure, parent exit, held until
interrupted, or held past the deadline. The test then requires the owned
processes and disposable home to be gone, an unrelated sentinel process alive,
and the home, temporary and program inventories unchanged. Windows uses its
own console interruption and Job ownership. Since `a278c443`, Hermes
verification refuses on Windows before starting a client, so all five Windows
cases failed at the first readiness file.

Codex verification also owns a disposable workspace (`CODEX_HOME` beneath
`aigw-codex-verification-*`). It starts its client through the same verification
runner, gives each turn its own `ProtocolTimeout` (one minute), and removes the
workspace on return.

## Goals / Non-Goals

**Goals:** Qualify verification process-tree reclamation natively on Windows
again, and exercise the same fixture path on every host.

**Non-Goals:** Changing verification behavior or timeouts, qualifying Claude
verification resources, and the Codex terminal lane's copy of the test.

## Decisions

1. Run the resource cases through Codex on every host, and also through Hermes
   on hosts where Hermes verification starts a client. macOS and Linux then
   exercise the exact fixture path Windows depends on, which can be run locally.
2. In the fixture, the first Codex turn (not `resume`) runs the shared resource
   parent when a resource case is set, and the success case answers with the
   existing Codex fixture. The recall turn keeps its normal behavior, so a
   successful case verifies both turns.
3. The parent records its disposable home from its own client's variable and
   passes it to its child through `AIGW_TEST_RESOURCE_HOME`. An operator's
   inherited `HERMES_HOME` therefore cannot be mistaken for the Codex workspace.
4. Add a reclamation scenario to "Real Codex client route verification", which
   this native acceptance now qualifies on every host.

## Risks / Trade-offs

- [macOS and Linux native acceptance grows by five Codex cases] → About one
  minute, mostly the deadline case; it buys a locally runnable copy of the
  Windows path.
- [The Codex deadline is per turn] → The held first turn reaches the product's
  one-minute turn deadline, inside the test's 70-second bound, as the Hermes
  probe did.
- [Windows coverage is still unmeasured] → Both earlier Windows jobs stopped
  before the aggregate coverage gate.

## Migration Plan

No data or configuration change. Rollback reverts this Change. Proof,
candidate landing, archive, closeout and publication are governed effects
recorded in ETHOS Attestations.

## Local Qualification

The failing evidence is the hosted Native Windows acceptance of `ef404988`
(run `38245567639`, job `114798438309`). The fixture changes were compiled,
vetted and linted with the `native_resource_acceptance` tag for darwin, linux
and windows targets. `mise run native` passed on macOS with the race detector,
running all five resource cases through both Hermes and Codex, and
`mise run check` passed on the staged tree. Both reported 95.12% aggregate
statement coverage (15,078 of 15,851 statements). An archive rehearsal in a
temporary copy merged the new scenario and passed strict validation. The Windows
branch runs only on a native Windows host, so the next hosted Native Windows
acceptance is the passing check.
