// Package terminal renders human and machine output and reads interactive
// text, secret and selection input, without owning command results, domain
// behavior or client configuration.
package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// State classifies one human-facing status row.
type State int

const (
	// Plain identifies an unclassified row, which has no state symbol.
	Plain State = iota
	// OK identifies a successful status.
	OK
	// Warn identifies an actionable but non-fatal status.
	Warn
	// Fail identifies a failed status.
	Fail
	// Info identifies neutral contextual information.
	Info
)

// Renderer writes width-aware, optionally styled command output while retaining the first write failure.
type Renderer struct {
	out        io.Writer
	err        error
	width      int
	hasContent bool
	styles     styles
}

// Field is one row of an aligned group: a label, its value, an optional state
// symbol and subordinate details that continue under the value.
type Field struct {
	State   State
	Label   string
	Value   string
	Details []string
	// Command marks a label that is executable text. It is written whole at
	// every width, so a copy keeps the command; the terminal soft-wraps it.
	Command bool
}

// productName is the sole human-facing product identity used in rendered output.
const productName = "AIGW"

const (
	// labelColumn is the narrowest label column, which keeps the values of
	// short-labelled groups in column 23.
	labelColumn = 21
	// columnGap separates a group's longest label from its values.
	columnGap = 2
	// valueRoom is the narrowest value column that wraps rather than stacks.
	valueRoom = 20
	// noteIndent places notes and stacked values under a statement's text.
	noteIndent = 4
)

var stateSymbols = map[State]string{OK: "✓", Warn: "!", Fail: "✗", Info: "·"}

var lineBreaks = strings.NewReplacer("\r\n", "\n", "\r", "\n")

type styles struct {
	title   lipgloss.Style
	section lipgloss.Style
	dim     lipgloss.Style
	ok      lipgloss.Style
	warn    lipgloss.Style
	fail    lipgloss.Style
	info    lipgloss.Style
	command lipgloss.Style
	problem lipgloss.Style
}

// Problem contains one actionable failure explanation with evidence, impact, and recovery guidance.
type Problem struct {
	Title    string
	Evidence string
	Impact   string
	Fix      string
}

// WriteJSON emits one two-space-indented JSON document and its trailing newline.
// Command owners retain their result schema and exit-status semantics.
func WriteJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// NewRenderer returns a renderer without an explicit width constraint.
func NewRenderer(out io.Writer, color bool) *Renderer { return NewRendererWithWidth(out, color, 0) }

// NewRendererWithWidth returns a renderer constrained to the supplied display width; zero leaves wrapping unconstrained.
func NewRendererWithWidth(out io.Writer, color bool, width int) *Renderer {
	base := styles{}
	if color {
		base.title = lipgloss.NewStyle().Bold(true)
		base.section = lipgloss.NewStyle().Bold(true)
		base.dim = lipgloss.NewStyle().Faint(true)
		base.ok = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		base.warn = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		base.fail = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		base.info = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		base.command = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		base.problem = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	}
	return &Renderer{out: out, width: width, styles: base}
}

// Err reports the first output failure observed while rendering. Renderer
// methods remain intentionally fluent so command layouts stay declarative;
// callers can inspect this once after a complete presentation.
func (r *Renderer) Err() error { return r.err }

func (r *Renderer) printf(format string, args ...any) {
	if r.err != nil {
		return
	}
	_, r.err = fmt.Fprintf(r.out, format, args...)
}

func (r *Renderer) println(args ...any) {
	if r.err != nil {
		return
	}
	_, r.err = fmt.Fprintln(r.out, args...)
}

// Title starts a rendered result with a product and operation title.
func (r *Renderer) Title(product, title string) {
	if r.width > 0 {
		for _, line := range wrap(product+"  "+title, r.width) {
			r.println(r.styles.title.Render(line))
		}
		r.println(strings.Repeat("─", min(40, r.width)))
	} else {
		r.printf("%s  %s\n%s\n", r.styles.title.Render(product), title, strings.Repeat("─", 40))
	}
	r.hasContent = true
}

// ProductTitle renders one title under the single product display identity.
func (r *Renderer) ProductTitle(title string) { r.Title(productName, title) }

// Section starts a named result section with stable spacing.
func (r *Renderer) Section(title string) {
	if r.hasContent {
		r.println()
	}
	if r.width > 0 {
		r.writeWrapped(title, 0, r.styles.section)
	} else {
		r.println(r.styles.section.Render(title))
	}
	r.hasContent = true
}

// Rows renders one semantic group. Its values share one column after the
// group's longest label and state symbol, and values and details continue
// under that column. The whole group stacks when a label spans lines or the
// width leaves too little room for a value that does not fit; a stacked
// command label stays whole.
func (r *Renderer) Rows(fields ...Field) {
	column := labelColumn
	for _, field := range fields {
		column = max(column, DisplayWidth(r.labelCell(field))+columnGap)
	}
	valueColumn := 2 + column
	stacked := r.stacks(fields, valueColumn)
	for _, field := range fields {
		cell := r.labelCell(field)
		switch {
		case stacked && field.Command:
			r.printf("  %s\n", cell)
		case stacked:
			r.writeHanging("  ", cell, 2+DisplayWidth(cell)-DisplayWidth(field.Label), lipgloss.NewStyle())
		case field.Value == "":
			r.printf("  %s\n", cell)
		default:
			r.writeHanging("  "+cell+strings.Repeat(" ", column-DisplayWidth(cell)), field.Value, valueColumn, lipgloss.NewStyle())
		}
		if stacked {
			for _, text := range append([]string{field.Value}, field.Details...) {
				if text != "" {
					r.writeWrapped(text, noteIndent, r.styles.dim)
				}
			}
		} else {
			for _, detail := range field.Details {
				r.writeHanging(strings.Repeat(" ", valueColumn), detail, valueColumn, r.styles.dim)
			}
		}
		r.hasContent = true
	}
}

// stacks reports whether a group places each value under its label: a label
// spans lines, no width remains at the value column, or too little remains for
// a value or detail that would have to wrap.
func (r *Renderer) stacks(fields []Field, valueColumn int) bool {
	for _, field := range fields {
		if strings.ContainsAny(field.Label, "\n\r") {
			return true
		}
	}
	room := r.width - valueColumn
	if r.width <= 0 || room >= valueRoom {
		return false
	}
	for _, field := range fields {
		for _, text := range append([]string{field.Value}, field.Details...) {
			for _, line := range wrap(text, 0) {
				if DisplayWidth(line) > room {
					return true
				}
			}
		}
	}
	return room < 1
}

// StatusLine renders a compact classified line without a label column.
func (r *Renderer) StatusLine(state State, label, value string) {
	cell := r.labelCell(Field{State: state, Label: label})
	if strings.ContainsAny(label+value, "\n\r") || r.width > 0 && DisplayWidth("  "+cell+"  "+value) > r.width {
		r.writeHanging("  ", cell, 2+DisplayWidth(cell)-DisplayWidth(label), lipgloss.NewStyle())
		r.writeWrapped(value, noteIndent, r.styles.dim)
	} else {
		r.printf("  %s  %s\n", cell, value)
	}
	r.hasContent = true
}

// Detail renders a subordinate note under the preceding statement or group,
// at one indentation for every width.
func (r *Renderer) Detail(value string) {
	r.writeWrapped(value, noteIndent, r.styles.dim)
	r.hasContent = true
}

// Text renders ordinary human-facing text.
func (r *Renderer) Text(value string) {
	r.writeHumanText(value, r.styles.dim)
}

// Command preserves executable text; the terminal owns visual soft wrapping.
func (r *Renderer) Command(value string) {
	for line := range strings.SplitSeq(value, "\n") {
		r.printf("  %s\n", r.styles.command.TabWidth(lipgloss.NoTabConversion).Render(line))
	}
	r.hasContent = true
}

// Success renders a successful terminal statement whose wrapped lines continue
// under its text.
func (r *Renderer) Success(value string) {
	r.writeHanging("  "+r.styles.ok.Render("✓")+" ", value, noteIndent, lipgloss.NewStyle())
	r.hasContent = true
}

// Next renders the single recommended follow-up command.
func (r *Renderer) Next(command string) {
	r.Section("Next")
	r.Command(command)
}

// Problem renders a complete actionable problem report.
func (r *Renderer) Problem(problem Problem) {
	r.ProductTitle("Action required")
	r.Section("Problem")
	r.writeHumanText(problem.Title, r.styles.problem)
	if problem.Evidence != "" {
		r.Section("Evidence")
		r.writeHumanText(problem.Evidence, r.styles.dim)
	}
	if problem.Impact != "" {
		r.Section("Impact")
		r.writeHumanText(problem.Impact, r.styles.dim)
	}
	if problem.Fix != "" {
		r.Section("Recommended action")
		r.writeHumanText(problem.Fix, r.styles.command)
	}
}

func (r *Renderer) stateStyle(state State) lipgloss.Style {
	return map[State]lipgloss.Style{OK: r.styles.ok, Warn: r.styles.warn, Fail: r.styles.fail, Info: r.styles.info}[state]
}

// labelCell returns a field's label, preceded by its styled state symbol when
// the field is classified.
func (r *Renderer) labelCell(field Field) string {
	symbol, classified := stateSymbols[field.State]
	if !classified {
		return field.Label
	}
	return r.stateStyle(field.State).Render(symbol) + " " + field.Label
}

func (r *Renderer) writeHumanText(value string, style lipgloss.Style) {
	if strings.ContainsAny(value, "\n\r") || r.width > 0 && DisplayWidth("  "+value) > r.width {
		r.writeWrapped(value, 2, style)
	} else {
		r.printf("  %s\n", style.Render(value))
	}
	r.hasContent = true
}

// writeWrapped writes value at indent, which narrows to fit a tiny width.
func (r *Renderer) writeWrapped(value string, indent int, style lipgloss.Style) {
	if r.width > 0 {
		indent = min(indent, max(r.width-1, 0))
	}
	r.writeHanging(strings.Repeat(" ", indent), value, indent, style)
}

// writeHanging writes value after head and continues each further line at
// indent, wrapping within the known width; text of unknown width keeps its
// logical lines.
func (r *Renderer) writeHanging(head, value string, indent int, style lipgloss.Style) {
	width := 0
	if r.width > 0 {
		width = max(r.width-indent, 1)
	}
	for index, line := range wrap(value, width) {
		if index > 0 {
			head = strings.Repeat(" ", indent)
		}
		r.printf("%s%s\n", head, style.Render(line))
	}
}

// wrap splits explanatory text into lines of at most width display cells. It
// breaks only at spaces, so hyphenated identifiers stay whole, and hard-wraps
// only a word wider than a line. A zero width keeps each logical line whole.
func wrap(value string, width int) []string {
	paragraphs := strings.Split(strings.TrimSpace(lineBreaks.Replace(value)), "\n")
	if width <= 0 {
		return paragraphs
	}
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		for index, word := range strings.Split(paragraph, " ") {
			switch {
			case index == 0:
				lines = append(lines, word)
			case DisplayWidth(lines[len(lines)-1]+" "+word) <= width:
				lines[len(lines)-1] += " " + word
			default:
				lines[len(lines)-1] = strings.TrimRight(lines[len(lines)-1], " ")
				lines = append(lines, word)
			}
			if last := lines[len(lines)-1]; DisplayWidth(last) > width {
				lines = append(lines[:len(lines)-1], strings.Split(ansi.Hardwrap(last, width, true), "\n")...)
			}
		}
		lines[len(lines)-1] = strings.TrimRight(lines[len(lines)-1], " ")
	}
	return lines
}

// DisplayWidth returns the terminal cell width of a string.
func DisplayWidth(value string) int { return lipgloss.Width(value) }
