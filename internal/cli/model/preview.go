package model

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// change is one model write: the configuration before and after it, and
// the document and rows that report it.
type change struct {
	before, after domain.Config
	subject       string
	version       int
	document      changeDocument
	title         string
	rows          []terminal.Field
	// next is the step after the write; a preview continues with the write.
	next string
}

// apply commits a change, or with dryRun previews it, and reports it. A
// preview validates the change as the write does, names the client files
// that the write would change and writes nothing.
func (c change) apply(cmd *cobra.Command, runtime invocation.Context, args []string, dryRun, jsonMode bool) error {
	preview, err := invocation.PreviewChange(runtime, c.before, c.after)
	if err != nil {
		return err
	}
	if dryRun {
		c.next = invocation.ApplyCommand(cmd, args)
	} else if err := runtime.Synchronizer().Commit(cmd.Context(), c.before, c.after, c.subject); err != nil {
		return err
	}
	c.document.DryRun = dryRun
	c.document.ClientFiles = clientFiles(runtime, preview)
	c.document.CredentialReader = preview.CredentialEntrypoint
	if jsonMode {
		c.document.NextAction, c.document.NextDetail = terminal.NextSteps(c.next)
		return terminal.WriteDocument(runtime.Out, c.version, true, c.document)
	}
	r := invocation.Renderer(runtime)
	title := c.title
	if dryRun {
		title += " (preview)"
	}
	r.ProductTitle(title)
	r.Rows(c.rows...)
	renderClientFiles(r, runtime, c.document.ClientFiles)
	if dryRun {
		r.Success("Nothing was written: configuration, credentials and client files are unchanged")
	} else {
		r.Success("Saved; no client's selection changed")
	}
	r.Next(c.next)
	return r.Err()
}

// renderClientFiles names the client files that a change writes.
func renderClientFiles(r *terminal.Renderer, runtime invocation.Context, files []clientFile) {
	if len(files) == 0 {
		return
	}
	r.Section("Client files")
	rows := make([]terminal.Field, 0, len(files))
	for _, file := range files {
		label := runtime.Clients.Admission().Label(file.Client)
		if file.File != "" {
			label += " " + file.File
		}
		rows = append(rows, terminal.Field{Label: label, Value: file.Action})
	}
	r.Rows(rows...)
}
