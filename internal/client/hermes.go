package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	hermesconfig "aigw-cli/internal/hermes/configuration"
	"aigw-cli/internal/process"

	"github.com/rogpeppe/go-internal/robustio"
)

type hermesAdapter struct{}

const hermesSurface = "hermes-home-default"

func (hermesAdapter) Spec() configuration.ClientSpec {
	return mustClientSpec(configuration.ClientHermes)
}

func (hermesAdapter) Discover(source DiscoverySource) discovery.Result {
	executable := source.Executable(configuration.ClientHermes)
	path := filepath.Join(source.HermesHomeDirectory(), "config.yaml")
	return discovery.Result{
		Executables: map[string]string{configuration.ClientHermes: executable},
		Surfaces:    []discovery.Surface{{ID: hermesSurface, Product: "Hermes", Authority: "aigw", Executable: executable, ConfigPath: path, Present: source.FilePresent(path)}},
	}
}

func (hermesAdapter) Converge(deps Dependencies, cfg *configuration.Config, discovered discovery.Result) error {
	selected, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if _, absent := errors.AsType[*configuration.RuntimeBindingUnselectedError](err); absent {
		return nil
	}
	if err != nil {
		return err
	}
	adapter, explicitlyConfigured := cfg.Clients[configuration.ClientHermes]
	if !explicitlyConfigured || !adapter.Enabled {
		return nil
	}
	executable, err := resolveExecutable(configuration.ClientHermes, adapter.Executable, discovered.Executable(configuration.ClientHermes))
	if err != nil {
		return err
	}
	available, err := discovery.ExecutableAvailable(executable)
	if err != nil {
		return err
	}
	if !available {
		return nil
	}
	credentialAvailable := !selected.UsesAIGWCredentialStore()
	if selected.UsesAIGWCredentialStore() {
		credentialAvailable, err = secretAvailable(deps.Secrets, selected.AccountID)
		if err != nil {
			return err
		}
	}
	if !credentialAvailable {
		return nil
	}
	if len(adapter.Targets) == 0 {
		surface, found := discovered.Surface(hermesSurface)
		if !found || surface.ConfigPath == "" {
			return nil
		}
		adapter.Targets = []string{surface.ConfigPath}
	}
	cfg.SetClientActivation(configuration.ClientHermes, true, executable, adapter.Targets)
	return nil
}

func hermesDesired(deps Dependencies, cfg configuration.Config, selected configuration.Runtime) (hermesconfig.Desired, error) {
	spec := mustClientSpec(configuration.ClientHermes)
	providers := make(map[string]hermesconfig.Provider)
	connected := map[string]bool{selected.AccountID: true}
	for _, profileID := range cfg.ProfileIDs() {
		profile := cfg.Profiles[profileID]
		if _, observed := connected[profile.Account]; !observed {
			available, err := secretAvailable(deps.Secrets, profile.Account)
			if err != nil {
				return hermesconfig.Desired{}, fmt.Errorf("inspect Hermes Account %q credential: %w", profile.Account, err)
			}
			connected[profile.Account] = available
		}
		if !connected[profile.Account] {
			continue
		}
		account := cfg.Accounts[profile.Account]
		protocols := spec.CompatibleProfileProtocols(account, profile)
		if profile.Protocols == nil {
			if profileID != selected.ProfileID {
				continue
			}
			protocols = []configuration.EndpointProtocol{selected.Protocol}
		}
		for _, protocol := range protocols {
			providerID := hermesProviderID(profile.Account, protocol)
			provider, exists := providers[providerID]
			if !exists {
				providerRuntime, err := cfg.ResolveProfileProtocol(configuration.ClientHermes, profileID, protocol)
				if err != nil {
					return hermesconfig.Desired{}, err
				}
				command, err := credential.Command(providerRuntime.CredentialExecutable(deps.AIGWExecutable), configuration.ClientHermes, providerRuntime.CredentialProjectionFingerprint(configuration.ClientHermes), runtime.GOOS)
				if err != nil {
					return hermesconfig.Desired{}, err
				}
				provider = hermesconfig.Provider{
					ID: providerID, Endpoint: providerRuntime.Endpoint, Protocol: string(protocol), CredentialCommand: command,
				}
			}
			provider.Models = append(provider.Models, profile.Model)
			providers[providerID] = provider
		}
	}
	desired := hermesconfig.Desired{SelectedProvider: hermesProviderID(selected.AccountID, selected.Protocol), SelectedModel: selected.Model}
	for _, provider := range providers {
		slices.Sort(provider.Models)
		provider.Models = slices.Compact(provider.Models)
		desired.Providers = append(desired.Providers, provider)
	}
	sort.Slice(desired.Providers, func(left, right int) bool { return desired.Providers[left].ID < desired.Providers[right].ID })
	if err := desired.Validate(); err != nil {
		return hermesconfig.Desired{}, err
	}
	return desired, nil
}

func hermesProviderID(accountID string, protocol configuration.EndpointProtocol) string {
	return "aigw-" + accountID + "-" + strings.ReplaceAll(string(protocol), "_", "-")
}

func hermesPlans(deps Dependencies, before, after configuration.Config) ([]hermesconfig.Plan, []string, error) {
	previous, current := before.Clients[configuration.ClientHermes], after.Clients[configuration.ClientHermes]
	if !previous.Enabled && !current.Enabled {
		return nil, nil, nil
	}
	var desired hermesconfig.Desired
	if current.Enabled && len(current.Targets) > 0 {
		if len(current.Targets) != 1 {
			return nil, nil, errors.New("hermes requires one configured home")
		}
		selected, err := after.ResolveRuntime(configuration.ClientHermes, "")
		if err != nil {
			return nil, nil, err
		}
		desired, err = hermesDesired(deps, after, selected)
		if err != nil {
			return nil, nil, err
		}
	}
	targets := slices.Clone(current.Targets)
	for _, target := range previous.Targets {
		if !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}
	plans := make([]hermesconfig.Plan, 0, len(targets))
	for _, target := range targets {
		var projection *hermesconfig.Desired
		if current.Enabled && len(current.Targets) > 0 && slices.Contains(current.Targets, target) {
			projection = &desired
		}
		plan, err := hermesconfig.Prepare(target, projection)
		if err != nil {
			return nil, nil, err
		}
		plans = append(plans, plan)
	}
	return plans, targets, nil
}

func (hermesAdapter) Plan(deps Dependencies, before, after configuration.Config) ([]ProjectionPlan, error) {
	plans, targets, err := hermesPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectionPlan, 0, len(plans))
	for index, plan := range plans {
		result = append(result, ProjectionPlan{Client: configuration.ClientHermes, Target: targets[index], Action: plan.Action})
	}
	return result, nil
}

type hermesReceipt []hermesconfig.Receipt

func (receipt hermesReceipt) Rollback() error {
	var result error
	for _, item := range slices.Backward(receipt) {
		result = errors.Join(result, item.Rollback())
	}
	return result
}

func (hermesAdapter) Apply(_ context.Context, deps Dependencies, before, after configuration.Config) (ProjectionReceipt, error) {
	plans, _, err := hermesPlans(deps, before, after)
	if err != nil {
		return nil, err
	}
	receipts := hermesReceipt{}
	for _, plan := range plans {
		receipt, err := plan.Apply()
		if err != nil {
			return nil, errors.Join(err, receipts.Rollback())
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func (hermesAdapter) ProjectionChanged(before, after configuration.Config) bool {
	previous, current := before.Clients[configuration.ClientHermes], after.Clients[configuration.ClientHermes]
	if previous.Enabled != current.Enabled {
		return true
	}
	if !current.Enabled {
		return false
	}
	if previous.CredentialCommand != current.CredentialCommand || !slices.Equal(previous.Targets, current.Targets) {
		return true
	}
	left, leftErr := before.ResolveRuntime(configuration.ClientHermes, "")
	right, rightErr := after.ResolveRuntime(configuration.ClientHermes, "")
	return leftErr != nil || rightErr != nil || left != right
}

func (hermesAdapter) Inspect(ctx context.Context, deps Dependencies, cfg configuration.Config, selected configuration.Runtime) Status {
	if err := ctx.Err(); err != nil {
		return Status{Issue: err.Error()}
	}
	adapter := cfg.Clients[configuration.ClientHermes]
	available, err := discovery.ExecutableAvailable(adapter.Executable)
	if err != nil || !adapter.Enabled || !available || len(adapter.Targets) != 1 {
		return Status{Issue: "Hermes executable or configuration home is unavailable", RepairAction: "aigw sync"}
	}
	desired, err := hermesDesired(deps, cfg, selected)
	if err == nil {
		var plan hermesconfig.Plan
		plan, err = hermesconfig.Prepare(adapter.Targets[0], &desired)
		if err == nil && plan.Action != "unchanged" {
			err = errors.New("hermes configuration projection differs from the selected route")
		}
	}
	if err != nil {
		return Status{Issue: err.Error(), RepairAction: "aigw sync"}
	}
	return Status{Ready: true}
}

func (hermesAdapter) Withdraw(cfg *configuration.Config) {
	delete(cfg.Clients, configuration.ClientHermes)
}

func (adapter hermesAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, selected configuration.Runtime, _ string) (_ Verification, result error) {
	if deps.Runner == nil {
		return Verification{}, errors.New("hermes verification requires a process runner")
	}
	configured := cfg.Clients[configuration.ClientHermes]
	if !configured.Enabled {
		return Verification{}, errors.New("hermes adapter is disabled; run aigw sync")
	}
	home, err := os.MkdirTemp("", "aigw-hermes-verification-")
	if err != nil {
		return Verification{}, err
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(home)) }()
	desired, err := hermesDesired(deps, cfg, selected)
	if err != nil {
		return Verification{}, err
	}
	plan, err := hermesconfig.Prepare(filepath.Join(home, "config.yaml"), &desired)
	if err != nil {
		return Verification{}, err
	}
	if _, err := plan.Apply(); err != nil {
		return Verification{}, err
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HERMES_HOME=") {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, "HERMES_HOME="+home)
	probeCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	probe := process.Plan{Executable: configured.Executable, Directory: home, Env: environment, Args: []string{"--version"}}
	version, err := deps.Runner.RunCapture(probeCtx, probe)
	if err != nil {
		return Verification{}, errors.New("hermes executable identity could not be observed")
	}
	probe.Args = []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	probe.Stdin = "Reply with exactly AIGW_OK."
	response, err := deps.Runner.RunCapture(probeCtx, probe)
	if err != nil {
		return Verification{}, errors.Join(errors.New("hermes inference failed; external credential diagnostics suppressed"), probeCtx.Err())
	}
	if !strings.Contains(string(response), "AIGW_OK") {
		return Verification{}, errors.New("hermes model response did not return the expected AIGW_OK verification marker")
	}
	executable, err := os.ReadFile(configured.Executable)
	if err != nil {
		return Verification{}, err
	}
	return Verification{Version: strings.TrimSpace(string(version)), SHA256: fmt.Sprintf("%x", sha256.Sum256(executable))}, nil
}
