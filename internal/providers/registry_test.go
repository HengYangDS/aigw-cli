package providers_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/providers"
)

func TestRegistryDoesNotDependOnProviderOwnedTransportContracts(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "registry.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	concreteProviders := map[string]struct{}{}
	for _, imported := range file.Imports {
		path, unquoteErr := strconv.Unquote(imported.Path.Value)
		if unquoteErr != nil || !strings.Contains(path, "/internal/providers/") {
			continue
		}
		name := filepath.Base(path)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		concreteProviders[name] = struct{}{}
	}
	checkContract := func(owner string, node ast.Node) {
		ast.Inspect(node, func(candidate ast.Node) bool {
			selector, ok := candidate.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, concrete := concreteProviders[qualifier.Name]; concrete {
				t.Errorf("%s contract depends on concrete provider type %s.%s", owner, qualifier.Name, selector.Sel.Name)
			}
			return true
		})
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if ok && typeSpec.Name.Name == "probeFunc" {
					checkContract("probeFunc", typeSpec.Type)
				}
			}
		case *ast.FuncDecl:
			if declaration.Name.Name == "Probe" {
				checkContract("Probe", declaration.Type)
			}
		}
	}
}

func TestUnknownDiagnosticProviderIsRejectedAtExecutionNotConfiguration(t *testing.T) {
	providerAccount := configuration.Account{
		ID:           "future",
		Label:        "Future Gateway",
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://diagnostics.example.test"},
	}
	if providers.Supports(providerAccount.AccountProbe.Kind) {
		t.Fatal("an unbundled provider must not report support")
	}
	_, err := providers.Probe(context.Background(), nil, providerAccount, "api-token", account.Credential{SystemToken: "platform-token", UserID: "1"})
	if err == nil || !strings.Contains(err.Error(), "not included in this AIGW build") {
		t.Fatalf("error = %v", err)
	}
}

func TestBundledDMXAPIDiagnosticsAreExplicitlyRegistered(t *testing.T) {
	if !providers.Supports("dmxapi") {
		t.Fatal("DMXAPI diagnostics should be an explicit bundled provider integration")
	}
}

func TestProbeRequiresAccountProbeConfiguration(t *testing.T) {
	_, err := providers.Probe(context.Background(), nil, configuration.Account{ID: "no-probe"}, "api-token", account.Credential{})
	if err == nil || !strings.Contains(err.Error(), "no exact diagnostic provider") {
		t.Fatalf("error = %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestProbeDispatchesToDMXAPI(t *testing.T) {
	client := roundTrip(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"quota":100}}`))}, nil
	})
	providerAccount := configuration.Account{
		ID:           "dmx",
		AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://example.com"},
	}
	// We expect an error because the second call (fetchTokens) will fail due to our simple mock,
	// but this confirms it reached the dmxapi case.
	_, err := providers.Probe(context.Background(), client, providerAccount, "api-token", account.Credential{})
	if err == nil || (!strings.Contains(err.Error(), "DMXAPI token query failed") && !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "not found in the DMXAPI account")) {
		t.Fatalf("error = %v", err)
	}
}
