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
)

// VerifiedCheckpoint is a secret-free record written only after all requested
// client protocol verifications succeed. It is suitable for rollback, not for
// credential recovery.
type VerifiedCheckpoint struct {
	Config     domain.Config `json:"config"`
	Clients    []string      `json:"clients"`
	VerifiedAt time.Time     `json:"verified_at"`
}

// SaveVerifiedCheckpoint records a completed verification only while its
// configuration remains current. It serializes with AIGW mutations, preserves
// newer checkpoints, and compensates observed external configuration changes.
func (s Store) SaveVerifiedCheckpoint(ctx context.Context, cfg domain.Config, clients []string) (resultErr error) {
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
		return errors.New("configuration changed during verification; run `aigw verify --for all` again")
	}
	checkpointPath := s.path + ".verified.json"
	checkpointBefore, err := CaptureFileSnapshot(checkpointPath)
	if err != nil {
		return err
	}
	checkpoint := VerifiedCheckpoint{
		Config:     publishedCheckpointConfig(current),
		Clients:    append([]string(nil), clients...),
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
		err = errors.New("configuration changed during checkpoint commit; run `aigw verify --for all` again")
	}
	rollbackErr := RestoreFileAtomicIfPostimage(checkpointPath, checkpointBefore, checkpointAfter)
	return errors.Join(err, rollbackErr)
}

// publishedCheckpointConfig omits display order and later Route capabilities,
// which their components restore when the checkpoint is read.
func publishedCheckpointConfig(current domain.Config) domain.Config {
	cfg := current.Clone()
	for id, account := range cfg.Accounts {
		account.Order = 0
		cfg.Accounts[id] = account
	}
	for id, model := range cfg.Models {
		model.Order = 0
		cfg.Models[id] = model
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
	catalog, catalogErr := CaptureFileSnapshot(s.catalogPath())
	capabilities, capabilitiesErr := CaptureFileSnapshot(s.capabilitiesPath())
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

func validateCheckpointClients(admission domain.Admission, clients []string) error {
	if len(clients) == 0 {
		return errors.New("verified checkpoint is incomplete: no clients")
	}
	for index, client := range clients {
		if !admission.Admits(client) {
			return fmt.Errorf("verified checkpoint contains unknown client %q", client)
		}
		if slices.Contains(clients[:index], client) {
			return fmt.Errorf("verified checkpoint repeats client %q", client)
		}
	}
	return nil
}
