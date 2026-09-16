// Package presentation renders human and machine output without owning
// command results, domain behavior or client configuration.
package presentation

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
	// OK identifies a successful status.
	OK State = iota
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

// Field keeps a row's label separate from its explanatory value.
type Field struct {
	Label string
	Value string
}

// ProductName is the sole human-facing product identity used in rendered output.
const ProductName = "AIGW"

const (
	rowKeyWidth   = 21
	stateKeyWidth = 19
	detailIndent  = 2 + 2 + stateKeyWidth
	compactIndent = 4
)

type styles struct {
	title    lipgloss.Style
	section  lipgloss.Style
	dim      lipgloss.Style
	ok       lipgloss.Style
	warn     lipgloss.Style
	fail     lipgloss.Style
	info     lipgloss.Style
	command  lipgloss.Style
	problem  lipgloss.Style
	rowKey   lipgloss.Style
	stateKey lipgloss.Style
}

// Problem contains one actionable failure explanation with evidence, impact, and recovery guidance.
type Problem struct {
	Title    string `json:"error"`
	Evidence string `json:"evidence,omitempty"`
	Impact   string `json:"impact,omitempty"`
	Fix      string `json:"next_action"`
}

// WriteJSON emits one two-space-indented JSON document and its trailing newline.
// Command owners retain their result schema and exit-status semantics.
func WriteJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// New returns a renderer without an explicit width constraint.
func New(out io.Writer, color bool) *Renderer { return NewWithWidth(out, color, 0) }

// NewWithWidth returns a renderer constrained to the supplied display width; zero leaves wrapping unconstrained.
func NewWithWidth(out io.Writer, color bool, width int) *Renderer {
	base := styles{
		rowKey:   lipgloss.NewStyle().Width(rowKeyWidth).MaxWidth(rowKeyWidth),
		stateKey: lipgloss.NewStyle().Width(stateKeyWidth).MaxWidth(stateKeyWidth),
	}
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
func (r *Renderer) ProductTitle(title string) { r.Title(ProductName, title) }

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

// Row renders one label and value, switching to a compact layout when needed.
func (r *Renderer) Row(label, value string) {
	r.Rows(Field{Label: label, Value: value})
}

// Rows aligns a semantic group, stacking the whole group when a row cannot fit.
func (r *Renderer) Rows(fields ...Field) {
	columnWidth := rowKeyWidth
	for _, field := range fields {
		columnWidth = max(columnWidth, DisplayWidth(field.Label)+1)
	}
	compact := false
	for _, field := range fields {
		compact = compact || r.requiresCompactColumn(field.Label, field.Value, columnWidth)
	}
	for _, field := range fields {
		if compact {
			r.writeWrapped(field.Label, 2, lipgloss.NewStyle())
			r.writeWrapped(field.Value, compactIndent, r.styles.dim)
		} else {
			r.printf("  %s%s\n", r.fixedLabel(r.styles.rowKey, field.Label, columnWidth), field.Value)
		}
		r.hasContent = true
	}
}

// Status renders one classified label and value with a stable state symbol.
func (r *Renderer) Status(state State, label, value string) {
	symbol := map[State]string{OK: "✓", Warn: "!", Fail: "✗", Info: "·"}[state]
	if r.requiresCompactColumn(label, value, stateKeyWidth+2) {
		r.writeWrapped(r.stateStyle(state).Render(symbol)+" "+label, 2, lipgloss.NewStyle())
		r.writeWrapped(value, compactIndent, r.styles.dim)
		r.hasContent = true
		return
	}
	symbol = r.stateStyle(state).Render(symbol)
	r.printf("  %s %s%s\n", symbol, r.fixedLabel(r.styles.stateKey, label, stateKeyWidth), value)
	r.hasContent = true
}

// StatusLine renders a compact classified line without a fixed label column.
func (r *Renderer) StatusLine(state State, label, value string) {
	symbol := map[State]string{OK: "✓", Warn: "!", Fail: "✗", Info: "·"}[state]
	if r.compactRow(symbol+" "+label, value) {
		return
	}
	symbol = r.stateStyle(state).Render(symbol)
	r.printf("  %s %s  %s\n", symbol, label, value)
	r.hasContent = true
}

// Detail renders subordinate explanatory text.
func (r *Renderer) Detail(value string) {
	if r.width > 0 {
		r.writeWrapped(value, compactIndent, r.styles.dim)
		r.hasContent = true
		return
	}
	r.println(lipgloss.NewStyle().MarginLeft(detailIndent).Inherit(r.styles.dim).Render(value))
	r.hasContent = true
}

// Text renders ordinary human-facing text.
func (r *Renderer) Text(value string) {
	r.writeHumanText(value, r.styles.dim)
}

// Command renders a copyable command and wraps it without inserting shell syntax.
func (r *Renderer) Command(value string) {
	r.writeHumanText(value, r.styles.command)
}

// Success renders a successful terminal statement.
func (r *Renderer) Success(value string) {
	if r.compactRow("✓", value) {
		return
	}
	r.printf("  %s %s\n", r.styles.ok.Render("✓"), value)
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

func (r *Renderer) writeHumanText(value string, style lipgloss.Style) {
	if strings.ContainsAny(value, "\n\r") || r.width > 0 && DisplayWidth("  "+value) > r.width {
		r.writeWrapped(value, 2, style)
	} else {
		r.printf("  %s\n", style.Render(value))
	}
	r.hasContent = true
}

func (r *Renderer) requiresCompact(label, value string, gap int) bool {
	if strings.ContainsAny(label+value, "\n\r") {
		return true
	}
	if r.width <= 0 {
		return false
	}
	return DisplayWidth("  "+label+strings.Repeat(" ", gap)+value) > r.width
}

func (r *Renderer) requiresCompactColumn(label, value string, columnWidth int) bool {
	if strings.ContainsAny(label+value, "\n\r") {
		return true
	}
	if r.width <= 0 {
		return false
	}
	labelWidth := max(DisplayWidth(label)+1, columnWidth)
	return 2+labelWidth+DisplayWidth(value) > r.width
}

func (r *Renderer) compactRow(label, value string) bool {
	if !r.requiresCompact(label, value, 2) {
		return false
	}
	symbol, rest, _ := strings.Cut(label, " ")
	r.writeWrapped(r.stateStyleForSymbol(symbol).Render(symbol)+" "+rest, 2, lipgloss.NewStyle())
	r.writeWrapped(value, compactIndent, r.styles.dim)
	r.hasContent = true
	return true
}

func (r *Renderer) writeWrapped(value string, indent int, style lipgloss.Style) {
	available := DisplayWidth(value)
	if r.width > 0 {
		indent = min(indent, max(r.width-1, 0))
		available = max(r.width-indent, 1)
	}
	for _, line := range wrap(value, available) {
		r.printf("%s%s\n", strings.Repeat(" ", indent), style.Render(line))
	}
}

func (r *Renderer) stateStyleForSymbol(symbol string) lipgloss.Style {
	return map[string]lipgloss.Style{
		"✓": r.styles.ok,
		"!": r.styles.warn,
		"✗": r.styles.fail,
		"·": r.styles.info,
	}[symbol]
}

func wrap(value string, width int) []string {
	width = max(width, 1)
	return strings.Split(ansi.Hardwrap(ansi.Wordwrap(strings.TrimSpace(value), width, ""), width, true), "\n")
}

func (r *Renderer) fixedLabel(style lipgloss.Style, label string, width int) string {
	if DisplayWidth(label) >= width {
		return label + " "
	}
	return style.Width(width).MaxWidth(width).Render(label + " ")
}

// DisplayWidth returns the terminal cell width of a string.
func DisplayWidth(value string) int { return lipgloss.Width(value) }
