package cli

import (
	"slices"
	"strings"

	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// savingCommands are the commands that save the configuration. Each works
// from the configuration that follows the clients' own choices, so its save
// records them, and the invocation says so.
var savingCommands = []string{
	"setup",
	"model add", "model edit", "model remove", "client enable", "client disable", "client edit",
	"provider add", "provider edit", "provider remove",
	"use", "sync",
	"config rollback", "self uninstall",
}

// programCommands replace the running program under the configuration lock
// and save no configuration, so they record no choice.
var programCommands = []string{"self update", "self rollback"}

// reportingCommands read the working configuration and save nothing: status,
// which bare aigw runs too, and check. They name the choices that the next
// saving command records.
var reportingCommands = []string{"", "status", "check"}

// savesConfiguration reports whether command is one that saves the
// configuration, and whether this invocation only previews it.
func savesConfiguration(command *cobra.Command) (saves, preview bool) {
	if !slices.Contains(savingCommands, commandPath(command)) {
		return false, false
	}
	return true, previews(command)
}

// commandPath is command's words after the program's name; bare aigw is "".
func commandPath(command *cobra.Command) string {
	return strings.TrimPrefix(strings.TrimPrefix(command.CommandPath(), command.Root().Name()), " ")
}

// previews reports whether the invocation runs command with --dry-run.
func previews(command *cobra.Command) bool {
	preview, err := command.Flags().GetBool("dry-run")
	return err == nil && preview
}

// requiresConfigurationLock reports whether command writes under the
// configuration lock: a saving or program-replacing command that is no
// preview, and the Token rotation, which writes the credential store.
func requiresConfigurationLock(command *cobra.Command) bool {
	path := commandPath(command)
	if path == "provider rotate" {
		return true
	}
	return (slices.Contains(savingCommands, path) || slices.Contains(programCommands, path)) && !previews(command)
}

// followedSelection is one client's own choice in a command's document: the
// model that the client's file selects, the selection that AIGW held before,
// and the command that selects that one again, absent when the model would
// need quoting.
type followedSelection struct {
	Client      string `json:"client"`
	Model       string `json:"model"`
	Earlier     string `json:"earlier"`
	ReturnsWith string `json:"returns_with,omitempty"`
}

// followedReport is what an invocation says about the clients' own choices
// that its command worked from.
type followedReport struct {
	app      *App
	followed []synchronization.Followed
	preview  bool
}

// observe reads, before command runs, the choices that it will work from.
func (report *followedReport) observe(command *cobra.Command) {
	saves, preview := savesConfiguration(command)
	reports := slices.Contains(reportingCommands, commandPath(command))
	if !saves && !reports {
		return
	}
	report.preview = preview || reports
	_, report.followed, _ = report.app.invocationContext().WorkingConfiguration()
}

// selections are the followed choices to report once the command has
// succeeded: in a preview, and in a report that saves nothing, every choice
// that the command worked from; otherwise those that the saved configuration
// now records, which leaves out a choice that the command itself replaced
// and a command that saved nothing.
func (report *followedReport) selections() []synchronization.Followed {
	if len(report.followed) == 0 || report.preview {
		return report.followed
	}
	stored, err := report.app.Config.Load()
	if err != nil {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(report.followed), func(choice synchronization.Followed) bool {
		return stored.SelectedModel(choice.Client) != choice.Model
	})
}

// DocumentFields adds the followed choices to the command's document.
func (report *followedReport) DocumentFields() []terminal.DocumentField {
	selections := report.selections()
	if len(selections) == 0 {
		return nil
	}
	document := make([]followedSelection, 0, len(selections))
	for _, choice := range selections {
		entry := followedSelection{Client: choice.Client, Model: choice.Model.String(), Earlier: choice.Earlier.Model.String()}
		if action, _ := terminal.NextSteps(choice.Earlier.Restore()); action != "" {
			entry.ReturnsWith = action
		}
		document = append(document, entry)
	}
	return []terminal.DocumentField{{Name: "followed_selections", Value: document}}
}

// render ends the command's human output with the followed choices.
func (report *followedReport) render(r *terminal.Renderer) {
	selections := report.selections()
	if len(selections) == 0 {
		return
	}
	rows := make([]terminal.Field, 0, len(selections))
	for _, choice := range selections {
		returns := "To return to " + choice.Earlier.Model.String() + ": " + choice.Earlier.Restore()
		if action, _ := terminal.NextSteps(choice.Earlier.Restore()); action != "" {
			returns = "`" + action + "` returns to " + choice.Earlier.Model.String()
		}
		rows = append(rows, terminal.Field{
			State: terminal.Info, Label: choice.Product,
			Value: choice.Model.String() + ", chosen in " + choice.Product, Details: []string{returns},
		})
	}
	r.Section("Followed selection")
	r.Rows(rows...)
}
