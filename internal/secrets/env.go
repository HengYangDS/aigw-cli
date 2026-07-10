package secrets

import (
	"regexp"
	"strings"
)

var nonEnv = regexp.MustCompile(`[^A-Za-z0-9]+`)

type EnvironmentStore struct{ getenv func(string) string }

func NewEnvironmentStore(getenv func(string) string) EnvironmentStore {
	return EnvironmentStore{getenv: getenv}
}

func EnvironmentKey(profile string) string {
	return "AIGW_TOKEN_" + strings.ToUpper(nonEnv.ReplaceAllString(profile, "_"))
}

func (s EnvironmentStore) Get(profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	value := s.getenv(EnvironmentKey(profile))
	if value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (EnvironmentStore) Set(profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	return ErrReadOnly
}

func (EnvironmentStore) Delete(profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	return ErrReadOnly
}

func (s EnvironmentStore) Has(profile string) bool {
	_, err := s.Get(profile)
	return err == nil
}
