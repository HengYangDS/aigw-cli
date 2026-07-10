package secrets

import "sync"

type MemoryStore struct {
	mu     sync.RWMutex
	values map[string]string
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{values: map[string]string{}} }

func (s *MemoryStore) Get(profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[profile]
	if !ok || value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *MemoryStore) Set(profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[profile] = value
	return nil
}

func (s *MemoryStore) Delete(profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, profile)
	return nil
}

func (s *MemoryStore) Has(profile string) bool {
	_, err := s.Get(profile)
	return err == nil
}
