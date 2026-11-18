## Context

The quality graph (`tools/ci/quality.go`) maps each carrier class to required concerns and to gates. The
Mermaid gate supplies the concerns documentation and schema to the classes `current-documentation` and
`active-openspec`. Markdown in the repository holds nine Mermaid blocks, one of them current; the others are in archived Changes, which the gate also reads.

## Goals / Non-Goals

**Goals:**

- No gate, adapter, test or npm package for Mermaid remains, and the lock carries no picomatch 2.3.2.
- Every concern that the gate supplied stays supplied by another gate.

**Non-Goals:**

- Another diagram validator.
- The picomatch 4.0.7 that OpenSpec brings: it has no patched version and stays disclosed by the scan.

## Decisions

### Remove the gate instead of replacing it

- **Decision:** remove `check-mermaid`, its adapter, its tests and its npm dependency; add no other diagram
  validator.
- **Reason:** one current diagram does not justify 144 packages, a jsdom override and an advisory without a
  fix. The owner chose this on 2026-11-18.
- **Alternatives:** keep the gate and record the advisory as accepted (rejected by the owner); validate with
  Mermaid's own CLI (it brings the same browser stack).
- **Consequence:** a diagram with a syntax error now passes the checks and shows as an error where the forge
  renders it. The concerns stay covered: `openspec` and `changelog` supply schema, and `markdown`, `links`
  and `spelling` supply documentation.

### Keep the KaTeX override

- **Decision:** keep `"katex": "0.19.1"` in `overrides`; drop the Mermaid consumer from the test that
  proves KaTeX rejects inherited trust.
- **Reason:** `micromark-extension-math` (through markdownlint) still resolves KaTeX; without the override it
  resolves 0.16.47, which the override was introduced to avoid.

## Risks / Trade-offs

- **A broken diagram is published.** The repository holds one current diagram; a syntax error in it shows as
  an error where the forge renders it, and a reader reports it. No check replaces that.
- **A concern left without a gate.** The quality graph refuses a class whose required concern no gate
  supplies, so `mise run check` shows it.
