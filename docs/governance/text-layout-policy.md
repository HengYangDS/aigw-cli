# Text Layout Policy

Status: canonical.

## Scope

This policy applies to every tracked UTF-8 text file, including Markdown, Go,
Go module metadata, Python, shell (including extensionless executable scripts),
YAML, TOML, INI, EditorConfig, PowerShell, and WiX. Generated files and binary
assets are outside its scope; binary files are detected by their NUL bytes.

## Shared invariants

- UTF-8, LF line endings, no trailing whitespace, and exactly one final newline.
- A blank line separates adjacent logical blocks. The default is **one blank
  line**. **Two blank lines are reserved for Python module-level declarations**;
  no other tracked source or configuration surface uses them as a separator.
- A blank line is not inserted inside a compact structure: a Markdown list,
  table, fenced code block, YAML mapping or sequence, TOML table, shell command
  continuation, or immediately after an opening delimiter.

The rule is semantic rather than decorative: whitespace separates units that
can be understood independently. It must not split one compact unit merely to
make a file look airy.

## Language-specific meaning

| Surface | Single blank line | Two blank lines | No blank line |
| --- | --- | --- |
| Markdown | Between headings, paragraphs, lists, tables, blockquotes, and fences | Never | Within one list, table, fenced block, or paragraph continuation |
| Go | Between top-level declarations and logical function sections | Never | Import groups, short guard clauses, and tightly coupled statements |
| Python | Between class methods and logical sections in a function | Only between module-level classes/functions | Inside a compact function or class section |
| Shell / PowerShell | Between functions and logical command phases | Never | Command continuations, `case` arms, pipelines, and tight guards |
| YAML | Between top-level documents, jobs, or mappings | Never | Within one mapping or sequence |
| TOML / INI | Before each table; also before a comment attached to that table | Never | Within one table or table-attached comment block |

The checker enforces the mechanical floor: no trailing blank lines, no blank-run
larger than the language permits, Python declaration spacing, Python function
interior compactness, and TOML/INI table separation. Review still owns whether
the remaining single separators express real logical boundaries.

## Generated configuration

AIGW-generated TOML follows the same rule: one blank line before every table,
including parent and child tables. It does not emit blank lines within a table
or double separators. Any hand-edited configuration remains semantically valid
TOML, but generated output is normalized to this presentation contract.
