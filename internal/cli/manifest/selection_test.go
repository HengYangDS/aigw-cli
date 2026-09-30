package manifest

import (
	"reflect"
	"testing"
)

func TestSelectorSetNormalizesExplicitNames(t *testing.T) {
	got := selectorSet([]string{" team ", "", "\t", "team", "fallback"})
	want := map[string]bool{"team": true, "fallback": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selector set = %#v, want %#v", got, want)
	}
}
