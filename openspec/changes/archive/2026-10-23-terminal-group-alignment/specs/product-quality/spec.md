## ADDED Requirements

### Requirement: Human output groups share one value column

A command SHALL submit each group of fields, with their state symbols and
details, as one unit. The group SHALL share one value column at least two cells
after its longest label and symbol; values and details continue under it, and
the group stacks as a whole only when too little width remains for values.
Standalone notes SHALL keep one indentation at every width, and explanatory
text SHALL break only at spaces unless one word exceeds the width.

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

#### Scenario: Help lists command options

- **WHEN** command help lists its options
- **THEN** option lines SHALL start at the indentation of the other section
  entries and keep pflag's own layout.
