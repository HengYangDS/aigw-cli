package manifest

import (
	"reflect"
	"testing"
)

func TestReplacementSetNormalizesExplicitNames(t *testing.T) {
	got := ReplacementSet([]string{" team ", "", "\t", "team", "fallback"})
	want := map[string]bool{"team": true, "fallback": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("replacement set = %#v, want %#v", got, want)
	}
}
