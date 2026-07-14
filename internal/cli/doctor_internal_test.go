package cli

import "testing"

func TestDoctorHumanProjectionFailsClosedForFutureChecks(t *testing.T) {
	for _, check := range []doctorCheck{
		{Name: "future:internal", OK: true, Detail: "internal success detail"},
		{Name: "future:internal", OK: false, Detail: "internal failure detail", Fix: "internal repair instruction"},
	} {
		if got := doctorCheckLabel(check.Name); got != "Other check" {
			t.Fatalf("label = %q, want Other check", got)
		}
		want := "Check failed"
		if check.OK {
			want = "Healthy"
		}
		if got := doctorCheckDetail(check); got != want {
			t.Fatalf("detail = %q, want %q", got, want)
		}
		if !check.OK {
			if got := doctorCheckFix(check); got != "aigw doctor --json" {
				t.Fatalf("fix = %q, want aigw doctor --json", got)
			}
		}
	}
}

func TestDoctorMixedFailuresFallbackToRepair(t *testing.T) {
	checks := []doctorCheck{
		{Name: "codex:target-1", OK: false, Fix: "run `aigw sync` to reconcile this target"},
		{Name: "shim:claude", OK: false, Fix: "run `aigw repair`"},
	}
	if got := doctorNextAction(checks); got != "aigw repair" {
		t.Fatalf("next action = %q, want aigw repair", got)
	}
}

func TestDoctorUnclassifiedFailureFallsBackToRepair(t *testing.T) {
	checks := []doctorCheck{{Name: "config", OK: false, Detail: "unexpected"}}
	if got := doctorNextAction(checks); got != "aigw repair" {
		t.Fatalf("next action = %q, want aigw repair", got)
	}
}
