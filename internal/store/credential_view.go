package store

import (
	"os"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"github.com/pelletier/go-toml/v2"
)

// credentialViewDocument is all of aigw.toml that the credential reader
// decodes: each provider's ID and endpoints, and each client's enabled state
// and authentication. The decoding is not strict, so the version and every
// other field, whatever its value, pass unread, as does an endpoint of a
// protocol that the reader does not know. A field of the view with a value of
// the wrong type is a parse failure.
type credentialViewDocument struct {
	Providers []struct {
		ID        string `toml:"id"`
		Endpoints struct {
			Anthropic string `toml:"anthropic"`
			Responses string `toml:"responses"`
			Chat      string `toml:"chat"`
		} `toml:"endpoints"`
	} `toml:"providers"`
	Clients map[string]struct {
		Enabled        bool   `toml:"enabled"`
		Authentication string `toml:"authentication"`
	} `toml:"clients"`
}

// LoadCredentialView reads the view of aigw.toml that the credential reader
// uses, which this release freezes: any other field can change without a
// reader noticing. The file is read as Load reads it, so an absent file is an
// empty view and a blocked one a read failure.
func (s Store) LoadCredentialView() (domain.CredentialView, error) {
	data, err := os.ReadFile(s.path)
	if failure := platform.ClassifyPath(s.path, platform.FileExpected, err); failure.Blocked() || failure.Absent() {
		if failure.Blocked() {
			return domain.CredentialView{}, newLoadError(LoadPhaseRead, failure)
		}
		return domain.CredentialView{}, nil
	}
	if err != nil {
		return domain.CredentialView{}, newLoadError(LoadPhaseRead, err)
	}
	var document credentialViewDocument
	if err := toml.Unmarshal(data, &document); err != nil {
		return domain.CredentialView{}, newLoadError(LoadPhaseParse, err)
	}
	view := domain.CredentialView{Clients: make(map[string]domain.CredentialClient, len(document.Clients))}
	for _, declared := range document.Providers {
		view.Providers = append(view.Providers, domain.CredentialProvider{ID: declared.ID, Endpoints: domain.Endpoints(declared.Endpoints)})
	}
	for client, declared := range document.Clients {
		view.Clients[client] = domain.CredentialClient{Enabled: declared.Enabled, Authentication: domain.Authentication(declared.Authentication)}
	}
	return view, nil
}
