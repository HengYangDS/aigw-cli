package invocation

import (
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// adoptedProgram is a program that the invocation's convergence adopted, as
// its command's document lists it: the client and the program that the
// configuration now declares for it.
type adoptedProgram struct {
	Client  string `json:"client"`
	Program string `json:"program"`
}

// AdoptionReport is one invocation's report of the programs that it adopted
// for clients that declared none: the one place where every converging
// command says so, in its document as adopted_programs and in human output as
// a section after the command's own.
type AdoptionReport struct {
	adoptions *synchronization.Adoptions
	admission domain.Admission
	written   bool
}

// NewAdoptionReport reports what adoptions records for one invocation of a
// program that admits admission's clients.
func NewAdoptionReport(adoptions *synchronization.Adoptions, admission domain.Admission) *AdoptionReport {
	return &AdoptionReport{adoptions: adoptions, admission: admission}
}

// adopted are the adoptions that the invocation's last transition, or its
// preview, writes, and whether it was a preview.
func (report *AdoptionReport) adopted() ([]synchronization.Adoption, bool) {
	if report == nil {
		return nil, false
	}
	return report.adoptions.Written()
}

// Document is the field that the invocation adds to its command's document
// for the programs that it adopted, and no field when it adopted none.
func (report *AdoptionReport) Document() []terminal.DocumentField {
	adopted, _ := report.adopted()
	if len(adopted) == 0 {
		return nil
	}
	listed := make([]adoptedProgram, 0, len(adopted))
	for _, adoption := range adopted {
		listed = append(listed, adoptedProgram{Client: adoption.Client, Program: adoption.Program})
	}
	return []terminal.DocumentField{{Name: "adopted_programs", Value: listed}}
}

// Render names each adopted program once, after the command's own human
// output.
func (report *AdoptionReport) Render(r *terminal.Renderer) {
	adopted, preview := report.adopted()
	if len(adopted) == 0 || report.written {
		return
	}
	report.written = true
	r.Section("Adopted program")
	for _, adoption := range adopted {
		product := report.admission.ProductName(adoption.Client)
		sentence := "AIGW found " + product + " at " + adoption.Program + " and saved it as its program in aigw.toml."
		if preview {
			sentence = "A run would save " + adoption.Program + ", where AIGW found " + product + ", as its program in aigw.toml."
		}
		r.Text(sentence)
	}
}
