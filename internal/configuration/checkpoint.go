package configuration

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

	"aigw-cli/internal/transaction"
)

// VerifiedCheckpoint is a secret-free record written only after all requested
// client protocol verifications succeed. It is suitable for rollback, not for
// credential recovery.
type VerifiedCheckpoint struct {
	Config     Config    `json:"config"`
	Clients    []string  `json:"clients"`
	VerifiedAt time.Time `json:"verified_at"`
}

// SaveVerifiedCheckpoint records a completed verification only while its
// configuration remains current. It serializes with AIGW mutations, preserves
// newer checkpoints, and compensates observed external configuration changes.
func (s Store) SaveVerifiedCheckpoint(ctx context.Context, cfg Config, clients []string) (resultErr error) {
	if err := validateCheckpointClients(clients); err != nil {
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
	configBefore, err := transaction.CaptureFileSnapshot(s.path)
	if err != nil {
		return err
	}
	if !configBefore.Exists {
		return errors.New("configuration is unavailable; verification checkpoint was not saved")
	}
	forwardingBefore, err := transaction.CaptureFileSnapshot(s.forwardingPath())
	if err != nil {
		return err
	}
	catalogBefore, err := transaction.CaptureFileSnapshot(s.catalogPath())
	if err != nil {
		return err
	}
	current, err := s.loadSnapshot(Snapshot{Config: configBefore, Forwarding: forwardingBefore, Catalog: catalogBefore})
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
	checkpointBefore, err := transaction.CaptureFileSnapshot(checkpointPath)
	if err != nil {
		return err
	}
	checkpoint := VerifiedCheckpoint{
		Config:     current,
		Clients:    append([]string(nil), clients...),
		VerifiedAt: time.Now().UTC(),
	}
	checkpoint.Config = checkpoint.Config.Clone()
	for id, account := range checkpoint.Config.Accounts {
		account.Order = 0
		checkpoint.Config.Accounts[id] = account
	}
	for id, model := range checkpoint.Config.Models {
		model.Order = 0
		checkpoint.Config.Models[id] = model
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
	configAfter, err := transaction.CaptureFileSnapshot(s.path)
	forwardingAfter, forwardingErr := transaction.CaptureFileSnapshot(s.forwardingPath())
	catalogAfter, catalogErr := transaction.CaptureFileSnapshot(s.catalogPath())
	err = errors.Join(err, forwardingErr, catalogErr)
	if err == nil && configBefore.Equal(configAfter) && forwardingBefore.Equal(forwardingAfter) && catalogBefore.Equal(catalogAfter) {
		return nil
	}
	if err == nil {
		err = errors.New("configuration changed during checkpoint commit; run `aigw verify --for all` again")
	}
	rollbackErr := transaction.RestoreFileAtomicIfPostimage(checkpointPath, checkpointBefore, checkpointAfter)
	return errors.Join(err, rollbackErr)
}

// LoadVerifiedCheckpoint restores verified routing with current presentation
// metadata for identities that still exist in the checkpoint.
func (s Store) LoadVerifiedCheckpoint() (VerifiedCheckpoint, error) {
	data, err := os.ReadFile(s.path + ".verified.json")
	if err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("read verified checkpoint: %w", err)
	}
	checkpoint, err := decodeVerifiedCheckpoint(data)
	if err != nil {
		return VerifiedCheckpoint{}, err
	}
	catalog, err := transaction.CaptureFileSnapshot(s.catalogPath())
	if err != nil {
		return VerifiedCheckpoint{}, err
	}
	checkpoint.Config, err = decodeCatalogOrder(checkpoint.Config, catalog)
	return checkpoint, err
}

func decodeVerifiedCheckpoint(data []byte) (VerifiedCheckpoint, error) {
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
	if err := validateCheckpointClients(checkpoint.Clients); err != nil {
		return VerifiedCheckpoint{}, err
	}
	checkpoint.Config.Normalize()
	if err := checkpoint.Config.Validate(); err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("validate verified checkpoint: %w", err)
	}
	return checkpoint, nil
}

func validateCheckpointClients(clients []string) error {
	if len(clients) == 0 {
		return errors.New("verified checkpoint is incomplete: no clients")
	}
	for index, client := range clients {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("verified checkpoint contains unknown client %q", client)
		}
		if slices.Contains(clients[:index], client) {
			return fmt.Errorf("verified checkpoint repeats client %q", client)
		}
	}
	return nil
}
