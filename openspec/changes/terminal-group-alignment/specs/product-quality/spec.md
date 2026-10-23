## MODIFIED Requirements

### Requirement: Terminal layout follows semantic fields and display width

Human output SHALL use one presentation owner for terminal-cell measurement,
ANSI-aware layout, wrapping, and aligned semantic fields. A command SHALL submit
each group of fields, with their state symbols and details, as one unit. The
group SHALL share one value column set by its longest label and at least two
cells beyond it; values and details continue under that column, and the whole
group stacks only when too little width remains for values. Standalone notes
SHALL keep one indentation at every width. Explanatory text SHALL break only at
spaces unless one word exceeds the width. Metadata SHALL own executable names
and pflag SHALL own option grammar. Color SHALL NOT alter alignment. Machine,
helper, and interactive contracts SHALL remain unchanged. Copyable commands
SHALL preserve characters, whitespace, and logical lines; only explanatory text
MAY wrap, without adding shell syntax.

#### Scenario: Copy an executable command from a narrow terminal

- **WHEN** an executable example or follow-up command exceeds the terminal width
- **THEN** its output SHALL preserve the command bytes apart from presentation
  indentation, optional styling and the final output newline
- **AND** neither quoted whitespace nor argument tokens SHALL be rewritten to
  satisfy an explanatory-text width rule.

#### Scenario: Starting commands have different lengths

- **WHEN** root help presents the setup, selection and readiness journey
- **THEN** command and explanation SHALL be distinct fields with one description
  column derived from the group's longest label
- **AND** a renamed root command SHALL propagate without hard-coded product text.

#### Scenario: A terminal is narrow or a value spans multiple lines

- **WHEN** human output contains long labels, unbroken values or multiple lines
- **THEN** the renderer SHALL retain text and continuation indentation within the
  available display width and use stacked rows where necessary
- **AND** colored and plain output SHALL preserve the same layout.

#### Scenario: Inspect the complete public command tree

- **WHEN** every public command renders help at supported narrow and wide widths
- **THEN** headings, descriptions, usage grammar and options SHALL fit those widths
- **AND** option lines SHALL share the indentation of other section entries
- **AND** native option meaning and credential-free help SHALL remain intact.

#### Scenario: Classified rows have labels of different lengths

- **WHEN** a section such as the doctor checks lists rows whose labels differ in
  length or exceed the default label column
- **THEN** every value of the group SHALL start in one column at least two
  cells after the longest label and its state symbol
- **AND** a value or detail wider than the remaining width SHALL continue under
  that column instead of stacking its row alone.

#### Scenario: Human output has no known width

- **WHEN** human output is piped or the terminal width is unknown
- **THEN** groups and notes SHALL keep the same columns and indentation as at a
  known width, without wrapping.

#### Scenario: Wrapped text contains hyphenated identifiers

- **WHEN** explanatory text containing client, Route or model identifiers wraps
- **THEN** lines SHALL break only at spaces, keeping each identifier whole unless
  it alone exceeds the available width.
