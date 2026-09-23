package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// LegacyConfigVersion is the sole predecessor schema accepted by the explicit
// migration operation. Normal configuration reads accept ConfigVersion only.
const LegacyConfigVersion = 5

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
	Required        bool                            `json:"required"`
	Direction       MigrationDirection              `json:"direction"`
	FromVersion     int                             `json:"from_version"`
	ToVersion       int                             `json:"to_version"`
	Accounts        []string                        `json:"accounts"`
	Routes          []string                        `json:"routes"`
	Clients         map[string]ClientBinding        `json:"clients"`
	Recommendations map[string]ClientRecommendation `json:"recommendations"`

	path   string
	before Snapshot
	data   []byte
}

type legacyConfig struct {
	Version         int                        `toml:"version"`
	Accounts        map[string]Account         `toml:"accounts"`
	Profiles        map[string]legacyProfile   `toml:"profiles"`
	Recommendations map[string]legacySelection `toml:"recommendations,omitempty"`
	Clients         map[string]legacyBinding   `toml:"clients,omitempty"`
}

type legacyProfile struct {
	Label     string             `toml:"label"`
	Purpose   string             `toml:"purpose,omitempty"`
	Account   string             `toml:"account"`
	Model     string             `toml:"model"`
	Tier      string             `toml:"tier,omitempty"`
	Protocols []EndpointProtocol `toml:"protocols,omitempty"`
}

type legacySelection struct {
	Profile        string           `toml:"profile,omitempty"`
	Protocol       EndpointProtocol `toml:"protocol,omitempty"`
	ModelProvider  string           `toml:"model_provider,omitempty"`
	Authentication Authentication   `toml:"authentication,omitempty"`
}

type legacyBinding struct {
	Profile           string           `toml:"profile,omitempty"`
	Enabled           bool             `toml:"enabled"`
	Protocol          EndpointProtocol `toml:"protocol,omitempty"`
	ModelProvider     string           `toml:"model_provider,omitempty"`
	Authentication    Authentication   `toml:"authentication,omitempty"`
	Executable        string           `toml:"executable,omitempty"`
	Targets           []string         `toml:"targets,omitempty"`
	CredentialCommand string           `toml:"credential_command,omitempty"`
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
	if version == ConfigVersion {
		cfg, err := decodeTOMLConfig(before.Config.Data)
		if err != nil {
			return MigrationPlan{}, err
		}
		return summarizeMigration(s.path, before, cfg, before.Config.Data, false, MigrationUpgrade), nil
	}
	if version != LegacyConfigVersion {
		return MigrationPlan{}, newLoadError(LoadPhaseValidate, &UnsupportedConfigVersionError{Version: version, ExpectedVersion: ConfigVersion})
	}
	legacy, err := decodeLegacyConfig(before.Config.Data)
	if err != nil {
		return MigrationPlan{}, err
	}
	cfg, err := migrateLegacyConfig(legacy)
	if err != nil {
		return MigrationPlan{}, err
	}
	data, err := encodeConfig(cfg)
	if err != nil {
		return MigrationPlan{}, err
	}
	return summarizeMigration(s.path, before, cfg, data, true, MigrationUpgrade), nil
}

func (s Store) prepareMigrationRollback(before Snapshot) (MigrationPlan, error) {
	if _, err := decodeTOMLConfig(before.Config.Data); err != nil {
		return MigrationPlan{}, fmt.Errorf("configuration migration rollback requires a valid current configuration: %w", err)
	}
	if !before.Backup.Exists {
		return MigrationPlan{}, errors.New("configuration migration rollback is unavailable: no retained predecessor")
	}
	legacy, err := decodeLegacyConfig(before.Backup.Data)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("configuration migration rollback is unavailable: %w", err)
	}
	cfg, err := migrateLegacyConfig(legacy)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("configuration migration rollback is unavailable: %w", err)
	}
	return summarizeMigration(s.path, before, cfg, before.Backup.Data, true, MigrationRollback), nil
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
	if version != LegacyConfigVersion {
		return legacyConfig{}, newLoadError(LoadPhaseValidate, &UnsupportedConfigVersionError{Version: version, ExpectedVersion: LegacyConfigVersion})
	}
	var legacy legacyConfig
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&legacy); err != nil {
		return legacyConfig{}, newLoadError(LoadPhaseParse, err)
	}
	return legacy, nil
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

func migrateLegacyConfig(legacy legacyConfig) (Config, error) {
	cfg := NewConfig()
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
			return Config{}, fmt.Errorf("profile %q has no model", id)
		}
		if _, exists := cfg.Accounts[profile.Account]; !exists {
			return Config{}, fmt.Errorf("profile %q references unknown account %q", id, profile.Account)
		}
		if len(profile.Protocols) == 0 {
			return Config{}, fmt.Errorf("profile %q has no explicit protocol and cannot be migrated without guessing", id)
		}
		if _, exists := cfg.Models[profile.Model]; !exists {
			cfg.Models[profile.Model] = Model{Label: profile.Model}
		}
		interfaces := make(map[EndpointProtocol][]Capability, len(profile.Protocols))
		for _, protocol := range profile.Protocols {
			interfaces[protocol] = []Capability{}
		}
		cfg.Routes[id] = Route{
			Label: profile.Label, Purpose: profile.Purpose, Account: profile.Account,
			Model: profile.Model, UpstreamModel: profile.Model, Interfaces: interfaces,
		}
	}
	for client, selection := range legacy.Recommendations {
		cfg.Recommendations[client] = ClientRecommendation{Primary: migrateLegacySelection(selection)}
	}
	for client, binding := range legacy.Clients {
		cfg.Clients[client] = ClientBinding{
			Route: binding.Profile, Enabled: binding.Enabled, Protocol: binding.Protocol,
			ModelProvider: binding.ModelProvider, Authentication: binding.Authentication,
			Executable: binding.Executable, Targets: slices.Clone(binding.Targets),
			CredentialCommand: binding.CredentialCommand,
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("migrated configuration is invalid: %w", err)
	}
	return cfg, nil
}

func migrateLegacySelection(selection legacySelection) ClientSelection {
	return ClientSelection{
		Route: selection.Profile, Protocol: selection.Protocol,
		ModelProvider: selection.ModelProvider, Authentication: selection.Authentication,
	}
}

func summarizeMigration(path string, before Snapshot, cfg Config, data []byte, required bool, direction MigrationDirection) MigrationPlan {
	from, to := ConfigVersion, ConfigVersion
	if required && direction == MigrationUpgrade {
		from = LegacyConfigVersion
	} else if required {
		to = LegacyConfigVersion
	}
	return MigrationPlan{
		Required: required, Direction: direction, FromVersion: from, ToVersion: to,
		Accounts: slices.Sorted(maps.Keys(cfg.Accounts)), Routes: cfg.RouteIDs(),
		Clients: maps.Clone(cfg.Clients), Recommendations: maps.Clone(cfg.Recommendations),
		path: path, before: before, data: slices.Clone(data),
	}
}
