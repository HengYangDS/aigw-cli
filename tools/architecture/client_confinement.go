package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// clientConfinement keeps each admitted client's knowledge in that client's
// adapter package. Outside it, only composition roots may name a client, and
// test files are fixtures. NativeTerms lists, per client, the exact literals of
// that client's own vocabulary that contain another client's ID; its adapter
// package may hold them, and an unused term is refused.
type clientConfinement struct {
	Clients          []string            `toml:"clients"`
	ProductRoots     []string            `toml:"product_roots"`
	AdapterRoot      string              `toml:"adapter_root"`
	CompositionRoots []string            `toml:"composition_roots"`
	NativeTerms      map[string][]string `toml:"native_terms"`
}

type clientOccurrence struct {
	path string
	line int
}

// validateClientConfinement accepts an absent table or one complete declaration
// of portable roots, unique kebab-case client IDs and unique native terms that
// each name another client than their own.
func validateClientConfinement(c clientConfinement) error {
	if len(c.Clients) == 0 && len(c.ProductRoots) == 0 && c.AdapterRoot == "" && len(c.CompositionRoots) == 0 && len(c.NativeTerms) == 0 {
		return nil
	}
	if len(c.Clients) == 0 || len(c.ProductRoots) == 0 || !isPortableRelativePath(c.AdapterRoot) {
		return fmt.Errorf("client_confinement requires clients, product_roots and a relative adapter_root")
	}
	seen := map[string]bool{}
	for _, id := range c.Clients {
		for segment := range strings.SplitSeq(id, "-") {
			if segment == "" || strings.IndexFunc(segment, func(r rune) bool { return !unicode.IsLower(r) && !unicode.IsDigit(r) }) >= 0 {
				return fmt.Errorf("client_confinement client %q must be a lowercase kebab-case ID", id)
			}
		}
		if seen[id] {
			return fmt.Errorf("client_confinement client %q is duplicated", id)
		}
		seen[id] = true
	}
	for field, paths := range map[string][]string{
		"product_roots": c.ProductRoots, "composition_roots": c.CompositionRoots,
	} {
		unique := map[string]bool{}
		for _, path := range paths {
			if !isPortableRelativePath(path) || unique[path] {
				return fmt.Errorf("client_confinement %s has an invalid or duplicate path %q", field, path)
			}
			unique[path] = true
		}
	}
	return validateNativeTerms(c, seen)
}

// validateNativeTerms accepts, per declared client, unique non-empty terms that
// each name a client other than their owner.
func validateNativeTerms(c clientConfinement, declared map[string]bool) error {
	clients := clientWords(c.Clients)
	for owner, terms := range c.NativeTerms {
		if !declared[owner] {
			return fmt.Errorf("client_confinement native_terms owner %q is not a declared client", owner)
		}
		unique := map[string]bool{}
		for _, term := range terms {
			if term == "" || unique[term] {
				return fmt.Errorf("client_confinement native term %q of %q is empty or duplicated", term, owner)
			}
			unique[term] = true
			if !slices.ContainsFunc(clientsInWords(splitWords(term), clients), func(named []string) bool {
				return !hasWordPrefix(strings.Split(owner, "-"), named)
			}) {
				return fmt.Errorf("client_confinement native term %q of %q names no other client and needs no declaration", term, owner)
			}
		}
	}
	return nil
}

func clientWords(ids []string) [][]string {
	clients := make([][]string, 0, len(ids))
	for _, id := range ids {
		clients = append(clients, strings.Split(id, "-"))
	}
	return clients
}

// checkClientConfinement scans identifiers and the decoded values of string and
// character literals, never comments, of non-test product files. A client is
// named when a token's words contain the words of its ID; in a client's own
// package, a shorter ID that prefixes the owner's ID names the owner, and a
// literal whose value is one of the owner's native terms names no client.
func checkClientConfinement(root string, files []goFileInfo, c clientConfinement, report *Report) error {
	if len(c.Clients) == 0 {
		return nil
	}
	clients := clientWords(c.Clients)
	naming := map[string]map[string]clientOccurrence{}
	usedTerms := map[string]bool{}
	for _, file := range files {
		if file.isTest || !withinAny(file.dir, c.ProductRoots) || withinAny(file.dir, c.CompositionRoots) {
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.relPath)))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.relPath, err)
		}
		owner := adapterOwner(file.dir, c)
		native := map[string]bool{}
		for _, term := range c.NativeTerms[strings.Join(owner, "-")] {
			native[term] = true
		}
		found, used := namedClients(file.relPath, source, clients, native)
		for _, term := range used {
			usedTerms[strings.Join(owner, "-")+"\x00"+term] = true
		}
		for _, item := range found {
			if owner != nil && hasWordPrefix(owner, item.client) {
				continue
			}
			id := strings.Join(item.client, "-")
			if naming[file.dir] == nil {
				naming[file.dir] = map[string]clientOccurrence{}
			}
			if _, seen := naming[file.dir][id]; !seen {
				naming[file.dir][id] = clientOccurrence{path: file.relPath, line: item.line}
			}
		}
	}
	reportClientConfinement(naming, usedTerms, c, report)
	return nil
}

func reportClientConfinement(naming map[string]map[string]clientOccurrence, usedTerms map[string]bool, c clientConfinement, report *Report) {
	packages := make([]string, 0, len(naming))
	for dir := range naming {
		packages = append(packages, dir)
	}
	sort.Strings(packages)
	report.ClientPackages = map[string][]string{}
	for _, dir := range packages {
		ids := make([]string, 0, len(naming[dir]))
		for id := range naming[dir] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			report.ClientPackages[id] = append(report.ClientPackages[id], dir)
			occurrence := naming[dir][id]
			report.addFinding(Finding{
				Rule:    "client_confinement",
				Path:    occurrence.path,
				Line:    occurrence.line,
				Package: dir,
				Name:    id,
				Message: fmt.Sprintf("package %q names client %q; move that knowledge into the client's adapter package", dir, id),
			})
		}
	}
	owners := make([]string, 0, len(c.NativeTerms))
	for owner := range c.NativeTerms {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	for _, owner := range owners {
		dir := c.AdapterRoot + "/" + strings.ReplaceAll(owner, "-", "")
		for _, term := range c.NativeTerms[owner] {
			if usedTerms[owner+"\x00"+term] {
				continue
			}
			report.addFinding(Finding{
				Rule:    "client_confinement_native_term",
				Path:    dir,
				Package: dir,
				Name:    term,
				Message: fmt.Sprintf("native term %q of client %q no longer occurs in %q; remove it from native_terms", term, owner, dir),
			})
		}
	}
}

type namedClient struct {
	client []string
	line   int
}

// namedClients returns every client that one file's identifiers and literals
// name, and the native terms it holds. A literal whose decoded value is a
// native term names no client.
func namedClients(relPath string, source []byte, clients [][]string, native map[string]bool) (found []namedClient, used []string) {
	files := token.NewFileSet()
	var scan scanner.Scanner
	scan.Init(files.AddFile(relPath, -1, len(source)), source, nil, 0)
	for {
		position, kind, literal := scan.Scan()
		if kind == token.EOF {
			return found, used
		}
		if kind != token.IDENT && kind != token.STRING && kind != token.CHAR {
			continue
		}
		// A literal names a client by its value, so escapes are decoded first.
		if kind != token.IDENT {
			if value, err := strconv.Unquote(literal); err == nil {
				literal = value
			}
			if native[literal] {
				used = append(used, literal)
				continue
			}
		}
		for _, client := range clientsInWords(splitWords(literal), clients) {
			found = append(found, namedClient{client: client, line: files.Position(position).Line})
		}
	}
}

// clientsInWords returns the clients that a word sequence contains, preferring
// the longest ID at each word position.
func clientsInWords(words []string, clients [][]string) [][]string {
	var named [][]string
	for index := 0; index < len(words); index++ {
		var match []string
		for _, client := range clients {
			if len(client) > len(match) && index+len(client) <= len(words) && slices.Equal(words[index:index+len(client)], client) {
				match = client
			}
		}
		if match != nil {
			named = append(named, match)
			index += len(match) - 1
		}
	}
	return named
}

func adapterOwner(dir string, c clientConfinement) []string {
	for _, id := range c.Clients {
		owned := c.AdapterRoot + "/" + strings.ReplaceAll(id, "-", "")
		if dir == owned || strings.HasPrefix(dir, owned+"/") {
			return strings.Split(id, "-")
		}
	}
	return nil
}

func hasWordPrefix(owner, client []string) bool {
	return len(client) <= len(owner) && slices.Equal(owner[:len(client)], client)
}

func withinAny(dir string, roots []string) bool {
	for _, root := range roots {
		if dir == root || strings.HasPrefix(dir, root+"/") {
			return true
		}
	}
	return false
}

// splitWords lowercases one identifier or literal into words: separators split
// it, a lower-to-upper change starts a word, an acronym ends before its last
// capital when a lowercase letter follows, and digits stay with their word.
func splitWords(text string) []string {
	var words []string
	var current []rune
	runes := []rune(text)
	flush := func() {
		if len(current) > 0 {
			words = append(words, strings.ToLower(string(current)))
			current = current[:0]
		}
	}
	for index, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if unicode.IsUpper(r) && len(current) > 0 {
			previous := current[len(current)-1]
			nextLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextLower) {
				flush()
			}
		}
		current = append(current, r)
	}
	flush()
	return words
}
