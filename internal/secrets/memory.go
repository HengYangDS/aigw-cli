package secrets

import "sync"

// memoryStore is a concurrent in-memory credential backend for tests and transient composition.
type memoryStore struct {
	mu     sync.RWMutex
	values map[Kind]map[string]string
}

// NewMemoryStore returns an empty in-memory backend with independent API-token and diagnostic namespaces.
func NewMemoryStore() Store {
	return scopedView{store: &memoryStore{values: map[Kind]map[string]string{
		APIToken: {}, ProviderDiagnostic: {},
	}}}
}

func (s *memoryStore) get(kind Kind, account string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[kind][account]
	if !ok || value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *memoryStore) set(kind Kind, account, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[kind][account] = value
	return nil
}

func (s *memoryStore) delete(kind Kind, account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values[kind], account)
	return nil
}

func (s *memoryStore) exists(kind Kind, account string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[kind][account]
	return ok && value != "", nil
}
