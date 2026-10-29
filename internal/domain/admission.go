package domain

import (
	"errors"
	"fmt"
	"slices"
)

// Admission is the ordered set of client specifications that one
// configuration admits. The composition root builds it from the adapters'
// specifications, and a configuration reads its admitted clients only from it.
type Admission struct {
	specs []ClientSpec
}

// Validate reports why a specification cannot be admitted: it has no
// identifier, or it offers client-native authentication without a model
// provider, which that authentication needs and only a default admits.
func (s ClientSpec) Validate() error {
	if s.ID == "" {
		return errors.New("client specification has no identifier")
	}
	if s.ClientNativeAuthentication && s.DefaultModelProvider == "" {
		return fmt.Errorf("client %q has client-native authentication but no model provider", s.ID)
	}
	return nil
}

// NewAdmission admits specs in the given order. An invalid or repeated
// specification is a build defect, so it panics instead of admitting part of
// the set.
func NewAdmission(specs ...ClientSpec) Admission {
	admitted := make([]ClientSpec, 0, len(specs))
	for _, spec := range specs {
		if err := spec.Validate(); err != nil {
			panic(err.Error())
		}
		if slices.ContainsFunc(admitted, func(existing ClientSpec) bool { return existing.ID == spec.ID }) {
			panic(fmt.Sprintf("client %q is admitted more than once", spec.ID))
		}
		admitted = append(admitted, spec.clone())
	}
	return Admission{specs: admitted}
}

// IDs returns the admitted client identifiers in admission order.
func (admission Admission) IDs() []string {
	ids := make([]string, 0, len(admission.specs))
	for _, spec := range admission.specs {
		ids = append(ids, spec.ID)
	}
	return ids
}

// Specs returns an independent copy of every admitted specification in
// admission order.
func (admission Admission) Specs() []ClientSpec {
	specs := make([]ClientSpec, 0, len(admission.specs))
	for _, spec := range admission.specs {
		specs = append(specs, spec.clone())
	}
	return specs
}

// Spec resolves one admitted client specification by identifier.
func (admission Admission) Spec(id string) (ClientSpec, bool) {
	for _, spec := range admission.specs {
		if spec.ID == id {
			return spec.clone(), true
		}
	}
	return ClientSpec{}, false
}

// Admits reports whether an identifier names an admitted client.
func (admission Admission) Admits(id string) bool {
	_, admitted := admission.Spec(id)
	return admitted
}

// Usage renders the admitted client choices for CLI help and errors. Extra
// choices are command-local values such as "all".
func (admission Admission) Usage(extra ...string) string {
	return naturalChoices(append(admission.IDs(), extra...))
}

// usageWith renders, in admission order, the admitted clients that have a
// capability, so an error that names them stays true for any admission.
func (admission Admission) usageWith(capable func(ClientSpec) bool) string {
	var ids []string
	for _, spec := range admission.specs {
		if capable(spec) {
			ids = append(ids, spec.ID)
		}
	}
	return naturalChoices(ids)
}

// Label returns the presentation label that human output uses for an admitted
// client, or the identifier itself for any other value.
func (admission Admission) Label(id string) string {
	if spec, admitted := admission.Spec(id); admitted {
		return spec.Label
	}
	return id
}

// ProductName returns an admitted client's full product name, which is its
// label unless the two differ, or the identifier itself for any other value.
func (admission Admission) ProductName(id string) string {
	if spec, admitted := admission.Spec(id); admitted && spec.ProductName != "" {
		return spec.ProductName
	}
	return admission.Label(id)
}

func (s ClientSpec) clone() ClientSpec {
	s.EndpointProtocols = slices.Clone(s.EndpointProtocols)
	s.QualifiedModes = slices.Clone(s.QualifiedModes)
	s.QualifiedPlatforms = slices.Clone(s.QualifiedPlatforms)
	return s
}
