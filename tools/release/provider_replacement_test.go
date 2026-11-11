package main

import (
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

// replaceProvider moves every client that selects a model of provider old to
// provider replacement, as 0.7.0 does without a rename command: `aigw provider
// add` adds replacement with old's endpoints and a Token that it accepts,
// `aigw model add` adds each selected model with its family, protocol and
// capabilities, and `aigw use` moves each client to it. The caller removes old
// with `aigw provider remove` once it has checked the clients. input is the
// Token for --token-stdin, or empty for the read-only environment backend,
// whose variable supplies it.
func (j *journeyFixture) replaceProvider(old, replacement, input string) {
	j.testing.Helper()
	cfg, err := store.NewStore(j.config, builtinclients.Admission()).Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	add := []string{"provider", "add", replacement}
	for _, protocol := range []domain.EndpointProtocol{domain.ProtocolAnthropic, domain.ProtocolResponses, domain.ProtocolChat} {
		if endpoint := cfg.Providers[old].Endpoints.For(protocol); endpoint != "" {
			add = append(add, "--endpoint", protocol.Name()+"="+endpoint)
		}
	}
	if input != "" {
		add = append(add, "--token-stdin")
	}
	j.runWithInput(j.binary, input, add...)
	added := map[string]bool{}
	for _, client := range builtinclients.Admission().IDs() {
		model, selected := cfg.Models[cfg.SelectedModel(client)]
		if !selected || model.Provider != old {
			continue
		}
		protocol := cfg.Clients[client].Protocol
		if protocol == "" {
			if len(model.Protocols) != 1 {
				j.testing.Fatalf("%s's model %s has %d protocols and the client names none", client, model.Ref(), len(model.Protocols))
			}
			for only := range model.Protocols {
				protocol = only
			}
		}
		ref := replacement + "/" + model.UpstreamModelID()
		if !added[ref] {
			args := []string{"model", "add", ref, "--family", model.Family, "--protocol", protocol.Name(), "--unlisted"}
			for _, capability := range model.Protocols[protocol] {
				args = append(args, "--capability", string(capability))
			}
			j.run(args...)
			added[ref] = true
		}
		j.run("use", client, ref, "--protocol", protocol.Name())
	}
	if len(added) == 0 {
		j.testing.Fatalf("no client selects a model of provider %s", old)
	}
}
