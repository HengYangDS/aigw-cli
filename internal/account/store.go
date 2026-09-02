package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	configuration "aigw-cli/internal/configuration"
)

var ErrNotFound = errors.New("account probe credential not found")

type Credential struct {
	SystemToken string `json:"system_token"`
	UserID      string `json:"user_id"`
}

type Store interface {
	Get(profile string) (Credential, error)
	Set(profile string, credential Credential) error
	Delete(profile string) error
	Exists(profile string) (bool, error)
}

type stringStore interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
	Exists(string) (bool, error)
}

// BackendStore serializes provider-diagnostic credentials into one typed view
// of the same backend that owns API tokens.
type BackendStore struct {
	backend    stringStore
	isNotFound func(error) bool
}

func NewBackendStore(backend stringStore, isNotFound func(error) bool) BackendStore {
	return BackendStore{backend: backend, isNotFound: isNotFound}
}

func (store BackendStore) Get(profile string) (Credential, error) {
	value, err := store.backend.Get(profile)
	if err != nil && store.isNotFound != nil && store.isNotFound(err) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("read provider diagnostic credential: %w", err)
	}
	var credential Credential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return Credential{}, fmt.Errorf("parse provider diagnostic credential: %w", err)
	}
	if credential.SystemToken == "" || credential.UserID == "" {
		return Credential{}, ErrNotFound
	}
	return credential, nil
}

func (store BackendStore) Set(profile string, credential Credential) error {
	if !configuration.ValidProfileName(profile) {
		return fmt.Errorf("invalid profile name %q", profile)
	}
	if credential.SystemToken == "" || credential.UserID == "" {
		return errors.New("system token and user ID are required")
	}
	value, _ := json.Marshal(credential)
	return store.backend.Set(profile, string(value))
}

func (store BackendStore) Delete(profile string) error { return store.backend.Delete(profile) }

func (store BackendStore) Exists(profile string) (bool, error) {
	present, err := store.backend.Exists(profile)
	if err != nil && store.isNotFound != nil && store.isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("observe provider diagnostic credential: %w", err)
	}
	return present, nil
}

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

func (s *MemoryStore) Exists(profile string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	credential, ok := s.values[profile]
	return ok && credential.SystemToken != "" && credential.UserID != "", nil
}
