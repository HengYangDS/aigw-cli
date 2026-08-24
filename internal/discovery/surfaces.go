package discovery

import (
	"os"
	"path/filepath"
	"sort"

	"aigw-cli/internal/surface"
)

// Surface is a stable host classification. Discovery only inspects paths; it
// never runs the executable or opens a client session.
type Surface struct {
	ID                    string `json:"surface_id"`
	Product               string `json:"product"`
	Authority             string `json:"authority"`
	Executable            string `json:"executable,omitempty"`
	ConfigPath            string `json:"config_path,omitempty"`
	Present               bool   `json:"present"`
	AutoManaged           bool   `json:"auto_managed"`
	ManualFallbackAllowed bool   `json:"manual_fallback_allowed"`
}

func (s System) surfaceCatalog() []Surface {
	return []Surface{{
		ID:          string(surface.CodexHomeDefault),
		Product:     "Codex",
		Authority:   string(surface.AuthorityAIGW),
		ConfigPath:  filepath.Join(s.Home, ".codex", "config.toml"),
		AutoManaged: true,
	}}
}

func (s System) discoverSurfaces() []Surface {
	surfaces := s.surfaceCatalog()
	for index := range surfaces {
		info, err := os.Lstat(surfaces[index].ConfigPath)
		surfaces[index].Present = err == nil && !info.IsDir()
	}
	return surfaces
}

func (r Result) Surface(id string) (Surface, bool) {
	for _, surface := range r.Surfaces {
		if surface.ID == id {
			return surface, true
		}
	}
	return Surface{}, false
}

func (r Result) SurfaceForConfigPath(path string) (Surface, bool) {
	for _, surface := range r.Surfaces {
		if sameSurfacePath(surface.ConfigPath, path) {
			return surface, true
		}
	}
	return Surface{}, false
}

func (r Result) SurfaceForExecutablePath(path string) (Surface, bool) {
	for _, surface := range r.Surfaces {
		if sameSurfacePath(surface.Executable, path) {
			return surface, true
		}
	}
	return Surface{}, false
}

// AutoManagedCodexTargets returns Codex homes that AIGW may project without
// further operator admission. Presence remains an observed readiness fact; it
// does not determine whether the declared default surface may be created.
func (r Result) AutoManagedCodexTargets() []string {
	targets := make([]string, 0)
	for _, surface := range r.Surfaces {
		if surface.AutoManaged && surface.ConfigPath != "" {
			targets = append(targets, surface.ConfigPath)
		}
	}
	sort.Strings(targets)
	return targets
}

func sameSurfacePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	left = resolveSurfacePath(left)
	right = resolveSurfacePath(right)
	return left == right
}

func resolveSurfacePath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return path
}
