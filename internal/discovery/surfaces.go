package discovery

import (
	"os"
	"path/filepath"
	"sort"
)

const (
	SurfaceCodexCLIStandalone = "codex-cli-standalone"
	SurfacePyCharmCodex       = "jetbrains-pycharm-codex"
	SurfaceAirCodex           = "jetbrains-air-codex"
	SurfaceJunieCLI           = "jetbrains-junie-cli"

	AuthorityAIGW        = "aigw"
	AuthorityJetBrainsAI = "jetbrains-ai"
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
	standalone := Surface{
		ID:          SurfaceCodexCLIStandalone,
		Product:     "Codex CLI",
		Authority:   AuthorityAIGW,
		ConfigPath:  filepath.Join(s.Home, ".codex", "config.toml"),
		AutoManaged: true,
	}
	if s.GOOS != "darwin" {
		return []Surface{standalone}
	}
	return []Surface{
		standalone,
		{
			ID:        SurfacePyCharmCodex,
			Product:   "PyCharm",
			Authority: AuthorityJetBrainsAI,
			ConfigPath: filepath.Join(
				s.Home, "Library", "Caches", "JetBrains", "PyCharm2026.1",
				"aia", "codex", "config.toml",
			),
		},
		{
			ID:        SurfaceAirCodex,
			Product:   "JetBrains Air",
			Authority: AuthorityJetBrainsAI,
			ConfigPath: filepath.Join(
				s.Home, "Library", "Application Support", "JetBrains", "Air",
				".codex", "config.toml",
			),
			ManualFallbackAllowed: true,
		},
		{
			ID:         SurfaceJunieCLI,
			Product:    "Junie CLI",
			Authority:  AuthorityJetBrainsAI,
			Executable: filepath.Join(s.Home, ".local", "bin", "junie"),
		},
	}
}

func (s System) discoverSurfaces() []Surface {
	surfaces := s.surfaceCatalog()
	for index := range surfaces {
		path := surfaces[index].ConfigPath
		if path == "" {
			path = surfaces[index].Executable
		}
		info, err := os.Lstat(path)
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

// AutoManagedCodexTargets returns only existing AIGW-owned standalone homes.
// JetBrains-backed surfaces are visible for diagnosis but excluded from generic
// setup and repair adoption.
func (r Result) AutoManagedCodexTargets() []string {
	targets := make([]string, 0)
	for _, surface := range r.Surfaces {
		if surface.Present && surface.AutoManaged && surface.ConfigPath != "" {
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
