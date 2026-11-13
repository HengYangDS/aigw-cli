## Context

The CHANGELOG holds 85 released sections. 51 of them are 0.1.0 release candidates (rc.58 to rc.118, July to
September 2026), written before the project stopped publishing candidates. Their tags have Git history but no
Releases, and the history rebuild retires them.

## Goals / Non-Goals

**Goals:**

- The CHANGELOG passes the release gate after the rebuild retires the candidate tags.
- No candidate's change disappears from the history.
- The requirement states the parity rule that the gate enforces.

**Non-Goals:**

- Rewording, deduplicating or reclassifying candidate items.
- Adding sections for v0.3.2 or v0.4.0, whose changes later sections already record.

## Decisions

### Fold by category, newest first

The fold appends each candidate's items, newest candidate first, to the 0.1.0 section's category of the same name,
after 0.1.0's own items. Keep a Changelog keeps a version's changes together, and the 0.1.0 release shipped every
candidate's change. One sentence under the heading names the candidates. Rejected: a separate prose block per
candidate, which keeps 51 headings in another form; and dropping the candidate items, which loses history.

### Restate parity in the direction the gate enforces

`validateChangelogProvenance` requires a tag for every released section, except one pending release, and
`TestChangelogAllowsUnlistedLocalTagAndRejectsVersionRegression` accepts a tag without a section. The requirement's
"one-to-one parity" and its scenario "a product tag lacks a released section" contradict that code, and no release
gate has ever applied them. The code is the intended rule: a release section may cover several tags, such as
candidates or a failed tag whose changes ship later. So the requirement changes, not the code.

## Risks / Trade-offs

- The 0.1.0 section grows to about 570 lines. It is history, and readers reach it by its heading.
- Until the rebuild, the 51 candidate tags remain without sections, which the gate already accepts.
