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
// test files are fixtures. TemporaryExceptions lists the packages that still
// name a client while the semantic-architecture Change moves that knowledge:
// the gate refuses an unlisted offender and a listed package that no longer
// offends, so the list can only shrink.
type clientConfinement struct {
	Clients             []string `toml:"clients"`
	ProductRoots        []string `toml:"product_roots"`
	AdapterRoot         string   `toml:"adapter_root"`
	CompositionRoots    []string `toml:"composition_roots"`
	TemporaryExceptions []string `toml:"temporary_exceptions"`
}

type clientOccurrence struct {
	path string
	line int
}

// validateClientConfinement accepts an absent table or one complete declaration
// of portable roots, unique kebab-case client IDs and unique exceptions.
func validateClientConfinement(c clientConfinement) error {
	if len(c.Clients) == 0 && len(c.ProductRoots) == 0 && c.AdapterRoot == "" && len(c.CompositionRoots) == 0 && len(c.TemporaryExceptions) == 0 {
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
		"product_roots": c.ProductRoots, "composition_roots": c.CompositionRoots, "temporary_exceptions": c.TemporaryExceptions,
	} {
		unique := map[string]bool{}
		for _, path := range paths {
			if !isPortableRelativePath(path) || unique[path] {
				return fmt.Errorf("client_confinement %s has an invalid or duplicate path %q", field, path)
			}
			unique[path] = true
		}
	}
	return nil
}

// checkClientConfinement scans identifiers and the decoded values of string and
// character literals, never comments, of non-test product files. A client is
// named when a token's words contain the words of its ID; in a client's own
// package, a shorter ID that prefixes the owner's ID names the owner.
func checkClientConfinement(root string, files []goFileInfo, c clientConfinement, report *Report) error {
	if len(c.Clients) == 0 {
		return nil
	}
	clients := make([][]string, 0, len(c.Clients))
	for _, id := range c.Clients {
		clients = append(clients, strings.Split(id, "-"))
	}
	naming := map[string]map[string]clientOccurrence{}
	for _, file := range files {
		if file.isTest || !withinAny(file.dir, c.ProductRoots) || withinAny(file.dir, c.CompositionRoots) {
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.relPath)))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.relPath, err)
		}
		owner := adapterOwner(file.dir, c)
		for _, item := range namedClients(file.relPath, source, clients) {
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
	reportClientConfinement(naming, c, report)
	return nil
}

func reportClientConfinement(naming map[string]map[string]clientOccurrence, c clientConfinement, report *Report) {
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
			if slices.Contains(c.TemporaryExceptions, dir) {
				continue
			}
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
	for _, dir := range c.TemporaryExceptions {
		if _, offends := naming[dir]; !offends {
			report.addFinding(Finding{
				Rule:    "client_confinement_exception",
				Path:    dir,
				Package: dir,
				Message: fmt.Sprintf("package %q no longer names a client; remove it from temporary_exceptions", dir),
			})
		}
	}
}

type namedClient struct {
	client []string
	line   int
}

// namedClients returns every client that one file's identifiers and literals
// name, preferring the longest ID at each word position.
func namedClients(relPath string, source []byte, clients [][]string) []namedClient {
	files := token.NewFileSet()
	var scan scanner.Scanner
	scan.Init(files.AddFile(relPath, -1, len(source)), source, nil, 0)
	var found []namedClient
	for {
		position, kind, literal := scan.Scan()
		if kind == token.EOF {
			return found
		}
		if kind != token.IDENT && kind != token.STRING && kind != token.CHAR {
			continue
		}
		// A literal names a client by its value, so escapes are decoded first.
		if kind != token.IDENT {
			if value, err := strconv.Unquote(literal); err == nil {
				literal = value
			}
		}
		words := splitWords(literal)
		for index := 0; index < len(words); index++ {
			var match []string
			for _, client := range clients {
				if len(client) > len(match) && index+len(client) <= len(words) && slices.Equal(words[index:index+len(client)], client) {
					match = client
				}
			}
			if match != nil {
				found = append(found, namedClient{client: match, line: files.Position(position).Line})
				index += len(match) - 1
			}
		}
	}
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
