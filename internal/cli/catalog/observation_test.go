package catalog

import (
	"bytes"
	"strings"
	"testing"

	"aigw-cli/internal/app"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

func TestDeprecatedCatalogEntryDoesNotPresentAsAdmitted(t *testing.T) {
	state, detail := catalogModelDisplay(app.CatalogModel{
		ID:    "legacy",
		State: app.CatalogDeprecated,
		Routes: []app.CatalogRoute{{
			ID: "legacy-route", Lifecycle: domain.RouteDeprecated, Evidence: app.CatalogObserved,
		}},
	})
	if state == 0 || detail != "Deprecated Routes: legacy-route" {
		t.Fatalf("deprecated display state=%v detail=%q", state, detail)
	}
}

func TestDefaultCatalogRenderingKeepsDeprecatedRoutesSeparateFromCandidates(t *testing.T) {
	out := new(bytes.Buffer)
	renderCatalogObservation(terminal.NewRendererWithWidth(out, false, 120), nil, app.CatalogObservation{
		Models: []app.CatalogModel{
			{ID: "current", State: app.CatalogAdmitted, Routes: []app.CatalogRoute{{ID: "current-route"}}},
			{ID: "legacy", State: app.CatalogDeprecated, Routes: []app.CatalogRoute{{ID: "legacy-route"}}},
			{ID: "future", State: app.CatalogCandidate},
		},
	}, false)

	text := out.String()
	for _, want := range []string{"1 admitted", "1 deprecated", "legacy", "Deprecated Routes: legacy-route", "1 candidate models require qualification"} {
		if !strings.Contains(text, want) {
			t.Fatalf("default catalogue output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "future") {
		t.Fatalf("default catalogue output exposed candidate detail:\n%s", text)
	}
}
