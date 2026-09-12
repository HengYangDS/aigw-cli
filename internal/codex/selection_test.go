package codex

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func BenchmarkCodexProjection(b *testing.B) {
	var source strings.Builder
	source.WriteString("model_provider = 'native'\nmodel = 'original'\n")
	for index := range 100 {
		fmt.Fprintf(&source, "\n[profiles.profile_%d]\npersonality = 'pragmatic'\nmodel_reasoning_effort = 'high'\n", index)
	}
	original := source.String()
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	b.ReportAllocs()
	b.SetBytes(int64(len(original)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := projectCodex(original, block, runtime.Model, "", "aigw"); err != nil {
			b.Fatal(err)
		}
	}
}

func TestCodexSelectionEditsOnlyNativeRootAssignments(t *testing.T) {
	for _, test := range []struct {
		name, source, original, replacement, want string
	}{
		{"empty", "", "", "model = 'new'", "model = 'new'\n"},
		{"absent withdrawal", "user = true", "", "", "user = true"},
		{"last line", "user = true\nmodel = 'old'", "model = 'old'", "", "user = true"},
		{"CRLF withdrawal", "  model = 'old' # original\r\nuser = true\r\n", "  model = 'old' # original", "", "user = true\r\n"},
		{"quoted key", "\"model\" = 'old' # original\n", "\"model\" = 'old' # original", "model = '$1-${model}'", "model = '$1-${model}'\n"},
		{"multiline", "model = '''old\nvalue''' # original\nuser = true\n", "model = '''old\nvalue''' # original", "model = 'new'", "model = 'new'\nuser = true\n"},
		{"nested key", "[profiles.user]\nmodel = 'user'\n", "", "model = 'root'", "model = 'root'\n[profiles.user]\nmodel = 'user'\n"},
		{"dotted key", "profiles.user.model = 'user'\n", "", "", "profiles.user.model = 'user'\n"},
		{"table array", "[[examples]]\nmodel = 'user'\n", "", "", "[[examples]]\nmodel = 'user'\n"},
		{"string content", "user = '''\n[profiles.fake]\nmodel = 'inside'\n'''\nmodel = 'root'\n", "model = 'root'", "model = 'new'", "user = '''\n[profiles.fake]\nmodel = 'inside'\n'''\nmodel = 'new'\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			line, err := codexSelectionLine(test.source, "model")
			if err != nil || line != test.original {
				t.Fatalf("original selection = %q, %v; want %q", line, err, test.original)
			}
			got, err := setCodexSelection(test.source, "model", test.replacement)
			if err != nil || got != test.want {
				t.Fatalf("projected source = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestCodexSelectionEncodingRoundTripsLiteralValues(t *testing.T) {
	for _, value := range []string{"$1-${route}", `C:\Users\operator\catalog.json`, `model-"quoted"`, "中文 <model> & value", "line\u2028separator"} {
		encoded, err := codexTOMLString(value)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct{ Value string }
		if err := toml.Unmarshal([]byte("Value = "+encoded), &decoded); err != nil || decoded.Value != value {
			t.Fatalf("literal round trip = %q, %v; want %q", decoded.Value, err, value)
		}
	}
}

func TestCodexSelectionRejectsMalformedSourceBeforeEditing(t *testing.T) {
	for _, source := range []string{"model =", "model = 'root'\n[broken", "model = 'first'\nmodel = 'second'"} {
		if _, err := codexSelectionLine(source, "model"); err == nil {
			t.Fatalf("read accepted malformed TOML: %q", source)
		}
		if _, err := setCodexSelection(source, "model", "model = 'new'"); err == nil {
			t.Fatalf("write accepted malformed TOML: %q", source)
		}
	}
}

func TestCodexManagedWithdrawalPreservesUnownedKeys(t *testing.T) {
	for _, source := range []string{
		"model = 'user' # ordinary comment\n",
		"[profiles.user]\nmodel = 'user' # managed by AIGW\n",
		"user = '''\nmodel = 'example' # managed by AIGW\n'''\n",
	} {
		got, err := removeManagedCodexLine(source, "model")
		if err != nil || got != source {
			t.Fatalf("unowned source changed: %q, %v", got, err)
		}
	}
	got, err := removeManagedCodexLine("model = 'owned' #\tmanaged by AIGW\r\nuser = true\n", "model")
	if err != nil || got != "user = true\n" {
		t.Fatalf("managed withdrawal = %q, %v", got, err)
	}
	if _, err := removeManagedCodexLine("model =", "model"); err == nil || !strings.Contains(err.Error(), "parse Codex selection") {
		t.Fatalf("malformed withdrawal = %v", err)
	}
}
