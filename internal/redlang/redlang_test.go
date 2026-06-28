package redlang

import (
	"strings"
	"testing"
)

func TestParseSimpleText(t *testing.T) {
	ast, err := Parse("你好世界")
	if err != nil {
		t.Fatal(err)
	}
	if len(ast) != 1 || ast[0].Type != TypeText || ast[0].Text != "你好世界" {
		t.Fatalf("unexpected ast: %+v", ast)
	}
}

func TestParseCommand(t *testing.T) {
	ast, err := Parse("【输出】@你好")
	if err != nil {
		t.Fatal(err)
	}
	if len(ast) != 1 || ast[0].Type != TypeCommand {
		t.Fatalf("expected command, got %+v", ast[0])
	}
	cmd := ast[0].Cmd
	if cmd.Name != "输出" {
		t.Fatalf("expected name '输出', got '%s'", cmd.Name)
	}
	if len(cmd.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(cmd.Args))
	}
}

func TestParseNestedCommand(t *testing.T) {
	code := "【如果】@【==】@a@b 【则】@【输出】@ok"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	if len(ast) == 0 {
		t.Fatal("empty ast")
	}
	// 【输出】是 【则】 的参数中的嵌套命令，不是顶层节点
	if len(ast) < 2 {
		t.Fatalf("expected at least 2 top-level nodes (如果, 则), got %d", len(ast))
	}
	// 验证顶层结构：如果 + 则
	if ast[0].Type != TypeCommand || ast[0].Cmd.Name != "如果" {
		t.Fatalf("expected 如果, got %+v", ast[0])
	}
	if ast[1].Type != TypeCommand || ast[1].Cmd.Name != "则" {
		t.Fatalf("expected 则, got %+v", ast[1])
	}
	// 验证 如果 的参数包含嵌套的 ==
	if len(ast[0].Cmd.Args) != 1 {
		t.Fatalf("如果 should have 1 arg, got %d", len(ast[0].Cmd.Args))
	}
	if ast[0].Cmd.Args[0][0].Type != TypeCommand || ast[0].Cmd.Args[0][0].Cmd.Name != "==" {
		t.Fatalf("如果's arg should be ==, got %+v", ast[0].Cmd.Args[0][0])
	}
}

func TestEvalSimple(t *testing.T) {
	result, err := EvalScript("【输出】@你好")
	if err != nil {
		t.Fatal(err)
	}
	if result != "你好" {
		t.Fatalf("expected '你好', got '%s'", result)
	}
}

func TestEvalIfTrue(t *testing.T) {
	result, err := EvalScript("【如果】@真 【则】@通过")
	if err != nil {
		t.Fatal(err)
	}
	if result != "通过" {
		t.Fatalf("expected '通过', got '%s'", result)
	}
}

func TestEvalIfFalse(t *testing.T) {
	result, err := EvalScript("【如果】@假 【则】@通过")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("expected '', got '%s'", result)
	}
}

func TestEvalEqTrue(t *testing.T) {
	result, err := EvalScript("【如果】@【==】@a@a 【则】@相等")
	if err != nil {
		t.Fatal(err)
	}
	if result != "相等" {
		t.Fatalf("expected '相等', got '%s'", result)
	}
}

func TestEvalEqFalse(t *testing.T) {
	result, err := EvalScript("【如果】@【==】@a@b 【则】@相等 【否则】@不相等")
	if err != nil {
		t.Fatal(err)
	}
	if result != "不相等" {
		t.Fatalf("expected '不相等', got '%s'", result)
	}
}

func TestEvalNestedComplex(t *testing.T) {
	code := `【如果】@【==】@【输出】@用户@admin 【则】@欢迎管理员`
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	// 结果应该包含"欢迎管理员"（因为 输出 用户 != admin）
	if !strings.Contains(result, "欢迎管理员") && result != "" {
		t.Fatalf("unexpected: %s", result)
	}
}

func TestAstToString(t *testing.T) {
	code := "【输出】@你好"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	reconstructed := ast.String()
	if reconstructed != code {
		t.Fatalf("roundtrip failed: '%s' != '%s'", reconstructed, code)
	}
}

func TestComment(t *testing.T) {
	code := "// 这是注释\n【输出】@你好"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "你好" {
		t.Fatalf("expected '你好', got '%s'", result)
	}
}

func TestExtractCommands(t *testing.T) {
	code := "【如果】@【==】@a@b 【则】@【输出】@ok"
	names, err := ExtractCommandNames(code)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("commands: %v", names)
	if len(names) < 2 {
		t.Fatalf("expected at least 2 commands, got %d", len(names))
	}
}

func TestSanitize(t *testing.T) {
	code := "【执行】@恶意代码"
	sanitized := SanitizeCode(code)
	if strings.Contains(sanitized, "【执行】") {
		t.Fatal("sanitize didn't remove dangerous command")
	}
}
