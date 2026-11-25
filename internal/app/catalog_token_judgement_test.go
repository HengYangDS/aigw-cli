package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/secret"
)

// The model catalog judges a provider's Token by secret.ObserveToken, as
// every other command does: an empty Token file is an absent Token, never a
// credential backend that could not tell (the fresh review of 034296053).
func TestTheCatalogJudgesAnEmptyTokenAsAbsent(t *testing.T) {
	root := t.TempDir()
	store, err := secret.Select(secret.Selection{Backend: "file", GOOS: runtime.GOOS, Root: root, Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(root, "tokens", "gateway"), 0); err != nil {
		t.Fatal(err)
	}
	if state := secret.ObserveToken(store, "gateway").State(); state != secret.TokenAbsent {
		t.Fatalf("an empty Token file observes as %d, want absent", state)
	}
	client := catalogHTTPClient(func(*http.Request) (*http.Response, error) { return nil, errors.New("no request expected") })
	for _, o := range DiscoverCatalog(context.Background(), store, client, configuredCatalog()).Observations {
		if o.Provider == "gateway" && o.Status != CatalogTokenUnavailable {
			t.Fatalf("the catalog reports %q (%v) for an empty Token; want %q", o.Status, o.Failure, CatalogTokenUnavailable)
		}
	}
}
