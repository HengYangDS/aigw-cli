package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type attestFailWriter struct{ err error }

func (writer attestFailWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestRouteAttestCommandValidationAndHumanOutput(t *testing.T) {
	t.Run("invalid surface", func(t *testing.T) {
		out := &bytes.Buffer{}
		store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
		err := Execute(&App{Config: store, Out: out, Err: out}, []string{"route", "attest", "other"})
		if err == nil || !strings.Contains(err.Error(), "only `aigw route attest air`") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("attested", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
		if err := Execute(h.app, []string{"route", "attest", "air"}); err != nil {
			t.Fatal(err)
		}
		text := h.app.Out.(*bytes.Buffer).String()
		for _, want := range []string{"Air runtime attestation", "Window start", "Runtime authority", "Fresh Air router evidence matches"} {
			if !strings.Contains(text, want) {
				t.Fatalf("output lacks %q: %s", want, text)
			}
		}
	})

	t.Run("unattested", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://other.test/responses")
		err := Execute(h.app, []string{"route", "attest", "air"})
		if err == nil || !strings.Contains(err.Error(), "not attested") || !strings.Contains(h.app.Out.(*bytes.Buffer).String(), "was not established") {
			t.Fatalf("error=%v output=%s", err, h.app.Out.(*bytes.Buffer).String())
		}
	})

	t.Run("JSON output failure", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
		want := errors.New("output failed")
		h.app.Out = attestFailWriter{err: want}
		if err := Execute(h.app, []string{"route", "attest", "air", "--json"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestBuildAirRuntimeAttestationDependencyErrors(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app := &App{Config: config.NewStore(t.TempDir())}
		if _, err := buildAirRuntimeAttestation(app); err == nil || !strings.Contains(err.Error(), "configuration is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route", func(t *testing.T) {
		root := t.TempDir()
		store := config.NewStore(filepath.Join(root, "config.toml"))
		cfg := domain.NewConfig()
		cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
		cfg.Profiles["one"] = domain.Profile{Label: "One", Account: "one", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude"}}
		cfg.Routes.Default = "one"
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if _, err := buildAirRuntimeAttestation(&App{Config: store}); err == nil || !strings.Contains(err.Error(), "Codex route") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("discovery", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
		h.app.Discovery = nil
		if _, err := buildAirRuntimeAttestation(h.app); err == nil || !strings.Contains(err.Error(), "discovery") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("standalone", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
		h.app.Discovery = reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID: discovery.SurfaceAirCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: h.air, Present: true, ManualFallbackAllowed: true,
		}}}}
		if _, err := buildAirRuntimeAttestation(h.app); err == nil || !strings.Contains(err.Error(), "standalone") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("configuration inspection", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
		discovered := h.app.Discovery.(reconciliationDiscovery).result
		for index := range discovered.Surfaces {
			if discovered.Surfaces[index].ID == discovery.SurfaceAirCodex {
				discovered.Surfaces[index].ConfigPath = t.TempDir()
			}
		}
		h.app.Discovery = reconciliationDiscovery{result: discovered}
		if _, err := buildAirRuntimeAttestation(h.app); err == nil || !strings.Contains(err.Error(), "inspection") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("log directory", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
		h.app.GOOS = "unsupported"
		if _, err := buildAirRuntimeAttestation(h.app); err == nil || !strings.Contains(err.Error(), "log discovery") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("log inspection", func(t *testing.T) {
		h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
		logDir := filepath.Dir(h.log)
		if err := os.RemoveAll(logDir); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(logDir, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := buildAirRuntimeAttestation(h.app); err == nil {
			t.Fatal("expected log inspection failure")
		}
	})
}

func TestAppNowUsesInjectedClock(t *testing.T) {
	want := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	if got := appNow(&App{Now: func() time.Time { return want }}); !got.Equal(want) {
		t.Fatalf("time = %v, want %v", got, want)
	}
	if appNow(&App{}).IsZero() {
		t.Fatal("system time is zero")
	}
}
