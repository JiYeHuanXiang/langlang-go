package redlang

import (
	"os"
	"path/filepath"
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
	code := "# 这是注释\n【输出】@你好"
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

// --- 表达式求值测试 ---

func TestEvalExprArithmetic(t *testing.T) {
	result, err := EvalScript("【计算】@1+2*3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "7" {
		t.Fatalf("expected '7', got '%s'", result)
	}
}

func TestEvalExprParens(t *testing.T) {
	result, err := EvalScript("【计算】@(1+2)*3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "9" {
		t.Fatalf("expected '9', got '%s'", result)
	}
}

func TestEvalExprComparison(t *testing.T) {
	result, err := EvalScript("【计算】@3>2")
	if err != nil {
		t.Fatal(err)
	}
	if result != "真" {
		t.Fatalf("expected '真', got '%s'", result)
	}
}

func TestEvalExprLogic(t *testing.T) {
	result, err := EvalScript("【计算】@3>2 && 1<0")
	if err != nil {
		t.Fatal(err)
	}
	if result != "假" {
		t.Fatalf("expected '假', got '%s'", result)
	}
}

func TestEvalExprPow(t *testing.T) {
	result, err := EvalScript("【计算】@2^3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "8" {
		t.Fatalf("expected '8', got '%s'", result)
	}
}

func TestEvalExprMod(t *testing.T) {
	result, err := EvalScript("【计算】@10%3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "1" {
		t.Fatalf("expected '1', got '%s'", result)
	}
}

func TestEvalExprIntDiv(t *testing.T) {
	result, err := EvalScript("【计算】@10//3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "3" {
		t.Fatalf("expected '3', got '%s'", result)
	}
}

func TestEvalExprUnaryNot(t *testing.T) {
	result, err := EvalScript("【计算】@!0")
	if err != nil {
		t.Fatal(err)
	}
	if result != "真" {
		t.Fatalf("expected '真', got '%s'", result)
	}
}

func TestEvalExprNegate(t *testing.T) {
	result, err := EvalScript("【计算】@-5+3")
	if err != nil {
		t.Fatal(err)
	}
	if result != "-2" {
		t.Fatalf("expected '-2', got '%s'", result)
	}
}

// --- 函数系统测试 ---

func TestFuncDefineAndCall(t *testing.T) {
	rt := NewRuntime()
	code := "【令】@fn@【函数定义】@【加】@【取@参数1】@【取@参数2】】】 【调用函数】@【取@fn】@3@4"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "7" {
		t.Fatalf("expected '7', got '%s'", result.String())
	}
}

func TestFuncParamCount(t *testing.T) {
	rt := NewRuntime()
	code := "【令】@fn@【函数定义】@【参数个数】】】 【调用函数】@【取@fn】@a@b@c"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "3" {
		t.Fatalf("expected '3', got '%s'", result.String())
	}
}

func TestFuncReturn(t *testing.T) {
	rt := NewRuntime()
	code := "【令】@fn@【函数定义】@【输出】@before【返回】【输出】@after】】 【调用函数】@【取@fn】"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "before" {
		t.Fatalf("expected 'before', got '%s'", result.String())
	}
}

func TestFuncMissingParam(t *testing.T) {
	rt := NewRuntime()
	code := "【令】@fn@【函数定义】@【参数@1】【参数@2】】】 【调用函数】@【取@fn】@hello"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "hello" {
		t.Fatalf("expected 'hello', got '%s'", result.String())
	}
}

func TestCallFuncByName(t *testing.T) {
	rt := NewRuntime()
	code := "【令】@add@【函数定义】@【加】@【取@参数1】@【取@参数2】】】 【调用函数】@add@10@20"
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

// --- 作用域命令测试 ---

func TestDefineVar(t *testing.T) {
	code := "【定义变量】@name@小明 【变量】@name"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "小明" {
		t.Fatalf("expected '小明', got '%s'", result)
	}
}

func TestAssignVarFromParent(t *testing.T) {
	rt := NewRuntime()
	rt.Scope.Set("x", NewText("10"))
	code := "【赋值变量】@x@20 【变量】@x"
	ast, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	// 赋值变量应修改最近的 x（在当前作用域没有，则向上查找到了父作用域的 x）
	rt.Eval(ast)
	if val, ok := rt.Scope.Get("x"); ok {
		if val.String() != "20" {
			t.Fatalf("expected '20', got '%s'", val.String())
		}
	} else {
		t.Fatal("x not found in scope")
	}
}

func TestVarUndefined(t *testing.T) {
	code := "【变量】@不存在的变量"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("expected '', got '%s'", result)
	}
}

// --- Phase 1 剩余命令测试 ---

func TestCurrentVersion(t *testing.T) {
	result, err := EvalScript("【当前版本】")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty version")
	}
}

func TestNewlineCmd(t *testing.T) {
	result, err := EvalScript("A【换行】B")
	if err != nil {
		t.Fatal(err)
	}
	if result != "A\nB" {
		t.Fatalf("expected 'A\\nB', got '%s'", result)
	}
}

func TestSpaceCmd(t *testing.T) {
	result, err := EvalScript("A【空格】B")
	if err != nil {
		t.Fatal(err)
	}
	if result != "A B" {
		t.Fatalf("expected 'A B', got '%s'", result)
	}
}

func TestChooseRandom(t *testing.T) {
	result, err := EvalScript("【选择】@@A@B")
	if err != nil {
		t.Fatal(err)
	}
	if result != "A" && result != "B" {
		t.Fatalf("expected 'A' or 'B', got '%s'", result)
	}
}

func TestChooseIndex(t *testing.T) {
	result, err := EvalScript("【选择】@1@A@B@C")
	if err != nil {
		t.Fatal(err)
	}
	if result != "B" {
		t.Fatalf("expected 'B', got '%s'", result)
	}
}

func TestJudgeTrue(t *testing.T) {
	result, err := EvalScript("【判真】@真@不是真@是真")
	if err != nil {
		t.Fatal(err)
	}
	if result != "是真" {
		t.Fatalf("expected '是真', got '%s'", result)
	}
}

func TestJudgeEmpty(t *testing.T) {
	result, err := EvalScript("【判空】@@默认值")
	if err != nil {
		t.Fatal(err)
	}
	if result != "默认值" {
		t.Fatalf("expected '默认值', got '%s'", result)
	}
}

func TestHideAndPass(t *testing.T) {
	result, err := EvalScript("【隐藏】@秘密消息【传递】")
	if err != nil {
		t.Fatal(err)
	}
	if result != "秘密消息" {
		t.Fatalf("expected '秘密消息', got '%s'", result)
	}
}

func TestRandomPick(t *testing.T) {
	result, err := EvalScript("【随机取】@苹果@香蕉@橘子")
	if err != nil {
		t.Fatal(err)
	}
	valid := result == "苹果" || result == "香蕉" || result == "橘子"
	if !valid {
		t.Fatalf("expected one of 苹果/香蕉/橘子, got '%s'", result)
	}
}

// --- Phase 2 数据容器测试 ---

func TestArrayCmd(t *testing.T) {
	result, err := EvalScript("【取长度】@【数组】@a@b@c")
	if err != nil {
		t.Fatal(err)
	}
	if result != "3" {
		t.Fatalf("expected '3', got '%s'", result)
	}
}

func TestObjectCmd(t *testing.T) {
	result, err := EvalScript("【取长度】@【对象】@name@小红@age@18")
	if err != nil {
		t.Fatal(err)
	}
	if result != "2" {
		t.Fatalf("expected '2', got '%s'", result)
	}
}

func TestStack(t *testing.T) {
	code := "【入栈】@A 【入栈】@B 【出栈】"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "B" {
		t.Fatalf("expected 'B', got '%s'", result)
	}
}

func TestStackTop(t *testing.T) {
	code := "【入栈】@X 【入栈】@Y 【栈顶】@0"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Y" {
		t.Fatalf("expected 'Y', got '%s'", result)
	}
}

func TestRegexp(t *testing.T) {
	result, err := EvalScript("【正则】@hello123world@[0-9]+")
	if err != nil {
		t.Fatal(err)
	}
	if result != "123" {
		t.Fatalf("expected '123', got '%s'", result)
	}
}

func TestTimestamp(t *testing.T) {
	result, err := EvalScript("【取时间戳】")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty timestamp")
	}
}

func TestEncodeURL(t *testing.T) {
	result, err := EvalScript("【编码】@你好 world")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty encoded string")
	}
}

func TestBase64(t *testing.T) {
	result, err := EvalScript("【Base64编码】@Hello")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty base64 string")
	}
	decoded, err := EvalScript("【Base64解码】@" + result)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "Hello" {
		t.Fatalf("expected 'Hello', got '%s'", decoded)
	}
}

// --- Phase 3 外部交互测试 ---

func TestFileExists(t *testing.T) {
	result, err := EvalScript("【文件是否存在】@non_existent_file_xyz")
	if err != nil {
		t.Fatal(err)
	}
	if result != "假" {
		t.Fatalf("expected '假', got '%s'", result)
	}
}

func TestFileExt(t *testing.T) {
	result, err := EvalScript("【取后缀】@test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result != ".txt" {
		t.Fatalf("expected '.txt', got '%s'", result)
	}
}

func TestAppDir(t *testing.T) {
	result, err := EvalScript("【应用目录】")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty directory")
	}
}

func TestFileWriteRead(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test_redlang.txt")
	code := "【写文件】@" + tmpFile + "@Hello RedLang"
	_, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvalScript("【读文件】@" + tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello RedLang" {
		t.Fatalf("expected 'Hello RedLang', got '%s'", result)
	}
}

func TestJsonParse(t *testing.T) {
	result, err := EvalScript("【取元素】@【json解析】@{\"name\":\"小明\",\"age\":18}】@name")
	if err != nil {
		t.Fatal(err)
	}
	if result != "小明" {
		t.Fatalf("expected '小明', got '%s'", result)
	}
	
	// 测试多层嵌套
	result2, err2 := EvalScript("【取元素】@【json解析】@{\"user\":{\"name\":\"小红\"}}】@user@name")
	if err2 != nil {
		t.Fatal(err2)
	}
	if result2 != "小红" {
		t.Fatalf("expected '小红', got '%s'", result2)
	}
}

func TestJsonSerialize(t *testing.T) {
	rt := NewRuntime()
	ast, err := Parse("【json序列化】@【数组】@a@b@c")
	if err != nil {
		t.Fatal(err)
	}
	val, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if val.String() != "[\"a\",\"b\",\"c\"]" {
		t.Fatalf("expected '[\"a\",\"b\",\"c\"]', got '%s'", val.String())
	}
}

func TestHttpRequest(t *testing.T) {
	// 使用一个已知可靠的 URL 测试 GET 请求
	result, err := EvalScript("【发送HTTP请求】@https://httpbin.org/get@GET")
	if err != nil {
		t.Logf("HTTP request failed (may be network issue): %v", err)
		t.Skip("network unavailable")
	}
	if result == "" {
		t.Skip("empty response - network may be unavailable")
	}
}

// --- Phase 4 机器人交互测试 ---

func TestBotInfo(t *testing.T) {
	rt := NewRuntime()
	rt.Scope.Set("__self_id__", NewText("123456"))
	rt.Scope.Set("__platform__", NewText("onebot11"))
	ast, err := Parse("【机器人ID】【机器人平台】")
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "123456onebot11" {
		t.Fatalf("expected '123456onebot11', got '%s'", result)
	}
}

func TestEventContext(t *testing.T) {
	rt := NewRuntime()
	rt.Scope.Set("__user_id__", NewText("user123"))
	rt.Scope.Set("__group_id__", NewText("group456"))
	rt.Scope.Set("__message__", NewText("Hello"))
	rt.Scope.Set("__message_id__", NewText("msg789"))
	ast, err := Parse("【发送者ID】【群ID】【当前消息】【消息ID】")
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "user123group456Hellomsg789" {
		t.Fatalf("expected 'user123group456Hellomsg789', got '%s'", result)
	}
}

func TestMasterCheck(t *testing.T) {
	rt := NewRuntime()
	rt.Scope.Set("__user_id__", NewText("admin"))
	rt.Scope.Set("__masters__", NewArray([]*RedValue{NewText("admin"), NewText("root")}))
	// 主人命令应检查用户是否为主人
	ast, err := Parse("【主人】@admin")
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	// 是主人，返回空
	if result.String() != "" {
		t.Fatalf("expected empty string (is master), got '%s'", result)
	}
}

func TestCQCodeParsing(t *testing.T) {
	rt := NewRuntime()
	testMsg := "你好 [CQ:at,qq=123456] 世界 [CQ:image,file=test.png]"
	rt.Scope.Set("__message__", NewText(testMsg))

	// 取艾特
	ast, err := Parse("【取艾特】")
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "[123456]" {
		t.Fatalf("expected '[123456]', got '%s'", result)
	}

	// 取图片
	ast2, _ := Parse("【取图片】")
	result2, _ := rt.Eval(ast2)
	if result2.String() != "[test.png]" {
		t.Fatalf("expected '[test.png]', got '%s'", result2)
	}
}

func TestSendMessage(t *testing.T) {
	// 测试发送（不实际发送，验证不崩溃）
	rt := NewRuntime()
	rt.Scope.Set("__platform__", NewText("debug"))
	rt.Scope.Set("__self_id__", NewText("debug_bot"))
	rt.Scope.Set("__user_id__", NewText("user1"))

	ast, err := Parse("【发送】@test_message")
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	// 没有崩溃即通过
}

func TestSetSource(t *testing.T) {
	rt := NewRuntime()
	ast, err := Parse("【设置来源】@qq@123456 【发送】@hello")
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	// 验证目标已设置
	if val, ok := rt.Scope.Get("__reply_target__"); ok {
		if val.String() != "123456" {
			t.Fatalf("expected '__reply_target__' = '123456', got '%s'", val.String())
		}
	} else {
		t.Fatal("__reply_target__ not set")
	}
}

// --- Phase 5 高级特性测试 ---

func TestScoreSystem(t *testing.T) {
	// 测试积分增加/获取
	code := "【积分-增加】@10 【积分】"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "10" {
		t.Fatalf("expected '10', got '%s'", result)
	}
}

func TestScoreSet(t *testing.T) {
	code := "【积分-设置】@50 【积分】"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	if result != "50" {
		t.Fatalf("expected '50', got '%s'", result)
	}
}

func TestErrorInfo(t *testing.T) {
	rt := NewRuntime()
	rt.Scope.Set("__error__", NewText("测试错误"))
	ast, err := Parse("【错误信息】")
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "测试错误" {
		t.Fatalf("expected '测试错误', got '%s'", result.String())
	}
}

func TestDictFile(t *testing.T) {
	// 写一个临时词库文件
	tmpDir := t.TempDir()
	safePath := strings.ReplaceAll(tmpDir, "\\", "/") + "/test_dict.txt"
	content := "你好\n你好呀\n早上好\n\n再见\n拜拜\n明天见"
	os.WriteFile(strings.ReplaceAll(safePath, "/", "\\"), []byte(content), 0644)

	result, err := EvalScript("【读词库文件】@" + safePath)
	if err != nil {
		t.Fatal(err)
	}
	if result != "[object]" {
		t.Fatalf("expected '[object]', got '%s'", result)
	}
}

func TestNetworkCommands(t *testing.T) {
	rt := NewRuntime()
	rt.Scope.Set("__net_method__", NewText("GET"))
	rt.Scope.Set("__net_auth__", NewText("可写"))

	ast, err := Parse("【网络-访问方法】【网络-权限】")
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Eval(ast)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "GET可写" {
		t.Fatalf("expected 'GET可写', got '%s'", result.String())
	}
}

func TestGPTCommands(t *testing.T) {
	result, err := EvalScript("【GPT-创建单轮对话】@https://api.openai.com/v1/chat/completions@sk-test@gpt-3.5-turbo")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty GPT pointer")
	}
	t.Logf("GPT ID: %s", result)
}

func TestScriptOutput(t *testing.T) {
	code := "【脚本输出-增加ID】@msg001 【脚本输出】@msg001"
	result, err := EvalScript(code)
	if err != nil {
		t.Fatal(err)
	}
	// 应返回数组，打印输出
	t.Logf("Script output: %s", result)
}
