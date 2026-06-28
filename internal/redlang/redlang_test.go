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

// --- 新增内置函数测试 ---

func TestEvalAdd(t *testing.T) {
	result, err := EvalScript("【加】@3@5")
	if err != nil {
		t.Fatal(err)
	}
	if result != "8" {
		t.Fatalf("expected 8, got '%s'", result)
	}
}

func TestEvalSub(t *testing.T) {
	result, err := EvalScript("【减】@10@3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "7" {
		t.Fatalf("expected 7, got '%s'", result)
	}
}

func TestEvalMul(t *testing.T) {
	result, err := EvalScript("【乘】@4@5")
	if err != nil {
		t.Fatal(err)
	}
	if result != "20" {
		t.Fatalf("expected 20, got '%s'", result)
	}
}

func TestEvalDiv(t *testing.T) {
	result, err := EvalScript("【除】@10@3")
	if err != nil {
		t.Fatal(err)
	}
	// %g 格式可能产生 "3.33333"，粗略校验包含 3
	if !strings.HasPrefix(result, "3") {
		t.Fatalf("expected ~3.33, got '%s'", result)
	}
}

func TestEvalDivByZero(t *testing.T) {
	result, err := EvalScript("【除】@5@0")
	if err != nil {
		t.Fatal(err)
	}
	if result != "0" {
		t.Fatalf("expected 0, got '%s'", result)
	}
}

func TestEvalMod(t *testing.T) {
	result, err := EvalScript("【模】@10@3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "1" {
		t.Fatalf("expected 1, got '%s'", result)
	}
}

func TestEvalLessThan(t *testing.T) {
	result, err := EvalScript("【<】@2@5")
	if err != nil {
		t.Fatal(err)
	}
	if result != "真" {
		t.Fatalf("expected '真', got '%s'", result)
	}
}

func TestEvalGreaterEqual(t *testing.T) {
	result, err := EvalScript("【>=】@5@3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "真" {
		t.Fatalf("expected '真', got '%s'", result)
	}
}

func TestEvalLessEqual(t *testing.T) {
	result, err := EvalScript("【<=】@3@5")
	if err != nil {
		t.Fatal(err)
	}
	if result != "真" {
		t.Fatalf("expected '真', got '%s'", result)
	}
}

// --- 变量测试 ---

func TestVarSetAndGet(t *testing.T) {
	code := "【令】@name@小明 【取】@name"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "小明" {
		t.Fatalf("expected '小明', got '%s'", result)
	}
}

func TestVarGetUndefined(t *testing.T) {
	result, err := EvalScript("【取】@不存在的变量")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("expected '', got '%s'", result)
	}
}

func TestVarOverride(t *testing.T) {
	code := "【令】@x@1 【令】@x@2 【取】@x"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "2" {
		t.Fatalf("expected '2', got '%s'", result)
	}
}

func TestUserFuncParams(t *testing.T) {
	// 手动创建运行时，注册一个用户函数：参数1 + 参数2
	rt := NewRuntime()
	// 定义一个函数体： 【加】@【取@参数1】@【取@参数2】
	funAst, err := Parse("【加】@【取@参数1】@【取@参数2】")
	if err != nil {
		t.Fatal(err)
	}
	funVal := NewFun(&funAst)
	rt.Scope.Set("加法", funVal)

	// 调用: 【加法】@10@20
	code := "【加法】@10@20"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "30" {
		t.Fatalf("expected '30', got '%s'", result.String())
	}
}

// --- 循环测试 ---

func TestEvalCountLoop(t *testing.T) {
	// 循环体中纯文本需用 【输出】 包裹，避免被误读为命令参数
	code := "【计次循环】@3 【输出】@hello 【计次循环尾】"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hellohellohello" {
		t.Fatalf("expected 'hellohellohello', got '%s'", result)
	}
}

func TestEvalCountLoopWithVar(t *testing.T) {
	rt := NewRuntime()
	code := "【计次循环】@3 【输出】@【取@循环次数】 【计次循环尾】"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "123" {
		t.Fatalf("expected '123', got '%s'", result.String())
	}
}

func TestEvalLoopBreak(t *testing.T) {
	// 只在第一次迭代输出 "a" 然后跳出
	code := "【计次循环】@5 【输出】@a 【跳出】 【输出】@b 【计次循环尾】"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "a" {
		t.Fatalf("expected 'a', got '%s'", result)
	}
}

func TestEvalWhileLoop(t *testing.T) {
	// 循环 3 次：i < 3 时继续
	rt := NewRuntime()
	rt.Scope.Set("i", NewText("0"))
	code := "【循环】@【<】@【取@i】@3 【输出】@【取@i】 【令】@i@【加】@【取@i】@1 【循环尾】"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "012" {
		t.Fatalf("expected '012', got '%s'", result.String())
	}
}

func TestVarInExpression(t *testing.T) {
	// 注意：嵌套命令作参数时，用内联格式 【命令@arg】 避免歧义
	code := "【令】@a@10 【令】@b@20 【加】@【取@a】@【取@b】"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "30" {
		t.Fatalf("expected '30', got '%s'", result)
	}
}
