package contract

import "aigw-cli/internal/domain"

// testAdmission admits the suite's own fixture client and the foreign client
// that its fixture configuration also binds.
func testAdmission() domain.Admission {
	return domain.NewAdmission(fixtureAdapter{}.Spec(), domain.ClientSpec{ID: "foreign", Label: "Foreign"})
}
