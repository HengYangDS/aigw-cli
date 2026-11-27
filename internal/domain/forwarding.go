package domain

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// A forwarding service, such as openai-responses-proxy 8.0.0, knows no
// provider: the path of a request names the upstream as
// <root>/<scheme>/<authority>/<base path>. It relays the Responses protocol
// only, and refuses a path outside its grammar. One upstream has one
// spelling, because the service keeps one cooldown for each spelling: the
// host in lower case and no default port.

// forwardingLoopbackHosts are the hosts, as the forwarding service spells
// them, to which an upstream may use plain HTTP; the service admits no other
// spelling of a loopback address.
var forwardingLoopbackHosts = []string{"127.0.0.1", "localhost", "::1"}

// defaultPorts are the ports that an upstream's authority leaves out.
var defaultPorts = map[string]string{"https": "443", "http": "80"}

// ValidateForwardingRoot checks the root of a forwarding service: an HTTPS
// URL, or a plain HTTP URL of a loopback host, without user information, a
// query or a fragment. Its errors name no value from the URL.
func ValidateForwardingRoot(root string) error {
	u, err := url.Parse(root)
	switch {
	case err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "":
		return errors.New("the root must be an http or https URL with a host")
	case u.User != nil:
		return errors.New("the root must not carry user information")
	case u.RawQuery != "" || u.ForceQuery:
		return errors.New("the root must not carry a query")
	case u.Fragment != "" || strings.Contains(root, "#"):
		return errors.New("the root must not carry a fragment")
	case u.Scheme == "http" && !IsLoopbackHost(u.Hostname()):
		return errors.New("plain HTTP is allowed only for a loopback root")
	}
	return nil
}

// forwardingDestination composes the address through which the forwarding
// service at root reaches upstream, a provider's Responses endpoint:
// <root>/<scheme>/<authority>/<base path>, as upstreamAuthority and
// upstreamBasePath spell them. An upstream with user information, a query or
// a fragment, or one that they refuse, is refused, as the service would
// refuse it. The refusal names no value from the URL.
func forwardingDestination(root, upstream string) (string, error) {
	u, err := url.Parse(upstream)
	if err != nil || u.Opaque != "" || u.Host == "" {
		return "", errors.New("is not a URL with a host")
	}
	switch {
	case u.Scheme != "https" && u.Scheme != "http":
		return "", errors.New("uses neither https nor http")
	case u.User != nil:
		return "", errors.New("carries user information")
	case u.RawQuery != "" || u.ForceQuery:
		return "", errors.New("carries a query")
	case u.Fragment != "" || strings.Contains(upstream, "#"):
		return "", errors.New("carries a fragment")
	}
	authority, err := upstreamAuthority(u)
	if err != nil {
		return "", err
	}
	base, err := upstreamBasePath(u)
	if err != nil {
		return "", err
	}
	return strings.Join(slices.Concat([]string{strings.TrimRight(root, "/"), u.Scheme, authority}, base), "/"), nil
}

// upstreamAuthority spells the authority of u as the forwarding grammar
// does: the host in lower case, an IPv6 address in brackets, and the port in
// decimal unless it is the scheme's default. The host must already be in its
// URL-safe form, ASCII letters, digits and "-._~" (and the colons of an IPv6
// address), so a non-ASCII host is refused whether the URL writes it raw or
// percent-encoded, as is an IPv6 zone. A port above 65535 is refused, and so
// is plain HTTP to any host but 127.0.0.1, localhost and ::1.
func upstreamAuthority(u *url.URL) (string, error) {
	host := strings.ToLower(u.Hostname())
	bracketed := strings.HasPrefix(u.Host, "[")
	if host == "" {
		return "", errors.New("is not a URL with a host")
	}
	if strings.ContainsFunc(host, func(r rune) bool { return !isUnreserved(r) && (r != ':' || !bracketed) }) {
		return "", errors.New(`has a host with a character other than an ASCII letter, a digit, "-", ".", "_" or "~"`)
	}
	if u.Scheme == "http" && !slices.Contains(forwardingLoopbackHosts, host) {
		return "", errors.New("uses plain HTTP to a host other than 127.0.0.1, localhost or ::1")
	}
	authority := host
	if bracketed {
		authority = "[" + host + "]"
	}
	if port := u.Port(); port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return "", errors.New("has a port above 65535")
		}
		if decimal := strconv.FormatUint(number, 10); decimal != defaultPorts[u.Scheme] {
			authority += ":" + decimal
		}
	}
	return authority, nil
}

// upstreamBasePath is the segments of u's path without its trailing slash.
// Every segment must be non-empty, not a dot segment, and in its URL-safe
// form, ASCII letters, digits and "-._~", so an empty segment anywhere, as in
// "//", and a percent-encoded or other reserved character are refused, as
// the forwarding service refuses them.
func upstreamBasePath(u *url.URL) ([]string, error) {
	path := u.EscapedPath()
	if path == "" {
		return nil, nil
	}
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if segments[len(segments)-1] == "" {
		segments = segments[:len(segments)-1]
	}
	for _, segment := range segments {
		switch {
		case segment == "" || segment == "." || segment == "..":
			return nil, errors.New("has an empty or dot path segment")
		case strings.ContainsFunc(segment, func(r rune) bool { return !isUnreserved(r) }):
			return nil, errors.New(`has a path segment with a character other than an ASCII letter, a digit, "-", ".", "_" or "~", such as percent-encoding`)
		}
	}
	return segments, nil
}

// isUnreserved reports an unreserved character of RFC 3986, the only
// characters that a URL-safe host name or path segment holds.
func isUnreserved(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || strings.ContainsRune("-._~", r)
}

// ForwardingUpstreamError refuses a client's forwarding that cannot carry its
// traffic for a provider: the forwarding service relays only the Responses
// protocol, to an upstream that its grammar names. Err says why the grammar
// cannot name the provider's Responses endpoint; without it, the selection
// uses Protocol, another protocol than Responses. The fix connects the
// client to its provider directly, or, when Requested says that the command
// sets this forwarding, selects a model that it can carry first.
type ForwardingUpstreamError struct {
	Client    string
	Provider  string
	Protocol  EndpointProtocol
	Err       error
	Requested bool
}

// Error names the client, the provider and the reason, and the command that
// fixes it. It names no URL.
func (e *ForwardingUpstreamError) Error() string {
	reason := fmt.Sprintf("the forwarding service relays only %s, and the selection uses %s", ProtocolLabel(ProtocolResponses), ProtocolLabel(e.Protocol))
	if e.Err != nil {
		reason = "the forwarding service cannot name the provider's " + ProtocolLabel(ProtocolResponses) + " endpoint, which " + e.Err.Error()
	}
	fix := fmt.Sprintf("run `aigw client edit %s --direct` to connect it to the provider directly", e.Client)
	if e.Requested {
		fix = fmt.Sprintf("first select a %s model that it can carry with `aigw use %s <provider/model>`, then set the forwarding again",
			ProtocolLabel(ProtocolResponses), e.Client)
	}
	return fmt.Sprintf("client %q cannot forward its traffic for provider %q: %s; %s", e.Client, e.Provider, reason, fix)
}

// Unwrap returns why the grammar cannot name the upstream.
func (e *ForwardingUpstreamError) Unwrap() error { return e.Err }

// forwardingFor returns the destination through which a client's forwarding
// sends traffic for provider on protocol to upstream: none without a
// forwarding root, or on a protocol other than Responses, which the
// forwarding service does not relay. An upstream that its grammar cannot
// name is refused for the provider of the client's selection, which the
// forwarding was set to carry. Any other provider, which the client reaches
// when it switches models within a session or a check names one of its
// models, is reached directly then: an entry that the operator did not
// select never refuses a command.
func (c *Config) forwardingFor(client, provider, upstream string, protocol EndpointProtocol) (string, error) {
	binding := c.Clients[client]
	if binding.ForwardingRoot == "" || protocol != ProtocolResponses {
		return "", nil
	}
	destination, err := forwardingDestination(binding.ForwardingRoot, upstream)
	switch {
	case err == nil:
		return destination, nil
	case provider != c.Models[binding.Model].Provider:
		return "", nil
	}
	return "", &ForwardingUpstreamError{Client: client, Provider: provider, Protocol: protocol, Err: err}
}

// validateForwarding refuses a forwarded binding that its forwarding cannot
// carry: a selection on another protocol than Responses, or one whose
// upstream the grammar cannot name. Its retry budgets stay with the direct
// connection, which reaches the same provider.
func (c *Config) validateForwarding(client string, binding ClientBinding) error {
	if binding.ForwardingRoot == "" {
		return nil
	}
	runtime, err := c.ResolveRuntime(client, ModelRef{})
	if err != nil {
		return err
	}
	if runtime.Protocol != ProtocolResponses {
		return &ForwardingUpstreamError{Client: client, Provider: runtime.ProviderID, Protocol: runtime.Protocol}
	}
	return nil
}
