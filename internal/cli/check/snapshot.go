package check

import (
	"context"
	"slices"

	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
)

// snapshot is what a check to the client depth of every enabled client did
// with the verified snapshot: written, or refused with its problem and fix.
type snapshot struct {
	written bool
	problem string
	fix     string
	cause   error
}

// writesSnapshot reports whether the check writes the verified snapshot that
// a rollback restores: a check to the client depth that names no client, in
// which no client failed and every client that can run a session ran one.
// Claude Desktop, which needs acceptance inside the app, is recorded at the
// depth that it reached.
func (o outcome) writesSnapshot() bool {
	if o.requested != depthClient || o.named || len(o.clients) == 0 {
		return false
	}
	return !slices.ContainsFunc(o.clients, func(client result) bool {
		return client.failure != nil || client.reached != depthClient && !client.spec.InAppAcceptance
	})
}

// saveSnapshot writes the verified snapshot of the configuration that the
// check loaded, with the depth that each client reached. The store writes it
// only while that configuration is still current, and otherwise names the
// same check again.
func (c *checker) saveSnapshot(ctx context.Context, o *outcome) {
	clients := make([]store.VerifiedClient, 0, len(o.clients))
	for _, client := range o.clients {
		clients = append(clients, store.VerifiedClient{Client: client.spec.ID, Depth: string(client.reached)})
	}
	err := c.deps.Config.SaveVerifiedCheckpoint(ctx, c.cfg, clients)
	o.snapshot = &snapshot{written: err == nil, cause: err}
	if err != nil {
		problem := terminal.ProblemFor(err)
		o.snapshot.problem, o.snapshot.fix = "The verified snapshot was not written: "+problem.Title, problem.Fix
	}
}
