package onboarding

import (
	"errors"
	"fmt"

	"aigw-cli/internal/cli/invocation"
)

type setupCredential struct {
	account     string
	token       string
	previous    string
	hadPrevious bool
	write       bool
}

func writeSetupCredentials(runtime invocation.Context, credentials []setupCredential) ([]int, error) {
	written := make([]int, 0, len(credentials))
	for index, credential := range credentials {
		if !credential.write {
			continue
		}
		if err := runtime.Secrets.Set(credential.account, credential.token); err != nil {
			if rollbackErr := rollbackSetupCredentials(runtime, credentials, written); rollbackErr != nil {
				return nil, fmt.Errorf("store Token for Account %q: %w; credential rollback also failed: %v", credential.account, err, rollbackErr)
			}
			return nil, fmt.Errorf("store Token for Account %q: %w", credential.account, err)
		}
		written = append(written, index)
	}
	return written, nil
}

func rollbackSetupCredentials(runtime invocation.Context, credentials []setupCredential, written []int) error {
	var rollbackErr error
	for position := len(written) - 1; position >= 0; position-- {
		credential := credentials[written[position]]
		var err error
		if credential.hadPrevious {
			err = runtime.Secrets.Set(credential.account, credential.previous)
		} else {
			err = runtime.Secrets.Delete(credential.account)
		}
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore Token for Account %q: %w", credential.account, err))
		}
	}
	return rollbackErr
}
