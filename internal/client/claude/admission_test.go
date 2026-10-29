package claude

import configuration "aigw-cli/internal/configuration"

// testAdmission admits the built-in clients. This package's tests cannot take
// the composition root's admission, because the composition root imports it.
func testAdmission() configuration.Admission {
	return configuration.NewAdmission(configuration.AdmittedClientSpecs()...)
}
