package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	configuration "aigw-cli/internal/configuration"
	keyring "github.com/zalando/go-keyring"
)

const Service = "AIGW_ACCOUNT"

var ErrNotFound = errors.New("account probe credential not found")

type Credential struct {
	SystemToken string `json:"system_token"`
	UserID      string `json:"user_id"`
}

type Store interface {
	Get(profile string) (Credential, error)
	Set(profile string, credential Credential) error
	Delete(profile string) error
	Has(profile string) bool
}

type KeyringStore struct{}

func NewKeyringStore() KeyringStore { return KeyringStore{} }

func (KeyringStore) Get(profile string) (Credential, error) {
	if !configuration.ValidProfileName(profile) {
		return Credential{}, fmt.Errorf("invalid profile name %q", profile)
	}
	value, err := keyring.Get(Service, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("read account probe credential: %w", err)
	}
	var credential Credential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return Credential{}, fmt.Errorf("parse account probe credential: %w", err)
	}
	if credential.SystemToken == "" || credential.UserID == "" {
		return Credential{}, ErrNotFound
	}
	return credential, nil
}

func (KeyringStore) Set(profile string, credential Credential) error {
	if !configuration.ValidProfileName(profile) {
		return fmt.Errorf("invalid profile name %q", profile)
	}
	if credential.SystemToken == "" || credential.UserID == "" {
		return errors.New("system token and user ID are required")
	}
	// Credential contains only strings, so encoding cannot fail. Keeping a
	// synthetic error branch here would describe an impossible product state.
	data, _ := json.Marshal(credential)
	if err := keyring.Set(Service, profile, string(data)); err != nil {
		return fmt.Errorf("write account probe credential: %w", err)
	}
	return nil
}

func (KeyringStore) Delete(profile string) error {
	err := keyring.Delete(Service, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (s KeyringStore) Has(profile string) bool { _, err := s.Get(profile); return err == nil }

type MemoryStore struct {
	mu     sync.RWMutex
	values map[string]Credential
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{values: map[string]Credential{}} }

func (s *MemoryStore) Get(profile string) (Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[profile]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return value, nil
}

func (s *MemoryStore) Set(profile string, credential Credential) error {
	if !configuration.ValidProfileName(profile) || credential.SystemToken == "" || credential.UserID == "" {
		return errors.New("valid profile, system token and user ID are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[profile] = credential
	return nil
}

func (s *MemoryStore) Delete(profile string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, profile)
	return nil
}

func (s *MemoryStore) Has(profile string) bool { _, err := s.Get(profile); return err == nil }
