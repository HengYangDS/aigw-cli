## Context

AIGW reads and writes Hermes's `config.yaml` in `internal/adapter/hermes/`: it parses the whole document into
the library's node tree, changes its own keys, and encodes the whole tree again (`document.go`, `encode`). It
keeps a JSON ownership record beside the file, whose field `original_model` is the model selection that it
replaced, stored as the library's `*yaml.Node` (`projection.go`, `ownership`), and reads it back with unknown
fields refused (`selection.go`, `decodeOwnership`). The proposal names the two changes of v4.0.0 that matter
here.

## Goals / Non-Goals

**Goals:**

- The move to v4 keeps every value that AIGW writes, and a test shows its bytes for each kind of content that
  an operator's file can hold.
- The ownership record's form belongs to this repository.

**Non-Goals:**

- Changing what AIGW does to a Hermes file today, including a re-encoding that a test records as it is.
- A reader of another record form: the form does not change.

## Decisions

### The bytes are pinned before the library moves

- **Rests on:** no test compares a written Hermes file or record byte for byte (a reading of every test that
  writes one, 2026-11-17). Two encodings of one tree compare equal in some tests, which holds under any library.
- **Considered:** move first and compare outputs by eye; pin a few cases; pin each kind of content that a
  Hermes file can hold.
- **Chosen because:** only a test that fixes the full written bytes under v3 can show that v4 writes the same.
  The kinds: values past column 80; comments of each place (head, line, foot, between keys) and blank lines;
  quoted strings, strings that look like other types; block scalars; layouts (four-space input, compact
  sequences, a document marker, no final newline, CRLF, a byte-order mark); empty and comment-only files;
  anchors outside AIGW's keys; the refusal texts; and the ownership record of a mapping model and of a scalar
  one. Where today's output is not what an operator wants, the test still records it, and the finding goes to
  its own Change.
- **Applies to:** new test files under `internal/adapter/hermes/` with their expected bytes in `testdata/`.

### The ownership record's node is a type of this repository

- **Rests on:** `encoding/json` writes every exported field of a struct; v3's `yaml.Node` has twelve (Kind,
  Style, Tag, Value, Anchor, Alias, Content, HeadComment, LineComment, FootComment, Line, Column) and v4's adds
  `Stream`. Released AIGW reads the record with unknown fields refused.
- **Considered:** store the library's node as today; store the model's YAML text; a type of this repository
  with today's twelve fields, converted to and from the library's node.
- **Chosen because:** the library's type makes the record's form follow its next release, and a text form
  changes the record, which released programs then refuse. A type of its own with today's fields keeps every
  record byte for byte, needs no second reader, and states the form in this repository.
- **Applies to:** `projection.go` (`ownership.OriginalModel`), `selection.go`, and the conversions.

### The writer states its line width and indentation

- **Rests on:** v4.0.0's `NewEncoder` takes the v3 defaults, which set a line width of 80, and only `Marshal`
  overrides it (read in its source); v3's emitter never folds.
- **Considered:** keep `NewEncoder`; build the writer with the library's options, width unlimited and
  indentation 2.
- **Chosen because:** the options say what the writer does, so a later default does not change it.
- **Applies to:** `document.go` (`encode`), which every writer of the adapter calls.

### The differences of v4 are taken, not reproduced

- **Rests on:** the pinned bytes under v4 and a fresh-context review of about 180 inputs (2026-11-18). v3
  changes two values: it adds a line break to a folded scalar that keeps its final line breaks (`>+`) and a blank
  line inside a folded scalar with a more-indented line. v4 keeps both. The other differences keep the value
  and change only the form: no blank line after a folded scalar; a literal block kept for text with trailing
  spaces; no `|2` indicator; `*k : v` for an alias key. Installed Hermes's own reader (`hermes_yaml.safe_load`,
  ruamel.yaml 0.18.16) reads every v4 form to the operator's value
  (`build/evidence/yaml-v4-hermes-read-20261118/` in the main checkout). Several refusals change their wording
  (a duplicate key, an unknown anchor, a directive, a merge onto a sequence, invalid UTF-8), and two inputs that
  v3 refused are accepted (a file of `...` only, a tab inside a literal block); no caller or test matches a
  refusal's text.
- **Considered:** reproduce v3's bytes and refusals in AIGW; take v4's.
- **Chosen because:** v4 keeps every value and repairs two that v3 changed; reproducing v3 would be code of AIGW
  against its library. A refusal is a message, not a file.
- **Applies to:** the pinned cases `block-scalars`, `folded-keep`, `folded-more-indented`, `trailing-spaces`
  and `duplicate-key`, `.editorconfig` (the pinned bytes keep trailing spaces), and `CHANGELOG.md`.

## Risks / Trade-offs

- **A difference that the pinned kinds do not cover.** The kinds are those that a reading of v4's changes and
  open issues names (comments with CRLF, comments on block scalar keys, comments before a document marker,
  numbers quoted as strings); an operator's file can hold more. The released program's own journey on three
  platforms writes a real Hermes file.
- **Not measured here:** the libraries' behavior is read from their sources and pinned by tests; nothing of
  this design rests on timing.

## Migration Plan

None for an operator: the record keeps its bytes, and a file keeps every value.
