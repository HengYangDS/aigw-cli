//go:build client_acceptance

package main

import (
	"path/filepath"

	hermesconfig "aigw-cli/internal/hermes/configuration"
)

const nativeHermesPreferences = `model:
  temperature: 0.3
providers:
  personal:
    base_url: https://personal.test
terminal:
  backend: local
memory:
  enabled: false
plugins:
  enabled: []
security:
  allow_lazy_installs: false
updates:
  check: false
cli:
  expose_on_path: false
`

func (j *journeyFixture) prepareHermesHome() {
	j.testing.Helper()
	nativeHome, err := requiredClientInput("AIGW_ACCEPTANCE_HERMES_HOME", true)
	if err != nil {
		j.testing.Fatal(err)
	}
	home, cleanup, err := hermesconfig.CreateVerificationHome(filepath.Join(nativeHome, "config.yaml"))
	if err != nil {
		j.testing.Fatal(err)
	}
	j.testing.Cleanup(func() {
		if err := cleanup(); err != nil {
			j.testing.Error(err)
		}
	})
	j.setEnvironment("HERMES_HOME", home)
	j.setEnvironment("HERMES_DISABLE_LAZY_INSTALLS", "1")
}
