package contract

import configuration "aigw-cli/internal/configuration"

// testAdmission admits the suite's own fixture client and the foreign client
// that its fixture configuration also binds.
func testAdmission() configuration.Admission {
	return configuration.NewAdmission(fixtureAdapter{}.Spec(), configuration.ClientSpec{ID: "foreign", Label: "Foreign"})
}
