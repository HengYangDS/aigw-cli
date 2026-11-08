package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// VerifiedCheckpoint is the secret-free verified snapshot of a configuration
// that a check of every enabled client proved, with the depth that each
// client reached. It is suitable for rollback, not for credential recovery.
type VerifiedCheckpoint struct {
	Config     domain.Config    `json:"config"`
	Clients    []VerifiedClient `json:"clients"`
	VerifiedAt time.Time        `json:"verified_at"`
}

// VerifiedClient is one client that a verified snapshot covers and the depth
// of `aigw check` that it reached: client for a completed session, or less
// for a client that cannot run one, such as Claude Desktop.
type VerifiedClient struct {
	Client string `json:"client"`
	Depth  string `json:"depth"`
}

// verifiedDepths are the depths of `aigw check` that a snapshot records.
var verifiedDepths = []string{"local", "endpoint", "model", "client"}

// SaveVerifiedCheckpoint records a completed check only while its
// configuration remains current. It serializes with AIGW mutations, preserves
// newer checkpoints, and compensates observed external configuration changes.
func (s Store) SaveVerifiedCheckpoint(ctx context.Context, cfg domain.Config, clients []VerifiedClient) (resultErr error) {
	if err := validateCheckpointClients(s.admission, clients); err != nil {
		return err
	}
	_, err := encodeConfig(cfg)
	if err != nil {
		return err
	}
	unlock, err := s.Lock(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := unlock(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("release checkpoint lock: %w", err))
		}
	}()
	before, err := s.captureCurrent()
	if err != nil {
		return err
	}
	if !before.Config.Exists {
		return errors.New("configuration is unavailable; verification checkpoint was not saved")
	}
	current, err := s.loadSnapshot(before)
	if err != nil {
		return err
	}
	equal, err := configurationsEqual(cfg, current)
	if err != nil {
		return err
	}
	if !equal {
		return errors.New("configuration changed during the check; run `aigw check --depth client` again")
	}
	checkpointPath := s.path + ".verified.json"
	checkpointBefore, err := platform.CaptureFileSnapshot(checkpointPath)
	if err != nil {
		return err
	}
	checkpoint := VerifiedCheckpoint{
		Config:     publishedCheckpointConfig(current),
		Clients:    append([]VerifiedClient(nil), clients...),
		VerifiedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode verified checkpoint: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	checkpointAfter, err := writeConfigurationFileIfUnchanged(checkpointPath, checkpointBefore, append(data, '\n'), 0o600)
	if err != nil {
		return err
	}
	after, err := s.captureCurrent()
	if err == nil && before.sameCurrent(after) {
		return nil
	}
	if err == nil {
		err = errors.New("configuration changed while the verified snapshot was written; run `aigw check --depth client` again")
	}
	rollbackErr := platform.RestoreFileAtomicIfPostimage(checkpointPath, checkpointBefore, checkpointAfter)
	return errors.Join(err, rollbackErr)
}

// publishedCheckpointConfig omits display order and later Route capabilities,
// which their components restore when the checkpoint is read.
func publishedCheckpointConfig(current domain.Config) domain.Config {
	cfg := current.Clone()
	for id, account := range cfg.Providers {
		account.Order = 0
		cfg.Providers[id] = account
	}
	for id, model := range cfg.Families {
		model.Order = 0
		cfg.Families[id] = model
	}
	keepPublishedCapabilities(cfg)
	return cfg
}

// LoadVerifiedCheckpoint restores verified routing with current presentation
// metadata and recorded Route capabilities for identities that still exist in
// the checkpoint.
func (s Store) LoadVerifiedCheckpoint() (VerifiedCheckpoint, error) {
	data, err := os.ReadFile(s.path + ".verified.json")
	if err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("read verified checkpoint: %w", err)
	}
	checkpoint, err := decodeVerifiedCheckpoint(data, s.admission)
	if err != nil {
		return VerifiedCheckpoint{}, err
	}
	catalog, catalogErr := platform.CaptureFileSnapshot(s.componentPath(catalogComponent))
	capabilities, capabilitiesErr := platform.CaptureFileSnapshot(s.componentPath(capabilitiesComponent))
	if err = errors.Join(catalogErr, capabilitiesErr); err != nil {
		return VerifiedCheckpoint{}, err
	}
	checkpoint.Config, err = decodeComponents(checkpoint.Config, catalog, capabilities)
	return checkpoint, err
}

func decodeVerifiedCheckpoint(data []byte, admission domain.Admission) (VerifiedCheckpoint, error) {
	var checkpoint VerifiedCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&checkpoint); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("parse verified checkpoint: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return VerifiedCheckpoint{}, errors.New("parse verified checkpoint: expected one complete JSON document")
	}
	if checkpoint.VerifiedAt.IsZero() {
		return VerifiedCheckpoint{}, errors.New("verified checkpoint is incomplete")
	}
	if err := validateCheckpointClients(admission, checkpoint.Clients); err != nil {
		return VerifiedCheckpoint{}, err
	}
	checkpoint.Config.SetAdmission(admission)
	checkpoint.Config.Normalize()
	// The checkpoint keeps the shape a retained reader decodes, so it carries no
	// recorded upstream; each forwarded binding is bound to the upstream it
	// reaches in the checkpoint itself.
	if err := recordForwardingUpstreams(&checkpoint.Config); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("validate verified checkpoint: %w", err)
	}
	if err := checkpoint.Config.Validate(); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("validate verified checkpoint: %w", err)
	}
	return checkpoint, nil
}

func validateCheckpointClients(admission domain.Admission, clients []VerifiedClient) error {
	if len(clients) == 0 {
		return errors.New("verified checkpoint is incomplete: no clients")
	}
	for index, client := range clients {
		if !admission.Admits(client.Client) {
			return fmt.Errorf("verified checkpoint contains unknown client %q", client.Client)
		}
		if slices.ContainsFunc(clients[:index], func(earlier VerifiedClient) bool { return earlier.Client == client.Client }) {
			return fmt.Errorf("verified checkpoint repeats client %q", client.Client)
		}
		if !slices.Contains(verifiedDepths, client.Depth) {
			return fmt.Errorf("verified checkpoint records client %q at no depth of aigw check", client.Client)
		}
	}
	return nil
}
