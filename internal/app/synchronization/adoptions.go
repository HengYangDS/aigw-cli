package synchronization

import (
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// Adoption is a program that a converging command adopted for a client that
// declared none: discovery found it, and the configuration declares it as
// the client's program from then on.
type Adoption struct {
	Client, Program string
}

// Adoptions is what one invocation's convergence adopted: the programs that
// discovery found for clients that declared none, and, once a transition or
// its preview ran, those that it writes. The zero value is ready; a nil
// Adoptions records nothing.
type Adoptions struct {
	observed, written []Adoption
	preview           bool
}

// observe records the adoptions of a convergence from before to after: each
// client that before declares no program for and after does. A later
// adoption of the same client replaces an earlier one.
func (adoptions *Adoptions) observe(before, after domain.Config) {
	if adoptions == nil {
		return
	}
	for client, binding := range after.Clients {
		if before.Clients[client].DeclaresProgram() || binding.Program == "" {
			continue
		}
		adoptions.observed = slices.DeleteFunc(adoptions.observed, func(adoption Adoption) bool { return adoption.Client == client })
		adoptions.observed = append(adoptions.observed, Adoption{Client: client, Program: binding.Program})
	}
}

// settle records, of the observed adoptions, those that a transition from
// before to after, or with preview the preview of one, writes: after still
// declares the adopted program for a client that before declares none for. A
// later transition of the same invocation replaces it.
func (adoptions *Adoptions) settle(before, after domain.Config, preview bool) {
	if adoptions == nil {
		return
	}
	adoptions.written, adoptions.preview = nil, preview
	for _, adoption := range adoptions.observed {
		if !before.Clients[adoption.Client].DeclaresProgram() && after.Clients[adoption.Client].Program == adoption.Program {
			adoptions.written = append(adoptions.written, adoption)
		}
	}
	slices.SortFunc(adoptions.written, func(left, right Adoption) int {
		return slices.Index(after.Admission().IDs(), left.Client) - slices.Index(after.Admission().IDs(), right.Client)
	})
}

// Written returns the adoptions that the invocation's last transition, or the
// preview of it, writes, in admission order, and whether it was a preview.
func (adoptions *Adoptions) Written() ([]Adoption, bool) {
	if adoptions == nil {
		return nil, false
	}
	return adoptions.written, adoptions.preview
}

// AdoptProgram adopts, for a client of cfg that declares no program, the
// program that discovery finds as its declaration, and records the adoption
// for the invocation's report. A client that declares one, or of which
// discovery finds none, is returned as it is.
func (s Synchronizer) AdoptProgram(cfg domain.Config, client string) (domain.Config, error) {
	if cfg.Clients[client].DeclaresProgram() {
		return cfg, nil
	}
	discovered, err := s.discoveredResult(client)
	if err != nil {
		return domain.Config{}, err
	}
	program, _ := adapter.ClientProgram(cfg, client, discovered, s.Host)
	if program.Path == "" {
		return cfg, nil
	}
	adopted := cfg.Clone()
	adopted.DeclareProgram(client, program.Path, "")
	s.Adoptions.observe(cfg, adopted)
	return adopted, nil
}
