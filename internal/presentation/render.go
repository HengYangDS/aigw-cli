package presentation

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

type State int

const (
	OK State = iota
	Warn
	Fail
	Info
)

type Renderer struct {
	out        io.Writer
	color      bool
	width      int
	hasContent bool
	inSection  bool
	styles     styles
}

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

type Problem struct {
	Title    string
	Evidence string
	Impact   string
	Fix      string
}

func New(out io.Writer, color bool) *Renderer { return NewWithWidth(out, color, 0) }

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
	return &Renderer{out: out, color: color, width: width, styles: base}
}

func (r *Renderer) Title(product, title string) {
	if r.width > 0 {
		for _, line := range wrap(product+"  "+title, r.width) {
			fmt.Fprintln(r.out, r.styles.title.Render(line))
		}
		fmt.Fprintln(r.out, strings.Repeat("─", min(40, r.width)))
	} else {
		fmt.Fprintf(r.out, "%s  %s\n%s\n", r.styles.title.Render(product), title, strings.Repeat("─", 40))
	}
	r.hasContent = true
	r.inSection = false
}

func (r *Renderer) Section(title string) {
	if r.hasContent {
		fmt.Fprintln(r.out)
	}
	fmt.Fprintln(r.out, r.styles.section.Render(title))
	r.hasContent = true
	r.inSection = true
}

func (r *Renderer) Row(label, value string) {
	if r.requiresCompact(label, value, 2) {
		fmt.Fprintf(r.out, "  %s\n", label)
		r.writeWrapped(value, compactIndent, r.styles.dim)
		r.hasContent = true
		return
	}
	if r.width > 0 {
		fmt.Fprintf(r.out, "  %s  %s\n", label, value)
	} else {
		fmt.Fprintf(r.out, "  %s%s\n", r.fixedLabel(r.styles.rowKey, label, rowKeyWidth), value)
	}
	r.hasContent = true
}

func (r *Renderer) Status(state State, label, value string) {
	symbol := map[State]string{OK: "✓", Warn: "!", Fail: "✗", Info: "·"}[state]
	if r.requiresCompact(symbol+" "+label, value, 2) {
		fmt.Fprintf(r.out, "  %s %s\n", r.stateStyle(state).Render(symbol), label)
		r.writeWrapped(value, compactIndent, r.styles.dim)
		r.hasContent = true
		return
	}
	symbol = r.stateStyle(state).Render(symbol)
	if r.width > 0 {
		fmt.Fprintf(r.out, "  %s %s  %s\n", symbol, label, value)
	} else {
		fmt.Fprintf(r.out, "  %s %s%s\n", symbol, r.fixedLabel(r.styles.stateKey, label, stateKeyWidth), value)
	}
	r.hasContent = true
}

func (r *Renderer) StatusLine(state State, label, value string) {
	symbol := map[State]string{OK: "✓", Warn: "!", Fail: "✗", Info: "·"}[state]
	if r.compactRow(symbol+" "+label, value, true) {
		return
	}
	symbol = r.stateStyle(state).Render(symbol)
	fmt.Fprintf(r.out, "  %s %s  %s\n", symbol, label, value)
	r.hasContent = true
}

func (r *Renderer) Detail(value string) {
	if r.width > 0 {
		r.writeWrapped(value, compactIndent, r.styles.dim)
		r.hasContent = true
		return
	}
	fmt.Fprintln(r.out, lipgloss.NewStyle().MarginLeft(detailIndent).Inherit(r.styles.dim).Render(value))
	r.hasContent = true
}

func (r *Renderer) Text(value string) {
	if r.compactText(value, 2, r.styles.dim) {
		return
	}
	fmt.Fprintf(r.out, "  %s\n", value)
	r.hasContent = true
}

func (r *Renderer) Command(value string) {
	if r.width > 0 && DisplayWidth("  "+value) >= r.width {
		r.writeWrapped(value, 2, r.styles.command)
		r.hasContent = true
		return
	}
	fmt.Fprintf(r.out, "  %s\n", r.styles.command.Render(value))
	r.hasContent = true
}

func (r *Renderer) Success(value string) {
	if r.compactRow("✓", value, true) {
		return
	}
	fmt.Fprintf(r.out, "  %s %s\n", r.styles.ok.Render("✓"), value)
	r.hasContent = true
}

func (r *Renderer) Next(command string) {
	r.Section("Next")
	r.Command(command)
}

func (r *Renderer) Problem(problem Problem) {
	r.Title("AIGW", "Action required")
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
	if r.width > 0 {
		r.writeWrapped(value, 2, style)
	} else {
		fmt.Fprintf(r.out, "  %s\n", style.Render(value))
	}
	r.hasContent = true
}

func (r *Renderer) requiresCompact(label, value string, gap int) bool {
	if r.width <= 0 {
		return false
	}
	return DisplayWidth("  "+label+strings.Repeat(" ", gap)+value) > r.width
}

func (r *Renderer) compactRow(label, value string, status bool) bool {
	if !r.requiresCompact(label, value, 2) {
		return false
	}
	if status {
		symbol, rest, _ := strings.Cut(label, " ")
		fmt.Fprintf(r.out, "  %s %s\n", r.stateStyleForSymbol(symbol).Render(symbol), rest)
	} else {
		fmt.Fprintf(r.out, "  %s\n", label)
	}
	r.writeWrapped(value, compactIndent, r.styles.dim)
	r.hasContent = true
	return true
}

func (r *Renderer) compactText(value string, indent int, style lipgloss.Style) bool {
	if r.width <= 0 || DisplayWidth(strings.Repeat(" ", indent)+value) < r.width {
		return false
	}
	r.writeWrapped(value, indent, style)
	r.hasContent = true
	return true
}

func (r *Renderer) writeWrapped(value string, indent int, style lipgloss.Style) {
	available := r.width - indent
	if available < 1 {
		available = 1
	}
	for _, line := range wrap(value, available) {
		fmt.Fprintf(r.out, "%s%s\n", strings.Repeat(" ", indent), style.Render(line))
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
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	line := ""
	for _, word := range words {
		if line == "" {
			line = word
			continue
		}
		if DisplayWidth(line+" "+word) <= width {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	return append(lines, line)
}

func (r *Renderer) fixedLabel(style lipgloss.Style, label string, width int) string {
	if DisplayWidth(label) >= width {
		return label + " "
	}
	return style.Render(label + " ")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(value string) string { return ansiPattern.ReplaceAllString(value, "") }

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func DisplayWidth(value string) int { return lipgloss.Width(value) }
