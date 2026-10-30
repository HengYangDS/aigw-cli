package doctor

import (
	"strings"

	"aigw-cli/internal/domain"
)

// checkLabel maps a diagnostic to a stable human label. A check about one native
// file under the home directory names that file. Admission supplies the label
// of the client that a check concerns.
func checkLabel(admission domain.Admission, check checkResult) string {
	name := check.Name
	if clientID, ok := strings.CutPrefix(name, "adapter:"); ok {
		if spec, admitted := admission.Spec(clientID); admitted {
			return spec.Label + " adapter"
		}
	}
	if clientID, ok := strings.CutPrefix(name, "projection:"); ok {
		if spec, admitted := admission.Spec(clientID); admitted {
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
	case isTargetCheck(admission, name):
		spec, position, _ := targetCheck(admission, name)
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
func targetCheck(admission domain.Admission, name string) (domain.ClientSpec, string, bool) {
	clientID, position, ok := strings.Cut(name, ":target-")
	if !ok {
		return domain.ClientSpec{}, "", false
	}
	spec, admitted := admission.Spec(clientID)
	return spec, position, admitted
}

func isTargetCheck(admission domain.Admission, name string) bool {
	_, _, ok := targetCheck(admission, name)
	return ok
}

// doctorWording returns the doctor wording of the client that a check named
// "<prefix><client>" concerns.
func doctorWording(admission domain.Admission, name, prefix string) (domain.ClientSpec, domain.DoctorWording) {
	clientID, ok := strings.CutPrefix(name, prefix)
	spec, admitted := admission.Spec(clientID)
	if !ok || !admitted {
		return domain.ClientSpec{}, domain.DoctorWording{}
	}
	return spec, spec.Doctor
}

// checkDetail maps machine diagnostics to non-sensitive human text. Admission
// supplies the wording of the client that a check concerns.
func checkDetail(admission domain.Admission, check checkResult) string {
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
	case adapterStates(admission, name):
		spec, _ := doctorWording(admission, name, "adapter:")
		if check.OK {
			return doctorTitle(detail)
		}
		if strings.Contains(detail, "executable is not configured") {
			return "Enabled, but no executable is configured"
		}
		if detail == spec.Label+" configuration target is missing" {
			return "Enabled, but no " + spec.Label + " configuration file is configured"
		}
	case routeBinding(admission, name):
		spec, _ := doctorWording(admission, name, "projection:")
		return "Current " + spec.Label + " binding cannot be resolved"
	case isTargetCheck(admission, name):
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

func adapterStates(admission domain.Admission, name string) bool {
	_, wording := doctorWording(admission, name, "adapter:")
	return wording.AdapterStates
}

func routeBinding(admission domain.Admission, name string) bool {
	_, wording := doctorWording(admission, name, "projection:")
	return wording.RouteBinding
}

func doctorTitle(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
