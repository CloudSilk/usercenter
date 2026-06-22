package prompt

import (
	"strings"
	"testing"
)

func TestRender_BasicSubstitution(t *testing.T) {
	tpl := "你好 {{name}}，你的角色是 {{ role }}。"
	got := Render(tpl, map[string]string{"name": "Alice", "role": "admin"})
	want := "你好 Alice，你的角色是 admin。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRender_MissingVarBecomesEmpty(t *testing.T) {
	got := Render("[{{a}}][{{b}}]", map[string]string{"a": "1"})
	if got != "[1][]" {
		t.Fatalf("got %q", got)
	}
}

func TestRender_NoInjection(t *testing.T) {
	// 确认采用白名单替换而非 text/template，模板控制结构不被执行
	tpl := "{{printf \"evil\"}}"
	got := Render(tpl, map[string]string{"printf": "X"})
	// printf 不是单个变量 token（含空格+字符串字面量），整体不匹配，原样保留
	if got != tpl {
		t.Fatalf("unexpected execution: got %q", got)
	}
}

func TestExtractVariables_OrderedUnique(t *testing.T) {
	got := ExtractVariables("{{b}} {{a}} {{a}} {{c}}")
	want := []string{"b", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestNormalizeVariables_FallsBackToScan(t *testing.T) {
	got := normalizeVariables("{{zoo}} {{apple}}", "")
	if !strings.Contains(got, "apple") || !strings.Contains(got, "zoo") {
		t.Fatalf("expected sorted scan result, got %q", got)
	}
}
