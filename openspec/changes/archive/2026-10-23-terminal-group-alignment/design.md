# Design

## Context

`internal/presentation` owns terminal-cell measurement, wrapping and aligned
fields. Its `Rows` method already aligns a group of plain fields, but `Status`
and `Row` lay out each row on its own: a fixed 19- or 21-cell label column, a
one-space fallback when a label is longer, and a per-row choice between columns
and stacking. `Detail` indents four cells when the width is known and
twenty-three cells otherwise. Commands compute their result before rendering,
except `verify`, which streams one client at a time.

## Goals / Non-Goals

**Goals:** every group of rows shares one value column at every width; wrapped
values, details and statements keep their indentation; identifiers stay whole.

**Non-Goals:** changing what commands report, their JSON, prompt or exit
contracts, or pflag's option grammar.

## Decisions

### A group is the unit of alignment

`Rows` is the only aligned primitive. A `Field` carries an optional state
symbol and its details, so a command submits a section's rows as one group,
and the per-row `Row` and `Status` primitives are removed. Inferring groups by
buffering rows until the next section was rejected: output would then depend
on a later flush, before prompts, errors and streamed results.

### Column and stacking are group decisions

The label column is the longest label with its symbol plus two cells, and
never narrower than the previous 21 cells, so groups of short labels keep their
value column at 23. Values and details continue under that column. The whole
group stacks only when the known width leaves fewer than 20 cells for values,
or when a label spans lines. Output of unknown width neither wraps nor stacks.

### Notes have one indentation

A row's details belong to its field. A standalone `Detail` is a note under a
statement or group, at four cells at every width. Introductory notes under a
title use `Text`.

### Wrap only at spaces

`ansi.Wordwrap` always breaks at hyphens, which splits client, Route and model
identifiers. The renderer wraps at spaces with ANSI-aware widths and hard-wraps
only a single word wider than the line. A wrapped success statement continues
under its text.

### Streamed verification keeps its rows

`verify` renders each client as a one-row group as its result arrives, details
after the row. Client titles fit the default column, so the rows stay aligned
without waiting for every client.

### Help options keep pflag's layout

pflag owns option grammar and its own internal alignment. Help removes only the
two-cell indent pflag adds, so option lines share the section indentation.

## Risks / Trade-offs

- [Scripts parsed human output] → human layout was never a machine contract;
  JSON remains the stable interface.
- [One long label pushes a group's values right] → the group stacks when fewer
  than 20 cells remain, and sections keep unrelated rows in separate groups.

## Migration Plan

None; the next release renders the new layout.

## Open Questions

- None.
