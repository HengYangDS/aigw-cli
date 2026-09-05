package publication

import "testing"

func TestAuthorityNormalizesDefaultPorts(t *testing.T) {
	same, err := sameAuthority("https://gitlab.example/api/v4", "https://gitlab.example:443/assets/a")
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatal("default HTTPS port should identify the same authority")
	}
	got, err := authority("http://EXAMPLE.test./asset")
	if err != nil || got != "http://example.test:80" {
		t.Fatalf("authority=%q err=%v", got, err)
	}
}

func TestAuthorityRejectsInvalidInputs(t *testing.T) {
	if _, err := authority("https://example.test:bad"); err == nil {
		t.Fatal("invalid port accepted")
	}
	if _, err := sameAuthority("bad", "https://example.test"); err == nil {
		t.Fatal("invalid left authority accepted")
	}
	if _, err := sameAuthority("https://example.test", "bad"); err == nil {
		t.Fatal("invalid right authority accepted")
	}
}
