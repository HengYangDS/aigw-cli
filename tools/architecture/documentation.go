package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func checkDocumentationNavigation(root string, contract policy, report *Report) error {
	if len(contract.DocumentationRoots) == 0 {
		return nil
	}
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}
	canonical := canonicalDocumentation(files, contract.DocumentationRoots)
	if len(canonical) == 0 {
		return nil
	}
	reachable := map[string]bool{}
	queue := make([]string, 0, len(contract.DocumentationEntries))
	for _, entry := range contract.DocumentationEntries {
		if !canonical[entry] {
			return fmt.Errorf("documentation entrypoint is not a canonical Markdown file: %s", entry)
		}
		reachable[entry] = true
		queue = append(queue, entry)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		links, err := localMarkdownLinks(filepath.Join(root, filepath.FromSlash(current)), current)
		if err != nil {
			return err
		}
		for _, target := range links {
			if !canonical[target] || reachable[target] {
				continue
			}
			reachable[target] = true
			queue = append(queue, target)
		}
	}
	orphans := make([]string, 0)
	for file := range canonical {
		if !reachable[file] {
			orphans = append(orphans, file)
		}
	}
	sort.Strings(orphans)
	for _, orphan := range orphans {
		report.addFinding(Finding{
			Rule:    "documentation_navigation",
			Path:    orphan,
			Message: "canonical documentation must be reachable from a declared entrypoint",
		})
	}
	return nil
}

func canonicalDocumentation(files, roots []string) map[string]bool {
	canonical := map[string]bool{}
	for _, file := range files {
		if path.Ext(file) != ".md" {
			continue
		}
		for _, root := range roots {
			if file == root || strings.HasSuffix(root, "/") && strings.HasPrefix(file, root) || !strings.Contains(path.Base(root), ".") && strings.HasPrefix(file, root+"/") {
				canonical[file] = true
				break
			}
		}
	}
	return canonical
}

func localMarkdownLinks(filename, relative string) ([]string, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read documentation %s: %w", relative, err)
	}
	links := make([]string, 0)
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var destination []byte
		switch typed := node.(type) {
		case *ast.Link:
			destination = typed.Destination
		case *ast.Image:
			destination = typed.Destination
		default:
			return ast.WalkContinue, nil
		}
		if resolved, ok := resolveLocalMarkdownTarget(relative, string(destination)); ok {
			links = append(links, resolved)
		}
		return ast.WalkContinue, nil
	})
	return links, nil
}

func resolveLocalMarkdownTarget(source, target string) (string, bool) {
	target = strings.TrimSpace(strings.Trim(target, "<>"))
	if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return "", false
	}
	if space := strings.IndexAny(target, " \t"); space >= 0 {
		target = target[:space]
	}
	if fragment := strings.IndexByte(target, '#'); fragment >= 0 {
		target = target[:fragment]
	}
	resolved := path.Clean(path.Join(path.Dir(source), target))
	if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") || path.Ext(resolved) != ".md" {
		return "", false
	}
	return resolved, true
}
