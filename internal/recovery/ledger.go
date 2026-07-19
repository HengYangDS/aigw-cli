package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const (
	airLedgerSchemaVersion = 1
	airSurfaceID           = "jetbrains-air-codex"

	AirRecoveryStatePrepared              = "prepared"
	AirRecoveryStateAwaitingHostRoundtrip = "awaiting-host-roundtrip"
	AirRecoveryStateQuarantined           = "quarantined"
	AirRecoveryStateSettled               = "settled"
)

var airCaseIDPattern = regexp.MustCompile(`^air-[0-9]{6}-[0-9a-f]{12}$`)

// Store owns the private, AIGW-local Air recovery journal and quarantine.
// The root is the platform data directory's recovery subdirectory.
type Store struct {
	root    string
	now     func() time.Time
	capture func(string) (transaction.FileSnapshot, error)
	write   func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error)
	remove  func(string, transaction.FileSnapshot) (transaction.FileSnapshot, error)
	restore func(string, transaction.FileSnapshot, transaction.FileSnapshot) error
}

// NewStore creates an Air recovery store without creating directories or
// files. Read-only planning therefore remains side-effect free.
func NewStore(root string) Store {
	return Store{
		root:    root,
		now:     time.Now,
		capture: transaction.CaptureFileSnapshot,
		write:   transaction.WriteFileAtomicIfUnchanged,
		remove:  transaction.RemoveFileIfUnchanged,
		restore: transaction.RestoreFileAtomicIfPostimage,
	}
}

type airLedger struct {
	SchemaVersion               int        `json:"schema_version"`
	SurfaceID                   string     `json:"surface_id"`
	RecoveryGeneration          int        `json:"recovery_generation"`
	CaseID                      string     `json:"case_id"`
	State                       string     `json:"state"`
	CreatedAt                   time.Time  `json:"created_at"`
	RecoveredAt                 *time.Time `json:"recovered_at,omitempty"`
	SettledAt                   *time.Time `json:"settled_at,omitempty"`
	ProjectionFingerprintSHA256 string     `json:"projection_fingerprint_sha256"`
	ConfigPreimageSHA256        string     `json:"config_preimage_sha256"`
	ConfigPreimageMode          uint32     `json:"config_preimage_mode"`
	CleanedPostimageSHA256      string     `json:"cleaned_postimage_sha256"`
	ObservedRoundtripSHA256     string     `json:"observed_roundtrip_sha256,omitempty"`
	QuarantineSHA256            string     `json:"quarantine_sha256"`
}

func (s Store) airLedgerPath() string {
	return filepath.Join(s.root, "air", "ledger.json")
}

func (s Store) airQuarantinePath(caseID string) string {
	return filepath.Join(s.root, "air", "quarantine", caseID, "config.toml")
}

func (s Store) loadAirLedger() (airLedger, bool, error) {
	ledger, snapshot, err := s.captureAirLedger()
	return ledger, snapshot.Exists, err
}

func (s Store) captureAirLedger() (airLedger, transaction.FileSnapshot, error) {
	snapshot, err := s.capture(s.airLedgerPath())
	if err != nil {
		return airLedger{}, transaction.FileSnapshot{}, errors.New("read Air recovery ledger")
	}
	if !snapshot.Exists {
		return airLedger{}, snapshot, nil
	}
	var ledger airLedger
	decoder := json.NewDecoder(bytes.NewReader(snapshot.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ledger); err != nil {
		return airLedger{}, transaction.FileSnapshot{}, errors.New("Air recovery ledger is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return airLedger{}, transaction.FileSnapshot{}, errors.New("Air recovery ledger has trailing content")
	}
	if err := validateAirLedger(ledger); err != nil {
		return airLedger{}, transaction.FileSnapshot{}, err
	}
	return ledger, snapshot, nil
}

func validateAirLedger(ledger airLedger) error {
	if ledger.SchemaVersion != airLedgerSchemaVersion || ledger.SurfaceID != airSurfaceID {
		return errors.New("Air recovery ledger has an unsupported identity")
	}
	if ledger.RecoveryGeneration < 1 || !airCaseIDPattern.MatchString(ledger.CaseID) {
		return errors.New("Air recovery ledger has an invalid case identity")
	}
	if isSHA256Hex(ledger.ConfigPreimageSHA256) && ledger.CaseID != airCaseID(ledger.RecoveryGeneration, ledger.ConfigPreimageSHA256) {
		return errors.New("Air recovery ledger case is not bound to its preimage")
	}
	switch ledger.State {
	case AirRecoveryStatePrepared, AirRecoveryStateAwaitingHostRoundtrip, AirRecoveryStateQuarantined, AirRecoveryStateSettled:
	default:
		return errors.New("Air recovery ledger has an unsupported state")
	}
	for _, digest := range []string{
		ledger.ProjectionFingerprintSHA256,
		ledger.ConfigPreimageSHA256,
		ledger.CleanedPostimageSHA256,
		ledger.QuarantineSHA256,
	} {
		if !isSHA256Hex(digest) {
			return errors.New("Air recovery ledger has an invalid digest")
		}
	}
	if ledger.ObservedRoundtripSHA256 != "" && !isSHA256Hex(ledger.ObservedRoundtripSHA256) {
		return errors.New("Air recovery ledger has an invalid roundtrip digest")
	}
	if ledger.CreatedAt.IsZero() {
		return errors.New("Air recovery ledger is missing its creation time")
	}
	return nil
}

func encodeAirLedger(ledger airLedger) ([]byte, error) {
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Air recovery ledger: %w", err)
	}
	return append(data, '\n'), nil
}

func desiredSnapshot(data []byte, mode os.FileMode) transaction.FileSnapshot {
	sum := sha256.Sum256(data)
	return transaction.FileSnapshot{
		Exists: true,
		Data:   append([]byte(nil), data...),
		SHA256: hex.EncodeToString(sum[:]),
		Mode:   mode,
	}
}

func airCaseID(generation int, preimageSHA256 string) string {
	prefix := preimageSHA256
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return fmt.Sprintf("air-%06d-%s", generation, prefix)
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
