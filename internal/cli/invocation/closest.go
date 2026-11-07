package invocation

import (
	"cmp"
	"slices"
	"strings"
)

// maximumSuggestions bounds the closest matches that one refusal names.
const maximumSuggestions = 3

// closestMatches returns up to three candidates that look like what the user
// typed, closest first: those that differ from it by at most a third of its
// length in single-character edits, ignoring case, and those that contain it or
// that it contains. Equal distances keep the candidates' order.
func closestMatches(typed string, candidates []string) []string {
	typed = strings.ToLower(typed)
	threshold := max(1, len([]rune(typed))/3)
	type match struct {
		candidate string
		distance  int
	}
	var matches []match
	for _, candidate := range candidates {
		lower := strings.ToLower(candidate)
		distance := editDistance(typed, lower)
		if distance > threshold && !strings.Contains(lower, typed) && !strings.Contains(typed, lower) {
			continue
		}
		matches = append(matches, match{candidate: candidate, distance: distance})
	}
	slices.SortStableFunc(matches, func(left, right match) int { return cmp.Compare(left.distance, right.distance) })
	closest := make([]string, 0, min(len(matches), maximumSuggestions))
	for _, match := range matches[:min(len(matches), maximumSuggestions)] {
		closest = append(closest, match.candidate)
	}
	return closest
}

// editDistance is the Levenshtein distance between two strings, counted in
// runes: the fewest insertions, deletions and substitutions that turn one into
// the other.
func editDistance(left, right string) int {
	a, b := []rune(left), []rune(right)
	previous := make([]int, len(b)+1)
	for index := range previous {
		previous[index] = index
	}
	current := make([]int, len(b)+1)
	for i := range a {
		current[0] = i + 1
		for j := range b {
			substitution := previous[j]
			if a[i] != b[j] {
				substitution++
			}
			current[j+1] = min(previous[j+1]+1, current[j]+1, substitution)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}
