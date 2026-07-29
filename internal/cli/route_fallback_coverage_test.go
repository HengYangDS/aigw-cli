package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

func TestRouteFallbackCommandsRejectOtherSurfaces(t *testing.T) {
	for _, verb := range []string{"fallback", "restore", "recover"} {
		t.Run(verb, func(t *testing.T) {
			store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
			err := Execute(&App{Config: store, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}, []string{"route", verb, "other"})
			if err == nil || !strings.Contains(err.Error(), "only `aigw route") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAirRouteHumanPreviews(t *testing.T) {
	t.Run("fallback", func(t *testing.T) {
		h := newAirRouteHarness(t)
		if err := Execute(h.app, []string{"route", "fallback", "air", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
		text := h.app.Out.(*bytes.Buffer).String()
		for _, want := range []string{"Air route preview", "namespaced-fallback", "Preview made no persistent changes", "route fallback air --confirm-host-idle"} {
			if !strings.Contains(text, want) {
				t.Fatalf("preview lacks %q:\n%s", want, text)
			}
		}
	})

	t.Run("restore", func(t *testing.T) {
		h := newAirRouteHarness(t)
		if err := Execute(h.app, []string{"route", "fallback", "air", "--confirm-host-idle"}); err != nil {
			t.Fatal(err)
		}
		h.app.Out.(*bytes.Buffer).Reset()
		if err := Execute(h.app, []string{"route", "restore", "air", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
		if text := h.app.Out.(*bytes.Buffer).String(); !strings.Contains(text, "Air route preview") || !strings.Contains(text, "restore-external") {
			t.Fatalf("preview = %s", text)
		}
	})

	t.Run("recover", func(t *testing.T) {
		h := newAirRouteHarness(t)
		stageStaleAirFullSelection(t, h)
		if err := Execute(h.app, []string{"route", "recover", "air", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
		text := h.app.Out.(*bytes.Buffer).String()
		for _, want := range []string{"recover-stale-full-selection", "remove-air-fallback-target", "route recover air --confirm-host-idle"} {
			if !strings.Contains(text, want) {
				t.Fatalf("preview lacks %q:\n%s", want, text)
			}
		}
	})
}

func TestAirRouteLoadSelectedTokenAndConfirmationErrors(t *testing.T) {
	for _, verb := range []string{"fallback", "restore", "recover"} {
		t.Run(verb+" load", func(t *testing.T) {
			h := newAirRouteHarness(t)
			h.app.Config = config.NewStore(t.TempDir())
			if err := Execute(h.app, []string{"route", verb, "air", "--dry-run"}); err == nil {
				t.Fatal("expected config load failure")
			}
		})
	}

	t.Run("fallback selected", func(t *testing.T) {
		h := newAirRouteHarness(t)
		if err := os.WriteFile(h.air, []byte("model_provider = \"aigw\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Execute(h.app, []string{"route", "fallback", "air", "--dry-run"})
		if err == nil || !strings.Contains(err.Error(), "currently selects AIGW") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("restore selected", func(t *testing.T) {
		h := newAirRouteHarness(t)
		if err := os.WriteFile(h.air, []byte("model_provider = \"aigw_fallback\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Execute(h.app, []string{"route", "restore", "air", "--dry-run"})
		if err == nil || !strings.Contains(err.Error(), "currently selects AIGW") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		h := newAirRouteHarness(t)
		if err := h.secrets.Delete("gateway"); err != nil {
			t.Fatal(err)
		}
		err := Execute(h.app, []string{"route", "fallback", "air", "--dry-run"})
		if err == nil || !strings.Contains(err.Error(), "has no token") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("restore confirmation", func(t *testing.T) {
		h := newAirRouteHarness(t)
		if err := Execute(h.app, []string{"route", "fallback", "air", "--confirm-host-idle"}); err != nil {
			t.Fatal(err)
		}
		err := Execute(h.app, []string{"route", "restore", "air"})
		if err == nil || !strings.Contains(err.Error(), "--confirm-host-idle") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestAirRecoverAlreadyExternalAppliesWithoutChangingMembership(t *testing.T) {
	h := newAirRouteHarness(t)
	before, err := os.ReadFile(h.app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := Execute(h.app, []string{"route", "recover", "air", "--confirm-host-idle"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(h.app.Config.Path())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("configuration changed: %v", err)
	}
	if !strings.Contains(h.app.Out.(*bytes.Buffer).String(), "stale full selection recovered") {
		t.Fatalf("output = %s", h.app.Out.(*bytes.Buffer).String())
	}
}

func TestResolveAirSurfaceAndFallbackPreconditions(t *testing.T) {
	t.Run("discovery unavailable", func(t *testing.T) {
		if _, _, err := resolveAirSurface(&App{}); err == nil {
			t.Fatal("expected discovery error")
		}
	})

	for _, test := range []struct {
		name    string
		surface discovery.Surface
		want    string
	}{
		{name: "missing", want: "was not found"},
		{name: "not present", surface: discovery.Surface{ID: discovery.SurfaceAirCodex, ConfigPath: "/air", ManualFallbackAllowed: true}, want: "was not found"},
		{name: "empty path", surface: discovery.Surface{ID: discovery.SurfaceAirCodex, Present: true, ManualFallbackAllowed: true}, want: "was not found"},
		{name: "not admitted", surface: discovery.Surface{ID: discovery.SurfaceAirCodex, Present: true, ConfigPath: "/air"}, want: "does not admit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			surfaces := []discovery.Surface{}
			if test.surface.ID != "" {
				surfaces = append(surfaces, test.surface)
			}
			app := &App{Discovery: reconciliationDiscovery{result: discovery.Result{Surfaces: surfaces}}}
			_, _, err := resolveAirSurface(app)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	t.Run("adapter disabled", func(t *testing.T) {
		if _, err := validateAirFallbackPreconditions(&App{}, domain.NewConfig()); err == nil || !strings.Contains(err.Error(), "enable the standalone") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route resolution", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/codex", Targets: []string{"/config"}}
		if _, err := validateAirFallbackPreconditions(&App{}, cfg); err == nil || !strings.Contains(err.Error(), "unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestAirFallbackPureHelpersAndRollback(t *testing.T) {
	t.Run("top-level parsing", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing.toml")
		if _, err := airTopLevelSelectsAIGW(missing); err == nil {
			t.Fatal("expected read failure")
		}
		for name, fixture := range map[string]struct {
			body string
			want bool
		}{
			"selected": {body: "# comment\nmodel_provider = \"aigw\" # owner\n", want: true},
			"native":   {body: "ignored\nmodel_provider = \"native\"\n", want: false},
			"section":  {body: "[model_providers.aigw]\nmodel_provider = \"aigw\"\n", want: false},
		} {
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(dir, name+".toml")
				if err := os.WriteFile(path, []byte(fixture.body), 0o600); err != nil {
					t.Fatal(err)
				}
				got, err := airTopLevelSelectsAIGW(path)
				if err != nil || got != fixture.want {
					t.Fatalf("selected=%v error=%v", got, err)
				}
			})
		}
	})

	if _, err := planForAirSurface(discovery.Result{}, nil); err == nil {
		t.Fatal("missing plan unexpectedly succeeded")
	}
	if slicesEqual([]string{"a"}, nil) || slicesEqual([]string{"a"}, []string{"b"}) || !slicesEqual([]string{"a"}, []string{"a"}) {
		t.Fatal("slicesEqual returned an incorrect result")
	}

	t.Run("reference error", func(t *testing.T) {
		before := domain.NewConfig()
		before.Adapters[domain.ClientCodex] = domain.AdapterConfig{Targets: []string{""}}
		if _, _, err := airFallbackRefs(discovery.Result{}, before, domain.NewConfig()); err == nil {
			t.Fatal("expected empty target error")
		}
	})

	t.Run("commit reconciliation rollback", func(t *testing.T) {
		dir := t.TempDir()
		store := config.NewStore(filepath.Join(dir, "aigw.toml"))
		before := reconciliationConfig(filepath.Join(dir, "standalone.toml"))
		if err := store.Save(before); err != nil {
			t.Fatal(err)
		}
		after := cloneConfig(before)
		targetDir := t.TempDir()
		adapter := after.Adapters[domain.ClientCodex]
		adapter.Targets = append(adapter.Targets, targetDir)
		after.Adapters[domain.ClientCodex] = adapter
		runtime, _, err := after.ResolveRuntime(domain.ClientCodex, "")
		if err != nil {
			t.Fatal(err)
		}
		refs := []adapters.CodexTargetRef{{SurfaceID: "bad", Authority: discovery.AuthorityAIGW, ProjectionMode: adapters.CodexProjectionFullSelection, Path: targetDir}}
		app := &App{Config: store, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Secrets: secrets.NewMemoryStore()}
		err = commitAirFallback(app, before, after, nil, refs, runtime)
		if err == nil {
			t.Fatal("expected reconciliation failure")
		}
		got, loadErr := store.Load()
		if loadErr != nil || !slicesEqual(got.Adapters[domain.ClientCodex].Targets, before.Adapters[domain.ClientCodex].Targets) {
			t.Fatalf("config not rolled back: %#v, %v", got.Adapters, loadErr)
		}
	})
}

func TestAirFallbackAuthenticationFailureIsWrapped(t *testing.T) {
	h := newAirRouteHarness(t)
	want := errors.New("login failed")
	h.app.Runner = setupCoverageRunner{err: want}
	err := runAirFallback(context.Background(), h.app, false, false, true)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("error = %v", err)
	}
}
