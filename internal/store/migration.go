package store

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

// MigrationDirection identifies the only two bounded schema transitions.
type MigrationDirection string

const (
	// MigrationUpgrade replaces the sole supported predecessor schema.
	MigrationUpgrade MigrationDirection = "upgrade"
	// MigrationRollback restores the exact retained predecessor schema.
	MigrationRollback MigrationDirection = "rollback"
)

// MigrationPlan is a secret-free preview bound to the exact configuration
// preimage from which it was prepared.
type MigrationPlan struct {
	Required        bool                                   `json:"required"`
	Direction       MigrationDirection                     `json:"direction"`
	FromVersion     int                                    `json:"from_version"`
	ToVersion       int                                    `json:"to_version"`
	Accounts        []string                               `json:"accounts"`
	Routes          []string                               `json:"routes"`
	Clients         map[string]domain.ClientBinding        `json:"clients"`
	Recommendations map[string]domain.ClientRecommendation `json:"recommendations"`

	path   string
	before Snapshot
	data   []byte
}

type legacyConfig struct {
	Version         int                        `toml:"version"`
	Accounts        map[string]domain.Account  `toml:"accounts"`
	Profiles        map[string]legacyProfile   `toml:"profiles"`
	Recommendations map[string]legacySelection `toml:"recommendations,omitempty"`
	Clients         map[string]legacyBinding   `toml:"clients,omitempty"`
}

type legacyProfile struct {
	Label     string                    `toml:"label"`
	Purpose   string                    `toml:"purpose,omitempty"`
	Account   string                    `toml:"account"`
	Model     string                    `toml:"model"`
	Tier      string                    `toml:"tier,omitempty"`
	Protocols []domain.EndpointProtocol `toml:"protocols,omitempty"`
}

type legacySelection struct {
	Profile        string                  `toml:"profile,omitempty"`
	Protocol       domain.EndpointProtocol `toml:"protocol,omitempty"`
	ModelProvider  string                  `toml:"model_provider,omitempty"`
	Authentication domain.Authentication   `toml:"authentication,omitempty"`
}

type legacyBinding struct {
	Profile           string                  `toml:"profile,omitempty"`
	Enabled           bool                    `toml:"enabled"`
	Protocol          domain.EndpointProtocol `toml:"protocol,omitempty"`
	ModelProvider     string                  `toml:"model_provider,omitempty"`
	Authentication    domain.Authentication   `toml:"authentication,omitempty"`
	Executable        string                  `toml:"executable,omitempty"`
	Targets           []string                `toml:"targets,omitempty"`
	CredentialCommand string                  `toml:"credential_command,omitempty"`
}

type publishedConfig struct {
	Version           int                         `toml:"version"`
	Accounts          map[string]domain.Account   `toml:"accounts"`
	Profiles          map[string]publishedProfile `toml:"profiles"`
	Routes            map[string]string           `toml:"routes"`
	RecommendedRoutes map[string]string           `toml:"recommended_routes,omitempty"`
	Adapters          map[string]publishedAdapter `toml:"adapters,omitempty"`
}

type publishedProfile struct {
	Label          string                `toml:"label"`
	Purpose        string                `toml:"purpose,omitempty"`
	Account        string                `toml:"account"`
	Client         string                `toml:"client"`
	Model          string                `toml:"model"`
	ModelProvider  string                `toml:"model_provider,omitempty"`
	Authentication domain.Authentication `toml:"authentication,omitempty"`
}

type publishedAdapter struct {
	Enabled           bool     `toml:"enabled"`
	Executable        string   `toml:"executable,omitempty"`
	Targets           []string `toml:"targets,omitempty"`
	CredentialCommand string   `toml:"credential_command,omitempty"`
}

// PrepareMigration reads either the current configuration or its one-version
// rollback input and returns a deterministic, non-mutating migration preview.
func (s Store) PrepareMigration(rollback bool) (MigrationPlan, error) {
	before, err := s.CaptureSnapshot()
	if err != nil {
		return MigrationPlan{}, err
	}
	if !before.Config.Exists {
		return MigrationPlan{}, errors.New("configuration is unavailable; run `aigw setup` before migration")
	}
	if rollback {
		return s.prepareMigrationRollback(before)
	}
	version, err := configurationVersion(before.Config.Data)
	if err != nil {
		return MigrationPlan{}, err
	}
	if version == domain.ConfigVersion {
		cfg, err := s.loadSnapshot(before)
		if err != nil {
			return MigrationPlan{}, err
		}
		return summarizeMigration(s.path, before, cfg, before.Config.Data, false, MigrationUpgrade, domain.ConfigVersion), nil
	}
	if version != domain.LegacyConfigVersion && version != domain.PublishedConfigVersion {
		return MigrationPlan{}, newLoadError(LoadPhaseValidate, &domain.UnsupportedConfigVersionError{Version: version, ExpectedVersion: domain.ConfigVersion})
	}
	cfg, err := migratePredecessorConfig(before.Config.Data, version, s.admission)
	if err != nil {
		return MigrationPlan{}, err
	}
	data, err := encodeConfig(cfg)
	if err != nil {
		return MigrationPlan{}, err
	}
	return summarizeMigration(s.path, before, cfg, data, true, MigrationUpgrade, version), nil
}

func (s Store) prepareMigrationRollback(before Snapshot) (MigrationPlan, error) {
	if _, err := s.loadSnapshot(before); err != nil {
		return MigrationPlan{}, fmt.Errorf("configuration migration rollback requires a valid current configuration: %w", err)
	}
	if !before.Backup.Exists {
		return MigrationPlan{}, errors.New("configuration migration rollback is unavailable: no retained predecessor")
	}
	version, err := configurationVersion(before.Backup.Data)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("configuration migration rollback is unavailable: %w", err)
	}
	cfg, err := migratePredecessorConfig(before.Backup.Data, version, s.admission)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("configuration migration rollback is unavailable: %w", err)
	}
	return summarizeMigration(s.path, before, cfg, before.Backup.Data, true, MigrationRollback, version), nil
}

// ApplyMigration writes only the prepared configuration bytes while the exact
// source preimage is unchanged. Store commit owns backup and compensation.
func (s Store) ApplyMigration(plan MigrationPlan) error {
	if plan.path != s.path || plan.path == "" {
		return errors.New("migration plan belongs to a different configuration store")
	}
	if !plan.Required {
		return nil
	}
	_, err := s.commitData(plan.before, plan.data)
	return err
}

func decodeLegacyConfig(data []byte) (legacyConfig, error) {
	version, err := configurationVersion(data)
	if err != nil {
		return legacyConfig{}, err
	}
	if version != domain.LegacyConfigVersion {
		return legacyConfig{}, newLoadError(LoadPhaseValidate, &domain.UnsupportedConfigVersionError{Version: version, ExpectedVersion: domain.LegacyConfigVersion})
	}
	var legacy legacyConfig
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&legacy); err != nil {
		return legacyConfig{}, newLoadError(LoadPhaseParse, err)
	}
	return legacy, nil
}

func migratePredecessorConfig(data []byte, version int, admission domain.Admission) (domain.Config, error) {
	switch version {
	case domain.LegacyConfigVersion:
		legacy, err := decodeLegacyConfig(data)
		if err != nil {
			return domain.Config{}, err
		}
		return migrateLegacyConfig(legacy, admission)
	case domain.PublishedConfigVersion:
		published, err := decodePublishedConfig(data)
		if err != nil {
			return domain.Config{}, err
		}
		return migratePublishedConfig(published, admission)
	default:
		return domain.Config{}, newLoadError(LoadPhaseValidate, &domain.UnsupportedConfigVersionError{Version: version, ExpectedVersion: domain.ConfigVersion})
	}
}

func decodePublishedConfig(data []byte) (publishedConfig, error) {
	var published publishedConfig
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&published); err != nil {
		return publishedConfig{}, newLoadError(LoadPhaseParse, err)
	}
	return published, nil
}

func configurationVersion(data []byte) (int, error) {
	var header struct {
		Version int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &header); err != nil {
		return 0, newLoadError(LoadPhaseParse, err)
	}
	return header.Version, nil
}

func migrateLegacyConfig(legacy legacyConfig, admission domain.Admission) (domain.Config, error) {
	cfg := domain.NewConfig(admission)
	for id, account := range legacy.Accounts {
		if account.AccountProbe != nil {
			probe := *account.AccountProbe
			account.AccountProbe = &probe
		}
		cfg.Accounts[id] = account
	}
	for _, id := range slices.Sorted(maps.Keys(legacy.Profiles)) {
		profile := legacy.Profiles[id]
		if strings.TrimSpace(profile.Model) == "" {
			return domain.Config{}, fmt.Errorf("profile %q has no model", id)
		}
		if _, exists := cfg.Accounts[profile.Account]; !exists {
			return domain.Config{}, fmt.Errorf("profile %q references unknown account %q", id, profile.Account)
		}
		if len(profile.Protocols) == 0 {
			return domain.Config{}, fmt.Errorf("profile %q has no explicit protocol and cannot be migrated without guessing", id)
		}
		if _, exists := cfg.Models[profile.Model]; !exists {
			cfg.Models[profile.Model] = domain.Model{Label: profile.Model}
		}
		interfaces := make(map[domain.EndpointProtocol][]domain.Capability, len(profile.Protocols))
		for _, protocol := range profile.Protocols {
			interfaces[protocol] = []domain.Capability{}
		}
		cfg.Routes[id] = domain.Route{
			Label: profile.Label, Purpose: profile.Purpose, Account: profile.Account,
			Model: profile.Model, UpstreamModel: profile.Model, Interfaces: interfaces,
		}
	}
	for client, selection := range legacy.Recommendations {
		cfg.Recommendations[client] = domain.ClientRecommendation{Primary: migrateLegacySelection(selection)}
	}
	for client, binding := range legacy.Clients {
		cfg.Clients[client] = domain.ClientBinding{
			Route: binding.Profile, Enabled: binding.Enabled, Protocol: binding.Protocol,
			ModelProvider: binding.ModelProvider, Authentication: binding.Authentication,
			Executable: binding.Executable, Targets: slices.Clone(binding.Targets),
			CredentialCommand: binding.CredentialCommand,
		}
	}
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("migrated configuration is invalid: %w", err)
	}
	return cfg, nil
}

func migratePublishedConfig(published publishedConfig, admission domain.Admission) (domain.Config, error) {
	cfg := domain.NewConfig(admission)
	maps.Copy(cfg.Accounts, published.Accounts)
	selections := make(map[string]domain.ClientSelection, len(published.Profiles))
	for _, id := range slices.Sorted(maps.Keys(published.Profiles)) {
		profile := published.Profiles[id]
		spec, _ := admission.Spec(profile.Client)
		protocol := spec.PublishedProfileProtocol
		if protocol == "" {
			return domain.Config{}, fmt.Errorf("published profile %q has no admitted client protocol for %q", id, profile.Client)
		}
		if _, exists := cfg.Accounts[profile.Account]; !exists {
			return domain.Config{}, fmt.Errorf("published profile %q references unknown account %q", id, profile.Account)
		}
		if strings.TrimSpace(profile.Model) == "" {
			return domain.Config{}, fmt.Errorf("published profile %q has no model", id)
		}
		if _, exists := cfg.Models[profile.Model]; !exists {
			cfg.Models[profile.Model] = domain.Model{Label: profile.Model}
		}
		cfg.Routes[id] = domain.Route{
			Label: profile.Label, Purpose: profile.Purpose, Account: profile.Account,
			Model: profile.Model, UpstreamModel: profile.Model,
			Interfaces: map[domain.EndpointProtocol][]domain.Capability{protocol: {}},
		}
		selections[id] = domain.ClientSelection{
			Route: id, Protocol: protocol,
			ModelProvider: profile.ModelProvider, Authentication: profile.Authentication,
		}
	}
	for client, id := range published.RecommendedRoutes {
		selection, exists := selections[id]
		if !exists {
			return domain.Config{}, fmt.Errorf("published recommendation %q references unknown profile %q", client, id)
		}
		cfg.Recommendations[client] = domain.ClientRecommendation{Primary: selection}
	}
	for client, id := range published.Routes {
		selection, exists := selections[id]
		if !exists {
			return domain.Config{}, fmt.Errorf("published selection %q references unknown profile %q", client, id)
		}
		cfg.Clients[client] = domain.ClientBinding{
			Route: selection.Route, Protocol: selection.Protocol,
			ModelProvider: selection.ModelProvider, Authentication: selection.Authentication,
		}
	}
	for client, adapter := range published.Adapters {
		binding := cfg.Clients[client]
		binding.Enabled = adapter.Enabled
		binding.Executable = adapter.Executable
		binding.Targets = slices.Clone(adapter.Targets)
		binding.CredentialCommand = adapter.CredentialCommand
		cfg.Clients[client] = binding
	}
	for id, profile := range published.Profiles {
		if profile.ModelProvider == "" && profile.Authentication == "" {
			continue
		}
		selected := published.Routes[profile.Client] == id || published.RecommendedRoutes[profile.Client] == id
		if !selected {
			return domain.Config{}, fmt.Errorf("published profile %q has client-specific options without a selected or recommended owner", id)
		}
	}
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("migrated published configuration is invalid: %w", err)
	}
	return cfg, nil
}

func migrateLegacySelection(selection legacySelection) domain.ClientSelection {
	return domain.ClientSelection{
		Route: selection.Profile, Protocol: selection.Protocol,
		ModelProvider: selection.ModelProvider, Authentication: selection.Authentication,
	}
}

func summarizeMigration(path string, before Snapshot, cfg domain.Config, data []byte, required bool, direction MigrationDirection, predecessorVersion int) MigrationPlan {
	from, to := domain.ConfigVersion, domain.ConfigVersion
	if required && direction == MigrationUpgrade {
		from = predecessorVersion
	} else if required {
		to = predecessorVersion
	}
	return MigrationPlan{
		Required: required, Direction: direction, FromVersion: from, ToVersion: to,
		Accounts: slices.Sorted(maps.Keys(cfg.Accounts)), Routes: cfg.RouteIDs(),
		Clients: maps.Clone(cfg.Clients), Recommendations: maps.Clone(cfg.Recommendations),
		path: path, before: before, data: slices.Clone(data),
	}
}
