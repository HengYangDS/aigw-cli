package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
)

// DiagnosticCredential carries provider-owned values used only
// for optional Account diagnostics, independently of inference API Tokens.
type DiagnosticCredential struct {
	SystemToken string `json:"system_token"`
	UserID      string `json:"user_id"`
}

// DiagnosticCredentialStore reads and writes provider-diagnostic credentials
// belonging to Account identifiers in one selected backend.
type DiagnosticCredentialStore interface {
	// Get decodes both required fields; it does not authenticate with the Provider.
	// Missing storage and invalid stored data produce different errors.
	Get(account string) (DiagnosticCredential, error)
	Set(account string, credential DiagnosticCredential) error
	Delete(account string) error
	// Exists observes slot presence without reading or validating its contents.
	Exists(account string) (bool, error)
}

type diagnosticCredentialStore struct{ backend Store }

// NewDiagnosticCredentialStore adapts the selected backend's diagnostic slots
// to typed credentials without selecting a backend or accessing values.
func NewDiagnosticCredentialStore(backend Store) (DiagnosticCredentialStore, error) {
	view, err := ForKind(backend, ProviderDiagnostic)
	if err != nil {
		return nil, err
	}
	return diagnosticCredentialStore{backend: view}, nil
}

func (store diagnosticCredentialStore) Get(account string) (DiagnosticCredential, error) {
	value, err := store.backend.Get(account)
	if err != nil {
		return DiagnosticCredential{}, fmt.Errorf("read provider diagnostic credential: %w", err)
	}
	var credential DiagnosticCredential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return DiagnosticCredential{}, fmt.Errorf("parse provider diagnostic credential: %w", err)
	}
	if credential.SystemToken == "" || credential.UserID == "" {
		return DiagnosticCredential{}, errors.New("stored provider diagnostic credential requires a system token and user ID")
	}
	return credential, nil
}

func (store diagnosticCredentialStore) Set(account string, credential DiagnosticCredential) error {
	if credential.SystemToken == "" || credential.UserID == "" {
		return errors.New("system token and user ID are required")
	}
	value, _ := json.Marshal(credential)
	return store.backend.Set(account, string(value))
}

func (store diagnosticCredentialStore) Delete(account string) error {
	return store.backend.Delete(account)
}

func (store diagnosticCredentialStore) Exists(account string) (bool, error) {
	present, err := store.backend.Exists(account)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("observe provider diagnostic credential: %w", err)
	}
	return present, nil
}
