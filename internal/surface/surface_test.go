package surface

import "testing"

func TestStableIdentities(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "default Codex Home", got: string(CodexHomeDefault), want: "codex-home-default"},
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

func TestIDClassifiesCodexHomes(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want bool
	}{
		{name: "default", id: CodexHomeDefault, want: true},
		{name: "explicit", id: CodexHomeExplicit("0123456789ab"), want: true},
		{name: "legacy empty explicit identifier", id: CodexHomeExplicit(""), want: true},
		{name: "embedded prefix", id: ID("other-codex-home-explicit-0123456789ab"), want: false},
		{name: "empty", id: ID(""), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.IsCodexHome(); got != tt.want {
				t.Fatalf("IsCodexHome() = %v, want %v", got, tt.want)
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
		{name: "default", id: CodexHomeDefault, authority: AuthorityAIGW, known: true},
		{name: "explicit", id: CodexHomeExplicit("0123456789ab"), authority: AuthorityAIGW, known: true},
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
