package client

import configuration "aigw-cli/internal/configuration"

// builtinRegistry is the part of the built-in adapter set that this package
// still owns; the complete set is the composition root's internal/clients.
func builtinRegistry() Registry {
	adapters := []Adapter{claudeDesktopAdapter{}, hermesAdapter{}}
	return mustRegistry(specsOf(adapters...), adapters...)
}

// specsOf returns the admission records that the given adapters implement.
func specsOf(adapters ...Adapter) []configuration.ClientSpec {
	specs := make([]configuration.ClientSpec, 0, len(adapters))
	for _, adapter := range adapters {
		specs = append(specs, adapter.Spec())
	}
	return specs
}

func mustRegistry(specs []configuration.ClientSpec, adapters ...Adapter) Registry {
	registry, err := NewRegistry(specs, adapters...)
	if err != nil {
		panic(err)
	}
	return registry
}
