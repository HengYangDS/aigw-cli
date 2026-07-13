package cli

import "testing"

func TestDoctorHumanProjectionFailsClosedForFutureChecks(t *testing.T) {
	for _, check := range []doctorCheck{
		{Name: "future:internal", OK: true, Detail: "internal success detail"},
		{Name: "future:internal", OK: false, Detail: "internal failure detail", Fix: "internal repair instruction"},
	} {
		if got := doctorCheckLabel(check.Name); got != "其他检查" {
			t.Fatalf("label = %q, want 其他检查", got)
		}
		want := "检查未通过"
		if check.OK {
			want = "正常"
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
