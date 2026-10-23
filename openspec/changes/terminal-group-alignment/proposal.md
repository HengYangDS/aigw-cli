# Terminal Group Alignment

## Why

`aigw doctor` lists its checks in ragged columns: a check label of 19 or more
cells pushes its value one space past the label instead of into the shared
column. An audit of every human surface at 100 and 72 columns and as piped
output found the same root cause elsewhere. The renderer decides each row's
column and stacking by itself, so one group can mix columns, stack one row and
leave its neighbours in columns, or place details at four cells in a terminal
but twenty-three when piped. It also breaks identifiers such as
`claude-desktop` at their hyphens, strands a wrapped success mark on its own
line and indents help options two cells deeper than every other section.

## What Changes

- The renderer lays out each group of label and value fields as one unit: one
  value column from the longest label and its state symbol, at least two cells
  of separation, values and details that continue under that column, and
  whole-group stacking only when fewer than twenty cells remain for values.
- Rows carry their state symbol and subordinate details, so a command submits
  a group instead of single rows; the single-row `Row` and `Status` primitives
  are removed. Standalone notes keep one indentation at every width.
- Explanatory text breaks only at spaces unless one word exceeds the width, and
  a wrapped success statement continues under its text.
- Every command renderer submits its groups whole: doctor, status, check, test,
  verify, account, route, selection, sync, repair, rollback, catalog, models,
  client, setup, configuration import and migration, rename and installation.
  The sync preview gives a pending credential entrypoint, Client Bindings and
  projection targets their own sections, the selection preview separates its
  targets, previews end with a success line, and verify prints each client's
  details after its row.
- Help option lines share the two-cell section indentation.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: the requirement "Terminal layout follows semantic fields
  and display width" states group-wide value columns, hanging continuation,
  width-independent note indentation and space-only wrapping.

## Impact

Human output only. JSON, credential-helper and interactive prompt contracts,
command names, exit statuses and copyable command bytes are unchanged. Groups
whose labels fit the default column keep their value column; a group with a
longer label moves all of its values to one shared column. A row's details sit
under its value at every width, and standalone notes sit four cells in, where
piped output used to put both at twenty-three.
