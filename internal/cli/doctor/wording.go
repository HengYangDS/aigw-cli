package doctor

import (
	"strings"

	configuration "aigw-cli/internal/configuration"
)

// Label maps a diagnostic to a stable human label. A check about one native
// file under the home directory names that file.
func Label(check Check) string {
	name := check.Name
	if clientID, ok := strings.CutPrefix(name, "adapter:"); ok {
		if spec, admitted := configuration.ClientSpecFor(clientID); admitted {
			return spec.Label + " adapter"
		}
	}
	if clientID, ok := strings.CutPrefix(name, "projection:"); ok {
		if spec, admitted := configuration.ClientSpecFor(clientID); admitted {
			return spec.Label + " route"
		}
	}
	switch {
	case name == "environment:client-token":
		return "Client token environment"
	case name == "config":
		return "Local configuration"
	case name == "credential:backend":
		return "Credential backend"
	case strings.HasPrefix(name, "secret:"):
		return "Account Token"
	case isTargetCheck(name):
		spec, position, _ := targetCheck(name)
		switch {
		case check.Target != "":
			return spec.Label + " configuration " + check.Target
		case check.outsideHome:
			return spec.Label + " configuration target " + position + " (outside the home directory)"
		}
		return spec.Label + " configuration target " + position
	default:
		return "Other check"
	}
}

// targetCheck parses an adapter's configuration target check, which is named
// "<client>:target-<position>".
func targetCheck(name string) (configuration.ClientSpec, string, bool) {
	clientID, position, ok := strings.Cut(name, ":target-")
	if !ok {
		return configuration.ClientSpec{}, "", false
	}
	spec, admitted := configuration.ClientSpecFor(clientID)
	return spec, position, admitted
}

func isTargetCheck(name string) bool {
	_, _, ok := targetCheck(name)
	return ok
}

// doctorWording returns the doctor wording of the client that a check named
// "<prefix><client>" concerns.
func doctorWording(name, prefix string) (configuration.ClientSpec, configuration.DoctorWording) {
	clientID, ok := strings.CutPrefix(name, prefix)
	spec, admitted := configuration.ClientSpecFor(clientID)
	if !ok || !admitted {
		return configuration.ClientSpec{}, configuration.DoctorWording{}
	}
	return spec, spec.Doctor
}

// Detail maps machine diagnostics to non-sensitive human text.
func Detail(check Check) string {
	name, detail := check.Name, check.Detail
	switch {
	case name == "environment:client-token":
		if check.OK {
			return "No global client token environment variables detected"
		}
		if names, ok := strings.CutPrefix(detail, "global client token environment variables are set: "); ok {
			return "Global client token environment variables detected: " + names
		}
		return "Global client token environment variables detected"
	case name == "config":
		if detail == "valid" {
			return "Configuration is valid"
		}
		if detail == "not configured" {
			return "First-time setup is incomplete"
		}
		return "Cannot read or validate configuration"
	case name == "credential:backend":
		return "Credential storage is unavailable"
	case strings.HasPrefix(name, "secret:"):
		account := strings.TrimPrefix(name, "secret:")
		if check.OK {
			return account + " · available"
		}
		if detail == credentialBackendUnavailable {
			return account + " · credential backend unavailable"
		}
		return account + " · missing"
	case adapterStates(name):
		spec, _ := doctorWording(name, "adapter:")
		if check.OK {
			return doctorTitle(detail)
		}
		if strings.Contains(detail, "executable is not configured") {
			return "Enabled, but no executable is configured"
		}
		if detail == spec.Label+" configuration target is missing" {
			return "Enabled, but no " + spec.Label + " configuration file is configured"
		}
	case routeBinding(name):
		spec, _ := doctorWording(name, "projection:")
		return "Current " + spec.Label + " binding cannot be resolved"
	case isTargetCheck(name):
		if check.OK {
			return "Matches the current binding"
		}
		return "Does not match the current binding"
	}
	if check.OK {
		return "Healthy"
	}
	return "Check failed"
}

func adapterStates(name string) bool {
	_, wording := doctorWording(name, "adapter:")
	return wording.AdapterStates
}

func routeBinding(name string) bool {
	_, wording := doctorWording(name, "projection:")
	return wording.RouteBinding
}

func doctorTitle(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
