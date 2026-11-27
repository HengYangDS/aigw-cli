package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

// VersionOf returns what follows the extension's ID and a hyphen in name, the
// name of an entry of an editor's extensions directory, when it starts as a
// version does, with a digit: the entry is then an installed version of the
// extension, whose name may end with the platform that the version was built
// for. Any other name, and every name for a client without an extension, is
// no installed version of it.
func (extension ClientExtension) VersionOf(name string) (string, bool) {
	if extension.ID == "" {
		return "", false
	}
	rest, named := strings.CutPrefix(name, extension.ID+"-")
	if !named || rest == "" || rest[0] < '0' || rest[0] > '9' {
		return "", false
	}
	return rest, true
}

// ExtensionsDirectoryOf returns the editor's extensions directory that holds
// path inside an installed version of the client's extension, for any
// platform, and false for a path that lies in none. A program there is that
// version's, which the editor's next version of the extension replaces, so a
// client declares that directory as its extensions, never such a path as its
// program, and adopts no such path that discovery finds.
func (s ClientSpec) ExtensionsDirectoryOf(path string) (string, bool) {
	for directory := filepath.Dir(path); filepath.Dir(directory) != directory; directory = filepath.Dir(directory) {
		if _, versioned := s.Extension.VersionOf(filepath.Base(directory)); versioned {
			return filepath.Dir(directory), true
		}
	}
	return "", false
}

// FileRefusal is a refusal of the configuration file when it loads. Every
// command loads the file first, so no command repairs it: Refusal says why
// the file is refused, naming no command, and Edit is the edit of the file
// that resolves it.
type FileRefusal interface {
	Refusal() string
	Edit() string
}

// ProgramDeclarationError refuses a client's declaration of its program:
// Reason says why, and Change is the change of the client's section that
// resolves it.
type ProgramDeclarationError struct {
	Client, Reason, Change string
}

// Error names the client, the reason and the change.
func (e *ProgramDeclarationError) Error() string { return e.Refusal() + "; " + e.Change }

// Refusal names the client and the reason.
func (e *ProgramDeclarationError) Refusal() string {
	return fmt.Sprintf("client %q %s", e.Client, e.Reason)
}

// Edit is the change, made in the client's section of the file.
func (e *ProgramDeclarationError) Edit() string {
	return fmt.Sprintf("in the section of client %s, %s", e.Client, e.Change)
}
