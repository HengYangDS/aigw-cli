package synchronization

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

// SelectProfile commits one Profile's Route and projection, optionally storing
// its validated Account Token. Failure compensates owned credential writes;
// success leaves no rollback obligation with the caller. The result reports
// configuration change, not Token acquisition or live inference.
func (s Synchronizer) SelectProfile(ctx context.Context, before configuration.Config, profileID, token string) (changed bool, resultErr error) {
	return s.selectProfile(ctx, before, before.Clone(), profileID, token)
}

func (s Synchronizer) selectProfile(ctx context.Context, before, after configuration.Config, profileID, token string) (changed bool, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	profile, exists := after.Profiles[profileID]
	if !exists {
		return false, fmt.Errorf("unknown profile %q", profileID)
	}
	selected, err := after.ResolveRuntime(profile.Client, profileID)
	if err != nil {
		return false, err
	}
	var tokens map[string]string
	if token != "" {
		if !selected.RequiresAccountToken() {
			return false, fmt.Errorf("profile %q uses client-owned authentication; Account Token storage is not part of selection", profileID)
		}
		tokens = map[string]string{selected.AccountID: token}
	}
	rollback, err := secrets.Replace(s.Secrets, tokens)
	if err != nil {
		return false, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, rollback())
		}
	}()
	after.Routes[profile.Client] = profileID
	after, _, err = s.DesiredClientConfiguration(after, profile.Client)
	if err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if reflect.DeepEqual(before, after) {
		return false, s.ReconcileClient(ctx, after, profile.Client)
	}
	if err := s.commit(ctx, before, after, "route", true, profile.Client); err != nil {
		return false, err
	}
	return true, nil
}
