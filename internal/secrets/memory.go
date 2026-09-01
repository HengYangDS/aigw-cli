package secrets

import "sync"

type MemoryStore struct {
	mu     sync.RWMutex
	values map[Kind]map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{values: map[Kind]map[string]string{
		APIToken: {}, ProviderDiagnostic: {},
	}}
}

func (s *MemoryStore) Get(profile string) (string, error) {
	return s.get(APIToken, profile)
}

func (s *MemoryStore) get(kind Kind, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[kind][profile]
	if !ok || value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *MemoryStore) Set(profile, value string) error {
	return s.set(APIToken, profile, value)
}

func (s *MemoryStore) set(kind Kind, profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[kind][profile] = value
	return nil
}

func (s *MemoryStore) Delete(profile string) error {
	return s.delete(APIToken, profile)
}

func (s *MemoryStore) delete(kind Kind, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values[kind], profile)
	return nil
}

func (s *MemoryStore) Exists(profile string) (bool, error) {
	return s.exists(APIToken, profile)
}

func (s *MemoryStore) exists(kind Kind, profile string) (bool, error) {
	if err := validate(profile, "", false); err != nil {
		return false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[kind][profile]
	return ok && value != "", nil
}
