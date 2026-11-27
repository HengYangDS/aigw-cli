package domain

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// ValidIdentifier reports whether a user-defined identifier is safe for configuration and credential slots.
func ValidIdentifier(name string) bool { return identifierPattern.MatchString(name) }

// Normalize fills deterministic defaults and canonicalizes collection state in
// place: every collection exists, and the display order names every entry.
func (c *Config) Normalize() {
	if c.Providers == nil {
		c.Providers = map[string]Provider{}
	}
	if c.Families == nil {
		c.Families = map[string]Family{}
	}
	if c.Models == nil {
		c.Models = map[ModelRef]Model{}
	}
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientRecommendation{}
	}
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	c.normalizeOrder()
}

// Validate checks schema, providers, families, models, recommendations, then
// client bindings without mutation. Within each collection, a fixed order
// determines the first diagnostic.
func (c *Config) Validate() error {
	if err := c.validateCollections(); err != nil {
		return err
	}
	if err := c.validateRecommendations(); err != nil {
		return err
	}
	return c.validateClientBindings()
}

// validateCollections checks the version and each provider, family and
// model. A configuration may hold providers without models, or nothing at
// all: `aigw provider add` starts from an empty home, and removing the last
// model or provider leaves a valid configuration whose clients select
// nothing.
func (c *Config) validateCollections() error {
	if c.Version != ConfigVersion {
		return &UnsupportedConfigVersionError{Version: c.Version, ExpectedVersion: ConfigVersion}
	}
	names := slices.Sorted(maps.Keys(c.Providers))
	for index, name := range names {
		for _, earlier := range names[:index] {
			if SameIdentifier(earlier, name) {
				return NewEntryError("provider `"+name+"`", errCaseCollision(name, earlier))
			}
		}
	}
	for _, name := range names {
		if err := c.Providers[name].validate(name); err != nil {
			return NewEntryError("provider `"+name+"`", err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Families)) {
		if err := c.Families[name].validate(name); err != nil {
			return NewEntryError("family `"+name+"`", err)
		}
	}
	for _, ref := range c.ModelRefs() {
		if err := c.Models[ref].validate(ref, c.Providers); err != nil {
			return NewEntryError("model `"+ref.String()+"`", err)
		}
	}
	return c.validateModelLabels()
}

// validateModelLabels refuses two models of one provider whose labels,
// explicit or derived, differ at most in case: a client that lists one
// provider's models would show both under one name.
func (c *Config) validateModelLabels() error {
	seen := map[string]ModelRef{}
	for _, ref := range c.ModelRefs() {
		key := ref.Provider() + "\x00" + strings.ToLower(c.ModelLabel(ref))
		if earlier, ok := seen[key]; ok {
			return NewEntryError("model `"+ref.String()+"`", &SharedModelLabelError{Label: c.ModelLabel(ref), Models: [2]ModelRef{earlier, ref}})
		}
		seen[key] = ref
	}
	return nil
}

func (c *Config) validateRecommendations() error {
	for _, client := range slices.Sorted(maps.Keys(c.Recommendations)) {
		entry := "the `" + client + "` recommendation"
		if !c.admission.Admits(client) {
			return NewEntryError(entry, fmt.Errorf("unknown client recommendation %q; supported clients are %s", client, c.admission.Usage()))
		}
		recommendation := c.Recommendations[client]
		if recommendation.Primary.Model.IsZero() {
			return NewEntryError(entry, fmt.Errorf("client recommendation %q has no primary model", client))
		}
		for index, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
			// A recommended protocol that the provider has no endpoint for
			// waits for one: selection skips the choice until then.
			if err := c.validateSelection(client, selection); err != nil && !c.waitsForEndpoint(client, selection, err) {
				return NewEntryError(entry, fmt.Errorf("client recommendation %q choice %d: %w", client, index+1, err))
			}
			if c.Models[selection.Model].Deprecated {
				return NewEntryError(entry, fmt.Errorf("client recommendation %q choice %d names the deprecated model %s", client, index+1, selection.Model))
			}
		}
	}
	return nil
}

func (c *Config) validateClientBindings() error {
	for _, client := range slices.Sorted(maps.Keys(c.Clients)) {
		spec, admitted := c.admission.Spec(client)
		if !admitted {
			return fmt.Errorf("unknown client %q", client)
		}
		binding := c.clientBinding(client)
		if binding.Model.IsZero() {
			if binding.Enabled || binding.Protocol != "" || binding.ForwardingRoot != "" || binding.RetryPolicy != nil {
				return fmt.Errorf("client %q must select a model before it can be enabled or define runtime options", client)
			}
			if err := binding.validate(client, spec); err != nil {
				return err
			}
			continue
		}
		if err := c.validateSelection(client, binding.selection()); err != nil {
			return fmt.Errorf("client %q: %w", client, err)
		}
		if err := binding.validate(client, spec); err != nil {
			return err
		}
		if err := c.validateForwarding(client, binding); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateSelection(client string, selection ClientSelection) error {
	model, ok := c.Models[selection.Model]
	if !ok {
		return fmt.Errorf("names the unknown model %s", selection.Model)
	}
	provider := c.Providers[model.Provider]
	provider.ID = model.Provider
	spec, _ := c.admission.Spec(client)
	if _, _, err := spec.resolveModelEndpoint(provider, model, selection.Protocol); err != nil {
		return fmt.Errorf("model %s: %w", selection.Model, err)
	}
	return nil
}

// waitsForEndpoint reports a recommendation choice that fails only because
// its provider has no endpoint for its protocol, or, without a named
// protocol, for a protocol that both the client and the model speak.
func (c *Config) waitsForEndpoint(client string, selection ClientSelection, err error) bool {
	if _, missing := errors.AsType[*RuntimeMissingEndpointError](err); missing || err == nil {
		return missing
	}
	model, known := c.Models[selection.Model]
	spec, admitted := c.admission.Spec(client)
	if !known || !admitted || selection.Protocol != "" {
		return false
	}
	provider := c.Providers[model.Provider]
	return slices.ContainsFunc(spec.EndpointProtocols, func(protocol EndpointProtocol) bool {
		_, declared := model.Protocols[protocol]
		return declared && provider.Endpoints.For(protocol) == ""
	})
}

func (binding ClientBinding) validate(client string, spec ClientSpec) error {
	if err := binding.RetryPolicy.validate(client, spec); err != nil {
		return err
	}
	if binding.ForwardingRoot != "" {
		if err := ValidateForwardingRoot(binding.ForwardingRoot); err != nil {
			return fmt.Errorf("client %q forwarding: %w", client, err)
		}
	}
	command := binding.CredentialCommand
	if command != "" && (!filepath.IsAbs(command) || strings.TrimSpace(command) != command || strings.ContainsFunc(command, unicode.IsControl)) {
		return fmt.Errorf("client %q credential_command must be one absolute executable path", client)
	}
	return binding.validateProgram(client, spec)
}

// validateProgram checks the client's program declaration: at most one of
// program and extensions, each an absolute, clean path without a control
// character; extensions only for a client whose program an editor extension
// ships; and no program inside an installed version of the client's
// extension, which the editor's next version replaces, so the client declares
// that extensions directory instead (see ClientSpec.ExtensionsDirectoryOf).
// Whether the path exists is each operation's to decide, so that a missing
// program stops only its client.
func (binding ClientBinding) validateProgram(client string, spec ClientSpec) error {
	refuse := func(reason, change string) error {
		return &ProgramDeclarationError{Client: client, Reason: reason, Change: change}
	}
	if binding.Program != "" && binding.Extensions != "" {
		return refuse("declares both program and extensions", "keep one of `program` and `extensions`")
	}
	for _, declared := range []struct{ field, path string }{{"program", binding.Program}, {"extensions", binding.Extensions}} {
		if declared.path != "" && !cleanAbsolutePath(declared.path) {
			return refuse(declared.field+" must be an absolute, clean path without control characters",
				fmt.Sprintf("write `%s` as one", declared.field))
		}
	}
	if binding.Extensions != "" && spec.Extension.ID == "" {
		return refuse("extensions cannot be declared, since "+spec.Label+" has no editor extension",
			"replace `extensions` with `program`, the path of its program")
	}
	if directory, inside := spec.ExtensionsDirectoryOf(binding.Program); inside {
		return refuse("program lies in one installed version of its editor extension, which the next version replaces",
			fmt.Sprintf("replace `program` with `extensions = %q`", directory))
	}
	return nil
}

// cleanAbsolutePath reports whether path is absolute and clean, as
// [filepath.Clean] leaves it, and holds no control character.
func cleanAbsolutePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsFunc(path, unicode.IsControl)
}

func (provider Provider) validate(name string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid provider ID %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if name != strings.ToLower(name) {
		return fmt.Errorf("provider %q must be lowercase; use %q", name, strings.ToLower(name))
	}
	if strings.TrimSpace(provider.Label) == "" {
		return fmt.Errorf("provider %q has an empty label", name)
	}
	if provider.Endpoints.Responses == "" && provider.Endpoints.Anthropic == "" && provider.Endpoints.Chat == "" {
		return fmt.Errorf("provider %q must define at least one endpoint", name)
	}
	for _, endpoint := range []struct {
		protocol EndpointProtocol
		url      string
	}{
		{ProtocolAnthropic, provider.Endpoints.Anthropic},
		{ProtocolResponses, provider.Endpoints.Responses},
		{ProtocolChat, provider.Endpoints.Chat},
	} {
		if endpoint.url == "" {
			continue
		}
		if err := ValidateEndpoint(endpoint.url); err != nil {
			return fmt.Errorf("provider %q endpoint %s: %w", name, endpoint.protocol, err)
		}
	}
	if provider.BalanceProbe == nil {
		return nil
	}
	if !ValidIdentifier(provider.BalanceProbe.Kind) {
		return fmt.Errorf("provider %q has invalid balance probe kind %q", name, provider.BalanceProbe.Kind)
	}
	if err := ValidateEndpoint(provider.BalanceProbe.BaseURL); err != nil {
		return fmt.Errorf("provider %q balance probe: %w", name, err)
	}
	return nil
}

func (family Family) validate(name string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid model name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if strings.TrimSpace(family.Label) == "" {
		return fmt.Errorf("model %q has an empty label", name)
	}
	return nil
}

// validate checks one model, stored under ref, against the providers. A
// family that the model names needs an entry only for its label: without one,
// its ID labels it. A model may declare a protocol for which its provider has
// no endpoint, as a team file leaves one whose provider keeps its local
// endpoints: the protocol is unusable until the provider has the endpoint,
// because no client selection resolves on it. Its window is one that
// ValidateContextWindow admits.
func (model Model) validate(ref ModelRef, providers map[string]Provider) error {
	if model.Ref() != ref {
		return fmt.Errorf("model %s is stored as %s", model.Ref(), ref)
	}
	if _, ok := providers[model.Provider]; !ok {
		return fmt.Errorf("model %s names the unknown provider %q", ref, model.Provider)
	}
	if strings.TrimSpace(model.ID) == "" {
		return fmt.Errorf("provider %q has a model with an empty ID", model.Provider)
	}
	if model.Family != "" && !ValidIdentifier(model.Family) {
		return fmt.Errorf("model %s names the invalid family %q", ref, model.Family)
	}
	if len(model.Protocols) == 0 {
		return fmt.Errorf("model %s declares no protocol, so no client can use it", ref)
	}
	for _, protocol := range model.AdmittedProtocols() {
		if !slices.Contains(knownProtocols, protocol) {
			return fmt.Errorf("model %s has the unknown protocol %q", ref, protocol)
		}
		capabilities := model.Protocols[protocol]
		seenCapabilities := make(map[Capability]bool, len(capabilities))
		for _, capability := range capabilities {
			if !capability.Known() {
				return fmt.Errorf("model %s has the unknown capability %q on %s", ref, capability, protocol)
			}
			if seenCapabilities[capability] {
				return fmt.Errorf("model %s repeats the capability %q on %s", ref, capability, protocol)
			}
			if !protocol.Admits(capability) {
				return fmt.Errorf("model %s declares %q on %s, which only the %s protocol admits", ref, capability, protocol, ProtocolAnthropic)
			}
			seenCapabilities[capability] = true
		}
	}
	if err := ValidateContextWindow(model.Window()); err != nil {
		return fmt.Errorf("model %s: %w", ref, err)
	}
	return nil
}

// ValidateContextWindow checks a context window, which is a whole number of
// tokens from MinContextWindow to MaxContextWindow; a refusal names both.
func ValidateContextWindow(tokens int) error {
	if tokens < MinContextWindow || tokens > MaxContextWindow {
		return fmt.Errorf("a context window is a whole number of tokens from %d to %d, not %d", MinContextWindow, MaxContextWindow, tokens)
	}
	return nil
}

// capabilities are the capabilities that AIGW defines, in display order.
var capabilities = []Capability{
	CapabilityText, CapabilityReasoning, CapabilityStreaming, CapabilityTools,
	CapabilityStructuredOutput, CapabilityContinuation, CapabilityCompaction,
	CapabilityMultimodalInput, CapabilityToolSearch,
}

// Capabilities returns the capabilities that AIGW defines, in display order.
func Capabilities() []Capability { return slices.Clone(capabilities) }

// Known reports whether AIGW defines this capability.
func (capability Capability) Known() bool { return slices.Contains(capabilities, capability) }

// Admits reports whether protocol admits capability: tool search is an
// Anthropic Messages shape, and every other capability may be qualified on any
// protocol.
func (protocol EndpointProtocol) Admits(capability Capability) bool {
	return capability != CapabilityToolSearch || protocol == ProtocolAnthropic
}

// ValidateEndpoint checks a URL that a provider endpoint or a balance probe
// may use: HTTPS, or plain HTTP to a loopback host, without userinfo or a
// credential-like query parameter. Its errors name at most a query
// parameter, never a value from the URL.
func ValidateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errors.New("URL must use http or https and include a host")
	}
	if u.User != nil {
		return errors.New("URL userinfo is forbidden")
	}
	if u.Scheme == "http" && !IsLoopbackHost(u.Hostname()) {
		return errors.New("plain HTTP is allowed only for a loopback endpoint")
	}
	for _, key := range slices.Sorted(maps.Keys(u.Query())) {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key") || strings.Contains(lower, "auth") || strings.Contains(lower, "password") {
			return fmt.Errorf("credential-like query parameter %q is forbidden", key)
		}
	}
	return nil
}

// IsLoopbackHost classifies a URL hostname without DNS or service discovery.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

// errCaseCollision refuses a provider ID that differs only in case from an
// earlier one, which would share its Token slot on a case-insensitive store.
func errCaseCollision(name, other string) error {
	return fmt.Errorf("provider %q differs only in case from provider %q, and both would share one Token slot on a case-insensitive store", name, other)
}
