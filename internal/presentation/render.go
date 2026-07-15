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
	hasContent bool
	inSection  bool
	styles     styles
}

const (
	rowKeyWidth   = 21
	stateKeyWidth = 19
	detailIndent  = 2 + 2 + stateKeyWidth
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

func New(out io.Writer, color bool) *Renderer {
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
	return &Renderer{out: out, color: color, styles: base}
}

func (r *Renderer) Title(product, title string) {
	fmt.Fprintf(r.out, "%s  %s\n%s\n", r.styles.title.Render(product), title, strings.Repeat("─", 40))
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
	fmt.Fprintf(r.out, "  %s%s\n", r.fixedLabel(r.styles.rowKey, label, rowKeyWidth), value)
	r.hasContent = true
}

func (r *Renderer) Status(state State, label, value string) {
	symbol := map[State]string{OK: "✓", Warn: "!", Fail: "✗", Info: "·"}[state]
	symbol = r.stateStyle(state).Render(symbol)
	fmt.Fprintf(r.out, "  %s %s%s\n", symbol, r.fixedLabel(r.styles.stateKey, label, stateKeyWidth), value)
	r.hasContent = true
}

func (r *Renderer) StatusLine(state State, label, value string) {
	symbol := map[State]string{OK: "✓", Warn: "!", Fail: "✗", Info: "·"}[state]
	symbol = r.stateStyle(state).Render(symbol)
	fmt.Fprintf(r.out, "  %s %s  %s\n", symbol, label, value)
	r.hasContent = true
}

func (r *Renderer) Detail(value string) {
	fmt.Fprintln(r.out, lipgloss.NewStyle().MarginLeft(detailIndent).Inherit(r.styles.dim).Render(value))
	r.hasContent = true
}

func (r *Renderer) Text(value string) {
	fmt.Fprintf(r.out, "  %s\n", value)
	r.hasContent = true
}

func (r *Renderer) Command(value string) {
	fmt.Fprintf(r.out, "  %s\n", r.styles.command.Render(value))
	r.hasContent = true
}

func (r *Renderer) Success(value string) {
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
	fmt.Fprintf(r.out, "  %s\n", r.styles.problem.Render(problem.Title))
	if problem.Evidence != "" {
		r.Section("Evidence")
		fmt.Fprintf(r.out, "  %s\n", problem.Evidence)
	}
	if problem.Impact != "" {
		r.Section("Impact")
		fmt.Fprintf(r.out, "  %s\n", problem.Impact)
	}
	if problem.Fix != "" {
		r.Section("Recommended action")
		fmt.Fprintf(r.out, "  %s\n", r.styles.command.Render(problem.Fix))
	}
}

func (r *Renderer) stateStyle(state State) lipgloss.Style {
	return map[State]lipgloss.Style{OK: r.styles.ok, Warn: r.styles.warn, Fail: r.styles.fail, Info: r.styles.info}[state]
}

func (r *Renderer) fixedLabel(style lipgloss.Style, label string, width int) string {
	if DisplayWidth(label) >= width {
		return label + " "
	}
	return style.Render(label + " ")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(value string) string { return ansiPattern.ReplaceAllString(value, "") }

func DisplayWidth(value string) int { return lipgloss.Width(value) }
