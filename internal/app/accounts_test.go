package app

import (
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"strings"
	"testing"
)

func TestAccountReferenceRequiresAChoiceOnlyWhenAmbiguous(t *testing.T) {
	tests := []struct {
		name     string
		accounts map[string]domain.Account
		args     []string
		want     string
		wantErr  string
	}{
		{name: "explicit", accounts: map[string]domain.Account{"one": {}, "two": {}}, args: []string{"two"}, want: "two"},
		{name: "only account", accounts: map[string]domain.Account{"one": {}}, want: "one"},
		{name: "none", accounts: map[string]domain.Account{}, wantErr: "0 accounts"},
		{name: "ambiguous", accounts: map[string]domain.Account{"one": {}, "two": {}}, wantErr: "2 accounts"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := domain.NewConfig(clients.Admission())
			cfg.Accounts = test.accounts
			got, err := AccountReference(cfg, test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("accountReference() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("accountReference() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}
