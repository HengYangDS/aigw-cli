# Text Layout Policy

Repository-wide authoring and formatting policy for contributors. Product
interaction belongs to [Terminal experience](../experience/terminal-experience.md).

## Scope

The byte invariants apply to every tracked text file, including root metadata,
source, documentation, configuration, locks and generated CI projections.
Disposable build output and binary content are not text-policy inputs.

## Enforced byte invariants

- UTF-8 text uses LF line endings.
- Non-empty text ends with a newline.
- Lines contain no trailing spaces or tabs.

Git's [text attributes](https://git-scm.com/docs/gitattributes#_text) establish
one LF checkout policy for automatically detected text, including extensions
not individually listed. The locked EditorConfig Checker enforces the byte
invariants from `.editorconfig`; native formatters retain ownership of layout.
Editor defaults are inherited once, with Go's tab indentation as an override. No
repository-specific parser duplicates those mature responsibilities or decides
how many blank lines are aesthetically appropriate.

## Format ownership

| Concern                 | Owner                         | What it proves                                                                                  |
| ----------------------- | ----------------------------- | ----------------------------------------------------------------------------------------------- |
| Source layout           | The locked language formatter | Canonical whitespace and syntax layout                                                          |
| Current Markdown layout | Locked Prettier               | Stable tables, lists, fences and wrapping policy                                                |
| Markdown structure      | Locked markdownlint           | Heading progression, sibling uniqueness, list structure, language-tagged fences and link syntax |
| Diagram syntax          | Locked Mermaid validator      | Grammar and diagnostics, not visual quality or hosted rendering                                 |
| Explicit links          | Locked lychee                 | Existing local files and heading anchors; external reachability requires an online run          |
| OpenSpec document shape | Locked OpenSpec validator     | Official Change and specification structure                                                     |

Do not duplicate formatter rules in a prose table or a custom parser. For
example, OpenSpec's official top-level sections need not be rewritten as H1
headings, and repeated scenario names under different requirements are valid.
The Markdown policy checks duplicate sibling headings, not repeated words
across unrelated sections.

Native heading checks reject decorative trailing punctuation and standalone
emphasis that hides a section from heading navigation. Question headings and
OpenSpec's `**Goals:**` / `**Non-Goals:**` labels remain valid. These are
structural constraints, not a custom prose parser or a ban on emphasized text.

Local links include their fragment targets. The native link command enables
anchor checking; an existing Markdown file with a missing heading is a broken
link. Source checks remain offline, so they do not establish external URL
availability or rendered browser behavior.

`package.json` exposes the repository-wide format check through the existing
CI owner. That owner streams the exact checkout-bound Git inventory to the
locked, checkout-local Prettier API. Git ignores filter untracked output, not
tracked source; `.prettierignore` excludes only immutable OpenSpec history.
A directory named `archive` elsewhere remains current source.

The adapter applies native defaults without discovering local or parent
Prettier configuration. EditorConfig owns editor defaults and its independent
byte checks, not Prettier options. Missing local dependencies, a scope with no
supported files or differing format fail without writing source or falling
back to another installation.

Markdown lint validates its one rule file against schemas shipped with the
locked package before checking source. It does not discover nested or parent
configuration, accept inline suppression, or ignore warnings. Current style
checks exclude only official OpenSpec archive history. Mermaid syntax checks
still include archived diagrams; that does not make their historical design
current. Both adapters receive exact file inventories on standard input,
preserve source bytes and require their checkout-local dependencies.

OpenSpec owns document validation. The CI consumer admits its result only when
the findings report identifies the requested checkout, covers `all` items in a
nonempty scope, and records consistent successful totals with no findings.
Exit status zero alone is insufficient: an empty project or a report discovered
in a parent directory does not validate this checkout. This result admission
does not parse OpenSpec documents or prove implementation completeness.

## Content and rendering

English is the canonical language for repository-authored documentation,
including root guides, design and research documents, and active OpenSpec
artifacts. Conversation language does not change this convention. Preserve
external quotations, proper names, and verbatim diagnostic evidence in their
original form when accuracy requires it; explain their relevance in English.
Translate a document in place rather than adding an unrequested bilingual copy.
This convention does not authorize rewriting immutable archives or third-party
license text.

A document has one audience, purpose and canonical owner. Organize its headings
by that purpose; use lists for alternatives or sequences, tables for comparable
attributes, and diagrams for relationships or behavior that benefit from a
visual explanation. Remove a diagram or table when it repeats nearby prose
without helping a reader decide or act.

- **Diagrams:** name the actors and edge meanings, keep direction consistent,
  and describe the diagram for nonvisual readers. Configuration files do not
  send requests; a success-only flow does not prove failure recovery.
- **Tables:** use comparable rows, clear headers and concise cells. Large prose
  cells belong in sections; verify the rendered width as well as source alignment.
- **Code blocks:** identify the language and platform, distinguish executable
  commands from output, and state prerequisites. Examples must preserve the
  documented credential, ownership and failure boundaries.
- **Lists and headings:** express semantic nesting and actual execution order.
  Keep peer items at the same level and make link targets unambiguous.
- **Content and links:** state supported behavior precisely, link to its owner,
  and remove duplicated explanations or stale claims. A valid URL alone does
  not prove that a reader can find the information they need.

Render changed diagrams and pages, inspect wide and narrow layouts, and compare
behavioral diagrams and command examples against their implementation. Syntax,
formatting and link checks do not establish semantic accuracy or visual quality.
The [documentation index](../README.md) owns navigation. Historical OpenSpec
archives remain immutable; current instructions must not depend on obsolete
archived guidance.

## Generated configuration

AIGW-generated TOML remains deterministic and readable, but its serializer is
the output owner. The repository-wide text checker does not duplicate serializer
formatting policy.
