package presentation_test

import (
	"bytes"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

func TestRendererProducesAlignedHumanReadableLayout(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Title("AIGW", "健康检查")
	r.Section("配置")
	r.Row("配置文件", "正常")
	r.Row("当前服务", "DMXAPI")
	r.Section("连接")
	r.Status(presentation.OK, "API Token", "正常")
	r.Status(presentation.Warn, "精确余额", "未启用")
	r.Detail("aigw account connect")
	r.Section("结果")
	r.Success("一切正常")
	r.Next("aigw balance")
	want := "AIGW  健康检查\n" +
		"────────────────────────────────────────\n\n" +
		"配置\n" +
		"  配置文件            正常\n" +
		"  当前服务            DMXAPI\n\n" +
		"连接\n" +
		"  ✓ API Token         正常\n" +
		"  ! 精确余额          未启用\n" +
		"                      aigw account connect\n\n" +
		"结果\n" +
		"  ✓ 一切正常\n\n" +
		"下一步\n" +
		"  aigw balance\n"
	if out.String() != want {
		t.Fatalf("layout mismatch\n--- want ---\n%s--- got ---\n%s", want, out.String())
	}
}

func TestRowsAndStatusesStartValuesAtSameDisplayColumn(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Row("配置文件", "VALUE")
	r.Status(presentation.OK, "API Token", "VALUE")
	r.Status(presentation.Warn, "精确余额", "VALUE")
	for index, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		prefix, _, ok := strings.Cut(line, "VALUE")
		if !ok {
			t.Fatalf("line lacks value: %q", line)
		}
		if got := presentation.DisplayWidth(prefix); got != 22 {
			t.Fatalf("line %d value column = %d, want 22: %q", index, got, line)
		}
	}
}

func TestStatusKeepsLongLabelsOnOneLine(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Status(presentation.OK, "environment:client-token", "正常")
	got := out.String()
	if strings.Count(got, "\n") != 1 || strings.Contains(got, "environment:client\n-token") {
		t.Fatalf("long status label wrapped: %q", got)
	}
}

func TestDisplayWidthTreatsCJKAsTwoColumnsAndANSICodesAsZero(t *testing.T) {
	if got := presentation.DisplayWidth("配置文件"); got != 8 {
		t.Fatalf("CJK width = %d, want 8", got)
	}
	if got := presentation.DisplayWidth("\x1b[32m配置\x1b[0m"); got != 4 {
		t.Fatalf("colored CJK width = %d, want 4", got)
	}
}

func TestRendererColorIsOptionalAndNeverAffectsSpacing(t *testing.T) {
	var plain, colored bytes.Buffer
	presentation.New(&plain, false).Status(presentation.OK, "Token", "正常")
	presentation.New(&colored, true).Status(presentation.OK, "Token", "正常")
	if strings.Contains(plain.String(), "\x1b[") {
		t.Fatalf("plain output contains ANSI: %q", plain.String())
	}
	stripped := presentation.StripANSI(colored.String())
	if stripped != plain.String() {
		t.Fatalf("color changed layout: plain=%q color=%q stripped=%q", plain.String(), colored.String(), stripped)
	}
}

func TestProblemUsesConsistentProblemEvidenceImpactFixOrder(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Problem(presentation.Problem{
		Title:    "Token 额度已耗尽",
		Evidence: "HTTP 403 · 令牌额度不足",
		Impact:   "Claude 与 Codex 无法继续调用",
		Fix:      "aigw rotate",
	})
	want := `AIGW  需要处理
────────────────────────────────────────

问题
  Token 额度已耗尽

判断依据
  HTTP 403 · 令牌额度不足

影响
  Claude 与 Codex 无法继续调用

建议操作
  aigw rotate
`
	if out.String() != want {
		t.Fatalf("problem layout mismatch\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}
