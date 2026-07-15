# Text Layout Policy

Status: canonical.

## Scope

This policy applies to every tracked UTF-8 text file, including Markdown, Go,
Go module metadata, Python, shell (including extensionless executable scripts),
YAML, TOML, INI, EditorConfig, PowerShell, and WiX. Generated files and binary
assets are outside its scope; binary files are detected by their NUL bytes.

## Shared invariants

- UTF-8, LF line endings, no trailing whitespace, and exactly one final newline.
- One blank line separates adjacent logical blocks in Markdown, Go, shell,
  YAML, TOML, INI, and PowerShell. Python uses two blank lines between
  module-level declarations and one blank line between class methods, as
  specified below.
- A blank line is not inserted inside a compact structure: a Markdown list,
  table, fenced code block, YAML mapping or sequence, TOML table, shell command
  continuation, or immediately after an opening delimiter.

## Language-specific meaning

| Surface | Single blank line | No blank line |
| --- | --- | --- |
| Markdown | Between headings, paragraphs, lists, tables, blockquotes, and fences | Within one list, table, fenced block, or paragraph continuation |
| Go | Between top-level declarations and logical function sections | Import groups, short guard clauses, and tightly coupled statements |
| Python | Two blank lines between module-level declarations; one blank line between class methods | Inside a compact function or class section |
| Shell / PowerShell | Between functions and logical command phases | Command continuations, `case` arms, pipelines, and tight guards |
| YAML / TOML / INI | Between top-level documents, jobs, or tables | Within one mapping, sequence, or table |

The checker enforces only objective invariants. It deliberately does not try to
infer subjective grouping inside a function or prose paragraph.
