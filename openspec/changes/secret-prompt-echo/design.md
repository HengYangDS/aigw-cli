## Context

The default Token prompt is a huh input field in password echo mode.
Accessible mode, for screen readers, instead read a plain line, which a
terminal echoes. `ReadHiddenToken` already reads with echo disabled through
`golang.org/x/term`.

## Goals / Non-Goals

**Goals:**

- No prompt mode echoes a Token typed at a terminal.
- One owner decides accessibility, from the CLI's injected environment.

**Non-Goals:**

- Changing which environment variables select accessible prompts.

## Decisions

### Read accessible Tokens through the hidden terminal reader

In accessible mode, `Secret` prints its label and reads with
`term.ReadPassword` when standard input is a terminal. Other input comes from
a pipe or a file, which no terminal echoes, so it is read as one line, as
before. huh's own accessible mode reads plain lines, so selecting its password
field there would not hide the Token.

### Decide accessibility once

`terminal.Accessible(env)` evaluates `NO_COLOR` and `AIGW_ACCESSIBLE` from the
environment the CLI already injects. Prompt methods only consult the value they
were constructed with.

### One hidden reader for every platform

The Unix and Windows hidden-reader files were identical apart from their build
tags, and `golang.org/x/term` builds on every supported operating system.

## Risks / Trade-offs

- A screen reader announces no typed Token characters, as for any password
  field.
