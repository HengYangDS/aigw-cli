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
	"runtime"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const (
	airLedgerSchemaVersion   = 1
	airSurfaceID             = "jetbrains-air-codex"
	maxAirRecoveryGeneration = 999_999

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

type airRecoveryStorageInventory struct {
	ledgerExists      bool
	permissionInvalid bool
	unsafeTraversal   bool
	quarantineEntries []airRecoveryQuarantineEntry
}

type airRecoveryQuarantineEntry struct {
	name      string
	directory bool
	files     []airRecoveryQuarantineFile
}

type airRecoveryQuarantineFile struct {
	name    string
	regular bool
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
	if runtime.GOOS != "windows" && snapshot.Mode.Perm() != 0o600 {
		return airLedger{}, transaction.FileSnapshot{}, errors.New("Air recovery ledger has unsafe permissions")
	}
	ledger, err := decodeAirLedger(snapshot.Data)
	if err != nil {
		return airLedger{}, transaction.FileSnapshot{}, err
	}
	return ledger, snapshot, nil
}

func decodeAirLedger(data []byte) (airLedger, error) {
	var ledger airLedger
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ledger); err != nil {
		return airLedger{}, errors.New("Air recovery ledger is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return airLedger{}, errors.New("Air recovery ledger has trailing content")
	}
	if err := validateAirLedger(ledger); err != nil {
		return airLedger{}, err
	}
	return ledger, nil
}

func (s Store) validateAirRecoveryStorage(caseID string, ledger, quarantine transaction.FileSnapshot) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if ledger.Exists && ledger.Mode.Perm() != 0o600 {
		return errors.New("Air recovery ledger has unsafe permissions")
	}
	if !quarantine.Exists || quarantine.Mode.Perm() != 0o600 {
		return errors.New("Air recovery quarantine has unsafe permissions")
	}
	return s.validateAirRecoveryDirectories(caseID)
}

func (s Store) validateAirRecoveryDirectories(caseID string) error {
	for _, path := range []string{
		s.root,
		filepath.Join(s.root, "air"),
		filepath.Join(s.root, "air", "quarantine"),
		filepath.Dir(s.airQuarantinePath(caseID)),
	} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o700) {
			return errors.New("Air recovery storage has unsafe directory permissions")
		}
	}
	return nil
}

func (s Store) inspectAirRecoveryStorage() (airRecoveryStorageInventory, error) {
	var inventory airRecoveryStorageInventory
	rootExists, rootSafe, err := inspectAirRecoveryDirectory(s.root)
	if err != nil || !rootExists {
		return inventory, err
	}
	if !rootSafe {
		inventory.permissionInvalid = true
		inventory.unsafeTraversal = true
		return inventory, nil
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Lstat(s.root); err != nil || info.Mode().Perm() != 0o700 {
			inventory.permissionInvalid = true
		}
	}

	airRoot := filepath.Join(s.root, "air")
	airExists, airSafe, err := inspectAirRecoveryDirectory(airRoot)
	if err != nil || !airExists {
		return inventory, err
	}
	if !airSafe {
		inventory.permissionInvalid = true
		inventory.unsafeTraversal = true
		return inventory, nil
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Lstat(airRoot); err != nil || info.Mode().Perm() != 0o700 {
			inventory.permissionInvalid = true
		}
	}

	ledgerInfo, err := os.Lstat(s.airLedgerPath())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return inventory, err
	case ledgerInfo.Mode()&os.ModeSymlink != 0 || !ledgerInfo.Mode().IsRegular():
		inventory.ledgerExists = true
		inventory.permissionInvalid = true
		inventory.unsafeTraversal = true
		return inventory, nil
	default:
		inventory.ledgerExists = true
	}

	quarantineRoot := filepath.Join(airRoot, "quarantine")
	quarantineExists, quarantineSafe, err := inspectAirRecoveryDirectory(quarantineRoot)
	if err != nil || !quarantineExists {
		return inventory, err
	}
	if !quarantineSafe {
		inventory.permissionInvalid = true
		inventory.unsafeTraversal = true
		return inventory, nil
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Lstat(quarantineRoot); err != nil || info.Mode().Perm() != 0o700 {
			inventory.permissionInvalid = true
		}
	}

	entries, err := os.ReadDir(quarantineRoot)
	if err != nil {
		return inventory, err
	}
	for _, entry := range entries {
		path := filepath.Join(quarantineRoot, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return inventory, err
		}
		item := airRecoveryQuarantineEntry{name: entry.Name()}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if info.Mode()&os.ModeSymlink != 0 {
				inventory.permissionInvalid = true
			}
			inventory.quarantineEntries = append(inventory.quarantineEntries, item)
			continue
		}
		item.directory = true
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
			inventory.permissionInvalid = true
		}
		files, err := os.ReadDir(path)
		if err != nil {
			return inventory, err
		}
		for _, file := range files {
			fileInfo, err := os.Lstat(filepath.Join(path, file.Name()))
			if err != nil {
				return inventory, err
			}
			item.files = append(item.files, airRecoveryQuarantineFile{
				name:    file.Name(),
				regular: fileInfo.Mode()&os.ModeSymlink == 0 && fileInfo.Mode().IsRegular(),
			})
		}
		inventory.quarantineEntries = append(inventory.quarantineEntries, item)
	}
	return inventory, nil
}

func inspectAirRecoveryDirectory(path string) (bool, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, true, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, info.Mode()&os.ModeSymlink == 0 && info.IsDir(), nil
}

func (inventory airRecoveryStorageInventory) hasQuarantineArtifacts() bool {
	return len(inventory.quarantineEntries) != 0
}

func (inventory airRecoveryStorageInventory) quarantineFile(caseID, name string) (bool, bool) {
	for _, entry := range inventory.quarantineEntries {
		if entry.name != caseID || !entry.directory {
			continue
		}
		for _, file := range entry.files {
			if file.name == name {
				return true, file.regular
			}
		}
	}
	return false, false
}

func (inventory airRecoveryStorageInventory) hasUnexpectedQuarantine(caseID string, expectConfig bool) bool {
	for _, entry := range inventory.quarantineEntries {
		if entry.name != caseID || !entry.directory {
			return true
		}
		for _, file := range entry.files {
			if !expectConfig || file.name != "config.toml" || !file.regular {
				return true
			}
		}
	}
	return false
}

func validateAirLedger(ledger airLedger) error {
	if ledger.SchemaVersion != airLedgerSchemaVersion || ledger.SurfaceID != airSurfaceID {
		return errors.New("Air recovery ledger has an unsupported identity")
	}
	if ledger.RecoveryGeneration < 1 || ledger.RecoveryGeneration > maxAirRecoveryGeneration || !airCaseIDPattern.MatchString(ledger.CaseID) {
		return errors.New("Air recovery ledger has an invalid case identity")
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
	if ledger.CaseID != airCaseID(ledger.RecoveryGeneration, ledger.ConfigPreimageSHA256) {
		return errors.New("Air recovery ledger case is not bound to its preimage")
	}
	if ledger.QuarantineSHA256 != ledger.ConfigPreimageSHA256 {
		return errors.New("Air recovery ledger quarantine is not bound to its preimage")
	}
	if ledger.CleanedPostimageSHA256 == ledger.ConfigPreimageSHA256 {
		return errors.New("Air recovery ledger cleanup did not change the preimage")
	}
	if ledger.ConfigPreimageMode > uint32(os.ModePerm) {
		return errors.New("Air recovery ledger has an invalid config mode")
	}
	if ledger.ObservedRoundtripSHA256 != "" && !isSHA256Hex(ledger.ObservedRoundtripSHA256) {
		return errors.New("Air recovery ledger has an invalid roundtrip digest")
	}
	if ledger.CreatedAt.IsZero() {
		return errors.New("Air recovery ledger is missing its creation time")
	}
	return validateAirLedgerState(ledger)
}

func validateAirLedgerState(ledger airLedger) error {
	recovered := ledger.RecoveredAt
	settled := ledger.SettledAt
	if recovered != nil && (recovered.IsZero() || recovered.Before(ledger.CreatedAt)) {
		return errors.New("Air recovery ledger has an invalid recovered time")
	}
	if settled != nil && (settled.IsZero() || recovered == nil || settled.Before(*recovered)) {
		return errors.New("Air recovery ledger has an invalid settled time")
	}
	switch ledger.State {
	case AirRecoveryStatePrepared:
		if recovered != nil || settled != nil || ledger.ObservedRoundtripSHA256 != "" {
			return errors.New("prepared Air recovery ledger contains later-state fields")
		}
	case AirRecoveryStateAwaitingHostRoundtrip:
		if recovered == nil || settled != nil || ledger.ObservedRoundtripSHA256 != "" {
			return errors.New("awaiting Air recovery ledger has inconsistent lifecycle fields")
		}
	case AirRecoveryStateQuarantined:
		if recovered == nil || settled != nil || ledger.ObservedRoundtripSHA256 == "" || ledger.ObservedRoundtripSHA256 == ledger.CleanedPostimageSHA256 {
			return errors.New("quarantined Air recovery ledger has inconsistent lifecycle fields")
		}
	case AirRecoveryStateSettled:
		if recovered == nil || settled == nil || ledger.ObservedRoundtripSHA256 == "" || ledger.ObservedRoundtripSHA256 == ledger.CleanedPostimageSHA256 {
			return errors.New("settled Air recovery ledger has inconsistent lifecycle fields")
		}
	default:
		return errors.New("Air recovery ledger has an unsupported state")
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
