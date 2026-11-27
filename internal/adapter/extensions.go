package adapter

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// hostPlatforms names, by Go's GOOS/GOARCH, the platform with which an editor
// marks an extension version built for each host that AIGW supports.
var hostPlatforms = map[string]string{"darwin/arm64": "darwin-arm64", "linux/amd64": "linux-x64", "linux/arm64": "linux-arm64"}

// editorPlatforms are the platforms that an editor's marketplace builds an
// extension version for. An installed version named with one of them is for
// that platform alone, so a version for another platform than the host's is
// ignored.
var editorPlatforms = []string{
	"darwin-arm64", "darwin-x64", "linux-arm64", "linux-armhf", "linux-x64",
	"alpine-arm64", "alpine-x64", "win32-arm64", "win32-x64", "web",
}

// obsoleteFile names the file in which an editor lists the installed versions
// that it has replaced and will delete later.
const obsoleteFile = ".obsolete"

// installedVersion is one installed version of a client's extension: its
// directory's name, its version and whether the name marks the host's
// platform.
type installedVersion struct {
	name, version string
	platform      bool
}

// newer reports whether v is a newer choice than other: the higher version,
// then, between equal versions, the one built for the host's platform, then
// the later name, so that the choice never depends on the order of entries.
func (v installedVersion) newer(other installedVersion) bool {
	if order := compareVersions(v.version, other.version); order != 0 {
		return order > 0
	}
	if v.platform != other.platform {
		return v.platform
	}
	return v.name > other.name
}

// resolveExtensions resolves an extensions directory to the program inside
// the highest installed version of the client's extension for this host,
// leaving out each version that the directory's .obsolete file lists. A lower
// version is never taken: the editor runs the highest one.
func resolveExtensions(spec domain.ClientSpec, product, directory string, host discovery.Host) (Program, *StopError) {
	stop := func(version, problem string, cause error) (Program, *StopError) {
		return Program{Version: version}, NewStop(spec.ID, problem, ProgramRepair(spec), cause)
	}
	extension := spec.Extension
	if extension.ID == "" {
		return stop("", product+" has no editor extension, so it cannot declare an extensions directory", nil)
	}
	entries, err := os.ReadDir(directory)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return stop("", product+"'s extensions directory cannot be read", err)
	}
	obsolete, err := obsoleteVersions(directory)
	if err != nil {
		return stop("", product+"'s extensions directory has a "+obsoleteFile+" file that AIGW cannot read", err)
	}
	platform := hostPlatforms[host.GOOS+"/"+host.GOARCH]
	var highest installedVersion
	for _, entry := range entries {
		version, platformed, named := entryVersion(extension.ID, entry.Name(), platform)
		if !named || obsolete[entry.Name()] || !isDirectory(directory, entry) {
			continue
		}
		if candidate := (installedVersion{entry.Name(), version, platformed}); highest.name == "" || candidate.newer(highest) {
			highest = candidate
		}
	}
	if highest.name == "" {
		return stop("", "The extensions directory holds no installed "+product+" extension ("+extension.ID+") for this host", nil)
	}
	inside, known := extensionProgram(extension, host)
	if !known {
		return stop(highest.version, "The "+product+" extension holds no program for this host", nil)
	}
	path := filepath.Join(directory, highest.name, filepath.FromSlash(inside))
	if runnable, err := discovery.ExecutableAvailable(path); err != nil || !runnable {
		return stop(highest.version, "Version "+highest.version+" of the "+product+" extension holds no program that can run at "+inside, err)
	}
	return Program{Path: path, Version: highest.version}, nil
}

// entryVersion reads the version of an extensions directory's entry named
// <id>-<version> or <id>-<version>-<platform>: named is false for an entry of
// another extension, an entry for another platform than platform, the host's,
// and a version that does not start with a digit.
func entryVersion(id, name, platform string) (version string, platformed, named bool) {
	version, named = strings.CutPrefix(name, id+"-")
	if !named {
		return "", false, false
	}
	for _, built := range editorPlatforms {
		if rest, marked := strings.CutSuffix(version, "-"+built); marked {
			if built != platform {
				return "", false, false
			}
			version, platformed = rest, true
			break
		}
	}
	if !versioned(version) {
		return "", false, false
	}
	return version, platformed, true
}

// versioned reports whether the part of an entry's name after its extension's
// ID and a hyphen starts as a version does, with a digit.
func versioned(rest string) bool { return rest != "" && rest[0] >= '0' && rest[0] <= '9' }

// ExtensionsDirectoryOf returns the extensions directory that holds path in an
// installed version of the client's extension, for any platform, and false
// for a path that lies in none: a program there is that version's, which the
// next version of the extension replaces, so the client declares the
// directory instead.
func ExtensionsDirectoryOf(spec domain.ClientSpec, path string) (string, bool) {
	if spec.Extension.ID == "" {
		return "", false
	}
	for directory := filepath.Dir(path); filepath.Dir(directory) != directory; directory = filepath.Dir(directory) {
		if rest, named := strings.CutPrefix(filepath.Base(directory), spec.Extension.ID+"-"); named && versioned(rest) {
			return filepath.Dir(directory), true
		}
	}
	return "", false
}

// isDirectory reports whether an entry of directory is one, or a link to one.
func isDirectory(directory string, entry fs.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(directory, entry.Name()))
	return err == nil && info.IsDir()
}

// obsoleteVersions reads the entries that directory's .obsolete file lists: a
// JSON object whose keys are entry names. A directory without the file has
// none; a file that cannot be read, or is no JSON object, is an error.
func obsoleteVersions(directory string) (map[string]bool, error) {
	path := filepath.Join(directory, obsoleteFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var listed map[string]json.RawMessage
	if err := json.Unmarshal(data, &listed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if listed == nil {
		return nil, fmt.Errorf("%s holds no JSON object", path)
	}
	obsolete := make(map[string]bool, len(listed))
	for name := range listed {
		obsolete[name] = true
	}
	return obsolete, nil
}

// extensionProgram is the program's path inside one installed version of
// extension on host, and false when the extension names no program for it.
func extensionProgram(extension domain.ClientExtension, host discovery.Host) (string, bool) {
	path := extension.Program
	for _, name := range []struct {
		placeholder string
		names       map[string]string
		host        string
	}{{"{os}", extension.OS, host.GOOS}, {"{arch}", extension.Arch, host.GOARCH}} {
		if !strings.Contains(path, name.placeholder) {
			continue
		}
		spelled, known := name.names[name.host]
		if !known {
			return "", false
		}
		path = strings.ReplaceAll(path, name.placeholder, spelled)
	}
	return path, true
}

// compareVersions compares two dotted versions part by part (see
// comparePart). When every part that both have is equal, the version with
// more parts is the higher, so that 1.2.0 is above 1.2.
func compareVersions(left, right string) int {
	leftParts, rightParts := strings.Split(left, "."), strings.Split(right, ".")
	for index := range min(len(leftParts), len(rightParts)) {
		if order := comparePart(leftParts[index], rightParts[index]); order != 0 {
			return order
		}
	}
	return cmp.Compare(len(leftParts), len(rightParts))
}

// comparePart compares one part of two dotted versions by its leading number
// and then by the text after it. A part that starts with a number is above
// one that does not; leading numbers compare by value at any length; between
// equal numbers, a part without text after it is above one with text, so
// that 1.2.3 is above 1.2.3-beta and 1.2.10-beta above 1.2.9; and the texts
// of two parts compare byte by byte, so that 1.2.3-beta is above 1.2.3-alpha.
func comparePart(left, right string) int {
	leftNumber, leftText := leadingNumber(left)
	rightNumber, rightText := leadingNumber(right)
	switch {
	case leftNumber != "" && rightNumber == "":
		return 1
	case leftNumber == "" && rightNumber != "":
		return -1
	}
	leftNumber, rightNumber = strings.TrimLeft(leftNumber, "0"), strings.TrimLeft(rightNumber, "0")
	if order := cmp.Compare(len(leftNumber), len(rightNumber)); order != 0 {
		return order
	}
	if order := strings.Compare(leftNumber, rightNumber); order != 0 {
		return order
	}
	switch {
	case leftText == rightText:
		return 0
	case leftText == "":
		return 1
	case rightText == "":
		return -1
	}
	return strings.Compare(leftText, rightText)
}

// leadingNumber splits part into its leading run of decimal digits and the
// text after it.
func leadingNumber(part string) (number, text string) {
	end := strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		return part, ""
	}
	return part[:end], part[end:]
}
