package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestMissingPreviewCasesNamesEachAbsentPreview(t *testing.T) {
	complete := "snapshot $HOME before.txt\n" +
		"exec aigw demo add item --dry-run\n" +
		"exec aigw demo remove item --dry-run --json\n" +
		"snapshot $HOME after.txt\n" +
		"cmp after.txt before.txt\n"
	previews := [][]string{{"add"}, {"remove"}}
	for _, test := range []struct {
		name     string
		script   string
		previews [][]string
		want     []string
	}{
		{"complete", complete, previews, nil},
		{"no previews", "exec aigw demo add item\n", nil, nil},
		{"applied only", strings.Replace(complete, "demo add item --dry-run", "demo add item", 1), previews,
			[]string{"a preview of `demo add` between snapshots that cmp compares"}},
		{"refused preview", strings.Replace(complete, "exec aigw demo add", "exitcode 1 aigw demo add", 1), previews,
			[]string{"a preview of `demo add` between snapshots that cmp compares"}},
		{"conditional preview", strings.Replace(complete, "exec aigw demo add", "[!windows] exec aigw demo add", 1), previews,
			[]string{"a preview of `demo add` between snapshots that cmp compares"}},
		{"before the snapshots", "exec aigw demo add item --dry-run\n" + strings.Replace(complete, "exec aigw demo add item --dry-run\n", "", 1), previews,
			[]string{"a preview of `demo add` between snapshots that cmp compares"}},
		{"no comparison", strings.Replace(complete, "cmp after.txt before.txt\n", "", 1), previews,
			[]string{"a preview of `demo add` between snapshots that cmp compares", "a preview of `demo remove` between snapshots that cmp compares"}},
		{"another directory", strings.Replace(complete, "snapshot $HOME after.txt", "snapshot $WORK after.txt", 1), [][]string{{"add"}},
			[]string{"a preview of `demo add` between snapshots that cmp compares"}},
		{"snapshots in the archive", "exec aigw demo --dry-run\n-- note.txt --\n" + complete, [][]string{{}},
			[]string{"a preview between snapshots that cmp compares"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := missingPreviewCases("demo", test.previews, test.script); !slices.Equal(got, test.want) {
				t.Fatalf("missingPreviewCases() = %q, want %q", got, test.want)
			}
		})
	}
}

// missingPreviewCases names, for each runnable command that offers --dry-run,
// named by its path below the command, the checked preview that the script
// lacks: a successful preview that lies between two snapshots of one
// directory, which a later cmp compares, so that the script shows that the
// preview changed nothing there.
func missingPreviewCases(name string, previews [][]string, script string) []string {
	lines := scriptLines(script)
	var missing []string
	for _, path := range previews {
		found := false
		for index, line := range lines {
			words, status, ok := scriptInvocation(line)
			if ok && status == 0 && words[0] == name && slices.Contains(words, "--dry-run") &&
				len(words) > len(path) && slices.Equal(words[1:1+len(path)], path) && betweenComparedSnapshots(lines, index) {
				found = true
				break
			}
		}
		if !found {
			of := ""
			if len(path) > 0 {
				of = fmt.Sprintf(" of `%s`", strings.Join(append([]string{name}, path...), " "))
			}
			missing = append(missing, "a preview"+of+" between snapshots that cmp compares")
		}
	}
	return missing
}

// betweenComparedSnapshots reports whether the line at index lies between two
// unconditional snapshots of one directory that a later cmp compares.
func betweenComparedSnapshots(lines []string, index int) bool {
	for _, earlier := range lines[:index] {
		before := strings.Fields(earlier)
		if len(before) != 3 || before[0] != "snapshot" {
			continue
		}
		for offset, later := range lines[index+1:] {
			after := strings.Fields(later)
			if len(after) != 3 || after[0] != "snapshot" || after[1] != before[1] {
				continue
			}
			for _, compare := range lines[index+2+offset:] {
				words := strings.Fields(compare)
				if len(words) == 3 && words[0] == "cmp" && (words[1] == after[2] && words[2] == before[2] || words[1] == before[2] && words[2] == after[2]) {
					return true
				}
			}
		}
	}
	return false
}
