package contract

import configuration "aigw-cli/internal/configuration"

// testAdmission admits the built-in clients for the suite's own fixture, which
// binds a client that no built-in admission knows.
func testAdmission() configuration.Admission {
	return configuration.NewAdmission(configuration.AdmittedClientSpecs()...)
}
