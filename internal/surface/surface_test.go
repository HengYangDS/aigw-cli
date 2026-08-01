package surface

import "testing"

func TestStableIdentities(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "standalone Codex CLI", got: string(CodexCLIStandalone), want: "codex-cli-standalone"},
		{name: "AIGW authority", got: string(AuthorityAIGW), want: "aigw"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("identity = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestIDClassifiesCodexCLISurfaces(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want bool
	}{
		{name: "standalone", id: CodexCLIStandalone, want: true},
		{name: "explicit", id: CodexCLIExplicit("0123456789ab"), want: true},
		{name: "legacy empty explicit identifier", id: CodexCLIExplicit(""), want: true},
		{name: "embedded prefix", id: ID("other-codex-cli-explicit-0123456789ab"), want: false},
		{name: "empty", id: ID(""), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.IsCodexCLI(); got != tt.want {
				t.Fatalf("IsCodexCLI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIDAuthority(t *testing.T) {
	tests := []struct {
		name      string
		id        ID
		authority Authority
		known     bool
	}{
		{name: "standalone", id: CodexCLIStandalone, authority: AuthorityAIGW, known: true},
		{name: "explicit", id: CodexCLIExplicit("0123456789ab"), authority: AuthorityAIGW, known: true},
		{name: "unknown", id: ID("unknown"), known: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := tt.id.Authority()
			if got != tt.authority || known != tt.known {
				t.Fatalf("Authority() = %q, %v; want %q, %v", got, known, tt.authority, tt.known)
			}
			if tt.known && !tt.id.HasAuthority(tt.authority) {
				t.Fatalf("HasAuthority(%q) = false", tt.authority)
			}
			if tt.id.HasAuthority(Authority("foreign")) {
				t.Fatal("HasAuthority(foreign) = true")
			}
		})
	}
}
