package discovery

import (
	"aigw-cli/internal/platform"
)

// Surface is a stable host classification. Discovery only inspects paths; it
// never runs the executable or opens a client session.
type Surface struct {
	ID          string
	Executable  string
	ConfigPath  string
	Present     bool
	AutoManaged bool
}

// Surface resolves one discovered configuration surface by its stable identifier.
func (r Result) Surface(id string) (Surface, bool) {
	for _, surface := range r.Surfaces {
		if surface.ID == id {
			return surface, true
		}
	}
	return Surface{}, false
}

// SurfaceForConfigPath resolves the discovered surface that owns a configuration path.
func (r Result) SurfaceForConfigPath(path string) (Surface, bool) {
	for _, surface := range r.Surfaces {
		if sameSurfacePath(surface.ConfigPath, path) {
			return surface, true
		}
	}
	return Surface{}, false
}

// SurfaceForExecutablePath resolves the discovered surface associated with a client executable.
func (r Result) SurfaceForExecutablePath(path string) (Surface, bool) {
	for _, surface := range r.Surfaces {
		if sameSurfacePath(surface.Executable, path) {
			return surface, true
		}
	}
	return Surface{}, false
}

// sameSurfacePath reports whether path names a surface's file, by the one
// judgement of file identity (platform.SameFile). A surface without the path
// owns nothing.
func sameSurfacePath(left, right string) bool {
	return left != "" && right != "" && platform.SameFile(left, right)
}
