package doctor

import "testing"

func TestUnknownCheckProjectionFailsClosed(t *testing.T) {
	check := Check{Name: "future:internal", Detail: "private implementation detail", Fix: "private repair"}
	if got := Label(check.Name); got != "Other check" {
		t.Fatalf("Label() = %q, want Other check", got)
	}
	if got := Detail(check); got != "Check failed" {
		t.Fatalf("Detail() = %q, want Check failed", got)
	}
	if got := Fix(check); got != "aigw doctor --json" {
		t.Fatalf("Fix() = %q, want aigw doctor --json", got)
	}
}

func TestCommandConstructorExposesStableName(t *testing.T) {
	if got := NewCommand(Dependencies{}).Name(); got != "doctor" {
		t.Fatalf("doctor command = %q", got)
	}
}
