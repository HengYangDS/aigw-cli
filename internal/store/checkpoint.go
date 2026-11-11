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
	Config     domain.Config
	Clients    []VerifiedClient
	VerifiedAt time.Time
}

// checkpointDocument is aigw.verified.json: the verified configuration as the
// exact aigw.toml text that it was, the clients and their depths, and the
// time of the check.
type checkpointDocument struct {
	Configuration string           `json:"configuration"`
	Clients       []VerifiedClient `json:"clients"`
	VerifiedAt    time.Time        `json:"verified_at"`
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
	data, err := encodeConfig(cfg)
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
	before, err := platform.CaptureFileSnapshot(s.path)
	if err != nil {
		return err
	}
	if !before.Exists {
		return errors.New("configuration is unavailable; verification checkpoint was not saved")
	}
	current, err := decodeTOMLConfig(before.Data, s.admission)
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
	checkpointPath := s.verifiedPath()
	checkpointBefore, err := platform.CaptureFileSnapshot(checkpointPath)
	if err != nil {
		return err
	}
	checkpoint := checkpointDocument{
		Configuration: string(data),
		Clients:       append([]VerifiedClient(nil), clients...),
		VerifiedAt:    time.Now().UTC(),
	}
	encoded, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode verified checkpoint: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	checkpointAfter, err := writeConfigurationFileIfUnchanged(checkpointPath, checkpointBefore, append(encoded, '\n'), 0o600)
	if err != nil {
		return err
	}
	after, err := platform.CaptureFileSnapshot(s.path)
	if err == nil && before.Equal(after) {
		return nil
	}
	if err == nil {
		err = errors.New("configuration changed while the verified snapshot was written; run `aigw check --depth client` again")
	}
	rollbackErr := platform.RestoreFileAtomicIfPostimage(checkpointPath, checkpointBefore, checkpointAfter)
	return errors.Join(err, rollbackErr)
}

// LoadVerifiedCheckpoint reads the verified snapshot, with the configuration
// that the check proved.
func (s Store) LoadVerifiedCheckpoint() (VerifiedCheckpoint, error) {
	data, err := os.ReadFile(s.verifiedPath())
	if err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("read verified checkpoint: %w", err)
	}
	return decodeVerifiedCheckpoint(data, s.admission)
}

func decodeVerifiedCheckpoint(data []byte, admission domain.Admission) (VerifiedCheckpoint, error) {
	var checkpoint checkpointDocument
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
	cfg, err := decodeTOMLConfig([]byte(checkpoint.Configuration), admission)
	if err != nil {
		return VerifiedCheckpoint{}, fmt.Errorf("validate verified checkpoint: %w", err)
	}
	return VerifiedCheckpoint{Config: cfg, Clients: checkpoint.Clients, VerifiedAt: checkpoint.VerifiedAt}, nil
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
