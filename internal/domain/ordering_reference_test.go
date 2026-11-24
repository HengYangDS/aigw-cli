package domain

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// The display order is the same as the one that sorted every entry on every
// read and append, for configurations built by appends, by direct writes to
// the maps, and by removals, mixed in every order: making it cheaper must not
// move one entry.
func TestDisplayOrderMatchesTheSortingOrder(t *testing.T) {
	for seed := range uint64(400) {
		random := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // A seeded generator reproduces a failing sequence from its seed; nothing here is secret.
		got, want := Config{}, referenceConfig{providers: map[string]bool{}, models: map[ModelRef]bool{}}
		for step := range 40 {
			provider := fmt.Sprintf("p%d", random.IntN(5))
			ref := NewModelRef(provider, fmt.Sprintf("m%d", random.IntN(12)))
			switch random.IntN(6) {
			case 0, 1, 2:
				if _, exists := got.Providers[provider]; !exists {
					if err := got.AppendProvider(provider, Provider{}); err != nil {
						t.Fatal(err)
					}
					want.appendProvider(provider)
				}
				if _, exists := got.Models[ref]; !exists {
					if err := got.AppendModel(Model{Provider: ref.provider, ID: ref.id}); err != nil {
						t.Fatal(err)
					}
					want.appendModel(ref)
				}
			case 3:
				if got.Providers == nil {
					got.Providers = map[string]Provider{}
				}
				got.Providers[provider] = Provider{}
				want.providers[provider] = true
			case 4:
				if got.Models == nil {
					got.Models = map[ModelRef]Model{}
				}
				got.Models[ref] = Model{Provider: ref.provider, ID: ref.id}
				want.models[ref] = true
			case 5:
				delete(got.Models, ref)
				delete(want.models, ref)
			}
			if !slices.Equal(got.ProviderIDs(), want.providerIDs()) {
				t.Fatalf("seed %d step %d: providers %v, want %v", seed, step, got.ProviderIDs(), want.providerIDs())
			}
			if !slices.Equal(got.ModelRefs(), want.modelRefs()) {
				t.Fatalf("seed %d step %d: models %v, want %v", seed, step, got.ModelRefs(), want.modelRefs())
			}
			if repeats(got.order.providers) || repeats(got.order.models) {
				t.Fatalf("seed %d step %d: a recorded order repeats an identity: %v, %v", seed, step, got.order.providers, got.order.models)
			}
		}
	}
}

// repeats reports whether list names one identity twice, which the display
// order's shortcut for a complete recorded order relies on never happening.
func repeats[K comparable](list []K) bool {
	seen := make(map[K]bool, len(list))
	for _, key := range list {
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}

// referenceConfig is the ordering as it was before it became cheaper: every
// read sorts every entry, and every append records the whole current order.
type referenceConfig struct {
	providers map[string]bool
	models    map[ModelRef]bool
	order     struct {
		providers []string
		models    []ModelRef
	}
}

func (r *referenceConfig) appendProvider(id string) {
	r.order.providers = append(r.providerIDs(), id)
	if r.providers == nil {
		r.providers = map[string]bool{}
	}
	r.providers[id] = true
}

func (r *referenceConfig) appendModel(ref ModelRef) {
	r.order.models = append(r.modelRefs(), ref)
	if r.models == nil {
		r.models = map[ModelRef]bool{}
	}
	r.models[ref] = true
}

func (r *referenceConfig) providerIDs() []string {
	return referenceOrder(r.order.providers, r.providers, strings.Compare)
}

func (r *referenceConfig) modelRefs() []ModelRef {
	refs := referenceOrder(r.order.models, r.models, compareModelRefs)
	rank := map[string]int{}
	for index, id := range r.providerIDs() {
		rank[id] = index
	}
	slices.SortStableFunc(refs, func(left, right ModelRef) int {
		leftRank, leftKnown := rank[left.provider]
		rightRank, rightKnown := rank[right.provider]
		switch {
		case leftKnown && rightKnown:
			return cmp.Compare(leftRank, rightRank)
		case leftKnown != rightKnown:
			if leftKnown {
				return -1
			}
			return 1
		default:
			return strings.Compare(left.provider, right.provider)
		}
	})
	return refs
}

func referenceOrder[K comparable](recorded []K, entries map[K]bool, compare func(K, K) int) []K {
	ordered := make([]K, 0, len(entries))
	listed := map[K]bool{}
	for _, key := range recorded {
		if entries[key] && !listed[key] {
			ordered = append(ordered, key)
			listed[key] = true
		}
	}
	rest := slices.SortedFunc(maps.Keys(entries), compare)
	return append(ordered, slices.DeleteFunc(rest, func(key K) bool { return listed[key] })...)
}
