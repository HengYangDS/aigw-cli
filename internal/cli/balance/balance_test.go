package balance

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/store"
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestBalanceRequiresAnAccountBeforeProviderWork(t *testing.T) {
	runtime := emptyBalanceRuntime(t)
	balanceCommand := NewCommand(runtime)
	balanceCommand.SilenceErrors = true
	balanceCommand.SilenceUsage = true
	if err := balanceCommand.Execute(); err == nil || !strings.Contains(err.Error(), "0 accounts") {
		t.Fatalf("balance error = %v", err)
	}
}

// emptyBalanceRuntime is an interactive invocation without configuration.
func emptyBalanceRuntime(t *testing.T) invocation.Context {
	t.Helper()
	return invocation.Context{
		Clients:     clients.Registry(),
		Config:      store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission()),
		Interactive: true,
		Out:         &bytes.Buffer{},
		RenderOut:   &bytes.Buffer{},
	}
}
