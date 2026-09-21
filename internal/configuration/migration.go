package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/pelletier/go-toml/v2"
)

// LegacyConfigVersion is the sole predecessor schema accepted by the explicit
// migration operation. Normal configuration reads accept ConfigVersion only.
const LegacyConfigVersion = 3

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
	Required        bool                       `json:"required"`
	Direction       MigrationDirection         `json:"direction"`
	FromVersion     int                        `json:"from_version"`
	ToVersion       int                        `json:"to_version"`
	Accounts        []string                   `json:"accounts"`
	Profiles        []string                   `json:"profiles"`
	Clients         map[string]ClientBinding   `json:"clients"`
	Recommendations map[string]ClientSelection `json:"recommendations"`

	path   string
	before Snapshot
	data   []byte
}

type legacyConfig struct {
	Version           int                      `toml:"version"`
	Accounts          map[string]legacyAccount `toml:"accounts"`
	Profiles          map[string]legacyProfile `toml:"profiles"`
	Routes            map[string]string        `toml:"routes"`
	RecommendedRoutes map[string]string        `toml:"recommended_routes,omitempty"`
	Adapters          map[string]legacyAdapter `toml:"adapters,omitempty"`
}

type legacyAccount struct {
	Label        string          `toml:"label"`
	Endpoints    legacyEndpoints `toml:"endpoints"`
	AccountProbe *AccountProbe   `toml:"account_probe,omitempty"`
}

type legacyEndpoints struct {
	OpenAIResponses string `toml:"openai_responses,omitempty"`
	Anthropic       string `toml:"anthropic,omitempty"`
}

type legacyProfile struct {
	Label          string         `toml:"label"`
	Purpose        string         `toml:"purpose,omitempty"`
	Account        string         `toml:"account"`
	Client         string         `toml:"client"`
	Model          string         `toml:"model"`
	ModelProvider  string         `toml:"model_provider,omitempty"`
	Authentication Authentication `toml:"authentication,omitempty"`
}

type legacyAdapter struct {
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
		cfg.Accounts[id] = migratedAccount(account)
	}
	for _, id := range slices.Sorted(maps.Keys(legacy.Profiles)) {
		profile := legacy.Profiles[id]
		if !legacyClient(profile.Client) {
			return Config{}, fmt.Errorf("profile %q has unknown client %q", id, profile.Client)
		}
		account, exists := cfg.Accounts[profile.Account]
		if !exists {
			return Config{}, fmt.Errorf("profile %q references unknown account %q", id, profile.Account)
		}
		account.ID = profile.Account
		if _, _, err := mustClientSpec(profile.Client).ResolveEndpoint(account, ""); err != nil {
			return Config{}, fmt.Errorf("profile %q: %w", id, err)
		}
		cfg.Profiles[id] = Profile{Label: profile.Label, Purpose: profile.Purpose, Account: profile.Account, Model: profile.Model}
	}
	for client, profileID := range legacy.RecommendedRoutes {
		selection, err := migrateLegacySelection(legacy, client, profileID)
		if err != nil {
			return Config{}, fmt.Errorf("recommended route %q: %w", client, err)
		}
		cfg.Recommendations[client] = selection
	}
	for client, profileID := range legacy.Routes {
		selection, err := migrateLegacySelection(legacy, client, profileID)
		if err != nil {
			return Config{}, fmt.Errorf("route %q: %w", client, err)
		}
		adapter := legacy.Adapters[client]
		cfg.Clients[client] = ClientBinding{
			Profile: selection.Profile, Protocol: selection.Protocol,
			ModelProvider: selection.ModelProvider, Authentication: selection.Authentication,
			Enabled: adapter.Enabled, Executable: adapter.Executable,
			Targets: slices.Clone(adapter.Targets), CredentialCommand: adapter.CredentialCommand,
		}
	}
	for client, adapter := range legacy.Adapters {
		if !legacyClient(client) {
			return Config{}, fmt.Errorf("unknown adapter %q", client)
		}
		if _, selected := cfg.Clients[client]; selected {
			continue
		}
		if adapter.Enabled || adapter.Executable != "" || len(adapter.Targets) > 0 || adapter.CredentialCommand != "" {
			return Config{}, fmt.Errorf("adapter %q has client-specific options but no selected route", client)
		}
		cfg.Clients[client] = ClientBinding{}
	}
	for id, profile := range legacy.Profiles {
		if profile.ModelProvider == "" && profile.Authentication == "" {
			continue
		}
		if legacy.Routes[profile.Client] != id && legacy.RecommendedRoutes[profile.Client] != id {
			return Config{}, fmt.Errorf("profile %q has client-specific options without an active or recommended selection", id)
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("migrated configuration is invalid: %w", err)
	}
	return cfg, nil
}

func migrateLegacySelection(legacy legacyConfig, client, profileID string) (ClientSelection, error) {
	if !legacyClient(client) {
		return ClientSelection{}, fmt.Errorf("unknown client %q", client)
	}
	profile, exists := legacy.Profiles[profileID]
	if !exists {
		return ClientSelection{}, fmt.Errorf("references unknown profile %q", profileID)
	}
	if profile.Client != client {
		return ClientSelection{}, fmt.Errorf("profile %q is for %s, not %s", profileID, profile.Client, client)
	}
	account := migratedAccount(legacy.Accounts[profile.Account])
	account.ID = profile.Account
	_, protocol, err := mustClientSpec(client).ResolveEndpoint(account, "")
	if err != nil {
		return ClientSelection{}, err
	}
	return ClientSelection{
		Profile: profileID, Protocol: protocol,
		ModelProvider: profile.ModelProvider, Authentication: profile.Authentication,
	}, nil
}

func migratedAccount(account legacyAccount) Account {
	return Account{
		Label: account.Label,
		Endpoints: Endpoints{
			OpenAIResponses: account.Endpoints.OpenAIResponses,
			Anthropic:       account.Endpoints.Anthropic,
		},
		AccountProbe: account.AccountProbe,
	}
}

func legacyClient(client string) bool {
	return client == ClientClaude || client == ClientCodex
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
		Accounts: slices.Sorted(maps.Keys(cfg.Accounts)), Profiles: cfg.ProfileIDs(),
		Clients: maps.Clone(cfg.Clients), Recommendations: maps.Clone(cfg.Recommendations),
		path: path, before: before, data: slices.Clone(data),
	}
}
