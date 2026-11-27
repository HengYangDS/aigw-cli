package synchronization

import (
	"context"
	"errors"
	"slices"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
)

// PrepareSelection returns cfg with client's selection of the model that ref
// names on protocol, or on the protocol that the configuration resolves when it is
// empty, and the runtime that it resolves to, without changing cfg. The
// client's binding is taken from the stored configuration first (see
// FromStoredBinding), so the selection replaces what was saved and not a
// model that the client's own file chose in between. Setting
// the selection keeps forwarding, its destination following the upstream, and
// refuses one that the forwarding cannot carry; it drops retry budgets when
// the provider changes. The same result is used for credential admission and
// commit.
func (s Synchronizer) PrepareSelection(cfg domain.Config, client string, ref domain.ModelRef, protocol domain.EndpointProtocol) (domain.Config, domain.Runtime, error) {
	if protocol == "" {
		resolved, err := cfg.ResolveRuntime(client, ref)
		if err != nil {
			return domain.Config{}, domain.Runtime{}, err
		}
		protocol = resolved.Protocol
	}
	selected := s.FromStoredBinding(cfg, client)
	selected.SetSelectedModel(client, ref, protocol)
	if err := selected.Validate(); err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	runtime, err := selected.ResolveRuntime(client, domain.ModelRef{})
	if err != nil {
		return domain.Config{}, domain.Runtime{}, err
	}
	return selected, runtime, nil
}

// FromStoredBinding returns cfg with client's binding as the stored
// configuration holds it. An explicit selection replaces the client's
// selection, so it starts from what the operator last saved for the client and
// not from a model that the client's own file chose in between: following
// that choice may have dropped the forwarding and the retry budgets of the
// stored selection, which selecting the stored model again must keep, and the
// protocol that an explicit selection keeps by default is the stored one.
// Without a stored binding, or without a store, cfg is used as it is.
func (s Synchronizer) FromStoredBinding(cfg domain.Config, client string) domain.Config {
	selected := cfg.Clone()
	stored, read := s.stored(cfg)
	if !read {
		return selected
	}
	binding, bound := stored.Clients[client]
	if _, held := selected.Models[binding.Model]; !bound || !held {
		return selected
	}
	selected.Clients[client] = binding
	return selected
}

// stored reads the stored configuration, which cfg, the configuration that
// the command works from, was derived from. It reports false without a store
// or when the stored configuration cannot be read.
func (s Synchronizer) stored(cfg domain.Config) (domain.Config, bool) {
	if s.Config == nil {
		return domain.Config{}, false
	}
	snapshot, err := s.Config.CaptureSnapshot()
	if err != nil {
		return domain.Config{}, false
	}
	stored, exists, err := snapshot.Configuration(cfg.Admission())
	return stored, exists && err == nil
}

// SelectModel commits after, which PrepareSelection made from before and may
// extend with what the selection adds, and the files of the client and of
// every other client whose files change, optionally storing the client's
// validated provider Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration and client file changes separately, not Token acquisition or
// live inference.
func (s Synchronizer) SelectModel(ctx context.Context, before, after domain.Config, client, token string) (changed, projectionChanged bool, binding domain.ClientBinding, resultErr error) {
	return s.selectModel(ctx, before, after, client, token)
}

func (s Synchronizer) selectModel(ctx context.Context, before, after domain.Config, client, token string) (changed, projectionChanged bool, binding domain.ClientBinding, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	selected, err := after.ResolveRuntime(client, domain.ModelRef{})
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	binding = after.Clients[client]
	binding.Enabled = selectionEnables(before.Clients[client], binding)
	after.Clients[client] = binding
	var tokens map[string]string
	if token != "" {
		tokens = map[string]string{selected.ProviderID: token}
	}
	rollback, err := secret.Replace(s.Secrets, tokens)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, platform.FailedUndo("removing the Token that this command stored for provider "+selected.ProviderID, rollback()))
		}
	}()
	after, _, err = s.DesiredClientConfiguration(after, client)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	if err := ctx.Err(); err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	binding = after.Clients[client]
	selector := s
	selector.AuthorizedSelectionClient = client
	result, err := selector.commit(ctx, before, after, "client selection", true, selector.selectionClients(before, after, client)...)
	if err != nil {
		return false, false, domain.ClientBinding{}, err
	}
	// The stored configuration also changes when the selection names the
	// model that the client had chosen itself, which nothing had recorded.
	return result.configuration, result.projection, binding, nil
}

// selectionClients returns the clients whose files a selection for client
// reconciles, in admission order: client itself, and every other client whose
// files change, such as one that lists a model that the selection adds.
func (s Synchronizer) selectionClients(before, after domain.Config, client string) []string {
	changed := s.registry().ChangedClients(before, after)
	clients := make([]string, 0, len(changed)+1)
	for _, id := range s.ClientIDs() {
		if id == client || slices.Contains(changed, id) {
			clients = append(clients, id)
		}
	}
	return clients
}

// selectionEnables reports whether a selection enables its client. An enabled
// client stays so, and one without a selection is enabled by its first. A
// client that is disabled on a model of its own is enabled only by selecting
// that model and protocol again, the step that resumes it; moving its
// selection to another model or protocol, which is what the fix of a refused
// edit or removal runs, keeps it disabled until `aigw client enable`. A
// caller that asks to use the client, as guided setup does, sets next.Enabled
// itself.
func selectionEnables(previous, next domain.ClientBinding) bool {
	return previous.Enabled || next.Enabled || previous.Model.IsZero() || (previous.Model == next.Model && (previous.Protocol == "" || previous.Protocol == next.Protocol))
}
