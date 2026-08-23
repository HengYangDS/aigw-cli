## Context

See `proposal.md`. The failure is in test data, not path validation or Codex
projection. A rooted POSIX literal is not absolute under Windows path rules.

## Goals / Non-Goals

**Goals:** Keep the same assertions while making fixture paths native to the
executing platform.

**Non-Goals:** Do not weaken absolute-path validation, add path translation, or
change production projection semantics.

## Decisions

Use `filepath.Join(t.TempDir(), "aigw")` as the single fixture source and
`strconv.Quote` as the same Go string representation used by production. This
tests the observable rendered command without maintaining platform branches.

## Risks / Trade-offs

- Test paths vary per run. The assertion derives from the same fixture value,
  so it remains deterministic within each test.
