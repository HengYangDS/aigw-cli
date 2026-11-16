## Context

See [proposal.md](proposal.md). The gate is `tools/ci openspec`: it runs the bundled validator over every
specification and every active Change with `--strict` and fails when the report holds any issue
(`tools/ci/openspec.go` at `9a3cf734`). "Warnings are owned failures" requires that, in its scenario "Native
validation distinguishes advice from failure".

## Goals / Non-Goals

**Goals:**

- A Change that changes no specification can say so, as OpenSpec 2.0 provides, and pass.
- The admission stays visible in the gate's output.

**Non-Goals:**

- A list of findings that are held harmless.
- A change to what ETHOS or OpenSpec validates.

## Decisions

### The gate admits what the Change declares, and no finding by its level

- **Rests on:** the validator's report for a Change with `skip_specs: true` and no delta, read on 2026-11-16
  with the bundled OpenSpec 2.0.0 in this lane: the item is `valid`, its one issue has level `INFO` and path
  `file`, and its message begins "skip_specs is set in .openspec.yaml". A Change without a delta and without
  the declaration is invalid, with an `ERROR` that names the declaration as the way to state the intent; one
  with both is invalid too. The gate's reader took each item's identifier and issues from that report at
  `9a3cf734` and now also takes its type and validity. Eight archived
  Changes carry a `MODIFIED` block of "Latest stable repository-owned supply chain", and that requirement
  holds 17 scenarios at `9a3cf734`.
- **Considered:** keep the rule, so that every Change writes a delta; admit every `INFO`; admit the
  acknowledgement by its message alone; admit it only for a Change whose own metadata declares
  `skip_specs: true`.
- **Chosen because:** the first makes a requirement grow with Changes that change no behavior. The second
  drops the rule for findings that nobody has read. The third rests the admission on another program's
  wording alone. The declaration is the repository's own tracked statement, written by the Change's author
  and reviewed with it; the finding is then the validator agreeing with it, and the gate checks both.
- **Applies to:** active Changes under `openspec/changes`, in the gate `tools/ci openspec`, which reads the
  metadata file of the checkout that it validates. It does not apply
  to an accepted specification, to an archived Change, or to any other finding of the same Change, which
  fails the gate as before.

## Risks / Trade-offs

- [A Change declares `skip_specs` although it changes behavior] → The declaration is in the Change's diff, and
  the Change's review asks it; the validator refuses the declaration when a delta exists.
- [OpenSpec rewords the acknowledgement] → The gate then fails a declared Change, which is the safe direction,
  and the test that drives the gate with the validator's report names the cause.

## Migration Plan

None. Changes that carry a delta are judged as before.
