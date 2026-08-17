# Text Layout Policy

Status: canonical.

## Scope

This policy applies to every tracked UTF-8 text file, including Markdown, Go,
Go module metadata, Python, shell (including extensionless executable scripts),
YAML, TOML, INI, EditorConfig, PowerShell, and WiX. Generated files and binary
assets are outside its scope; binary files are detected by their NUL bytes.

## Enforced byte invariants

- UTF-8 text uses LF line endings.
- Non-empty text ends with a newline.
- Lines contain no trailing spaces or tabs.

These rules are deterministic across editors and operating systems and prevent
semantic or review-noise drift. The repository checker does not decide how many
blank lines are aesthetically appropriate or require blank lines before TOML
and INI tables.

## Language-specific meaning

| Surface | Single blank line | Two blank lines | No blank line |
| --- | --- | --- |
| Markdown | Between headings, paragraphs, lists, tables, blockquotes, and fences | Never | Within one list, table, fenced block, or paragraph continuation |
| Go | Between top-level declarations and logical function sections | Never | Import groups, short guard clauses, and tightly coupled statements |
| Python | Between class methods and logical sections in a function | Only between module-level classes/functions | Inside a compact function or class section |
| Shell / PowerShell | Between functions and logical command phases | Never | Command continuations, `case` arms, pipelines, and tight guards |
| YAML | Between top-level documents, jobs, or mappings | Never | Within one mapping or sequence |
| TOML / INI | Before each table; also before a comment attached to that table | Never | Within one table or table-attached comment block |

Language-native formatters own source layout where they define it. Markdown and
configuration layout remains a review concern: spacing should reveal logical
structure, but a presentation preference cannot reject an otherwise correct
change without an admitted risk model.

## Generated configuration

AIGW-generated TOML remains deterministic and readable, but its serializer is
the output owner. The repository-wide text checker does not duplicate serializer
formatting policy.
