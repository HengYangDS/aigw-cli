package client

import configuration "aigw-cli/internal/configuration"

// builtinRegistry is the adapter set that the composition root registers,
// built here because these white-box tests reach unexported adapter types.
func builtinRegistry() Registry {
	return mustRegistry(configuration.AdmittedClientSpecs(), codexAdapter{}, claudeAdapter{}, claudeDesktopAdapter{}, hermesAdapter{})
}

func mustRegistry(specs []configuration.ClientSpec, adapters ...Adapter) Registry {
	registry, err := NewRegistry(specs, adapters...)
	if err != nil {
		panic(err)
	}
	return registry
}
